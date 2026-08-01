#!/usr/bin/env bash
# Test harness for the init-alloy service script.
# Usage: alloy/tests/init-alloy.test.sh
#
# init-alloy/run is what turns add-on options into an Alloy config, and every
# way it can refuse to start is a message a user has to act on. It runs under
# bashio, which reads options from the Supervisor API - but bashio caches that
# response on disk and the cache directory is overridable, so seeding the cache
# lets the real script run unmodified with no Supervisor present.
set -u

ADDON_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INIT="${ADDON_ROOT}/rootfs/etc/s6-overlay/s6-rc.d/init-alloy/run"
FINISH="${ADDON_ROOT}/rootfs/etc/s6-overlay/s6-rc.d/alloy/finish"
GENERATOR="${ADDON_ROOT}/rootfs/usr/share/alloy/generate-config.sh"
FAILS=0
TESTS=0

# bashio installs to /usr/lib/bashio in the add-on base image. Outside a
# container, point BASHIO_BIN at a checkout of hassio-addons/bashio.
BASHIO_BIN="${BASHIO_BIN:-/usr/lib/bashio/bashio}"
if [ ! -x "${BASHIO_BIN}" ]; then
  echo "SKIP: bashio not found at ${BASHIO_BIN}."
  echo "      Set BASHIO_BIN=/path/to/bashio/lib/bashio to run these tests."
  exit 0
fi

fail() { echo "  ✗ $1"; FAILS=$((FAILS + 1)); }
pass() { echo "  ✓ $1"; }
# Indent captured output so a failure's context is visually distinct from the
# check list. Parameter expansion rather than sed, per ShellCheck SC2001.
indent() { echo "      ${1//$'\n'/$'\n'      }"; }

# Run init-alloy/run against a given options.json.
# $1 = options JSON. Sets RUN_RC, RUN_OUT and RUN_CONFIG.
run_init() {
  local tmp
  tmp="$(mktemp -d)"
  # bashio reads its cached Supervisor response from $CACHE_DIR, and insists the
  # directory is owner-only.
  mkdir -p "${tmp}/cache" "${tmp}/etc" "${tmp}/data"
  chmod 0700 "${tmp}/cache"
  printf '%s' "$1" >"${tmp}/cache/addons.self.options.config.cache"

  # $2, when given, is written to the add-on config folder as a config.alloy
  # override.
  mkdir -p "${tmp}/addon_config"
  [ -n "${2:-}" ] && printf '%s' "$2" >"${tmp}/addon_config/config.alloy"

  RUN_OUT="$(
    CACHE_DIR="${tmp}/cache" \
    CONFIG_DIR="${tmp}/etc" \
    DATA_DIR="${tmp}/data" \
    ADDON_CONFIG_DIR="${tmp}/addon_config" \
    GENERATOR="${GENERATOR}" \
      "${BASHIO_BIN}" "${INIT}" 2>&1
  )"
  RUN_RC=$?
  RUN_CONFIG=""
  [ -f "${tmp}/etc/config.alloy" ] && RUN_CONFIG="$(cat "${tmp}/etc/config.alloy")"
  rm -rf "${tmp}"
}

# $1 = description, $2 = options JSON, $3 = expected substring of the output.
expect_fatal() {
  TESTS=$((TESTS + 1))
  run_init "$2"
  if [ "${RUN_RC}" -eq 0 ]; then
    fail "$1: expected a non-zero exit, got 0"
  elif ! grep -qF -- "$3" <<<"${RUN_OUT}"; then
    fail "$1: exit ${RUN_RC} but output did not mention '$3'"
    indent "${RUN_OUT}"
  else
    pass "$1"
  fi
}

# $1 = description, $2 = options JSON, $3.. = substrings expected in the config.
expect_ok() {
  local desc="$1" opts="$2"
  shift 2
  TESTS=$((TESTS + 1))
  run_init "${opts}"
  if [ "${RUN_RC}" -ne 0 ]; then
    fail "${desc}: exited ${RUN_RC}"
    indent "${RUN_OUT}"
    return
  fi
  local want
  for want in "$@"; do
    TESTS=$((TESTS + 1))
    if grep -qF -- "${want}" <<<"${RUN_CONFIG}"; then
      pass "${desc}: config contains ${want}"
    else
      fail "${desc}: config missing ${want}"
    fi
  done
  pass "${desc}: started"
}

