# Grafana Private Data Source Connect for Home Assistant

This Home Assistant App runs Grafana's PDC agent. It creates an outbound reverse
SSH tunnel so Grafana Cloud can query a private data source without publishing
that source to the internet. No inbound firewall rule or port forwarding is
required.

## Required configuration

In Grafana Cloud, open **Connections > Private data source connections >
Configuration details** and copy these values exactly:

- **Grafana Cloud PDC signing token** with `pdc-signing:write` scope;
- **Hosted Grafana ID**, a positive numeric instance ID; and
- **Cluster**, the lowercase cluster label shown by Grafana Cloud.

Do not derive the cluster from a region name. Grafana Cloud's configuration
details are authoritative.

```yaml
# Deliberately fake values
signing_token: "not-a-real-pdc-signing-token"
hosted_grafana_id: "123456789"
cluster: "example-cluster"
allowed_endpoints:
  - "homeassistant:8123"
log_level: info
```

The App needs outbound access to the PDC gateway on TCP 22 and the signing API
on HTTPS/TCP 443.

## Destination allowlist

`allowed_endpoints` restricts which `host:port` destinations Grafana can reach
through the tunnel. An empty list is **allow all reachable destinations**, not
deny all. Prefer explicit entries for known data sources.

OpenSSH accepts exact hosts, bracketed IPv6, wildcard hosts and wildcard ports,
for example:

```yaml
allowed_endpoints:
  - "homeassistant:8123"
  - "database.internal:5432"
  - "*.internal:443"
```

It does not accept CIDR networks. `any` and `none` may each be used as the only
list entry. The experimental Go SSH implementation accepts exact entries only;
it does not support wildcards or those sentinels.

The usual Home Assistant App network can reach LAN addresses, Home Assistant
Core at `homeassistant:8123`, and Supervisor-generated aliases for other Apps.
The target service must listen beyond its own `127.0.0.1` and should enforce its
normal authentication.

## Advanced optional fields

Home Assistant places fields without stored values behind **Show unused optional
configuration options**. That label is global Home Assistant UI text and cannot
be renamed per App. These PDC controls use the upstream pdc-agent 0.0.63 defaults
when omitted:

| Field | Upstream default | Meaning |
| --- | --- | --- |
| Connections | `1` | Parallel reverse SSH tunnels. Go SSH requires exactly one. |
| Use region endpoint format | off | Changes derived endpoint naming; enable only when Grafana instructs you. |
| Grafana domain | `grafana.net` | Base suffix used to derive API and gateway names. |
| API FQDN override | unset | Explicit signing API hostname; supersedes derivation. |
| Gateway FQDN override | unset | Explicit SSH gateway hostname; supersedes derivation. |
| Certificate expiry window | `5m` | How early certificate renewal starts. |
| Certificate check interval | `1m` | Renewal check frequency; `0` checks only at startup. |
| Maximum retries | `4` | Signing API retry limit. |
| Parse OpenSSH connection metrics | on | Parses verbose OpenSSH output into PDC Prometheus connection/forwarding metrics. |
| Use GoSSH | off | Experimental in-process SSH path with a narrower feature set. |
| Connection timeout | `1` second | Limit for each SSH connection attempt. |

`domain` is relevant only to automatically derived endpoints. If both explicit
FQDN overrides are supplied, those values take precedence and the domain and
region-format controls are effectively redundant. Keep all four unset unless
Grafana Cloud configuration details, Grafana support or a private deployment
gives you exact custom values.

`parse_metrics` does not scrape a remote service and does not decide whether the
metrics server runs. It reads the normal verbose OpenSSH process output and
turns supported connection events into Prometheus metrics. Disable it only when
diagnosing parsing overhead or unexpected agent output.

## Metrics and health

The PDC agent exposes unauthenticated Prometheus metrics at `/metrics` on
internal port 8090. No host port is assigned by default. Add a mapping in the
App's Network settings only when an external Prometheus server must scrape it,
and restrict network access to that mapped port.

The container health check probes this endpoint for liveness. A healthy result
means the process answers HTTP; it does not prove the tunnel is registered or a
data source is reachable. Use Grafana Cloud's PDC connection status and the App
log for readiness.

## Persistent identity and upgrades

The SSH private key and signed certificate live under `/data/ssh`. This location
persists across normal restarts and updates and is included in Home Assistant App
backups. Do not delete it as routine troubleshooting: deletion discards the
connector identity and forces registration with a new key.

Uncommon fields are deliberately omitted from stored defaults. The runtime
still supplies the matching pdc-agent defaults, so upgrading keeps existing
behavior while presenting a shorter main configuration form.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| PDC remains offline | Re-copy the token, Hosted Grafana ID and cluster; verify token scope and egress on TCP 22/443. |
| DNS lookup fails | Keep endpoint overrides unset unless exact values were supplied; check Home Assistant DNS and egress policy. |
| Tunnel is online but a data source fails | Verify the target is reachable from this App and exactly matches the allowlist. |
| Another App cannot be reached | Confirm its Supervisor-generated alias and that its service listens beyond loopback. |
| App is healthy but no tunnel appears | Health is liveness-only; inspect agent logs and Grafana Cloud's PDC status. |

See Grafana's current [PDC configuration
documentation](https://grafana.com/docs/grafana-cloud/observe-and-act/connect-externally-hosted/private-data-source-connect/configure-pdc/)
for Cloud-side setup.
