#!/usr/bin/env bash
# Runtime contract tests for the Grafana PDC app's S6 services.
# Usage: grafana_pdc/tests/service.test.sh
set -u

ADDON_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INIT="${ADDON_ROOT}/rootfs/etc/s6-overlay/s6-rc.d/init-grafana-pdc/run"
RUN="${ADDON_ROOT}/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/run"
FINISH="${ADDON_ROOT}/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/finish"
USER_CONTENT="${ADDON_ROOT}/rootfs/etc/s6-overlay/s6-rc.d/user/contents.d/grafana-pdc"
INIT_DEPENDENCY="${ADDON_ROOT}/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/dependencies.d/init-grafana-pdc"
BASHIO_BIN="${BASHIO_BIN:-${ADDON_ROOT}/tests/fixtures/bashio}"

FAILS=0
TESTS=0

fail() { echo "  x $1"; FAILS=$((FAILS + 1)); }
pass() { echo "  ok $1"; }
file_mode() {
    if [ "$(uname -s)" = Darwin ]; then
        stat -f '%Lp' "$1"
    else
        stat -c '%a' "$1"
    fi
}
file_owner() {
    if [ "$(uname -s)" = Darwin ]; then
        stat -f '%u:%g' "$1"
    else
        stat -c '%u:%g' "$1"
    fi
}
need_file() {
    if [ ! -x "$1" ]; then
        echo "FAIL: required service is missing or not executable: $1" >&2
        exit 1
    fi
}
need_path() {
    if [ ! -f "$1" ]; then
        echo "FAIL: required S6 declaration is missing: $1" >&2
        exit 1
    fi
}
contains() { grep -qF -- "$2" "$1"; }
absent() { ! grep -qF -- "$2" "$1"; }

need_file "${INIT}"
need_file "${RUN}"
need_file "${FINISH}"
need_path "${USER_CONTENT}"
need_path "${INIT_DEPENDENCY}"
need_file "${BASHIO_BIN}"

tmp=""
cleanup() { [ -z "${tmp}" ] || rm -rf "${tmp}"; }
trap cleanup EXIT

setup_case() {
    tmp="$(mktemp -d)"
    mkdir -p "${tmp}/cache" "${tmp}/bin" "${tmp}/data"
    chmod 0755 "${tmp}" "${tmp}/data"
    chmod 0700 "${tmp}/cache"
    cat >"${tmp}/bin/pdc" <<'EOF'
#!/usr/bin/env bash
printf '<%s>\n' "$@" >"${PDC_TEST_ARGV}"
env | sort >"${PDC_TEST_ENV}"
EOF
    cat >"${tmp}/bin/s6-setuidgid" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$1" >"${PDC_TEST_SETUIDGID}"
shift
exec "$@"
EOF
    cat >"${tmp}/bin/getent" <<'EOF'
#!/usr/bin/env bash
[[ "$1" == ahosts && "$2" == 2001:db8::10 ]]
EOF
    chmod +x "${tmp}/bin/pdc" "${tmp}/bin/s6-setuidgid" "${tmp}/bin/getent"
}

# $1 options JSON. Leaves RUN_RC, RUN_OUT, RUN_ARGV and RUN_ENV available.
run_service() {
    printf '%s' "$1" >"${tmp}/cache/addons.self.options.config.cache"
    RUN_ARGV="${tmp}/argv"
    RUN_ENV="${tmp}/env"
    RUN_SETUIDGID="${tmp}/setuidgid"
    RUN_OUT="$(
        CACHE_DIR="${tmp}/cache" \
        DATA_DIR="${tmp}/data" \
        PDC_BIN="${tmp}/bin/pdc" \
        S6_SETUIDGID="${tmp}/bin/s6-setuidgid" \
        GETENT_BIN="${tmp}/bin/getent" \
        PDC_TEST_ARGV="${RUN_ARGV}" \
        PDC_TEST_ENV="${RUN_ENV}" \
        PDC_TEST_SETUIDGID="${RUN_SETUIDGID}" \
        "${BASHIO_BIN}" "${RUN}" 2>&1
    )"
    RUN_RC=$?
}