LOKI='"loki_url":"http://loki:3100/loki/api/v1/push"'
PROM='"prometheus_url":"http://prom:9090/api/v1/write"'
FLEET='"fleet_url":"https://fleet.example.net"'

echo "== at least one destination is required =="
expect_fatal "no destinations at all" \
  '{"instance_name":"hass","log_level":"info"}' \
  "at least one of loki_url"
expect_fatal "all destinations empty strings" \
  '{"loki_url":"","prometheus_url":"","fleet_url":"","instance_name":"hass"}' \
  "at least one of loki_url"

echo
echo "== each destination works on its own =="
expect_ok "logs only" "{${LOKI},\"instance_name\":\"hass\"}" \
  "loki.write" "loki.source.journal"
expect_ok "metrics only" "{${PROM},\"instance_name\":\"hass\"}" \
  "prometheus.remote_write" "prometheus.exporter.unix"
# The regression that started all this: a Fleet-only install must be possible.
expect_ok "fleet only" "{${FLEET},\"instance_name\":\"hass\"}" \
  "remotecfg" 'url            = "https://fleet.example.net"'

echo
echo "== defaults are applied when an option is absent =="
expect_ok "instance_name default" "{${PROM}}" 'replacement  = "homeassistant"'
expect_ok "poll frequency default" "{${FLEET}}" 'poll_frequency = "1m"'
expect_ok "scrape interval default" "{${PROM}}" 'scrape_interval = "60s"'

echo
echo "== basic auth needs both halves =="
expect_fatal "loki username without password" \
  "{${LOKI},\"loki_username\":\"123\"}" \
  "loki_username is set but loki_password is empty"
expect_fatal "loki password without username" \
  "{${LOKI},\"loki_password\":\"secret\"}" \
  "loki_password is set but loki_username is empty"
expect_fatal "prometheus username without password" \
  "{${PROM},\"prometheus_username\":\"123\"}" \
  "prometheus_username is set but prometheus_password is empty"
expect_fatal "fleet username without password" \
  "{${FLEET},\"fleet_username\":\"123\"}" \
  "fleet_username is set but gcloud_rw_api_key is empty"
expect_ok "Fleet shared write key" \
  "{${FLEET},\"fleet_username\":\"123\",\"gcloud_rw_api_key\":\"secret\"}" \
  'password = sys.env("GCLOUD_RW_API_KEY")'
expect_ok "both halves present" \
  "{${LOKI},\"loki_username\":\"123\",\"loki_password\":\"secret\"}" \
  'username = "123"'

echo
echo "== operation modes are exclusive and upgrades preserve legacy hybrid =="
TESTS=$((TESTS + 1))
run_init "{${LOKI},${PROM},${FLEET},\"operation_mode\":\"fleet\",\"fleet_username\":\"123\",\"gcloud_rw_api_key\":\"secret\"}"
if ! grep -qF "remotecfg" <<<"${RUN_CONFIG}"; then
  fail "fleet mode did not emit remotecfg"
elif grep -qF "loki.write" <<<"${RUN_CONFIG}" || grep -qF "prometheus.remote_write" <<<"${RUN_CONFIG}"; then
  fail "fleet mode emitted a local pipeline"
else
  pass "fleet mode emits only remote configuration"
fi
TESTS=$((TESTS + 1))
run_init "{${LOKI},${FLEET},\"operation_mode\":\"local\"}"
if ! grep -qF "loki.write" <<<"${RUN_CONFIG}"; then
  fail "local mode did not emit the local log pipeline"
elif grep -qF "remotecfg" <<<"${RUN_CONFIG}"; then
  fail "local mode emitted remotecfg"
else
  pass "local mode emits no remote configuration"
