# Grafana Private Data Source Connect for Home Assistant

This Home Assistant App runs the Grafana PDC agent. It creates an outbound reverse SSH tunnel from
Home Assistant to Grafana Cloud, allowing a Grafana Cloud data source to query services reachable
from the App. It does not publish the data source to the internet and does not need port forwarding
or an inbound firewall rule.

## Before you start

Create the PDC connection in Grafana Cloud. Then open **Connections > Private data source
connections > Configuration Details** and copy these three values exactly:

- `signing_token`: create or copy the PDC signing token. It must have the
  `pdc-signing:write` access-policy scope.
- `hosted_grafana_id`: the hosted Grafana ID shown there.
- `cluster`: the cluster value shown there.

Do not infer the cluster from your region, select it from a map, or try to discover it. The App
accepts it as free text because Grafana Cloud is the authority for the value. Likewise, leave
`region_format` at its default unless Configuration Details explicitly says to use the regional
format.

Grafana Cloud needs outbound internet access from Home Assistant to the PDC gateway on TCP **22**
and to the certificate-signing API on HTTPS/TCP **443**. Permit both through an egress firewall,
proxy policy, or upstream network control.

## First configuration

Use only the values copied from Grafana Cloud. The values below are deliberately fake and cannot
connect to a real stack.

```yaml
signing_token: "not-a-real-pdc-signing-token"
hosted_grafana_id: "123456789"
cluster: "example-cluster-copied-from-grafana"
```

After saving the configuration, start the App and confirm that Grafana Cloud shows the PDC
connection online before attaching it to a data source.

### Main options

| Option | Default | Meaning |
| --- | --- | --- |
| `signing_token` | required | PDC token with `pdc-signing:write`. Home Assistant masks it in the form, but treat App configuration and backups as sensitive. |
| `hosted_grafana_id` | required | Grafana Cloud hosted Grafana ID from Configuration Details. |
| `cluster` | required | Free-text PDC cluster copied from Configuration Details. |
| `allowed_endpoints` | `[]` | Optional destination allowlist. Entries are `host:port` values. An empty list permits broad access to every destination reachable from the App. |
| `log_level` | `info` | Agent logging verbosity. |
| `connections` | `1` | Parallel tunnel connections. Keep the default for the experimental Go SSH path. |

### Restricting destinations

An allowlist limits the tunnel's *destination* reachability; it does not create network routes or
make a service listen on the network. For example:

```yaml
allowed_endpoints:
  - "homeassistant:8123"
  - "metrics.example.invalid:9090"
  - "database.example.invalid:5432"
```

With the normal OpenSSH implementation, entries become `PermitRemoteOpen` rules. Exact and
wildcard hostnames are valid OpenSSH forms:

```text
-o PermitRemoteOpen=database.example.invalid:5432
-o PermitRemoteOpen=*.example.invalid:443
-o PermitRemoteOpen=database.example.invalid:*
```

OpenSSH also accepts `none` as the only list entry to deny every destination, or `any` as the only
entry to allow every destination. Do not combine either sentinel with another entry. These sentinels
and wildcard ports are not supported when `use_gossh` is enabled.

Do **not** use CIDR notation such as `192.0.2.0/24:5432`. `PermitRemoteOpen` is an exact
`host:port` or wildcard-hostname rule, not an IP-network firewall language.

