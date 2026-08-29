#!/usr/bin/env bash
# Test harness for generate-config.sh
# Usage: alloy/tests/generate-config.test.sh
set -u

ADDON_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GEN="${ADDON_ROOT}/rootfs/usr/share/alloy/generate-config.sh"
# Parsed from the Dockerfile rather than duplicated, so the container used to
# validate the generated config can never drift from the shipped binary.
ALLOY_VERSION="$(sed -n 's/^[[:space:]]*ALLOY_VERSION="\([^"]*\)".*/\1/p' "${ADDON_ROOT}/Dockerfile")"
if [ -z "${ALLOY_VERSION}" ]; then
  echo "FATAL: could not parse ALLOY_VERSION out of ${ADDON_ROOT}/Dockerfile" >&2
  exit 1
fi
ALLOY_IMAGE="grafana/alloy:v${ALLOY_VERSION}"
FAILS=0
TESTS=0

fail() { echo "  ✗ $1"; FAILS=$((FAILS+1)); }
pass() { echo "  ✓ $1"; }
check_contains() { TESTS=$((TESTS+1)); if grep -qF -- "$2" <<<"$1"; then pass "contains: $2"; else fail "MISSING: $2"; fi; }
check_absent()   { TESTS=$((TESTS+1)); if grep -qF -- "$2" <<<"$1"; then fail "SHOULD BE ABSENT: $2"; else pass "absent: $2"; fi; }

# Run the generator with an explicit environment: `gen VAR=value ...`. Passing the
# options through `env` keeps each case isolated - nothing leaks into the parent
# shell or into the next case.
gen() { env "$@" bash "$GEN"; }

# Print one top-level config block: from the line starting with $2 to its closing
# brace in column 0. Used to assert a secret is referenced in the right block only.
block() { awk -v start="$2" 'index($0,start)==1{f=1} f{print} f&&/^}$/{exit}' <<<"$1"; }

# --- Alloy config validation --------------------------------------------------
# Every fixture's generated config is queued here and validated in one bounded
# parallel batch at the end of the run.
#
# `alloy validate` loads and type-checks a config without starting a single
# component, so the verdict is its exit status. The check it replaces ran
# `alloy run --server.http.listen-addr=0.0.0.0:0`: port 0 makes memberlist abort
# with "missing real listen port" ~0.5s in, before the config is ever read, and
# that string was absent from the stderr allowlist the check keyed off - so every
# fixture "passed" without being loaded, including one naming a component that
# does not exist. An exit status needs no allowlist kept in step with Alloy's
# error strings, and cannot pass a config that was never parsed.
#
# Nothing starts, so nothing dials the network: the fleet fixtures point at an
# unresolvable .invalid host on purpose and validate never resolves it. The grep
# that used to drop DNS and connection noise went with the probe.
#
# `--stability.level` is left at its default of generally-available to match the
# App's own default in s6-rc.d/alloy/run. A config the shipped default would
# refuse to load must not pass here.
#
# Docker dominates the runtime, so fixtures run concurrently. A counter
# incremented inside a backgrounded subshell is lost at the subshell boundary, so
# each worker writes its verdicts into its own slot directory and the parent
# tallies them in queue order after every worker has finished - which is also what
# keeps the output ordered rather than interleaved.
VALIDATE_JOBS="${VALIDATE_JOBS:-6}"
VALIDATE_QUEUE="$(mktemp -d)"
VALIDATE_COUNT=0
trap 'rm -rf "$VALIDATE_QUEUE"' EXIT
HAVE_DOCKER=1
command -v docker >/dev/null 2>&1 || HAVE_DOCKER=0

# Queue one generated config for validation: `validate_alloy "$OUT" <name>`.
validate_alloy() {
  if [ "$HAVE_DOCKER" -eq 0 ]; then
    echo "  ⚠ docker absent — skipping Alloy validation for $2"
    return 0
  fi
  local slot="$VALIDATE_QUEUE/$VALIDATE_COUNT"
  mkdir -p "$slot"
  printf '%s\n' "$1" >"$slot/config.alloy"
  printf '%s\n' "$2" >"$slot/name"
  VALIDATE_COUNT=$((VALIDATE_COUNT + 1))
}

# Validate one queued fixture. Runs backgrounded, so it reports through files in
# its own slot; it cannot reach TESTS/FAILS in the parent.
validate_alloy_worker() {
  local slot="$1"
  # `alloy fmt` parses River grammar; non-zero on a syntax error.
  if docker run --rm -v "$slot:/c:ro" "$ALLOY_IMAGE" fmt /c/config.alloy >/dev/null 2>"$slot/fmt.err"; then
    echo ok >"$slot/fmt.verdict"
  else
    echo fail >"$slot/fmt.verdict"
  fi
  # `alloy validate` additionally resolves every component, argument and function.
  if docker run --rm -v "$slot:/c:ro" "$ALLOY_IMAGE" validate /c/config.alloy >"$slot/validate.err" 2>&1; then
    echo ok >"$slot/validate.verdict"
  else
    echo fail >"$slot/validate.verdict"
  fi
}

# Drain the queue VALIDATE_JOBS at a time, then report the verdicts in queue order.
run_queued_validations() {
  [ "$VALIDATE_COUNT" -gt 0 ] || return 0
  echo ""
  echo "== Alloy config validation ($VALIDATE_COUNT fixtures, $VALIDATE_JOBS in parallel) =="
  local i=0 running=0
  while [ "$i" -lt "$VALIDATE_COUNT" ]; do
    validate_alloy_worker "$VALIDATE_QUEUE/$i" &
    running=$((running + 1))
    i=$((i + 1))
    if [ "$running" -ge "$VALIDATE_JOBS" ]; then wait; running=0; fi
  done
  wait
  i=0
  while [ "$i" -lt "$VALIDATE_COUNT" ]; do
    local slot="$VALIDATE_QUEUE/$i" name
    name="$(cat "$slot/name")"
    TESTS=$((TESTS + 1))
    if [ "$(cat "$slot/fmt.verdict" 2>/dev/null)" = ok ]; then
      pass "alloy fmt OK ($name)"
    else
      fail "alloy fmt FAILED ($name):"
      sed 's/^/      /' "$slot/fmt.err"
    fi
    TESTS=$((TESTS + 1))
    if [ "$(cat "$slot/validate.verdict" 2>/dev/null)" = ok ]; then
      pass "alloy validate OK ($name)"
    else
      fail "alloy validate FAILED ($name):"
      sed 's/^/      /' "$slot/validate.err"
    fi
    i=$((i + 1))
  done
}

echo "== generator shebang (must NOT be with-contenv: it resets env, wiping exported options) =="
TESTS=$((TESTS+1))
if head -1 "$GEN" | grep -q 'with-contenv'; then
  fail "generator uses with-contenv shebang — would wipe exported LOKI_URL/PROMETHEUS_URL"
else
  pass "generator shebang is env-preserving: $(head -1 "$GEN")"
fi

echo "== logs-only =="
OUT="$(gen LOG_LEVEL=info JOURNAL_PATH=/var/log/journal LOKI_URL=http://loki:3100/loki/api/v1/push)"
check_contains "$OUT" 'logging {'
check_contains "$OUT" 'level = "info"'
check_contains "$OUT" 'loki.source.journal "journal"'
check_contains "$OUT" 'path         = "/var/log/journal"'
check_contains "$OUT" 'loki.write "loki"'
check_contains "$OUT" 'url = "http://loki:3100/loki/api/v1/push"'
check_absent   "$OUT" 'prometheus.exporter.unix'
check_absent   "$OUT" 'basic_auth {'
validate_alloy "$OUT" "logs-only"

echo "== logs-only (with basic auth) =="
OUT="$(gen LOG_LEVEL=info JOURNAL_PATH=/var/log/journal LOKI_URL=https://logs-prod.example.net/loki/api/v1/push LOKI_USERNAME=123456)"
check_contains "$OUT" 'loki.write "loki"'
check_contains "$OUT" 'basic_auth {'
check_contains "$OUT" 'username = "123456"'
check_contains "$OUT" 'password = sys.env("LOKI_PASSWORD")'
validate_alloy "$OUT" "logs-auth"

echo "== metrics-only (with basic auth) =="
OUT="$(gen LOG_LEVEL=info PROMETHEUS_URL=http://prom:9090/api/v1/write PROMETHEUS_USERNAME=12345 INSTANCE_NAME=hass-test METRICS_SCRAPE_INTERVAL=30s)"
check_contains "$OUT" 'prometheus.exporter.unix "host"'
check_contains "$OUT" 'discovery.relabel "host"'
check_contains "$OUT" 'target_label = "instance"'
check_contains "$OUT" 'replacement  = "hass-test"'
check_contains "$OUT" 'prometheus.scrape "host"'
check_contains "$OUT" 'targets         = discovery.relabel.host.output'
check_contains "$OUT" 'scrape_interval = "30s"'
check_contains "$OUT" 'prometheus.remote_write "metrics"'
check_contains "$OUT" 'url = "http://prom:9090/api/v1/write"'
check_contains "$OUT" 'basic_auth {'
check_contains "$OUT" 'username = "12345"'
check_contains "$OUT" 'password = sys.env("PROMETHEUS_PASSWORD")'
check_absent   "$OUT" 'loki.source.journal'
validate_alloy "$OUT" "metrics-only"

echo "== Go duration forms accepted by Alloy =="
OUT="$(gen LOG_LEVEL=info PROMETHEUS_URL=http://prom:9090/api/v1/write METRICS_SCRAPE_INTERVAL=+15s)"
check_contains "$OUT" 'scrape_interval = "+15s"'
validate_alloy "$OUT" "metrics-leading-plus-duration"

OUT="$(gen LOG_LEVEL=info FLEET_URL=https://fleet-management-prod-001.example.invalid FLEET_POLL_FREQUENCY=+10s)"
check_contains "$OUT" 'poll_frequency = "+10s"'
validate_alloy "$OUT" "fleet-leading-plus-duration"

echo "== both, no auth =="
OUT="$(gen LOG_LEVEL=warn JOURNAL_PATH=/run/log/journal LOKI_URL=http://loki:3100/loki/api/v1/push PROMETHEUS_URL=http://prom:9090/api/v1/write)"
check_contains "$OUT" 'loki.source.journal "journal"'
check_contains "$OUT" 'prometheus.exporter.unix "host"'
check_contains "$OUT" 'prometheus.remote_write "metrics"'
check_contains "$OUT" 'replacement  = "homeassistant"'
check_contains "$OUT" 'scrape_interval = "60s"'
check_absent   "$OUT" 'basic_auth {'
validate_alloy "$OUT" "both-noauth"

echo "== both, both authed (each secret confined to its own block) =="
OUT="$(gen LOG_LEVEL=info LOKI_URL=https://logs-prod.example.net/loki/api/v1/push LOKI_USERNAME=111 \
  PROMETHEUS_URL=https://prom-prod.example.net/api/prom/push PROMETHEUS_USERNAME=222)"
LOKI_BLOCK="$(block "$OUT" 'loki.write "loki" {')"
PROM_BLOCK="$(block "$OUT" 'prometheus.remote_write "metrics" {')"
check_contains "$LOKI_BLOCK" 'username = "111"'
check_contains "$LOKI_BLOCK" 'sys.env("LOKI_PASSWORD")'
check_absent   "$LOKI_BLOCK" 'PROMETHEUS_PASSWORD'
check_contains "$PROM_BLOCK" 'username = "222"'
check_contains "$PROM_BLOCK" 'sys.env("PROMETHEUS_PASSWORD")'
check_absent   "$PROM_BLOCK" 'LOKI_PASSWORD'
validate_alloy "$OUT" "both-auth"

echo "== identity labels are forced to the instance name on metrics and logs =="
# Grafana's stock dashboards key off instance/hostname/nodename. Inside the App
# those default to the container's hostname (a141124a-alloy), so the generated
# config overwrites them with the operator-chosen instance name.
OUT="$(gen LOG_LEVEL=info LOKI_URL=https://logs-prod.example.net/loki/api/v1/push \
  PROMETHEUS_URL=https://prom-prod.example.net/api/prom/push INSTANCE_NAME=hass-test)"
IDENTITY_BLOCK="$(block "$OUT" 'prometheus.relabel "host_identity" {')"
check_contains "$IDENTITY_BLOCK" 'forward_to = [prometheus.remote_write.metrics.receiver]'
check_contains "$IDENTITY_BLOCK" 'target_label = "instance"'
check_contains "$IDENTITY_BLOCK" 'source_labels = ["nodename"]'
check_contains "$IDENTITY_BLOCK" 'target_label  = "nodename"'
check_contains "$IDENTITY_BLOCK" 'source_labels = ["hostname"]'
check_contains "$IDENTITY_BLOCK" 'target_label  = "hostname"'
check_contains "$IDENTITY_BLOCK" 'replacement  = "hass-test"'
check_contains "$OUT" 'forward_to      = [prometheus.relabel.host_identity.receiver]'
# The rewrite must NOT sit on the shared remote-write endpoint: additional user
# config forwards straight to it, and an unconditional instance would collapse
# distinct targets into one series.
PROM_BLOCK="$(block "$OUT" 'prometheus.remote_write "metrics" {')"
check_absent   "$PROM_BLOCK" 'write_relabel_config'
JOURNAL_BLOCK="$(block "$OUT" 'loki.relabel "journal" {')"
check_contains "$JOURNAL_BLOCK" 'target_label = "hostname"'
check_contains "$JOURNAL_BLOCK" 'target_label = "instance"'
check_contains "$JOURNAL_BLOCK" 'replacement  = "hass-test"'
check_absent   "$JOURNAL_BLOCK" '"__journal__hostname"'
validate_alloy "$OUT" "forced-identity-labels"

# Only the host exporter emits nodename/hostname, so the relabel belongs in its
# chain alone. Without host metrics there is nothing for it to rewrite.
OUT="$(gen LOG_LEVEL=info PROMETHEUS_URL=https://prom-prod.example.net/api/prom/push \
  HOST_METRICS=false ALLOY_METRICS=true INSTANCE_NAME=hass-test)"
check_absent   "$OUT" 'prometheus.relabel "host_identity"'
check_contains "$OUT" 'forward_to      = [prometheus.remote_write.metrics.receiver]'
validate_alloy "$OUT" "identity-relabel-scoped-to-host-metrics"

echo "== fleet-only =="
# .invalid never resolves, so `alloy run` fails DNS instead of reaching a real endpoint.
OUT="$(gen LOG_LEVEL=info FLEET_URL=https://fleet-management-prod-001.example.invalid \
  ADDON_SLUG=a141124a_alloy ALLOY_CONTAINER_NAME=app_a141124a_alloy)"
check_contains "$OUT" 'remotecfg {'
check_contains "$OUT" 'url            = "https://fleet-management-prod-001.example.invalid"'
check_contains "$OUT" 'id             = "homeassistant"'
check_contains "$OUT" 'poll_frequency = "1m"'
check_absent   "$OUT" 'loki.source.journal'
check_absent   "$OUT" 'prometheus.exporter.unix'
check_contains "$OUT" 'attributes     = {'
check_contains "$OUT" '"ha_addon_instance" = "homeassistant",'
check_contains "$OUT" '"haos" = "true",'
check_contains "$OUT" '"journal_path" = "/var/log/journal",'
check_contains "$OUT" '"alloy_container_name" = "app_a141124a_alloy",'
check_contains "$OUT" '"alloy_legacy_container_name" = "addon_a141124a_alloy",'
check_contains "$OUT" '"ha_addon_slug" = "a141124a_alloy",'
check_absent   "$OUT" 'name           ='
check_absent   "$OUT" 'basic_auth {'
validate_alloy "$OUT" "fleet-only"

OUT="$(gen LOG_LEVEL=info FLEET_URL=https://fleet-management-prod-001.example.invalid FLEET_DEFAULT_ATTRIBUTES=false)"
check_absent "$OUT" '"haos" = "true",'
check_absent "$OUT" '"journal_path" = '
check_absent "$OUT" '"alloy_container_name" = '
check_absent "$OUT" '"alloy_legacy_container_name" = '
check_absent "$OUT" '"ha_addon_slug" = '
validate_alloy "$OUT" "fleet-with-default-attributes-disabled"

echo "== fleet with auth, name and attributes =="
OUT="$(gen LOG_LEVEL=info FLEET_URL=https://fleet-management-prod-001.example.invalid \
  FLEET_USERNAME=987654 FLEET_COLLECTOR_NAME="Home Assistant" \
  FLEET_ATTRIBUTES=env=home,role=hass FLEET_POLL_FREQUENCY=30s INSTANCE_NAME=hass-test)"
check_contains "$OUT" 'id             = "hass-test"'
check_contains "$OUT" 'name           = "Home Assistant"'
check_contains "$OUT" 'attributes     = {'
check_contains "$OUT" '"ha_addon_instance" = "hass-test",'
check_contains "$OUT" '"env" = "home",'
check_contains "$OUT" '"role" = "hass",'
check_contains "$OUT" 'poll_frequency = "30s"'
check_contains "$OUT" 'username = "987654"'
check_contains "$OUT" 'password = sys.env("GCLOUD_RW_API_KEY")'
validate_alloy "$OUT" "fleet-full"

echo "== operation modes are exclusive =="
OUT="$(gen OPERATION_MODE=fleet FLEET_URL=https://fleet-management-prod-001.example.invalid \
  FLEET_USERNAME=987654 LOKI_URL=https://logs.example.net/loki/api/v1/push \
  PROMETHEUS_URL=https://prom.example.net/api/prom/push)"
check_contains "$OUT" 'remotecfg {'
check_absent   "$OUT" 'loki.write "loki"'
check_absent   "$OUT" 'prometheus.remote_write "metrics"'

OUT="$(gen OPERATION_MODE=fleet FLEET_URL=https://fleet-management-prod-001.example.invalid \
  ADDITIONAL_CONFIG='prometheus.exporter.self "unexpected" {}')"
check_absent   "$OUT" 'prometheus.exporter.self "unexpected" {}'

OUT="$(gen OPERATION_MODE=local FLEET_URL=https://fleet-management-prod-001.example.invalid \
  LOKI_URL=https://logs.example.net/loki/api/v1/push)"
check_contains "$OUT" 'loki.write "loki"'
check_absent   "$OUT" 'remotecfg {'

echo "== Fleet starter pipeline contains selected components without another controller =="
OUT="$(gen OPERATION_MODE=fleet FLEET_REFERENCE_PIPELINE=true \
  FLEET_URL=https://fleet-management-prod-001.example.invalid FLEET_USERNAME=987654 \
  LOKI_URL=https://logs.example.net/loki/api/v1/push LOKI_USERNAME=111 \
  PROMETHEUS_URL=https://prom.example.net/api/prom/push PROMETHEUS_USERNAME=222 \
  ADDITIONAL_CONFIG='prometheus.exporter.self "retained-local-config" {}')"
check_contains "$OUT" 'loki.source.journal "journal"'
check_contains "$OUT" 'prometheus.exporter.unix "host"'
check_contains "$OUT" 'password = sys.env("GCLOUD_RW_API_KEY")'
check_absent   "$OUT" 'prometheus.exporter.self "retained-local-config" {}'
check_absent   "$OUT" 'logging {'
check_absent   "$OUT" 'remotecfg {'
check_absent   "$OUT" 'sys.env("LOKI_PASSWORD")'
check_absent   "$OUT" 'sys.env("PROMETHEUS_PASSWORD")'
validate_alloy "$OUT" "fleet-starter-pipeline"

echo "== local signal pipelines =="
OUT="$(gen OPERATION_MODE=local PROMETHEUS_URL=https://prom.example.net/api/prom/push \
  HOST_METRICS=false HOMEASSISTANT_METRICS=false ALLOY_METRICS=true)"
check_contains "$OUT" 'prometheus.scrape "alloy"'
check_contains "$OUT" '"__address__" = "127.0.0.1:12345"'
validate_alloy "$OUT" "alloy-self-metrics"

OUT="$(gen OPERATION_MODE=local LOKI_URL=https://logs.example.net/loki/api/v1/push \
  LOGS_SYSTEM=true LOGS_HOMEASSISTANT=false LOGS_ADDONS=true \
  LOGS_EXCLUDE_ADDONS=alloy,example LOGS_MAX_AGE=24h)"
check_contains "$OUT" 'max_age      = "24h"'
check_contains "$OUT" 'action        = "drop"'
check_contains "$OUT" '(?:app|addon)_(?:[^_]+_)?alloy|(?:app|addon)_(?:[^_]+_)?example'
validate_alloy "$OUT" "filtered-journal"

OUT="$(gen OPERATION_MODE=local LOKI_URL=https://logs.example.net/loki/api/v1/push LOGS_ADDONS=false)"
check_contains "$OUT" 'regex         = "^(?:app|addon)_.*$"'
validate_alloy "$OUT" "all-add-on-journals-disabled"

OUT="$(gen OPERATION_MODE=local TRACES_ENABLED=true \
  TEMPO_URL=https://tempo.example.net/otlp TEMPO_USERNAME=123)"
check_contains "$OUT" 'otelcol.receiver.otlp "local"'
check_contains "$OUT" 'otelcol.exporter.otlphttp "tempo"'
check_contains "$OUT" 'endpoint = "127.0.0.1:4317"'
check_contains "$OUT" 'endpoint = "127.0.0.1:4318"'
check_absent "$OUT" 'endpoint = "0.0.0.0:4317"'
check_contains "$OUT" 'password = sys.env("TEMPO_PASSWORD")'
validate_alloy "$OUT" "tempo-otlp"

OUT="$(gen OPERATION_MODE=local TRACES_ENABLED=true TRACES_NETWORK_ACCESS=true \
  TEMPO_URL=https://tempo.example.net/otlp)"
check_contains "$OUT" 'endpoint = "0.0.0.0:4317"'
check_contains "$OUT" 'endpoint = "0.0.0.0:4318"'
validate_alloy "$OUT" "tempo-network-access"

OUT="$(gen OPERATION_MODE=local ALLOY_PROFILING=true \
  PYROSCOPE_URL=https://profiles.example.net PYROSCOPE_USERNAME=123)"
check_contains "$OUT" 'pyroscope.scrape "alloy"'
check_contains "$OUT" 'pyroscope.write "profiles"'
check_contains "$OUT" 'password = sys.env("PYROSCOPE_PASSWORD")'
validate_alloy "$OUT" "alloy-self-profiling"

echo "== fleet, single attribute =="
OUT="$(gen LOG_LEVEL=info FLEET_URL=https://fleet-management-prod-001.example.invalid FLEET_ATTRIBUTES=env=home)"
check_contains "$OUT" 'attributes     = {'
check_contains "$OUT" '"env" = "home",'
check_absent   "$OUT" '"role"'

echo "== fleet, equals signs in attribute values =="
OUT="$(gen LOG_LEVEL=info FLEET_URL=https://fleet-management-prod-001.example.invalid \
  FLEET_ATTRIBUTES='query=a=b,token=YWJjZA==')"
check_contains "$OUT" '"query" = "a=b",'
check_contains "$OUT" '"token" = "YWJjZA==",'
validate_alloy "$OUT" "fleet-attribute-equals"

echo "== all three backends (each secret confined to its own block) =="
OUT="$(gen LOG_LEVEL=info LOKI_URL=https://logs.example.net/loki/api/v1/push LOKI_USERNAME=111 \
  PROMETHEUS_URL=https://prom.example.net/api/prom/push PROMETHEUS_USERNAME=222 \
  FLEET_URL=https://fleet-management-prod-001.example.invalid FLEET_USERNAME=333)"
FLEET_BLOCK="$(block "$OUT" 'remotecfg {')"
LOKI_BLOCK="$(block "$OUT" 'loki.write "loki" {')"
PROM_BLOCK="$(block "$OUT" 'prometheus.remote_write "metrics" {')"
check_contains "$FLEET_BLOCK" 'username = "333"'
check_contains "$FLEET_BLOCK" 'sys.env("GCLOUD_RW_API_KEY")'
check_absent   "$FLEET_BLOCK" 'LOKI_PASSWORD'
check_absent   "$FLEET_BLOCK" 'PROMETHEUS_PASSWORD'
check_absent   "$LOKI_BLOCK"  'GCLOUD_RW_API_KEY'
check_absent   "$PROM_BLOCK"  'GCLOUD_RW_API_KEY'
validate_alloy "$OUT" "all-three"

echo "== metric sources are individually selectable =="
OUT="$(gen PROMETHEUS_URL=https://prom.example.net/api/prom/push HOST_METRICS=false)"
check_absent   "$OUT" 'prometheus.exporter.unix'
check_contains "$OUT" 'prometheus.remote_write "metrics"'
validate_alloy "$OUT" "no-host-metrics"

OUT="$(gen PROMETHEUS_URL=https://prom.example.net/api/prom/push HOMEASSISTANT_METRICS=true)"
check_contains "$OUT" 'prometheus.scrape "homeassistant"'
check_contains "$OUT" '"__address__" = "supervisor:80"'
check_contains "$OUT" 'metrics_path    = "/core/api/prometheus"'
# The Supervisor token is read from the environment, never written into the file.
check_contains "$OUT" 'bearer_token    = sys.env("SUPERVISOR_TOKEN")'
check_contains "$OUT" 'prometheus.exporter.unix'
validate_alloy "$OUT" "host-and-hass-metrics"

# Home Assistant metrics alone, with the host exporter switched off.
OUT="$(gen PROMETHEUS_URL=https://prom.example.net/api/prom/push HOMEASSISTANT_METRICS=true HOST_METRICS=false)"
check_contains "$OUT" 'prometheus.scrape "homeassistant"'
check_absent   "$OUT" 'prometheus.exporter.unix'
validate_alloy "$OUT" "hass-metrics-only"

# Nothing Home Assistant-shaped should appear unless it was asked for.
OUT="$(gen PROMETHEUS_URL=https://prom.example.net/api/prom/push)"
check_absent   "$OUT" 'core/api/prometheus'
check_absent   "$OUT" 'SUPERVISOR_TOKEN'

echo "== passwords never reach the generated config =="
# Passwords are in the generator's environment (as they are at runtime); the config
# must reference them by env-var name only, never interpolate the value.
OUT="$(gen LOG_LEVEL=info LOKI_URL=https://logs.example.net/loki/api/v1/push LOKI_USERNAME=111 \
  PROMETHEUS_URL=https://prom.example.net/api/prom/push PROMETHEUS_USERNAME=222 \
  FLEET_URL=https://fleet-management-prod-001.example.invalid FLEET_USERNAME=333 \
  LOKI_PASSWORD=SENTINELLOKISECRET PROMETHEUS_PASSWORD=SENTINELPROMSECRET \
  GCLOUD_RW_API_KEY=SENTINELFLEETSECRET)"
check_absent "$OUT" 'SENTINELLOKISECRET'
check_absent "$OUT" 'SENTINELPROMSECRET'
check_absent "$OUT" 'SENTINELFLEETSECRET'

run_queued_validations

echo ""
echo "== RESULTS: $((TESTS-FAILS))/$TESTS checks passed =="
[ "$FAILS" -eq 0 ] || { echo "FAILED ($FAILS)"; exit 1; }
echo "ALL PASS"
