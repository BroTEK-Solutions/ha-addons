#!/usr/bin/env bash
# Test harness for the init-alloy service script.
# Usage: alloy/tests/init-alloy.test.sh
#
# init-alloy/run is what turns ingress-managed settings into an Alloy config, and every
# way it can refuse to start is a message a user has to act on. Operational
# settings come from the v2 settings store; bashio contains recovery options only.
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

# --- how the slow half of this suite is made parallel -------------------------
# Every check below needs the result of one init-alloy/run or alloy/finish
# invocation, and the checks themselves are string comparisons that cost
# nothing. A single init-alloy/run costs ~1.4s, effectively all of it process
# startup: it reads one settings key per `jq` and starts 46 of them. Seventy-odd
# invocations back to back is the whole runtime of this suite.
#
# So the suite body runs twice. The recording pass writes each invocation's
# input into a numbered slot instead of executing it; the recorded slots are
# then drained through a bounded pool of parallel workers; the reporting pass
# replays the same body and each run_init/run_finish reads back the result its
# slot already holds. Every check runs exactly once, in the same order, against
# the same bytes - the only difference is that the slow part already happened.
#
# A counter incremented inside a backgrounded subshell is lost at the subshell
# boundary, so a worker cannot touch TESTS or FAILS. It writes into its own slot
# directory and the reporting pass tallies in queue order, which is also what
# keeps the output ordered rather than interleaved. Same shape as
# generate-config.test.sh's Alloy validation queue.
#
# The body is re-entered by re-executing this file rather than by wrapping it in
# a function, so the checks below stay exactly where they were and a reviewer can
# see that none of them changed.
INIT_JOBS="${INIT_JOBS:-6}"
SUITE_SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/${BASH_SOURCE[0]##*/}"
SLOT=0

# Both passes walk the slots in the same order. A slot that is missing, of the
# wrong kind, or was never executed means the two passes disagreed about the
# sequence of invocations, which would silently mis-attribute every later
# result. Fail loudly instead.
slot_ready() {
  if [ ! -f "${1}/rc" ] || [ "$(cat "${1}/kind")" != "${2}" ]; then
    echo "FATAL: queue slot ${1} is missing, empty or not a ${2} result." >&2
    exit 1
  fi
}

# Run init-alloy/run against a given settings.json.
# $1 = settings JSON. Sets RUN_RC, RUN_OUT, RUN_CONFIG and RUN_SETTINGS.
run_init() {
  local slot="${QUEUE}/${SLOT}"
  SLOT=$((SLOT + 1))
  if [ "${SUITE_PHASE}" = record ]; then
    mkdir -p "${slot}"
    printf 'init' >"${slot}/kind"
    printf '%s' "$1" >"${slot}/in.settings"
    # Placeholders: the recording pass evaluates the checks too, and its verdicts
    # are discarded. They only have to be well-formed enough not to abort it.
    RUN_RC=0
    RUN_OUT=""
    RUN_CONFIG=""
    RUN_SETTINGS="{}"
    return 0
  fi
  slot_ready "${slot}" init
  RUN_RC="$(cat "${slot}/rc")"
  RUN_OUT="$(cat "${slot}/out")"
  RUN_CONFIG="$(cat "${slot}/config")"
  RUN_SETTINGS="$(cat "${slot}/settings")"
}

# Execute one recorded init-alloy/run. Runs backgrounded, so it reports through
# files in its own slot; it cannot reach TESTS/FAILS in the parent.
run_init_worker() {
  local slot="$1" tmp out rc
  tmp="$(mktemp -d)"
  # bashio reads its cached Supervisor response from $CACHE_DIR, and insists the
  # directory is owner-only.
  mkdir -p "${tmp}/cache" "${tmp}/etc" "${tmp}/data"
  chmod 0700 "${tmp}/cache"
  printf '%s' '{"safe_mode":false,"ui_log_level":"info"}' >"${tmp}/cache/addons.self.options.config.cache"
  printf '%s' '{"slug":"a141124a_alloy"}' >"${tmp}/cache/addons.self.info.cache"
  cp "${slot}/in.settings" "${tmp}/data/settings.json"

  out="$(
    CACHE_DIR="${tmp}/cache" \
    CONFIG_DIR="${tmp}/etc" \
    DATA_DIR="${tmp}/data" \
    SETTINGS_FILE="${tmp}/data/settings.json" \
    LEGACY_OPTIONS_FILE="${tmp}/data/options.json" \
    GENERATOR="${GENERATOR}" \
      "${BASHIO_BIN}" "${INIT}" 2>&1
  )"
  rc=$?
  printf '%s' "${out}" >"${slot}/out"
  : >"${slot}/config"
  [ -f "${tmp}/etc/config.alloy" ] && cat "${tmp}/etc/config.alloy" >"${slot}/config"
  cat "${tmp}/data/settings.json" >"${slot}/settings"
  rm -rf "${tmp}"
  # rc is written last: its presence is what marks the slot complete.
  printf '%s' "${rc}" >"${slot}/rc"
}