fi
expect_ok "existing mixed install remains legacy hybrid" "{${LOKI},${FLEET}}" \
  "loki.write" "remotecfg"
expect_fatal "Fleet mode requires a Fleet endpoint" \
  "{${LOKI},\"operation_mode\":\"fleet\"}" \
  "Fleet mode requires fleet_url"
expect_fatal "Fleet mode requires its instance username" \
  "{${FLEET},\"operation_mode\":\"fleet\",\"gcloud_rw_api_key\":\"secret\"}" \
  "Fleet mode requires fleet_username"
expect_fatal "Fleet mode requires its shared write key" \
  "{${FLEET},\"operation_mode\":\"fleet\",\"fleet_username\":\"123\"}" \
  "Fleet mode requires gcloud_rw_api_key"
expect_fatal "Local mode requires a local destination" \
  "{${FLEET},\"operation_mode\":\"local\"}" \
  "Local mode requires at least one local destination"
expect_fatal "traces require a Tempo endpoint" \
  "{${LOKI},\"operation_mode\":\"local\",\"traces_enabled\":true}" \
  "traces_enabled is on but tempo_url is empty"
expect_fatal "profiling requires a Pyroscope endpoint" \
  "{${LOKI},\"operation_mode\":\"local\",\"alloy_profiling\":true}" \
  "alloy_profiling is on but pyroscope_url is empty"

echo
echo "== endpoints that would corrupt the generated config are refused =="
# The schema rejects these in the UI; options.json can still be hand-edited.
expect_fatal "quote in an endpoint" \
  '{"loki_url":"http://loki:3100/\""}' \
  "loki_url contains a quote, backslash or whitespace"
expect_fatal "whitespace in an endpoint" \
  '{"prometheus_url":"http://prom :9090/api/v1/write"}' \
  "prometheus_url contains a quote, backslash or whitespace"
expect_fatal "backslash in an endpoint" \
  '{"fleet_url":"https://fleet.example.net\\\\x"}' \
  "fleet_url contains a quote, backslash or whitespace"
expect_fatal "missing endpoint authority" \
  '{"loki_url":"http:///loki/api/v1/push"}' \
  "loki_url is not a valid HTTP(S) URL"
expect_fatal "non-numeric endpoint port" \
  '{"prometheus_url":"http://prom:abc/api/v1/write"}' \
  "prometheus_url is not a valid HTTP(S) URL"
expect_fatal "out-of-range endpoint port" \
  '{"fleet_url":"https://fleet.example.net:65536"}' \
  "fleet_url is not a valid HTTP(S) URL"
expect_fatal "invalid endpoint escape" \
  '{"loki_url":"http://loki:3100/%zz"}' \
  "loki_url is not a valid HTTP(S) URL"
expect_fatal "malformed bracketed IPv6 endpoint" \
  '{"loki_url":"http://[2001:db8:::1]/loki/api/v1/push"}' \
  "loki_url is not a valid HTTP(S) URL"
expect_ok "a clean endpoint still starts" "{${LOKI}}" "loki.write"
expect_ok "encoded path and query endpoint" \
  '{"loki_url":"http://loki:3100/a%20path?query=a=b"}' \
  'url = "http://loki:3100/a%20path?query=a=b"'
expect_ok "bracketed IPv6 endpoint" \
  '{"prometheus_url":"http://[2001:db8::1]:9090/api/v1/write"}' \
  'url = "http://[2001:db8::1]:9090/api/v1/write"'
expect_ok "IPv6 loopback endpoint" \
  '{"prometheus_url":"http://[::1]:9090/api/v1/write"}' \
  'url = "http://[::1]:9090/api/v1/write"'
expect_ok "scoped IPv6 endpoint" \
  '{"prometheus_url":"http://[fe80::1%25eth0]:9090/api/v1/write"}' \
  'url = "http://[fe80::1%25eth0]:9090/api/v1/write"'
expect_ok "IPv4-embedded IPv6 endpoint" \
  '{"prometheus_url":"http://[::ffff:192.0.2.1]:9090/api/v1/write"}' \
  'url = "http://[::ffff:192.0.2.1]:9090/api/v1/write"'
