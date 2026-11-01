#!/usr/bin/env bash
# Linux-only integration test. Missing confinement is a failure, never a skip.
set -euo pipefail

IMAGE="${1:?usage: apparmor.test.sh IMAGE (build with just test-alloy-image first)}"
APP_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [ "$(uname -s)" != Linux ] || [ "$(cat /sys/module/apparmor/parameters/enabled 2>/dev/null)" != Y ]; then
    echo "FAIL: this test requires a Linux Docker host with AppArmor enabled" >&2
    exit 1
fi
command -v apparmor_parser >/dev/null || {
    echo "FAIL: install the AppArmor parser with just setup-apparmor-tests" >&2
    exit 1
}
sudo -n true
docker info --format '{{json .SecurityOptions}}' | grep -q 'name=apparmor'
docker image inspect "${IMAGE}" >/dev/null
uname -r
apparmor_parser --version
docker info --format 'Docker {{.ServerVersion}} security={{json .SecurityOptions}}'

tmp="$(mktemp -d)"
profile="ha-alloy-test-$(basename "${tmp}")"
loaded=false
cleanup() {
    local rc=$?
    trap - EXIT
    if [ "${loaded}" = true ]; then
        sudo -n apparmor_parser -R "${tmp}/profile" || rc=1
    fi
    rm -rf "${tmp}"
    exit "${rc}"
}
trap cleanup EXIT
# Supervisor substitutes the top-level name too. Keep every policy rule intact.
sed "s/^profile alloy /profile ${profile} /" "${APP_ROOT}/apparmor.txt" >"${tmp}/profile"
sudo -n apparmor_parser -a -W "${tmp}/profile"
loaded=true
ALLOY_SMOKE_APPARMOR_PROFILE="${profile}" bash "${APP_ROOT}/tests/image-smoke.test.sh" "${IMAGE}"