# Run alloy/finish with s6's two arguments.
# Sets FINISH_RC, FINISH_OUT, FINISH_HALTED and FINISH_EXIT_CODE.
run_finish() {
  local slot="${QUEUE}/${SLOT}"
  SLOT=$((SLOT + 1))
  if [ "${SUITE_PHASE}" = record ]; then
    mkdir -p "${slot}"
    printf 'finish' >"${slot}/kind"
    printf '%s' "$1" >"${slot}/in.exit"
    printf '%s' "$2" >"${slot}/in.signal"
    FINISH_RC=0
    FINISH_OUT=""
    FINISH_HALTED=false
    FINISH_EXIT_CODE=""
    return 0
  fi
  slot_ready "${slot}" finish
  FINISH_RC="$(cat "${slot}/rc")"
  FINISH_OUT="$(cat "${slot}/out")"
  FINISH_HALTED="$(cat "${slot}/halted")"
  FINISH_EXIT_CODE="$(cat "${slot}/exitcode")"
}

# Execute one recorded alloy/finish. Backgrounded, same slot contract as above.
run_finish_worker() {
  local slot="$1" tmp out rc
  tmp="$(mktemp -d)"
  mkdir -p "${tmp}/results"
  # The generated helper expands ALLOY_TEST_HALT when it runs, not here.
  # shellcheck disable=SC2016
  printf '#!/usr/bin/env bash\nprintf halted >"${ALLOY_TEST_HALT}"\n' >"${tmp}/halt"
  chmod +x "${tmp}/halt"
  out="$(
    ALLOY_TEST_HALT="${tmp}/halted" \
    S6_RESULTS_DIR="${tmp}/results" \
    S6_HALT="${tmp}/halt" \
      "${BASHIO_BIN}" "${FINISH}" "$(cat "${slot}/in.exit")" "$(cat "${slot}/in.signal")" 2>&1
  )"
  rc=$?
  printf '%s' "${out}" >"${slot}/out"
  if [ -e "${tmp}/halted" ]; then printf 'true' >"${slot}/halted"; else printf 'false' >"${slot}/halted"; fi
  cat "${tmp}/results/exitcode" >"${slot}/exitcode" 2>/dev/null || : >"${slot}/exitcode"
  rm -rf "${tmp}"
  printf '%s' "${rc}" >"${slot}/rc"
}

execute_slot() {
  case "$(cat "${1}/kind")" in
    init) run_init_worker "${1}" ;;
    finish) run_finish_worker "${1}" ;;
    *)
      echo "FATAL: queue slot ${1} has no recognised kind." >&2
      exit 1
      ;;
  esac
}

# Drain the recorded queue INIT_JOBS at a time. Batch-and-wait rather than
# `wait -n`, which macOS's bash 3.2 does not have.
drain_queue() {
  local i=0 running=0
  while [ "${i}" -lt "${SLOT}" ]; do
    execute_slot "${QUEUE}/${i}" &
    running=$((running + 1))
    i=$((i + 1))
    if [ "${running}" -ge "${INIT_JOBS}" ]; then wait; running=0; fi
  done
  wait
}

