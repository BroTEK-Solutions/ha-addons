# {{APP_NAME}}

{{VARIANT_SENTENCE}}

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

The App runs as Grafana's non-root `sm` user. It requests `NET_RAW` for ICMP and traceroute, but not
host networking, host IPC, privileged-container access, or `SYS_ADMIN`. It retains Supervisor's
default AppArmor confinement. Chromium runs with the upstream no-sandbox flags inside the container;
do not compensate by granting it host IPC or broader Linux capabilities.

Anyone who can change checks assigned to this probe can cause requests from the Home Assistant App
network. Protect Grafana Cloud administrator access, and enable private k6 targets only when that
trust boundary is acceptable.

## Health and troubleshooting

The container health check calls the agent's local `/` endpoint, which proves the process and HTTP
server are alive. It deliberately does not call `/ready`: `/ready` returns 503 until the probe is
connected to Grafana Cloud, and an upstream outage must not create a restart loop.

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
