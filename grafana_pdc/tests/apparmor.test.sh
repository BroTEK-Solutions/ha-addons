#!/usr/bin/env bash
# Linux-only: boot the real agent and companions under the shipped policy.
set -euo pipefail
IMAGE="${1:?usage: apparmor.test.sh IMAGE (build with just test-pdc-image first)}"
APP_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ "$(uname -s)" != Linux || "$(cat /sys/module/apparmor/parameters/enabled 2>/dev/null)" != Y ]]; then
    echo 'FAIL: Linux Docker host with AppArmor enabled required' >&2
    exit 1
fi
command -v apparmor_parser >/dev/null
sudo -n true
docker info --format '{{json .SecurityOptions}}' | grep -q 'name=apparmor'
arch="$(docker image inspect --format '{{.Architecture}}' "$IMAGE")"
tmp="$(mktemp -d)"
profile="ha-pdc-$(basename "$tmp")_grafana_pdc"
container="ha-pdc-apparmor-$$"
loaded=false
cleanup() {
    local rc=$?
    trap - EXIT
    if [[ "$rc" != 0 ]]; then docker logs "$container" >&2 2>/dev/null || true; fi
    docker rm -f "$container" >/dev/null 2>&1 || true
    if [[ "$loaded" == true ]]; then sudo -n apparmor_parser -R "$tmp/profile" || rc=1; fi
    rm -rf "$tmp"
    exit "$rc"
}
trap cleanup EXIT
sed "s/^profile grafana_pdc /profile ${profile} /" "$APP_ROOT/apparmor.txt" >"$tmp/profile"
sudo -n apparmor_parser -a -W "$tmp/profile"
loaded=true
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -o "$tmp/fixture" "$APP_ROOT/tests/fixtures/apparmor-services.go"
# Disposable keys and fake credentials only. No external network is available.
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -keyout "$tmp/tls.key" -out "$tmp/tls.crt" \
    -subj '/CN=api.example.test' -addext 'subjectAltName=DNS:api.example.test' >/dev/null 2>&1
ssh-keygen -q -t ed25519 -N '' -f "$tmp/ssh-ca"
chmod 755 "$tmp"
chmod 644 "$tmp/tls.key" "$tmp/ssh-ca"
jq '.api_fqdn="api.example.test" | .gateway_fqdn="gateway.example.test"' \
    "$APP_ROOT/tests/fixtures/options.valid.json" >"$tmp/options.json"
docker run -d --name "$container" --network none --cap-drop FOWNER \
    --security-opt "apparmor=$profile" \
    --add-host api.example.test:127.0.0.1 --add-host gateway.example.test:127.0.0.1 \
    -e SUPERVISOR_TOKEN=apparmor-test -e SUPERVISOR_URL=http://127.0.0.1:18080 \
    -e INGRESS_SOURCE_IP=127.0.0.1 -e SSL_CERT_FILE=/etc/ssl/certs/pdc-test-ca.pem \
    --mount "type=bind,source=$tmp,target=/tmp/pdc-fixture,readonly" \
    --mount "type=bind,source=$tmp/fixture,target=/usr/bin/ha-pdc-fixture,readonly" \
    --mount "type=bind,source=$tmp/tls.crt,target=/etc/ssl/certs/pdc-test-ca.pem,readonly" \
    --mount "type=bind,source=$tmp/options.json,target=/tmp/.bashio/addons.self.options.config.cache,readonly" \
    --mount "type=bind,source=$tmp/options.json,target=/data/options.json,readonly" \
    --entrypoint /bin/sh "$IMAGE" -c '/usr/bin/ha-pdc-fixture & exec /init' >/dev/null
ready=false
for _ in $(seq 1 150); do
    if docker exec "$container" sh -c 'curl -fsS http://127.0.0.1:8099/api/status | jq -e .agent_responding >/dev/null' 2>/dev/null; then
        ready=true
        break
    fi
    sleep 0.2
done
[[ "$ready" == true ]] || { echo 'FAIL: real PDC/UI did not become ready under confinement' >&2; exit 1; }
[[ "$(docker exec "$container" cat /proc/1/attr/current)" == "$profile (enforce)" ]]
for service in grafana-pdc grafana-pdc-ui grafana-pdc-reporter; do
    [[ "$(docker exec "$container" s6-svstat -o up "/run/service/$service")" == true ]]
    pid="$(docker exec "$container" s6-svstat -o pid "/run/service/$service")"
    expected="$profile (enforce)"
    if [[ "$service" == grafana-pdc ]]; then expected="$profile//pdc (enforce)"; fi
    [[ "$(docker exec "$container" cat "/proc/$pid/attr/current")" == "$expected" ]]
done
logs="$(docker logs "$container" 2>&1)"
grep -qF 'fixture: signed PDC public key' <<<"$logs"
grep -qF 'fixture: reporter requested MQTT service' <<<"$logs"
docker exec "$container" s6-setuidgid 30000:30000 test -s /data/ssh/grafana_pdc-cert.pub
docker exec "$container" s6-setuidgid 30000:30000 touch /data/ssh/ci-allowed-write
docker exec "$container" touch /tmp/ci-allowed-write /run/ci-allowed-write
kernel_log="$(sudo -n dmesg)"
if grep -F "$profile" <<<"$kernel_log" | grep -F 'apparmor="DENIED"'; then
    echo 'FAIL: unexpected AppArmor denial during startup' >&2
    exit 1
fi
# Paired control proves filesystem ownership/mount flags do not cause the denial.
docker run --rm --network none --security-opt apparmor=unconfined --entrypoint /bin/sh "$IMAGE" \
    -c 'mkdir -p /data && touch /etc/s6-overlay/ci-denied-write /data/ci-denied-write'
for target in /etc/s6-overlay/ci-denied-write /data/ci-denied-write; do
    if docker exec "$container" touch "$target" >"$tmp/denial" 2>&1; then
        echo "FAIL: policy allowed write to $target" >&2; exit 1
    fi
    grep -q 'Permission denied' "$tmp/denial"
    docker exec "$container" test ! -e "$target"
done
echo 'PASS: real PDC, UI and reporter run confined; certificate signing and allowed/denied writes verified'
echo 'Scope: Ubuntu/Linux AppArmor startup only; real HAOS and a connected Grafana tunnel require separate proof'
