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

| Field | Option key | Upstream default | Meaning |
| --- | --- | --- | --- |
| Connections | `connections` | `1` | Parallel reverse SSH tunnels. Go SSH requires exactly one. |
| Use region endpoint format | `region_format` | off | Changes derived endpoint naming; enable only when Grafana instructs you. |
| Grafana domain | `domain` | `grafana.net` | Base suffix used to derive API and gateway names. |
| API FQDN override | `api_fqdn` | unset | Explicit signing API hostname; supersedes derivation. |
| Gateway FQDN override | `gateway_fqdn` | unset | Explicit SSH gateway hostname; supersedes derivation. |
| Certificate expiry window | `cert_expiry_window` | `5m` | How early certificate renewal starts. |
| Certificate check interval | `cert_check_expiry_period` | `1m` | Renewal check frequency; `0` checks only at startup. |
| Maximum retries | `retry_max` | `4` | Signing API retry limit. |
| Parse OpenSSH connection metrics | `parse_metrics` | on | Parses verbose OpenSSH output into PDC Prometheus connection/forwarding metrics. |
| Use GoSSH | `use_gossh` | off | Experimental in-process SSH path with a narrower feature set. |
| Connection timeout | `connect_timeout_seconds` | `1` second | Limit for each SSH connection attempt. |

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

### Failure handling

If the agent exits unexpectedly, or dies on any signal other than `SIGTERM`,
this App **stops the whole container** and preserves the agent's exit code.
That is deliberate: PDC has no other user-facing surface, so a silent restart
loop would look identical to a working tunnel from the Home Assistant side. A
stopped App is visible, and the Watchdog toggle then controls whether the
Supervisor restarts it.

This differs from the Grafana Alloy App, which keeps its container running when
Alloy exits so its configuration Web UI stays reachable for repair. Alloy has a
recovery surface worth preserving; PDC does not.

## Home Assistant entities over MQTT

If Home Assistant has an MQTT broker (the Mosquitto broker App is the usual
one), this App publishes its own health as entities using MQTT discovery. There
is nothing to configure and no credentials to copy: the broker details come from
the Supervisor, so rotating them does not strand a copy here. Without a broker
the App behaves exactly as before, and it keeps checking, so installing one
later needs no restart.

The point is to make the monitoring pipeline itself monitorable. These entities
let an automation notice that telemetry stopped - which is precisely the failure
that otherwise hides, because the thing that would have told you is the thing
that broke.

| Entity | Type | Meaning |
| --- | --- | --- |
| Agent responding | binary sensor | The PDC agent's metrics endpoint answers, so the process is alive. |

This is deliberately one entity. Whether the tunnel is *registered* is Grafana
Cloud's view, not something the local agent reports reliably, and the agent's
own metric names are not a documented contract. An entity that silently started
lying after an upstream rename would be worse than not having it. Continue to
use Grafana Cloud's PDC connection status for tunnel readiness.


Entities appear under a device named after the App. They report `unavailable`
when the App stops, through an MQTT last-will message, and an individual entity
reads `unknown` when its value cannot currently be observed. `unknown` means
this App could not measure the state - it is never a silent substitute for a
real "off".

## Persistent identity and upgrades

The SSH private key and signed certificate live under `/data/ssh`. This location
persists across normal restarts and updates. Do not delete it as routine
troubleshooting: deletion discards the connector identity and forces
registration with a new key.

`/data/ssh` is deliberately **excluded from Home Assistant backups**. A private
key does not belong in an archive that is often unencrypted and copied off the
device, and the material is reproducible: the signing token is an ordinary
option and is captured by the backup, so the agent mints a fresh keypair and
re-registers on its own.

After restoring a backup, expect the connector to be briefly offline while it
registers the new key. That first-boot delay is normal and needs no action. The
connector appears in Grafana Cloud under the same Hosted Grafana ID and cluster;
you do not need to re-copy the signing token or create a new PDC configuration.

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