# SUITE_PHASE is set only on the two child passes, so the outermost invocation is
# the one that orchestrates them.
if [ -z "${SUITE_PHASE:-}" ]; then
  QUEUE="$(mktemp -d)"
  export QUEUE
  trap 'rm -rf "${QUEUE}"' EXIT
  # The recording pass prints its placeholder verdicts; they are discarded. Only
  # the queue it leaves behind is used, and a non-zero exit from it means nothing.
  SUITE_PHASE=record bash "${SUITE_SELF}" >/dev/null 2>&1
  while [ -d "${QUEUE}/${SLOT}" ]; do SLOT=$((SLOT + 1)); done
  if [ "${SLOT}" -eq 0 ]; then
    echo "FATAL: the recording pass queued no invocations." >&2
    exit 1
  fi
  drain_queue
  SUITE_PHASE=report bash "${SUITE_SELF}"
  exit $?
fi

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

echo "== unconfigured startup and destination selection =="
expect_ok "fresh install starts for ingress configuration" \
  '{"instance_name":"hass","log_level":"info"}' \
  "logging {"
expect_ok "blank optional destinations still allow ingress configuration" \
  '{"loki_url":"","prometheus_url":"","fleet_url":"","instance_name":"hass"}' \
  "logging {"
expect_fatal "explicit Local mode still requires a destination" \
  '{"operation_mode":"local"}' \
  "Local mode requires at least one local destination"

echo
echo "== each destination works on its own =="
expect_ok "logs only" "{${LOKI},\"instance_name\":\"hass\"}" \
  "loki.write" "loki.source.journal"
expect_ok "metrics only" "{${PROM},\"instance_name\":\"hass\"}" \
  "prometheus.remote_write" "prometheus.exporter.unix"
# The regression that started all this: a Fleet-only install must be possible.
expect_ok "fleet only" "{${FLEET},\"instance_name\":\"hass\"}" \
  "remotecfg" 'url            = "https://fleet.example.net"' \
  '"haos" = "true"' '"journal_path" = "/run/log/journal"' \
  '"alloy_container_name" = "app_a141124a_alloy"' \
  '"alloy_legacy_container_name" = "addon_a141124a_alloy"' '"ha_addon_slug" = "a141124a_alloy"'
TESTS=$((TESTS + 1))
run_init "{${FLEET},\"fleet_default_attributes\":false}"
if [ "${RUN_RC}" -ne 0 ] || grep -qF '"haos" = "true"' <<<"${RUN_CONFIG}"; then
  fail "Fleet default attributes can be disabled"
else
  pass "Fleet default attributes can be disabled"
fi
TESTS=$((TESTS + 1))
run_init "{${FLEET},\"instance_name\":\"hass\",\"restart_required\":true}"
if [ "${RUN_RC}" -ne 0 ] || ! jq -e '.restart_required == false' <<<"${RUN_SETTINGS}" >/dev/null; then
  fail "successful initialization marks saved settings as applied"
else
  pass "successful initialization marks saved settings as applied"
fi

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
expect_fatal "quoted Loki username cannot break River syntax" \
  '{"loki_url":"http://loki:3100/loki/api/v1/push","loki_username":"tenant\"name","loki_password":"secret","operation_mode":"local"}' \
  "loki_username contains a quote, backslash or control character"
expect_fatal "backslashed Prometheus username cannot break River syntax" \
  '{"prometheus_url":"http://prometheus:9090/api/v1/write","prometheus_username":"tenant\\name","prometheus_password":"secret","operation_mode":"local"}' \
  "prometheus_username contains a quote, backslash or control character"
expect_fatal "quoted Tempo username cannot break River syntax" \
  '{"tempo_url":"http://tempo:4318","tempo_username":"tenant\"name","tempo_password":"secret","operation_mode":"local"}' \
  "tempo_username contains a quote, backslash or control character"
expect_fatal "backslashed Pyroscope username cannot break River syntax" \
  '{"pyroscope_url":"http://pyroscope:4040","pyroscope_username":"tenant\\name","pyroscope_password":"secret","operation_mode":"local"}' \
  "pyroscope_username contains a quote, backslash or control character"
expect_fatal "quoted Fleet username cannot break River syntax" \
  '{"fleet_url":"https://fleet.example.net","fleet_username":"tenant\"name","gcloud_rw_api_key":"secret","operation_mode":"fleet"}' \
  "fleet_username contains a quote, backslash or control character"