expect_ok "scoped IPv4-embedded IPv6 endpoint" \
  '{"prometheus_url":"http://[fe80::192.0.2.1%25eth0]:9090/api/v1/write"}' \
  'url = "http://[fe80::192.0.2.1%25eth0]:9090/api/v1/write"'
expect_fatal "unencoded IPv6 scope identifier" \
  '{"loki_url":"http://[fe80::1%eth0]/loki/api/v1/push"}' \
  "loki_url is not a valid HTTP(S) URL"
expect_fatal "empty IPv6 scope identifier" \
  '{"loki_url":"http://[fe80::1%25]/loki/api/v1/push"}' \
  "loki_url is not a valid HTTP(S) URL"
expect_fatal "out-of-range embedded IPv4 address" \
  '{"loki_url":"http://[::ffff:999.0.2.1]/loki/api/v1/push"}' \
  "loki_url is not a valid HTTP(S) URL"

echo
echo "== the full Go duration grammar reaches Alloy unchanged =="
expect_ok "compound duration" \
  "{${PROM},\"metrics_scrape_interval\":\"1m30s\"}" 'scrape_interval = "1m30s"'
expect_ok "fractional duration" \
  "{${PROM},\"metrics_scrape_interval\":\"1.5s\"}" 'scrape_interval = "1.5s"'
expect_ok "leading-plus duration" \
  "{${PROM},\"metrics_scrape_interval\":\"+15s\"}" 'scrape_interval = "+15s"'
expect_ok "leading-decimal duration" \
  "{${PROM},\"metrics_scrape_interval\":\".5s\"}" 'scrape_interval = ".5s"'
expect_ok "microsecond duration" \
  "{${PROM},\"metrics_scrape_interval\":\"100µs\"}" 'scrape_interval = "100µs"'

echo
echo "== fleet attributes are validated before Alloy sees them =="
expect_fatal "not key=value" \
  "{${FLEET},\"fleet_attributes\":\"env=home,role\"}" \
  "is not key=value"
expect_fatal "empty key" \
  "{${FLEET},\"fleet_attributes\":\"=home\"}" \
  "has an empty key"
expect_fatal "embedded quote breaks River syntax" \
  "{${FLEET},\"fleet_attributes\":\"env=ho\\\"me\"}" \
  "contains a quote or backslash"
expect_ok "well-formed attributes" \
  "{${FLEET},\"fleet_attributes\":\"env=home,role=hass\"}" \
  '"env" = "home"' '"role" = "hass"'
expect_ok "equals signs in attribute values" \
  "{${FLEET},\"fleet_attributes\":\"query=a=b,token=YWJjZA==\"}" \
  '"query" = "a=b"' '"token" = "YWJjZA=="'

echo
echo "== metric sources are opt-in and need a Prometheus endpoint =="
expect_ok "host metrics on by default" "{${PROM}}" "prometheus.exporter.unix"
TESTS=$((TESTS + 1))
run_init "{${PROM},\"host_metrics\":false}"
if grep -qF "prometheus.exporter.unix" <<<"${RUN_CONFIG}"; then
  fail "host_metrics=false still emitted the unix exporter"
elif ! grep -qF "prometheus.remote_write" <<<"${RUN_CONFIG}"; then
  fail "host_metrics=false dropped remote_write as well"
else
  pass "host_metrics=false removes the exporter but keeps remote_write"
fi
expect_ok "home assistant metrics" "{${PROM},\"homeassistant_metrics\":true}" \
  'prometheus.scrape "homeassistant"' \
  'metrics_path    = "/core/api/prometheus"' \
  'bearer_token    = sys.env("SUPERVISOR_TOKEN")'
TESTS=$((TESTS + 1))
run_init "{${PROM}}"
if grep -qF 'prometheus.scrape "homeassistant"' <<<"${RUN_CONFIG}"; then
  fail "home assistant metrics were emitted without being enabled"
else
  pass "home assistant metrics are off by default"