run_init() {
    printf '%s' "$1" >"${tmp}/cache/addons.self.options.config.cache"
    INIT_OUT="$(
        CACHE_DIR="${tmp}/cache" DATA_DIR="${tmp}/data" \
            "${BASHIO_BIN}" "${INIT}" 2>&1
    )"
    INIT_RC=$?
}

expect_fatal() {
    local description="$1" options="$2" expected="$3"
    TESTS=$((TESTS + 1))
    setup_case
    run_service "${options}"
    if [ "${RUN_RC}" -eq 0 ]; then
        fail "${description}: expected a non-zero exit"
    elif ! grep -qF -- "${expected}" <<<"${RUN_OUT}"; then
        fail "${description}: error did not contain ${expected}"
    elif grep -qF -- 'SENTINEL_TOKEN' <<<"${RUN_OUT}"; then
        fail "${description}: signing token leaked in error output"
    else
        pass "${description}"
    fi
    cleanup; tmp=""
}

base_options() {
    cat <<'JSON'
{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","allowed_endpoints":[],"log_level":"info","connections":1,"region_format":false,"domain":"grafana.net","api_fqdn":"","gateway_fqdn":"","cert_expiry_window":"5m","cert_check_expiry_period":"1m","retry_max":4,"parse_metrics":true,"use_gossh":false,"connect_timeout_seconds":1}
JSON
}

echo "== required identity and scalar validation =="
expect_fatal "missing signing token" \
  '{"signing_token":"","hosted_grafana_id":"12345","cluster":"prod-uk"}' \
  "signing_token is required"
expect_fatal "hosted Grafana ID must be positive digits" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12x","cluster":"prod-uk"}' \
  "hosted_grafana_id must be positive digits"
expect_fatal "cluster rejects shell metacharacters" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod;id"}' \
  "cluster must be a DNS-safe label"
expect_fatal "cluster rejects uppercase outside Supervisor" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"Prod-uk"}' \
  "cluster must be a DNS-safe label"
expect_fatal "cluster rejects a dotted hostname outside Supervisor" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod.uk"}' \
  "cluster must be a DNS-safe label"
expect_fatal "domain rejects a URL" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","domain":"https://grafana.net"}' \
  "domain must be a hostname or domain"
expect_fatal "duration is parsed before pdc starts" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","cert_expiry_window":"later"}' \
  "cert_expiry_window must be a positive Go duration"
expect_fatal "zero certificate expiry duration is rejected" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","cert_expiry_window":"0s"}' \
  "cert_expiry_window must be a positive Go duration"
expect_fatal "connections have an upper bound" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","connections":51}' \
  "connections must be an integer from 1 to 50"
expect_fatal "retry maximum has a practical bound" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","retry_max":21}' \
  "retry_max must be an integer from 0 to 20"
expect_fatal "timeout has a practical bound" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","connect_timeout_seconds":601}' \
  "connect_timeout_seconds must be an integer from 1 to 600"

echo "== init owns persistent key directory =="
if [ "$(id -u)" -ne 0 ]; then
    echo "  SKIP init ownership check requires root; the HA base-image run covers it"
else
    setup_case
    run_init "$(base_options)"
    TESTS=$((TESTS + 1))
    if [ "${INIT_RC}" -ne 0 ]; then
        fail "init service exited ${INIT_RC}: ${INIT_OUT}"
    elif [ ! -d "${tmp}/data/ssh" ] || [ "$(file_mode "${tmp}/data/ssh")" != 700 ]; then
        fail "init service did not make /data/ssh mode 0700"
    elif [ "$(file_owner "${tmp}/data/ssh")" != "30000:30000" ]; then
        fail "init service did not assign UID/GID 30000"
    else
        pass "init service makes /data/ssh owner-only for PDC UID/GID"
    fi
    cleanup; tmp=""
fi

echo "== OpenSSH launches without putting the token in argv or logs =="
setup_case
run_service "$(base_options)"
TESTS=$((TESTS + 1))
if [ "${RUN_RC}" -ne 0 ]; then
    fail "OpenSSH defaults exited ${RUN_RC}: ${RUN_OUT}"
