#!/usr/bin/env bash
# Unit tests for shared/lib/ha-validate.sh.
# Usage: bash tests/shared_validate_lib_test.sh
#
# These deliberately need neither Docker nor bashio, so the shared validators
# stay verifiable in a plain checkout. The App-level suites that exercise the
# same rules through init-alloy/run and grafana-pdc/run still run in CI against
# their real base images.
set -u

REPOSITORY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FAILS=0
TESTS=0

fail() { echo "  x $1"; FAILS=$((FAILS + 1)); }
pass() { echo "  ok $1"; }

# A getent stand-in so IPv6 endpoint checks do not depend on the host resolver.
STUB_DIR="$(mktemp -d)"
trap 'rm -rf "${STUB_DIR}"' EXIT
cat >"${STUB_DIR}/getent" <<'STUB'
#!/usr/bin/env bash
# Accept syntactically plausible IPv6 literals, reject everything else.
[[ "$2" =~ ^[0-9A-Fa-f:.]+$ ]] && [[ "$2" == *:* ]]
STUB
chmod +x "${STUB_DIR}/getent"
export HA_GETENT_BIN="${STUB_DIR}/getent"

# shellcheck source=../shared/lib/ha-validate.sh
. "${REPOSITORY_ROOT}/shared/lib/ha-validate.sh"

# $1 = description, $2 = expected rc, rest = predicate and arguments
check_rc() {
    local description="$1" expected="$2"
    shift 2
    TESTS=$((TESTS + 1))
    "$@" >/dev/null 2>&1
    local actual=$?
    if [ "${actual}" -eq "${expected}" ]; then
        pass "${description}"
    else
        fail "${description} (expected rc ${expected}, got ${actual})"
    fi
}

accepts() { check_rc "accepts $1: ${2:-<empty>}" 0 "$1" "${@:2}"; }
rejects() { check_rc "rejects $1: ${2:-<empty>}" 1 "$1" "${@:2}"; }

# $1 = value, $2 = expected classification token
url_is() {
    TESTS=$((TESTS + 1))
    local actual
    actual="$(ha_url_problem "$1")"
    if [ "${actual}" = "$2" ]; then
        pass "url '${1:-<empty>}' -> ${2:-ok}"
    else
        fail "url '${1:-<empty>}' -> expected '${2:-ok}', got '${actual:-ok}'"
    fi
}

echo "== ha_valid_dns_name =="
accepts ha_valid_dns_name grafana.net
accepts ha_valid_dns_name prod-eu-west-0
accepts ha_valid_dns_name a
rejects ha_valid_dns_name Grafana.net
rejects ha_valid_dns_name -leading.example
rejects ha_valid_dns_name trailing-.example
rejects ha_valid_dns_name .example.com
rejects ha_valid_dns_name "example..com"
rejects ha_valid_dns_name "$(printf 'a%.0s' {1..254})"

echo "== ha_valid_dns_label =="
accepts ha_valid_dns_label prod-eu-west-0
rejects ha_valid_dns_label with.dot
rejects ha_valid_dns_label UPPER
rejects ha_valid_dns_label "$(printf 'a%.0s' {1..64})"

echo "== ha_valid_duration =="
accepts ha_valid_duration 0
accepts ha_valid_duration 5m
accepts ha_valid_duration 1h30m
accepts ha_valid_duration 1.5s
accepts ha_valid_duration 500ms
accepts ha_valid_duration .5s
rejects ha_valid_duration 5
rejects ha_valid_duration 5x
rejects ha_valid_duration ""
rejects ha_valid_duration "-5m"

echo "== ha_valid_positive_duration =="
accepts ha_valid_positive_duration 5m
accepts ha_valid_positive_duration 0.5s
rejects ha_valid_positive_duration 0
rejects ha_valid_positive_duration 0s
rejects ha_valid_positive_duration 0h0m

echo "== ha_valid_integer_range =="
accepts ha_valid_integer_range 1 1 50
accepts ha_valid_integer_range 50 1 50
accepts ha_valid_integer_range 08 1 50
rejects ha_valid_integer_range 0 1 50
rejects ha_valid_integer_range 51 1 50
rejects ha_valid_integer_range abc 1 50
rejects ha_valid_integer_range "-1" 1 50

echo "== ha_valid_ipv6_literal =="
accepts ha_valid_ipv6_literal ::1
accepts ha_valid_ipv6_literal 2001:db8::1
accepts ha_valid_ipv6_literal 2001:0db8:0000:0000:0000:0000:0000:0001
accepts ha_valid_ipv6_literal ::ffff:192.0.2.1
rejects ha_valid_ipv6_literal 192.0.2.1
rejects ha_valid_ipv6_literal 2001:db8::1::2
rejects ha_valid_ipv6_literal "gggg::1"
rejects ha_valid_ipv6_literal "::ffff:192.0.2.256"

echo "== ha_url_problem =="
url_is "" ""
url_is "http://loki.example.com" ""
url_is "https://loki.example.com/loki/api/v1/push" ""
url_is "https://loki.example.com:3100/push" ""
url_is "http://[2001:db8::1]:3100/push" ""
url_is "http://[fe80::1%25eth0]:3100" ""
url_is "https://example.com/path%20with%20escape" ""
url_is 'https://exa"mple.com' charset
url_is 'https://exa\mple.com' charset
url_is "https://exa mple.com" charset
url_is "ftp://example.com" invalid
url_is "example.com" invalid
url_is "https://example.com:0" invalid
url_is "https://example.com:70000" invalid
url_is "http://[2001:db8::1::2]:3100" invalid
url_is "http://[fe80::1%eth0]:3100" invalid
url_is "https://example.com/%zz" invalid
url_is "https://example.com/%2" invalid

echo "== ha_valid_river_string =="
accepts ha_valid_river_string ""
accepts ha_valid_river_string homeassistant
accepts ha_valid_river_string "user@example.com"
rejects ha_valid_river_string 'has"quote'
rejects ha_valid_river_string 'has\backslash'
rejects ha_valid_river_string "$(printf 'has\tcontrol')"

echo "== ha_valid_endpoint =="
accepts ha_valid_endpoint any
accepts ha_valid_endpoint none
accepts ha_valid_endpoint homeassistant:8123
accepts ha_valid_endpoint "*:8123"
accepts ha_valid_endpoint "homeassistant:*"
accepts ha_valid_endpoint "[2001:db8::1]:8123"
rejects ha_valid_endpoint homeassistant
rejects ha_valid_endpoint "homeassistant:8123/path"
rejects ha_valid_endpoint "homeassistant:0"
rejects ha_valid_endpoint "homeassistant:70000"
rejects ha_valid_endpoint ".leading:8123"
rejects ha_valid_endpoint "double..dot:8123"
rejects ha_valid_endpoint "2001:db8::1:8123"

echo
echo "== RESULTS: $((TESTS - FAILS))/${TESTS} checks passed =="
if [ "${FAILS}" -ne 0 ]; then
    echo "FAILED (${FAILS})"
    exit 1
fi
echo "OK"
