#!/usr/bin/env bash
# Smoke-test the compiled ingress service from a complete Alloy image.
set -euo pipefail

IMAGE="${1:?usage: image-smoke.test.sh IMAGE}"
CONTAINER="ha-alloy-ui-smoke-$$"
tmp="$(mktemp -d)"
cleanup() {
    docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
    rm -rf "${tmp}"
}
trap cleanup EXIT

printf '%s\n' 'logging {}' >"${tmp}/config.alloy"
docker run -d --name "${CONTAINER}" \
    --entrypoint bash \
    -e SUPERVISOR_TOKEN=smoke \
    -e INGRESS_SOURCE_IP=127.0.0.1 \
    -v "${tmp}/config.alloy:/tmp/config.alloy:ro" \
    "${IMAGE}" -c '
        /usr/bin/alloy run \
            --server.http.listen-addr=127.0.0.1:12345 \
            --server.http.ui-path-prefix=/alloy \
            --storage.path=/tmp/alloy \
            /tmp/config.alloy >/tmp/alloy.log 2>&1 &
        exec /usr/bin/alloy-ui
    ' >/dev/null

for _ in {1..10}; do
    if docker exec "${CONTAINER}" curl -fsS http://127.0.0.1:8099/ >"${tmp}/index.html"; then
        break
    fi
    sleep 1
done
grep -qF 'Grafana Alloy configuration' "${tmp}/index.html"

http_code=000
for _ in {1..15}; do
    http_code="$(docker exec "${CONTAINER}" curl -sS -o /tmp/alloy-ui.html -w '%{http_code}' \
        http://127.0.0.1:8099/alloy/)"
    test "${http_code}" = 200 && break
    sleep 1
done
docker cp "${CONTAINER}:/tmp/alloy-ui.html" "${tmp}/alloy.html"
test "${http_code}" = 200
grep -qF '<base href="./" />' "${tmp}/alloy.html"
if grep -qF '<base href="/alloy/" />' "${tmp}/alloy.html"; then
    echo "FAIL: Alloy HTML escaped the Home Assistant ingress base" >&2
    exit 1
fi

asset_path="$(sed -n 's|.*src="\./\([^"]*\)".*|\1|p' "${tmp}/alloy.html" | head -n 1)"
test -n "${asset_path}"
docker exec "${CONTAINER}" curl -fsS \
    "http://127.0.0.1:8099/alloy/${asset_path}" >/dev/null

echo "PASS: compiled Alloy ingress UI serves its form and proxied Alloy assets"