elif ! contains "${RUN_SETUIDGID}" '30000:30000'; then
    fail "OpenSSH defaults did not use s6-setuidgid 30000 30000"
elif ! contains "${RUN_ARGV}" '<-ssh-key-file=/data/ssh/grafana_pdc>'; then
    fail "OpenSSH defaults did not use the persistent fixed key path"
elif ! contains "${RUN_ENV}" 'GCLOUD_PDC_SIGNING_TOKEN=SENTINEL_TOKEN'; then
    fail "OpenSSH defaults did not export the signing token"
elif contains "${RUN_ARGV}" 'SENTINEL_TOKEN' || grep -qF 'SENTINEL_TOKEN' <<<"${RUN_OUT}"; then
    fail "OpenSSH defaults exposed signing token in argv or logs"
else
    pass "OpenSSH uses private environment token and fixed key path"
fi
cleanup; tmp=""

echo "== upstream startup-only certificate check period is retained =="
setup_case
run_service '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","cert_check_expiry_period":"0"}'
TESTS=$((TESTS + 1))
if [ "${RUN_RC}" -ne 0 ]; then
    fail "cert_check_expiry_period=0 exited ${RUN_RC}: ${RUN_OUT}"
else
    pass "cert_check_expiry_period=0 reaches pdc for startup-only checks"
fi
cleanup; tmp=""

echo "== OpenSSH structured restrictions and timeout =="
setup_case
run_service '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","allowed_endpoints":["db.internal:5432","[2001:db8::10]:443","*.internal:*"],"connect_timeout_seconds":12}'
TESTS=$((TESTS + 1))
if [ "${RUN_RC}" -ne 0 ]; then
    fail "OpenSSH restrictions exited ${RUN_RC}: ${RUN_OUT}"
elif ! contains "${RUN_ARGV}" '<-ssh-flag=-o PermitRemoteOpen=db.internal:5432 [2001:db8::10]:443 *.internal:*>'; then
    fail "OpenSSH restriction did not stay as one safe ssh-flag argument"
elif ! contains "${RUN_ARGV}" '<-ssh-flag=-o ConnectTimeout=12>'; then
    fail "OpenSSH timeout was not converted to a safe ssh argument"
else
    pass "OpenSSH renders endpoint restriction and timeout safely"
fi
cleanup; tmp=""

expect_fatal "CIDR endpoint is rejected" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","allowed_endpoints":["10.0.0.0/8:443"]}' \
  "allowed_endpoints entry is not a valid host:port"
expect_fatal "malformed IPv6 endpoint is rejected" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","allowed_endpoints":["[:::]:443"]}' \
  "allowed_endpoints entry is not a valid host:port"
expect_fatal "endpoint sentinels cannot mix" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","allowed_endpoints":["any","db.internal:5432"]}' \
  "allowed_endpoints sentinel any cannot be combined"

echo "== Go SSH uses native restrictions and enforces its one-connection contract =="
setup_case
run_service '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","allowed_endpoints":["db.internal:5432","[2001:db8::10]:443"],"use_gossh":true,"connect_timeout_seconds":12}'
TESTS=$((TESTS + 1))
if [ "${RUN_RC}" -ne 0 ]; then
    fail "Go SSH restrictions exited ${RUN_RC}: ${RUN_OUT}"
elif ! contains "${RUN_ARGV}" '<-use-gossh>'; then
    fail "Go SSH did not receive -use-gossh"
elif [ "$(grep -cF '<-permit-domains=' "${RUN_ARGV}")" -ne 2 ]; then
    fail "Go SSH did not receive one permit-domains argument per endpoint"
elif ! contains "${RUN_ARGV}" '<-connect-timeout=12s>'; then
    fail "Go SSH timeout is not a Go duration"
elif contains "${RUN_ARGV}" 'PermitRemoteOpen'; then
    fail "Go SSH received an OpenSSH-only PermitRemoteOpen argument"
else
    pass "Go SSH receives native restrictions and duration timeout"
fi
cleanup; tmp=""

expect_fatal "Go SSH rejects wildcard endpoints" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","allowed_endpoints":["*.internal:443"],"use_gossh":true}' \
  "Go SSH allowed_endpoints cannot use wildcards"
