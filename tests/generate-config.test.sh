#!/usr/bin/env bash
# Test harness for generate-config.sh
# Usage: tests/generate-config.test.sh
set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GEN="${REPO_ROOT}/alloy/rootfs/usr/share/alloy/generate-config.sh"
ALLOY_IMAGE="grafana/alloy:v1.17.0"
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

# Validate Alloy config syntax+semantics via Docker, if available.
validate_alloy() {
  local cfg="$1" name="$2"
  if ! command -v docker >/dev/null 2>&1; then echo "  ⚠ docker absent — skipping Alloy validation for $name"; return 0; fi
  local tmp; tmp="$(mktemp -d)"; printf '%s\n' "$cfg" > "$tmp/config.alloy"
  TESTS=$((TESTS+1))
  # `alloy fmt` parses River grammar; non-zero on syntax error.
  if docker run --rm -v "$tmp:/c:ro" "$ALLOY_IMAGE" fmt /c/config.alloy >/dev/null 2>"$tmp/err"; then
    pass "alloy fmt OK ($name)"
  else
    fail "alloy fmt FAILED ($name): $(cat "$tmp/err")"
  fi
  # `alloy run` for ~4s catches semantic errors (unknown component/arg/function).
  # Expect it NOT to print a config load error; the timeout itself is success.
  TESTS=$((TESTS+1))
  local out
  out="$(docker run --rm -v "$tmp:/c:ro" "$ALLOY_IMAGE" run --server.http.listen-addr=0.0.0.0:0 --storage.path=/tmp/d /c/config.alloy 2>&1 & p=$!; sleep 4; kill $p 2>/dev/null; wait $p 2>/dev/null)"
  # Drop network noise: the fleet cases deliberately point at an unresolvable host, so
  # remotecfg polling fails. Config errors are reported at load time without these strings.
  out="$(grep -viE 'no such host|connection refused|dial tcp|context deadline exceeded' <<<"$out" || true)"
  if grep -qiE 'error during the initial|could not perform|failed to (build|load)|unrecognized|parse error|expected .* but got|invalid (argument|expression)' <<<"$out"; then
    fail "alloy run reported config error ($name): $(grep -iE 'error|expected|invalid' <<<"$out" | head -3)"
  else
    pass "alloy run loaded config ($name)"
  fi
  rm -rf "$tmp"
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

echo "== fleet-only =="
# .invalid never resolves, so `alloy run` fails DNS instead of reaching a real endpoint.
OUT="$(gen LOG_LEVEL=info FLEET_URL=https://fleet-management-prod-001.example.invalid)"
check_contains "$OUT" 'remotecfg {'
check_contains "$OUT" 'url            = "https://fleet-management-prod-001.example.invalid"'
check_contains "$OUT" 'id             = "homeassistant"'
check_contains "$OUT" 'poll_frequency = "5m"'
check_absent   "$OUT" 'loki.source.journal'
check_absent   "$OUT" 'prometheus.exporter.unix'
check_absent   "$OUT" 'attributes     = {'
check_absent   "$OUT" 'name           ='
check_absent   "$OUT" 'basic_auth {'
validate_alloy "$OUT" "fleet-only"

echo "== fleet with auth, name and attributes =="
OUT="$(gen LOG_LEVEL=info FLEET_URL=https://fleet-management-prod-001.example.invalid \
  FLEET_USERNAME=987654 FLEET_COLLECTOR_NAME="Home Assistant" \
  FLEET_ATTRIBUTES=env=home,role=hass FLEET_POLL_FREQUENCY=30s INSTANCE_NAME=hass-test)"
check_contains "$OUT" 'id             = "hass-test"'
check_contains "$OUT" 'name           = "Home Assistant"'
check_contains "$OUT" 'attributes     = {'
check_contains "$OUT" '"env" = "home",'
check_contains "$OUT" '"role" = "hass",'
check_contains "$OUT" 'poll_frequency = "30s"'
check_contains "$OUT" 'username = "987654"'
check_contains "$OUT" 'password = sys.env("FLEET_PASSWORD")'
validate_alloy "$OUT" "fleet-full"

echo "== fleet, single attribute =="
OUT="$(gen LOG_LEVEL=info FLEET_URL=https://fleet-management-prod-001.example.invalid FLEET_ATTRIBUTES=env=home)"
check_contains "$OUT" 'attributes     = {'
check_contains "$OUT" '"env" = "home",'
check_absent   "$OUT" '"role"'

echo "== all three backends (each secret confined to its own block) =="
OUT="$(gen LOG_LEVEL=info LOKI_URL=https://logs.example.net/loki/api/v1/push LOKI_USERNAME=111 \
  PROMETHEUS_URL=https://prom.example.net/api/prom/push PROMETHEUS_USERNAME=222 \
  FLEET_URL=https://fleet-management-prod-001.example.invalid FLEET_USERNAME=333)"
FLEET_BLOCK="$(block "$OUT" 'remotecfg {')"
LOKI_BLOCK="$(block "$OUT" 'loki.write "loki" {')"
PROM_BLOCK="$(block "$OUT" 'prometheus.remote_write "metrics" {')"
check_contains "$FLEET_BLOCK" 'username = "333"'
check_contains "$FLEET_BLOCK" 'sys.env("FLEET_PASSWORD")'
check_absent   "$FLEET_BLOCK" 'LOKI_PASSWORD'
check_absent   "$FLEET_BLOCK" 'PROMETHEUS_PASSWORD'
check_absent   "$LOKI_BLOCK"  'FLEET_PASSWORD'
check_absent   "$PROM_BLOCK"  'FLEET_PASSWORD'
validate_alloy "$OUT" "all-three"

echo "== passwords never reach the generated config =="
# Passwords are in the generator's environment (as they are at runtime); the config
# must reference them by env-var name only, never interpolate the value.
OUT="$(gen LOG_LEVEL=info LOKI_URL=https://logs.example.net/loki/api/v1/push LOKI_USERNAME=111 \
  PROMETHEUS_URL=https://prom.example.net/api/prom/push PROMETHEUS_USERNAME=222 \
  FLEET_URL=https://fleet-management-prod-001.example.invalid FLEET_USERNAME=333 \
  LOKI_PASSWORD=SENTINELLOKISECRET PROMETHEUS_PASSWORD=SENTINELPROMSECRET \
  FLEET_PASSWORD=SENTINELFLEETSECRET)"
check_absent "$OUT" 'SENTINELLOKISECRET'
check_absent "$OUT" 'SENTINELPROMSECRET'
check_absent "$OUT" 'SENTINELFLEETSECRET'

echo ""
echo "== RESULTS: $((TESTS-FAILS))/$TESTS checks passed =="
[ "$FAILS" -eq 0 ] || { echo "FAILED ($FAILS)"; exit 1; }
echo "ALL PASS"
