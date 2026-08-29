#!/usr/bin/env bash
# Start the complete S6 image with a harmless PDC stand-in and prove the App's
# user, environment, argument, persistence and health wiring.
set -euo pipefail

APP_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${1:-local/ha-grafana-pdc:smoke}"
CONTAINER="ha-pdc-smoke-$$"

cleanup() {
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Match the Home Assistant AppArmor profile's least-privilege capability set.
#
# Docker's health monitor leaves roughly five seconds before its first probe no
# matter what --health-interval says; only --health-start-interval shortens that,
# and it applies solely inside --health-start-period. This image serves /metrics
# in well under a second, so the start interval is the whole of what this test
# waits for. Measured on CI: 6.0s to 1.5s for the same probe and the same checks.
docker run -d --name "$CONTAINER" \
    --cap-drop FOWNER \
    --health-start-period 10s --health-start-interval 250ms \
    --health-interval 1s --health-timeout 2s --health-retries 5 \
    --mount "type=bind,source=${APP_ROOT}/tests/fixtures/fake-pdc,target=/usr/bin/pdc,readonly" \
    --mount "type=bind,source=${APP_ROOT}/tests/fixtures/options.valid.json,target=/tmp/.bashio/addons.self.options.config.cache,readonly" \
    "$IMAGE" >/dev/null

# 150 polls at 0.2s keeps the same 30s ceiling the 30x1s loop had.
health=""
for _ in $(seq 1 150); do
    health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$CONTAINER")"
    if [[ "$health" == healthy ]]; then
        break
    fi
    if [[ "$(docker inspect --format '{{.State.Running}}' "$CONTAINER")" != true ]]; then
        docker logs "$CONTAINER" >&2
        echo "FAIL: smoke container exited before becoming healthy" >&2
        exit 1
    fi
    sleep 0.2
done

if [[ "$health" != healthy ]]; then
    docker inspect --format '{{json .State.Health}}' "$CONTAINER" >&2
    docker logs "$CONTAINER" >&2
    echo "FAIL: smoke container did not become healthy" >&2
    exit 1
fi

# Health only proves grafana-pdc is serving. The log assertion at the end of this
# file has to see every service's startup output, so wait for s6 to report all of
# them explicitly. The old probe stall used to supply that window by accident.
services_started() {
    local log service
    log="$(docker logs "$CONTAINER" 2>&1)"
    for service in init-grafana-pdc grafana-pdc grafana-pdc-ui grafana-pdc-reporter; do
        grep -qF "service ${service} successfully started" <<<"$log" || return 1
    done
}

for _ in $(seq 1 150); do
    if services_started; then
        break
    fi
    sleep 0.2
done

if ! services_started; then
    docker logs "$CONTAINER" >&2
    echo "FAIL: s6 did not report every Grafana PDC service started" >&2
    exit 1
fi

owner_mode="$(docker exec "$CONTAINER" stat -c '%u:%g %a' /data/ssh)"
[[ "$owner_mode" == "30000:30000 700" ]] || {
    echo "FAIL: /data/ssh is ${owner_mode}, expected 30000:30000 700" >&2
    exit 1
}

if docker exec "$CONTAINER" sh -c 'command -v tempio >/dev/null'; then
    echo "FAIL: unused inherited tempio binary is present" >&2
    exit 1
fi

docker exec "$CONTAINER" grep -qxF 123456789 /data/ssh/smoke-hosted-id
docker exec "$CONTAINER" grep -qxF example-cluster /data/ssh/smoke-cluster
docker exec "$CONTAINER" grep -qxF present /data/ssh/smoke-token-state
docker exec "$CONTAINER" grep -qxF -- '-ssh-key-file=/data/ssh/grafana_pdc' /data/ssh/smoke-argv
docker exec "$CONTAINER" grep -qxF -- '-ssh-flag=-o PermitRemoteOpen=homeassistant:8123' /data/ssh/smoke-argv

if docker exec "$CONTAINER" grep -qF SMOKE_TEST_TOKEN_NOT_REAL /data/ssh/smoke-argv; then
    echo "FAIL: signing token appeared in PDC argv" >&2
    exit 1
fi
if docker logs "$CONTAINER" 2>&1 | grep -qF SMOKE_TEST_TOKEN_NOT_REAL; then
    echo "FAIL: signing token appeared in App logs" >&2
    exit 1
fi

echo "PASS: complete S6 image is healthy with UID/GID 30000 and secret-safe PDC wiring"