expect_fatal "Go SSH requires exactly one connection" \
  '{"signing_token":"SENTINEL_TOKEN","hosted_grafana_id":"12345","cluster":"prod-uk","connections":2,"use_gossh":true}' \
  "connections must be 1 when use_gossh is true"

echo "== finish distinguishes shutdown from an unexpected exit =="
setup_case
cat >"${tmp}/bin/halt" <<'EOF'
#!/usr/bin/env bash
printf 'halted\n' >"${PDC_TEST_HALT}"
EOF
chmod +x "${tmp}/bin/halt"
mkdir -p "${tmp}/results"
FINISH_OUT="$(
    CACHE_DIR="${tmp}/cache" PDC_TEST_HALT="${tmp}/halted" \
        S6_RESULTS_DIR="${tmp}/results" S6_HALT="${tmp}/bin/halt" \
        "${BASHIO_BIN}" "${FINISH}" 0 2>&1
)"
FINISH_RC=$?
TESTS=$((TESTS + 1))
if [ "${FINISH_RC}" -ne 0 ] || [ -e "${tmp}/halted" ] || [ -e "${tmp}/results/exitcode" ]; then
    fail "normal finish attempted to halt the App"
else
    pass "normal finish exits without halting the App"
fi
FINISH_OUT="$(
    CACHE_DIR="${tmp}/cache" PDC_TEST_HALT="${tmp}/halted" \
        S6_RESULTS_DIR="${tmp}/results" S6_HALT="${tmp}/bin/halt" \
        "${BASHIO_BIN}" "${FINISH}" 256 15 2>&1
)"
FINISH_RC=$?
TESTS=$((TESTS + 1))
if [ "${FINISH_RC}" -ne 0 ] || [ -e "${tmp}/halted" ] || [ -e "${tmp}/results/exitcode" ]; then
    fail "SIGTERM finish attempted to halt the App"
else
    pass "SIGTERM finish exits without halting the App"
fi
FINISH_OUT="$(
    CACHE_DIR="${tmp}/cache" PDC_TEST_HALT="${tmp}/halted" \
        S6_RESULTS_DIR="${tmp}/results" S6_HALT="${tmp}/bin/halt" \
        "${BASHIO_BIN}" "${FINISH}" 256 11 2>&1
)"
FINISH_RC=$?
TESTS=$((TESTS + 1))
if [ "${FINISH_RC}" -ne 0 ]; then
    fail "SIGSEGV finish did not hand off to halt"
elif [ ! -e "${tmp}/halted" ] || [ "$(cat "${tmp}/results/exitcode" 2>/dev/null)" != 139 ]; then
    fail "SIGSEGV finish did not preserve exit code 139 and halt the App"
elif ! grep -qF "signal 11" <<<"${FINISH_OUT}"; then
    fail "SIGSEGV finish did not report signal 11"
else
    pass "SIGSEGV finish preserves exit code 139 and halts the App"
fi
FINISH_OUT="$(
    CACHE_DIR="${tmp}/cache" PDC_TEST_HALT="${tmp}/halted" \
        S6_RESULTS_DIR="${tmp}/results" S6_HALT="${tmp}/bin/halt" \
        "${BASHIO_BIN}" "${FINISH}" 7 2>&1
)"
FINISH_RC=$?
TESTS=$((TESTS + 1))
if [ "${FINISH_RC}" -ne 0 ]; then
    fail "unexpected finish did not hand off to halt"
elif [ ! -e "${tmp}/halted" ] || [ "$(cat "${tmp}/results/exitcode" 2>/dev/null)" != 7 ]; then
    fail "unexpected finish did not preserve exit code 7 and halt the App"
elif grep -qF 'SENTINEL_TOKEN' <<<"${FINISH_OUT}"; then
    fail "finish output leaked a token"
else
    pass "unexpected finish preserves the exit code and halts the App"
fi
cleanup; tmp=""

echo
echo "== RESULTS: $((TESTS - FAILS))/${TESTS} checks passed =="
[ "${FAILS}" -eq 0 ] || exit 1
echo "ALL PASS"
