#!/usr/bin/env bash
# Boot the host-processes variant image (alloy_host) through its default
# entrypoint in both PID namespace shapes Supervisor can give it, and prove what
# each one sees. With a second image argument, also run it beside the standard
# Alloy App on one host network.
#
#   1. Host PID namespace (protection mode off): the entrypoint starts the s6-rc
#      services without s6-overlay's /init, the process exporter reports a
#      process that lives on the Docker host outside the container, and SIGTERM
#      shuts every service down cleanly. Under the shipped profile the App
#      cannot signal that host process.
#   2. SIGTERM while startup is stalled in init-alloy stops the App at once.
#   3. Own PID namespace (protection mode on): s6-overlay's /init runs as in the
#      standard App, the host process is invisible, every other pipeline starts,
#      and the log and the ingress status say why visibility is inactive.
#   4. Beside the standard App: both start in one network namespace without a port
#      collision, each with its own Alloy and default instance name.
#
# Set ALLOY_SMOKE_APPARMOR_PROFILE to run under a loaded copy of the variant's
# shipped policy (apparmor.test.sh does this).
set -euo pipefail

IMAGE="${1:?usage: host-pid.test.sh VARIANT_IMAGE [STANDARD_IMAGE]}"
STANDARD_IMAGE="${2:-}"
APPARMOR_PROFILE="${ALLOY_SMOKE_APPARMOR_PROFILE:-}"
UI_PORT=8098
ALLOY_PORT=12346
CONTAINER="ha-alloy-host-pid-$$"
STANDARD_CONTAINER="ha-alloy-standard-$$"
tmp="$(mktemp -d)"
probe_pid=""
lock_pid=""
cleanup() {
    local rc=$?
    trap - EXIT
    docker rm -f "${CONTAINER}" "${STANDARD_CONTAINER}" >/dev/null 2>&1 || true
    [ -z "${probe_pid}" ] || kill "${probe_pid}" 2>/dev/null || true
    [ -z "${lock_pid}" ] || kill "${lock_pid}" 2>/dev/null || true
    # The App writes root-owned state into each bind-mounted data directory.
    docker run --rm --entrypoint sh -v "${tmp}:/scratch" "${IMAGE}" \
        -c 'rm -rf /scratch/data-*' >/dev/null 2>&1 || true
    rm -rf "${tmp}"
    exit "${rc}"
}
trap cleanup EXIT
fail() {
    echo "FAIL: $*" >&2
    docker logs "${CONTAINER}" >&2 2>&1 || true
    exit 1
}

security_args=()
[ -z "${APPARMOR_PROFILE}" ] || security_args+=(--security-opt "apparmor=${APPARMOR_PROFILE}")

# A process on the Docker host, outside every container, with a name nothing
# else uses. A script is named after its file, which also works where sleep is a
# multi-call binary that refuses to run under another name.
probe_name=alloyhostprobe
printf '%s\n' '#!/bin/sh' 'sleep 600 &' "trap 'kill \$!; exit 0' TERM" 'wait' >"${tmp}/${probe_name}"
chmod +x "${tmp}/${probe_name}"
"${tmp}/${probe_name}" &
probe_pid=$!

mkdir -p "${tmp}/cache"
chmod 0700 "${tmp}/cache"
printf '%s' '{"safe_mode":false,"ui_log_level":"info","alloy_stability_level":"generally-available"}' \
    >"${tmp}/cache/addons.self.options.config.cache"

# A fresh data directory per boot, as root-owned files from the last one cannot
# be rewritten by an unprivileged runner. Remote write points at a closed
# loopback port: nothing leaves the host, and the exporter is read from Alloy's
# own component endpoint instead.
runs=0
new_data() {
    runs=$((runs + 1))
    data="${tmp}/data-${runs}"
    mkdir -p "${data}"
    chmod 0700 "${data}"
    printf '%s' '{"schema_version":2,"operation_mode":"local","prometheus_url":"http://127.0.0.1:9/api/v1/write","process_metrics":true}' \
        >"${data}/settings.json"
}

# start NAME IMAGE [docker args...]; uses the data directory from new_data.
start() {
    local name="$1" image="$2"
    shift 2
    docker rm -f "${name}" >/dev/null 2>&1 || true
    docker run -d --name "${name}" "$@" \
        -e SUPERVISOR_TOKEN=smoke \
        -e CACHE_DIR=/tmp/bashio-cache \
        -e INGRESS_SOURCE_IP=127.0.0.1 \
        -v "${tmp}/cache:/tmp/bashio-cache" \
        -v "${data}:/data" \
        "${security_args[@]}" \
        "${image}" >/dev/null
}