expect_fatal "quoted instance name cannot break River syntax" \
  '{"loki_url":"http://loki:3100/loki/api/v1/push","instance_name":"home\"assistant","operation_mode":"local"}' \
  "instance_name contains a quote, backslash or control character"
expect_fatal "backslashed Fleet collector name cannot break River syntax" \
  '{"fleet_url":"https://fleet.example.net","fleet_username":"123","gcloud_rw_api_key":"secret","fleet_collector_name":"Home\\Assistant","operation_mode":"fleet"}' \
  "fleet_collector_name contains a quote, backslash or control character"

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
expect_ok "Local mode ignores a retained Fleet key" \
  "{${LOKI},\"operation_mode\":\"local\",\"gcloud_rw_api_key\":\"secret\"}" \
  "loki.write"
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
TESTS=$((TESTS + 1))
run_init "{${FLEET},\"fleet_attributes\":\"env=home,ha_addon_instance=other,haos=false,journal_path=/old,alloy_container_name=old,alloy_legacy_container_name=old,ha_addon_slug=old,role=hass\"}"
if [ "${RUN_RC}" -ne 0 ]; then
  fail "reserved App targeting key is migrated: exited ${RUN_RC}"
  indent "${RUN_OUT}"
elif grep -qF '"ha_addon_instance" = "other"' <<<"${RUN_CONFIG}" \
  || grep -qF '"haos" = "false"' <<<"${RUN_CONFIG}" \
  || grep -qF '"journal_path" = "/old"' <<<"${RUN_CONFIG}" \
  || grep -qF '"alloy_container_name" = "old"' <<<"${RUN_CONFIG}" \
  || grep -qF '"alloy_legacy_container_name" = "old"' <<<"${RUN_CONFIG}" \
  || grep -qF '"ha_addon_slug" = "old"' <<<"${RUN_CONFIG}"; then
  fail "reserved App targeting key is migrated: retained the old value"
elif ! grep -qF '"ha_addon_instance" = "homeassistant"' <<<"${RUN_CONFIG}" \
  || ! grep -qF '"env" = "home"' <<<"${RUN_CONFIG}" \
  || ! grep -qF '"role" = "hass"' <<<"${RUN_CONFIG}"; then
  fail "reserved App targeting key is migrated: lost the App value or adjacent attributes"
else
  pass "reserved App targeting key is migrated"
fi
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
echo "== an ingress-managed manual config replaces the generated one =="
TESTS=$((TESTS + 1))
run_init '{"manual_config_enabled":true,"manual_config":"logging { level = \"debug\" }"}'
if [ "${RUN_RC}" -ne 0 ]; then
  fail "override run exited ${RUN_RC}"
  indent "${RUN_OUT}"
elif [ "${RUN_CONFIG}" != 'logging { level = "debug" }' ]; then
  fail "override was not used verbatim, got: ${RUN_CONFIG}"
else
  pass "the manual override is copied verbatim"
fi
TESTS=$((TESTS + 1))
if grep -qF "manual Alloy configuration override" <<<"${RUN_OUT}"; then
  pass "the override is announced in the log"
else
  fail "the override was silent"
fi
# With an override the options no longer describe what Alloy does, so the
# destination requirement must not be enforced.
TESTS=$((TESTS + 1))
run_init '{"manual_config_enabled":true,"manual_config":"logging { level = \"info\" }"}'
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
  fail "SIGSEGV finish returned a failure instead of allowing supervision to restart Alloy"
elif [ "${FINISH_HALTED}" = true ] || [ -n "${FINISH_EXIT_CODE}" ]; then
  fail "SIGSEGV halted the App and took down the configuration UI"
elif ! grep -qF "signal 11" <<<"${FINISH_OUT}"; then
  fail "SIGSEGV finish did not report signal 11"
else
  pass "SIGSEGV leaves the configuration UI running while Alloy is restarted"
fi

echo
if [ "${FAILS}" -gt 0 ]; then
  echo "== RESULTS: ${FAILS} of ${TESTS} checks FAILED =="
  exit 1
fi
echo "== RESULTS: ${TESTS}/${TESTS} checks passed =="
echo "ALL PASS"
