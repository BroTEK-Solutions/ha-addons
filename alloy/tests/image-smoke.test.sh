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

mkdir -p "${tmp}/data"
chmod 0700 "${tmp}/data"
printf '%s' '{"schema_version":2,"gcloud_rw_api_key":"stored-secret","alloy_additional_args":"--disable-reporting\n--server.http.memory-addr=alloy.test:12345"}' \
    >"${tmp}/data/settings.json"
# The generated helper expands these variables inside the container.
# shellcheck disable=SC2016
printf '%s\n' \
    '#!/bin/sh' \
    '[ -z "${FLEET_PASSWORD:-}" ] || exit 41' \
    '[ "${GCLOUD_RW_API_KEY:-}" = stored-secret ] || exit 42' \
    'case " $* " in *" --disable-reporting --server.http.memory-addr=alloy.test:12345 "*) ;; *) exit 43;; esac' \
    >"${tmp}/fake-alloy"
chmod +x "${tmp}/fake-alloy"
docker run --rm \
    --entrypoint /usr/lib/bashio/bashio \
    -e SETTINGS_FILE=/tmp/settings.json \
    -v "${tmp}/data/settings.json:/tmp/settings.json:ro" \
    -v "${tmp}/fake-alloy:/usr/bin/alloy:ro" \
    "${IMAGE}" \
    /etc/s6-overlay/s6-rc.d/alloy/run

printf '%s\n' 'logging {}' >"${tmp}/config.alloy"
docker run -d --name "${CONTAINER}" \
    --entrypoint bash \
    -e SUPERVISOR_TOKEN=smoke \
    -e INGRESS_SOURCE_IP=127.0.0.1 \
    -e SETTINGS_FILE=/data/settings.json \
    -v "${tmp}/data:/data" \
    -v "${tmp}/config.alloy:/tmp/config.alloy:ro" \
    "${IMAGE}" -c '
        /usr/bin/alloy run \
            --server.http.listen-addr=127.0.0.1:12345 \
            --server.http.ui-path-prefix=/alloy \
            --storage.path=/tmp/alloy \
            /tmp/config.alloy >/tmp/alloy.log 2>&1 &
        printf "%s" "$!" >/tmp/alloy.pid
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

valid_status="$(docker exec "${CONTAINER}" curl -sS -o /tmp/save-valid.json -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    --data '{"options":{"manual_config_enabled":true,"manual_config":"logging { level = \"info\" }"},"secrets":{}}' \
    http://127.0.0.1:8099/api/config)"
test "${valid_status}" = 200

invalid_status="$(docker exec "${CONTAINER}" curl -sS -o /tmp/save-invalid.json -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    --data '{"options":{"manual_config_enabled":true,"manual_config":"not valid alloy syntax"},"secrets":{}}' \
    http://127.0.0.1:8099/api/config)"
test "${invalid_status}" = 400
docker exec "${CONTAINER}" grep -qF 'logging { level = \"info\" }' /data/settings.json

attributes_status="$(docker exec "${CONTAINER}" curl -sS -o /tmp/save-attributes.json -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    --data '{"options":{"manual_config_enabled":false,"operation_mode":"fleet","fleet_url":"https://fleet.example","fleet_username":"123","fleet_attributes":"foo"},"secrets":{}}' \
    http://127.0.0.1:8099/api/config)"
test "${attributes_status}" = 400
docker exec "${CONTAINER}" grep -qF 'key=value' /tmp/save-attributes.json

arguments_status="$(docker exec "${CONTAINER}" curl -sS -o /tmp/save-arguments.json -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    --data '{"options":{"manual_config_enabled":true,"manual_config":"logging {}","alloy_additional_args":"--bogus"},"secrets":{}}' \
    http://127.0.0.1:8099/api/config)"
test "${arguments_status}" = 400
docker exec "${CONTAINER}" grep -qF 'unknown flag' /tmp/save-arguments.json

endpoint_status="$(docker exec "${CONTAINER}" curl -sS -o /tmp/save-endpoint.json -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    --data '{"options":{"manual_config_enabled":false,"operation_mode":"local","loki_url":"https://éxample.com/push"},"secrets":{}}' \
    http://127.0.0.1:8099/api/config)"
test "${endpoint_status}" = 400
docker exec "${CONTAINER}" grep -qF 'valid HTTP(S) URL' /tmp/save-endpoint.json

docker exec "${CONTAINER}" sh -c 'kill "$(cat /tmp/alloy.pid)"'
for _ in {1..10}; do
    if docker exec "${CONTAINER}" curl -fsS http://127.0.0.1:8099/healthz >/dev/null; then
        break
    fi
    sleep 1
done
docker exec "${CONTAINER}" curl -fsS http://127.0.0.1:8099/healthz >/dev/null
docker exec "${CONTAINER}" curl -fsS http://127.0.0.1:8099/ >/dev/null
proxy_status="$(docker exec "${CONTAINER}" curl -sS -o /dev/null -w '%{http_code}' \
    http://127.0.0.1:8099/alloy/)"
test "${proxy_status}" = 502

echo "PASS: compiled ingress validates candidates and remains available when Alloy stops"
