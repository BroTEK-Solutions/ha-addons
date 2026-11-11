#!/usr/bin/env bash
# Linux-only integration test. Missing confinement is a failure, never a skip.
# Boots the standard Alloy image under a copy of alloy/apparmor.txt and the
# host-processes variant under a copy of alloy_host/apparmor.txt.
set -euo pipefail

IMAGE="${1:?usage: apparmor.test.sh IMAGE VARIANT_IMAGE (build with just test-alloy-image first)}"
VARIANT_IMAGE="${2:?usage: apparmor.test.sh IMAGE VARIANT_IMAGE (build with just test-alloy-image first)}"
APP_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VARIANT_ROOT="${APP_ROOT}/../alloy_host"
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
docker image inspect "${VARIANT_IMAGE}" >/dev/null
uname -r
apparmor_parser --version
docker info --format 'Docker {{.ServerVersion}} security={{json .SecurityOptions}}'

tmp="$(mktemp -d)"
profile="ha-alloy-test-$(basename "${tmp}")"
variant_profile="ha-alloy-host-test-$(basename "${tmp}")"
loaded=()
cleanup() {
    local rc=$?
    trap - EXIT
    local file
    for file in "${loaded[@]}"; do
        sudo -n apparmor_parser -R "${file}" || rc=1
    done
    rm -rf "${tmp}"
    exit "${rc}"
}
trap cleanup EXIT
# Supervisor substitutes the top-level name too. Keep every policy rule intact.
sed "s/^profile alloy /profile ${profile} /" "${APP_ROOT}/apparmor.txt" >"${tmp}/profile"
sed "s/^profile alloy_host /profile ${variant_profile} /" "${VARIANT_ROOT}/apparmor.txt" >"${tmp}/variant-profile"
grep -q "^profile ${variant_profile} " "${tmp}/variant-profile"
sudo -n apparmor_parser -a -W "${tmp}/profile"
loaded+=("${tmp}/profile")
sudo -n apparmor_parser -a -W "${tmp}/variant-profile"
loaded+=("${tmp}/variant-profile")
ALLOY_SMOKE_APPARMOR_PROFILE="${profile}" bash "${APP_ROOT}/tests/image-smoke.test.sh" "${IMAGE}"
# The variant's host PID start path, under the variant's own policy.
ALLOY_SMOKE_APPARMOR_PROFILE="${variant_profile}" bash "${APP_ROOT}/tests/host-pid.test.sh" "${VARIANT_IMAGE}"