# wait_for_services NAME UI_PORT ALLOY_PORT
wait_for_services() {
    for _ in {1..40}; do
        if docker exec "$1" sh -c "
                for service in alloy alloy-ui alloy-reporter; do
                    [ \"\$(/command/s6-svstat -o up /run/service/\${service})\" = true ] || exit 1
                done
                curl -fsS http://127.0.0.1:$2/healthz >/dev/null
                curl -fsS http://127.0.0.1:$3/-/ready >/dev/null
            " >/dev/null 2>&1; then
            return 0
        fi
        [ "$(docker inspect --format '{{.State.Running}}' "$1")" = true ] || break
        sleep 1
    done
    docker logs "$1" >&2 2>&1 || true
    fail "the Alloy services in $1 did not all start"
}

check_confinement() {
    [ -n "${APPARMOR_PROFILE}" ] || return 0
    local alloy_pid label
    alloy_pid="$(docker exec "${CONTAINER}" /command/s6-svstat -o pid /run/service/alloy)"
    label="$(docker exec "${CONTAINER}" cat "/proc/${alloy_pid}/attr/current")"
    [ "${label}" = "${APPARMOR_PROFILE} (enforce)" ] || fail "Alloy runs under '${label}', not the enforcing test profile"
}

# The shipped profile accepts signals only from unconfined senders (the base
# abstraction's rule), which is what Home Assistant OS's runtime is. Where runc
# carries its own AppArmor label, as on current Ubuntu, `docker stop` cannot
# deliver SIGTERM to a confined container, so send it from an unconfined process.
stop_app() {
    if [ -n "${APPARMOR_PROFILE}" ]; then
        local pid
        pid="$(docker inspect --format '{{.State.Pid}}' "${CONTAINER}")"
        # Pid 0 means the container already exited; `kill 0` would hit this test.
        [ "${pid}" -gt 1 ] || fail "the App exited before it was stopped"
        sudo -n kill -TERM "${pid}"
        timeout 20 docker wait "${CONTAINER}" >/dev/null || fail "the App did not exit within 20s of SIGTERM"
    else
        docker stop -t 20 "${CONTAINER}" >/dev/null
    fi
}

process_metrics() {
    docker exec "${CONTAINER}" curl -fsS \
        "http://127.0.0.1:${ALLOY_PORT}/api/v0/component/prometheus.exporter.process.processes/metrics"
}

echo "== host PID namespace (protection mode off) =="
new_data
start "${CONTAINER}" "${IMAGE}" --pid=host
wait_for_services "${CONTAINER}" "${UI_PORT}" "${ALLOY_PORT}"
check_confinement
docker logs "${CONTAINER}" >"${tmp}/host.log" 2>&1
grep -qF '[alloy-init] Not PID 1' "${tmp}/host.log" || fail "the host PID start path did not run"
grep -qF 'Host process visibility is active: protection mode is off' "${tmp}/host.log" \
    || fail "init-alloy did not report active visibility"
docker exec "${CONTAINER}" sh -c "grep -qx '${probe_name}' /proc/${probe_pid}/comm" \
    || fail "the container cannot see host PID ${probe_pid}"
metrics=""
for _ in {1..20}; do
    metrics="$(process_metrics 2>/dev/null || true)"
    grep -qE "^namedprocess_namegroup_num_procs\{groupname=\"${probe_name}\"\} 1$" <<<"${metrics}" && break
    sleep 1
done
grep -qE "^namedprocess_namegroup_num_procs\{groupname=\"${probe_name}\"\} 1$" <<<"${metrics}" \
    || fail "prometheus.exporter.process did not report the host process group"
grep -qE '^namedprocess_namegroup_cpu_seconds_total\{groupname="alloy",mode="user"\}' <<<"${metrics}" \
    || fail "the exporter did not report the App's own Alloy group alongside the host"
if grep -q 'threadname=' <<<"${metrics}"; then fail "per-thread series are exported"; fi
if grep -q '^namedprocess_namegroup_memory_bytes{groupname="[^"]*",memtype="proportionalResident"}' <<<"${metrics}"; then
    fail "smaps-derived memory series are exported"
fi
status="$(docker exec "${CONTAINER}" curl -fsS "http://127.0.0.1:${UI_PORT}/api/status")"
grep -qF '"host_processes":{"variant":true,"active":true}' <<<"${status}" \
    || fail "ingress status does not report active visibility: ${status}"
echo "PASS: the exporter reports a process outside the container ($(grep -c '^namedprocess_namegroup_num_procs' <<<"${metrics}") groups)"

if [ -n "${APPARMOR_PROFILE}" ]; then
    # The App reads host processes; it must not be able to signal them. The
    # shell builtin, because the image has no standalone kill binary. A denial
    # must be the policy's, not a missing process.
    if docker exec "${CONTAINER}" sh -c "kill -TERM ${probe_pid}" >"${tmp}/kill.log" 2>&1; then
        fail "the confined App delivered SIGTERM to a host process"
    fi
    grep -qF 'Permission denied' "${tmp}/kill.log" || fail "signalling the host probe failed for another reason: $(cat "${tmp}/kill.log")"
    kill -0 "${probe_pid}" 2>/dev/null || fail "the host probe died"
    echo "PASS: the shipped profile stops the App signalling a host process"
fi

# Clean shutdown. Record the container's processes, stop it the way Supervisor
# does, and check every service ran its finish path and nothing outlived it.
mapfile -t container_pids < <(docker top "${CONTAINER}" -eo pid | tail -n +2)
[ "${#container_pids[@]}" -gt 3 ] || fail "could not list the container's processes"
stop_started=${SECONDS}
stop_app
stop_seconds=$((SECONDS - stop_started))
exit_code="$(docker inspect --format '{{.State.ExitCode}}' "${CONTAINER}")"
docker logs "${CONTAINER}" >"${tmp}/stop.log" 2>&1
[ "${exit_code}" = 0 ] || fail "SIGTERM left exit code ${exit_code}"
[ "${stop_seconds}" -lt 20 ] || fail "shutdown took ${stop_seconds}s and was killed at the stop timeout"
grep -qF '[alloy-init] Stopping services.' "${tmp}/stop.log" || fail "the entrypoint did not handle SIGTERM"
grep -qF 'service alloy successfully stopped' "${tmp}/stop.log" || fail "s6-rc did not stop Alloy"
grep -qF 'Alloy stopped.' "${tmp}/stop.log" || fail "Alloy's finish script did not report a clean stop"
for pid in "${container_pids[@]}"; do
    if kill -0 "${pid}" 2>/dev/null && [ "${pid}" != "${probe_pid}" ]; then
        fail "process ${pid} from the container outlived it"
    fi
done
echo "PASS: SIGTERM stops every service through s6-rc in ${stop_seconds}s with exit code 0"

echo "== SIGTERM during a stalled startup =="
# init-alloy takes the settings lock before generating the config. Holding it
# from the host stalls startup inside the oneshot, the way a hung init would.
new_data
# The lock file is root-owned, as the App would leave it: the shipped profile
# grants no CAP_FOWNER, so init-alloy could not chmod a runner-owned file. A
# read-only descriptor is enough to hold the lock. One process holds it and is
# the one killed, so nothing outlives it.
docker run --rm --entrypoint sh -v "${data}:/data" "${IMAGE}" \
    -c ': >/data/settings.json.lock && chmod 0644 /data/settings.json.lock'
(flock -x 9 && exec sleep 600) 9<"${data}/settings.json.lock" >/dev/null 2>&1 &
lock_pid=$!
start "${CONTAINER}" "${IMAGE}" --pid=host
stalled=false
for _ in {1..30}; do
    if docker logs "${CONTAINER}" 2>&1 | grep -qF 'service init-alloy: starting'; then
        stalled=true
        break
    fi
    sleep 1
done
"${stalled}" || fail "startup never reached init-alloy"
sleep 2
if docker logs "${CONTAINER}" 2>&1 | grep -qF 'service init-alloy successfully started'; then
    fail "init-alloy finished although the settings lock was held"
fi
stop_started=${SECONDS}
stop_app
stop_seconds=$((SECONDS - stop_started))
kill "${lock_pid}" 2>/dev/null || true
lock_pid=""
exit_code="$(docker inspect --format '{{.State.ExitCode}}' "${CONTAINER}")"
[ "${stop_seconds}" -lt 10 ] || fail "a stalled startup delayed shutdown for ${stop_seconds}s"
[ "${exit_code}" = 0 ] || fail "SIGTERM during startup left exit code ${exit_code}"
docker logs "${CONTAINER}" 2>&1 | grep -qF '[alloy-init] Stopping services.' \
    || fail "the entrypoint did not handle SIGTERM during startup"
echo "PASS: SIGTERM during a stalled startup stops the App in ${stop_seconds}s with exit code 0"

echo "== own PID namespace (protection mode on) =="
new_data
start "${CONTAINER}" "${IMAGE}"
wait_for_services "${CONTAINER}" "${UI_PORT}" "${ALLOY_PORT}"
check_confinement
docker logs "${CONTAINER}" >"${tmp}/own.log" 2>&1
[ "$(docker exec "${CONTAINER}" cat /proc/1/comm)" = s6-svscan ] || fail "s6-overlay is not PID 1"
if grep -qF '[alloy-init]' "${tmp}/own.log"; then fail "the host PID start path ran in an own namespace"; fi
grep -qF 'Host process visibility is not active: protection mode is on' "${tmp}/own.log" \
    || fail "the log does not say visibility is inactive and why"
if docker exec "${CONTAINER}" sh -c "grep -l '^${probe_name}\$' /proc/[0-9]*/comm" >/dev/null 2>&1; then
    fail "an own PID namespace can see the host process"
fi
docker exec "${CONTAINER}" grep -qF 'prometheus.exporter.unix "host"' /etc/alloy/config.alloy \
    || fail "the existing host metrics pipeline is missing"
if docker exec "${CONTAINER}" grep -qF 'prometheus.exporter.process' /etc/alloy/config.alloy; then
    fail "a process exporter was generated without host process visibility"
fi
status="$(docker exec "${CONTAINER}" curl -fsS "http://127.0.0.1:${UI_PORT}/api/status")"
grep -qF '"host_processes":{"variant":true,"active":false}' <<<"${status}" \
    || fail "ingress status does not report inactive visibility: ${status}"
sleep 5
[ "$(docker inspect --format '{{.State.Running}} {{.RestartCount}}' "${CONTAINER}")" = "true 0" ] \
    || fail "the App did not stay up"
echo "PASS: under protection mode the variant starts through s6-overlay and explains inactive visibility"

[ -n "${STANDARD_IMAGE}" ] || exit 0

echo "== beside the standard Alloy App in one network namespace =="
docker rm -f "${CONTAINER}" >/dev/null
security_args=()
# Home Assistant gives both Apps the host network. Sharing one container's
# network namespace reproduces that, without depending on which ports the test
# runner's own host already uses.
new_data
start "${STANDARD_CONTAINER}" "${STANDARD_IMAGE}"
new_data
start "${CONTAINER}" "${IMAGE}" --network "container:${STANDARD_CONTAINER}" --pid=host
wait_for_services "${STANDARD_CONTAINER}" 8099 12345
wait_for_services "${CONTAINER}" "${UI_PORT}" "${ALLOY_PORT}"
for name in "${STANDARD_CONTAINER}" "${CONTAINER}"; do
    if docker logs "${name}" 2>&1 | grep -qi 'address already in use'; then
        docker logs "${name}" >&2 2>&1
        fail "${name} hit a port collision"
    fi
done
docker exec "${STANDARD_CONTAINER}" grep -qF 'replacement  = "homeassistant"' /etc/alloy/config.alloy \
    || fail "the standard App lost its default instance name"
docker exec "${CONTAINER}" grep -qF 'replacement  = "homeassistant-host"' /etc/alloy/config.alloy \
    || fail "the variant does not default to its own instance name"
# Each port serves its own Alloy: only the variant's runs the process exporter.
exporter_path=/api/v0/component/prometheus.exporter.process.processes/metrics
docker exec "${CONTAINER}" curl -fsS -o /dev/null "http://127.0.0.1:${ALLOY_PORT}${exporter_path}" \
    || fail "the variant's Alloy is not the one on ${ALLOY_PORT}"
if docker exec "${STANDARD_CONTAINER}" curl -fsS -o /dev/null "http://127.0.0.1:12345${exporter_path}"; then
    fail "the standard App's port serves the variant's Alloy"
fi
std_status="$(docker exec "${STANDARD_CONTAINER}" curl -fsS http://127.0.0.1:8099/api/status)"
grep -qF '"host_processes":{"variant":false,"active":false}' <<<"${std_status}" \
    || fail "the standard App reports host process state: ${std_status}"
echo "PASS: both Apps run on one host network with separate ports, Alloy instances and instance names"