fi
expect_fatal "home assistant metrics without a Prometheus endpoint" \
  "{${LOKI},\"homeassistant_metrics\":true}" \
  "homeassistant_metrics is enabled but prometheus_url is empty"

echo
echo "== a user config.alloy replaces the generated one =="
TESTS=$((TESTS + 1))
run_init "{${LOKI}}" 'logging { level = "debug" }'
if [ "${RUN_RC}" -ne 0 ]; then
  fail "override run exited ${RUN_RC}"
  indent "${RUN_OUT}"
elif [ "${RUN_CONFIG}" != 'logging { level = "debug" }' ]; then
  fail "override was not used verbatim, got: ${RUN_CONFIG}"
else
  pass "the override is copied verbatim"
fi
TESTS=$((TESTS + 1))
if grep -qF "Add-on options are ignored" <<<"${RUN_OUT}"; then
  pass "the override is announced in the log"
else
  fail "the override was silent"
fi
# With an override the options no longer describe what Alloy does, so the
# destination requirement must not be enforced.
TESTS=$((TESTS + 1))
run_init '{"instance_name":"hass"}' 'logging { level = "info" }'
if [ "${RUN_RC}" -eq 0 ]; then
  pass "an override starts with no destination configured"
else
  fail "an override with no destination was rejected"
  indent "${RUN_OUT}"
fi

echo
echo "== secrets never reach the generated config =="
expect_ok "password is referenced by env var, not value" \
  "{${LOKI},\"loki_username\":\"123\",\"loki_password\":\"SENTINELSECRET\"}" \
  'sys.env("LOKI_PASSWORD")'
TESTS=$((TESTS + 1))
if grep -qF "SENTINELSECRET" <<<"${RUN_CONFIG}"; then
  fail "the password leaked into the config"
else
  pass "the password is absent from the config"
fi

echo
echo "== finish distinguishes shutdown signals from crashes =="
run_finish() {
  local tmp
  tmp="$(mktemp -d)"
  mkdir -p "${tmp}/results"
  # The generated helper expands ALLOY_TEST_HALT when it runs, not here.
  # shellcheck disable=SC2016
  printf '#!/usr/bin/env bash\nprintf halted >"${ALLOY_TEST_HALT}"\n' >"${tmp}/halt"
  chmod +x "${tmp}/halt"
  FINISH_OUT="$(
    ALLOY_TEST_HALT="${tmp}/halted" \
    S6_RESULTS_DIR="${tmp}/results" \
    S6_HALT="${tmp}/halt" \
      "${BASHIO_BIN}" "${FINISH}" "$@" 2>&1
  )"
  FINISH_RC=$?
  FINISH_HALTED=false
  [ -e "${tmp}/halted" ] && FINISH_HALTED=true
  FINISH_EXIT_CODE="$(cat "${tmp}/results/exitcode" 2>/dev/null || true)"
  rm -rf "${tmp}"
}

TESTS=$((TESTS + 1))
run_finish 256 15
if [ "${FINISH_RC}" -eq 0 ] && [ "${FINISH_HALTED}" = false ] && [ -z "${FINISH_EXIT_CODE}" ]; then
  pass "SIGTERM is treated as an intentional shutdown"
else
  fail "SIGTERM attempted to halt the App"
fi

TESTS=$((TESTS + 1))
run_finish 256 11
if [ "${FINISH_RC}" -ne 0 ]; then
  fail "SIGSEGV finish did not hand off to halt"
elif [ "${FINISH_HALTED}" != true ] || [ "${FINISH_EXIT_CODE}" != 139 ]; then
  fail "SIGSEGV finish did not preserve exit code 139 and halt the App"
elif ! grep -qF "signal 11" <<<"${FINISH_OUT}"; then
  fail "SIGSEGV finish did not report signal 11"
else
  pass "SIGSEGV stops the App with exit code 139"
fi

echo
if [ "${FAILS}" -gt 0 ]; then
  echo "== RESULTS: ${FAILS} of ${TESTS} checks FAILED =="
  exit 1
fi
echo "== RESULTS: ${TESTS}/${TESTS} checks passed =="
echo "ALL PASS"
