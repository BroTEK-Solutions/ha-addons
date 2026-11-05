# Grafana Synthetic Monitoring Probe with Browser Checks

This variant includes Chromium and can run every Synthetic Monitoring check type, including browser checks.

## Required configuration

Create the private probe in **Testing & synthetics > Synthetics > Probes**. Copy the one-time probe
authentication token and use Grafana's [regional server table](https://grafana.com/docs/grafana-cloud/observe-and-act/testing/synthetic-monitoring/set-up/set-up-private-probes/#probe-api-server-url)
to translate the stack's backend address into the gRPC API server.

```yaml
# Deliberately fake examples. Do not reuse them.
api_token: "not-a-real-probe-token"
api_server_address: "synthetic-monitoring-grpc-gb-south-1.grafana.net:443"
```

The address must include its port, normally `443`, and must not include `https://`. Each private
probe has a unique token. Create another probe in Grafana Cloud rather than sharing one token across
several running Apps.

The launcher passes the token through `SM_AGENT_API_TOKEN`, not the process argument list, and
redacts it from launcher errors. Home Assistant stores it as a password option. Treat App backups
and host-administrator access as credential-bearing boundaries.

## Variant selection and browser checks

The standard and browser Apps use Grafana's corresponding upstream images. Home Assistant cannot
switch an installed App's container image from a runtime option, so browser support is an install
choice:

- **Grafana Synthetic Monitoring Probe** omits Chromium and has the smaller image.
- **Grafana Synthetic Monitoring Probe with Browser Checks** includes Chromium and supports all
  check types.

For the browser variant, leave both **Disable scripted checks** and **Disable browser checks**
unchecked in the Grafana Cloud probe settings. Grafana recommends roughly 1 CPU and 1 GiB memory as
a browser-capable baseline, then additional capacity for concurrent checks. The browser variant
uses a memory-backed `/tmp`; Chromium is launched with Grafana's upstream
`disable-dev-shm-usage` setting. Supervisor still gives the App a private `/dev/shm`. The App does
not share the host IPC namespace and does not request `SYS_ADMIN`.

## Optional configuration

- `log_level`: `warn`, `info` (default), or `debug`. Debug can contain target details; use it only
  while diagnosing a problem.
- `allow_private_networks`: disables the agent's default k6 block on `10.0.0.0/8`. Leave this off
  unless scripted, MultiHTTP, or browser checks must reach that range. This does not create network
  access; it only removes the agent-side block.
- `disable_usage_reports`: opts out of Grafana's anonymous probe usage reporting.

Current agent releases support ad hoc and traceroute checks without feature switches. The App
requests `NET_RAW` for ICMP-based ping and traceroute checks.

## Networking and security

No inbound host port is published. The probe connects outbound to its regional Synthetic
Monitoring gRPC API, the Grafana Cloud Prometheus and Loki endpoints, and (when configured) the
regional secrets proxy. Checks also need outbound access to their targets. Script imports can need
access to hosts such as `jslib.k6.io`.

The launcher starts as root only long enough to read Supervisor-owned App options, then permanently
drops to Grafana's non-root `sm` user before starting the agent (and `tini` in the browser variant).
The App requests `NET_RAW` for ICMP and traceroute, but not
host networking, host IPC, privileged-container access, or `SYS_ADMIN`. It retains Supervisor's
default AppArmor confinement. Chromium runs with the upstream no-sandbox flags inside the container;
do not compensate by granting it host IPC or broader Linux capabilities.

Anyone who can change checks assigned to this probe can cause requests from the Home Assistant App
network. Protect Grafana Cloud administrator access, and enable private k6 targets only when that
trust boundary is acceptable.

## Home Assistant entities over MQTT

**MQTT health reporting** (`mqtt_enabled`) on the Home Assistant **Configuration**
tab controls whether this App publishes health entities through MQTT discovery.
It defaults to `true` for new and existing installations, including saved options
that do not yet contain the key. No broker credentials need to be entered: the
reporter uses the MQTT broker supplied by the Supervisor, usually the Mosquitto
broker App. Without a broker, the reporter waits and retries while the App keeps
running normally.

To turn reporting off, set `mqtt_enabled` to `false`, save and restart the App.
The reporter removes its retained discovery and state messages when it can reach
the broker, then idles without polling health. If the broker is unavailable,
cleanup is retried and the old entities may remain until it succeeds. To restore
reporting, enable the option and restart; discovery is published again.

These entities let Home Assistant automations detect application failures, such
as a probe losing its cloud connection while its process still runs. They depend
on Home Assistant and its MQTT broker being available. They cannot alert when the
whole Home Assistant host is down; use independent monitoring for that case.

| Entity | Type | Meaning |
| --- | --- | --- |
| Probe running | binary sensor | The agent's local HTTP server answers, so the process is alive. |
| Connected to Grafana Cloud | binary sensor | The agent's `/ready` endpoint answers. It returns 503 until the probe has actually connected, which makes this the entity worth alerting on. |


The device model names the probe variant: **Grafana Synthetic Monitoring Probe**
or **Grafana Synthetic Monitoring Probe with Browser Checks**. Device identifiers
and entity unique IDs stay unchanged when the model label is updated.

Entities appear under a device named after the App. They report `unavailable`
when the App stops, through an MQTT last-will message, and an individual entity
reads `unknown` when its value cannot currently be observed. `unknown` means
this App could not measure the state - it is never a silent substitute for a
real "off".

## Health and troubleshooting

Open **Probe status** from the App page for read-only operational diagnostics. The ingress page
shows local agent and Grafana Cloud connection state, performs a TCP reachability test to the
configured gRPC endpoint from the App, and summarizes publishing totals and running checks by safe
check type. It deliberately excludes the API token, check names and targets, tenant and probe IDs,
and arbitrary metric labels. The page is available only through Home Assistant ingress; it does not
publish a host port.

Its counters cover the current agent process and reset when the App restarts. They are diagnostic
totals, not a replacement for check results and dashboards in Grafana Cloud.

The container health check calls the agent's local `/` endpoint, which proves the process and HTTP
server are alive. It deliberately does not call `/ready`: `/ready` returns 503 until the probe is
connected to Grafana Cloud, and an upstream outage must not create a restart loop.

This App runs the agent as its only process rather than under a supervision tree, so if the agent
exits, the container exits with the same code and the Supervisor's Watchdog toggle decides whether
it restarts. The Grafana Alloy App behaves differently on purpose: it keeps running so its
configuration Web UI stays available for repair, which this probe has no equivalent of.

That choice has a consequence worth stating plainly: **the MQTT entities and the Probe status page
are best-effort side channels.** Both are started once before the agent takes over the container,
and nothing supervises them afterwards. If either exits, it stays down until the App is restarted -
the MQTT entities go `unavailable` and the status page stops answering, while the probe itself
carries on running checks normally. Restart the App to bring them back. A probe that is publishing
results to Grafana Cloud while its Home Assistant entities read `unavailable` is this case, not a
probe failure.

If the probe stays offline:

1. Verify the API address is the regional gRPC address with `:443`, not the backend URL and not an
   `https://` URL.
2. Create a fresh private probe and token if the token may have been copied incorrectly or reused.
3. Check outbound DNS, TCP 443, Prometheus, Loki, and secrets-proxy firewall access.
4. Temporarily select `debug`, restart the App, collect only the relevant log lines, then return to
   `info`.

If a browser check fails while other checks pass, confirm that this is the browser App, that browser
checks are enabled for the probe in Grafana Cloud, and that the host has enough memory. If k6 cannot
reach a `10.0.0.0/8` target, review `allow_private_networks` and the network route separately.