`use_gossh` enables the experimental Go SSH implementation. It supports exact allowlist matches
only: do not use wildcard entries with it, and leave `connections: 1`. Use the normal OpenSSH path
unless you are deliberately testing the experimental implementation. The PDC 0.0.62 release
packaged here also predates the upstream `x/crypto` update for a known GoSSH reconnect CPU loop
([grafana/pdc-agent#320](https://github.com/grafana/pdc-agent/issues/320)); keep GoSSH disabled until
this App ships a PDC release containing that fix.

Leaving `allowed_endpoints` empty is allowed, but it is not a deny-all mode. It grants Grafana
Cloud access to the broad set of IP addresses, names, and ports that the App can reach. Prefer an
explicit allowlist for a small, known set of data sources.

## Home Assistant networking and data sources

The normal Home Assistant bridge network lets this App make outbound connections to the internet,
your LAN, Home Assistant Core, and other Home Assistant Apps. Examples of target addresses are:

- LAN service: `192.0.2.44:5432` (replace with your real LAN address).
- Home Assistant Core: `homeassistant:8123`.
- Another App: an App-specific Supervisor-generated alias such as
  `a0d7b954-influxdb:8086`.

Supervisor generates internal DNS names and aliases. The exact aliases depend on the installed App,
its slug, and your installation. Docker DNS names use hyphens where generated container names use
underscores, so treat the examples as patterns and verify the alias in the target App's documentation
or network settings.

The data source must listen beyond loopback. A service bound only to `127.0.0.1` inside another
container is reachable only in that container; bind it to its container/LAN interface (often
`0.0.0.0`) and apply the data source's own authentication and access controls.

## Advanced endpoint settings

These settings are for Grafana Cloud instructions or a deliberate private-connectivity design, not
for normal installation:

| Option | Default | Use |
| --- | --- | --- |
| `region_format` | `false` | Enable only if Grafana Cloud Configuration Details requires the regional PDC hostname format. |
| `domain` | `grafana.net` | Overrides the PDC domain when Grafana Cloud gives a non-default domain. |
| `api_fqdn` | empty | Explicit PDC certificate-signing API hostname. It takes precedence over generated cluster/domain addressing. |
| `gateway_fqdn` | empty | Explicit PDC SSH gateway hostname. It takes precedence over generated cluster/domain addressing. |
| `cert_expiry_window` | `5m` | How long before certificate expiry the agent renews it. |
| `cert_check_expiry_period` | `1m` | Certificate-expiry check interval. |
| `retry_max` | `4` | Maximum retries for signing API requests. |
| `parse_metrics` | `true` | Parses agent/OpenSSH metrics from agent output. |
| `connect_timeout_seconds` | `1` | SSH connection-attempt timeout. |

For a custom domain or private API/gateway endpoint, use the exact values supplied by Grafana Cloud.
Set both `api_fqdn` and `gateway_fqdn` when Grafana Cloud provides a private endpoint pair; do not
construct endpoint names from examples in this document.

## Metrics and health

The agent always serves unauthenticated Prometheus metrics at `/metrics` on internal port `8090`.
It is not published to the Home Assistant host by default. If you want an external Prometheus
server to scrape it, add a host-port mapping for port `8090` in the App's Supervisor network
settings and protect network access appropriately. There is no App configuration key for this;
the listener remains on internal port `8090`.

The container health check requests the local metrics endpoint. This is a **liveness** check only:
it proves that the process is answering HTTP, not that the PDC tunnel is connected, authorised, or
able to reach a data source. Check App logs and the PDC connection status in Grafana Cloud for
tunnel readiness.

## Persistent SSH identity and backups

The agent's SSH private key and signed certificate live at `/data/ssh/grafana_pdc` (with related
SSH files in `/data/ssh`). `/data` persists across App restarts and normal updates, so an update or
restart reconnects with the retained identity and requests a fresh certificate when required.

Include the App's data in your Home Assistant backup policy. Restoring a backup restores the
identity material. Do not delete `/data/ssh` as routine troubleshooting: doing so discards the
current identity and forces the agent to create a new key and obtain a new certificate.

## Updates, restarts, and reconnects

App updates restart the container. A restart intentionally drops any active tunnel connections,
then the agent reconnects and renews credentials when necessary. Grafana Cloud may briefly show the
connection offline while this happens. If a network interruption drops the tunnel, the agent
retries; use the App logs to distinguish a temporary reconnect from an authentication or DNS error.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| PDC never becomes online | Verify all three Configuration Details values, especially the token scope `pdc-signing:write`, hosted Grafana ID, and cluster. Check egress to the PDC gateway on TCP 22 and the signing API on TCP 443. |
| DNS lookup fails | Use the exact cluster/domain or explicit `api_fqdn`/`gateway_fqdn` values provided by Grafana Cloud. Check Home Assistant DNS and any egress DNS policy. Do not guess the regional hostname format. |
| Grafana can connect but the data source fails | Confirm the target host and port are reachable from this App, the service is not loopback-only, and an `allowed_endpoints` rule (if used) exactly matches the target. |
| Another Home Assistant App cannot be reached | Confirm its actual Supervisor-generated alias; aliases vary. Check that the target service listens on a non-loopback interface and exposes the intended port. |
| Allowlist behaves unexpectedly | Check each entry is a `host:port`. OpenSSH supports exact hosts and wildcard hostnames, but not CIDR. With `use_gossh: true`, use exact entries only and `connections: 1`. |
| App is healthy but Grafana reports no tunnel | The `/metrics` health probe is liveness only. Read the App logs for SSH/authentication errors and check the connection state in Grafana Cloud. |
| Configuration cannot be saved or the App exits immediately | Check YAML types and formats: quote tokens and hostnames, use a YAML list for `allowed_endpoints`, use `true`/`false` for booleans, integers for retry/connection/time-out settings, and duration strings such as `5m` and `1m`. |

For Grafana Cloud's current PDC setup and endpoint requirements, see the
[Grafana PDC documentation](https://grafana.com/docs/grafana-cloud/observe-and-act/connect-externally-hosted/private-data-source-connect/configure-pdc/).
