# Grafana Alloy for Home Assistant

Ship Home Assistant OS logs to a remote [Loki](https://grafana.com/oss/loki/) instance and host system metrics to a remote Prometheus instance using [Grafana Alloy](https://grafana.com/docs/alloy/latest/).

This add-on replaces the deprecated Promtail add-on, which is incompatible with modern HAOS versions (11+) due to systemd 252+ compact journal format changes.

## Configuration

### Required

- **loki_url**: The full URL to your Loki push endpoint (e.g., `http://192.168.1.45:3100/loki/api/v1/push`)

### Optional

- **loki_username** / **loki_password**: optional HTTP basic auth for the Loki endpoint. For
  Grafana Cloud, username is your numeric Loki instance ID and password is an access-policy token.
  The password is passed to Alloy via an environment variable, not written into the config file.
  Set both or neither - a half-configured pair is rejected at startup.
- **log_level**: Alloy log verbosity (`debug`, `info`, `warn`, `error`). Default: `info`
- **additional_config**: Extra Alloy config blocks to append (advanced users)

> **`loki_url` is optional.** Configure at least one of `loki_url` (logs) or
> `prometheus_url` (metrics). Leave `loki_url` empty for a metrics-only deployment.

## Metrics (host monitoring)

Set `prometheus_url` to a Prometheus/Mimir `remote_write` endpoint to ship host metrics
collected by Alloy's `unix` exporter (the node_exporter equivalent).

### Options

- **prometheus_url**: remote_write endpoint, e.g. `http://192.168.1.45:9090/api/v1/write`
  (self-hosted) or `https://prometheus-prod-XX.grafana.net/api/prom/push` (Grafana Cloud).
- **prometheus_username** / **prometheus_password**: optional HTTP basic auth. For Grafana
  Cloud, username is your numeric metrics instance ID and password is an access-policy token.
  The password is passed to Alloy via an environment variable, not written into the config file.
- **instance_name**: value of the `instance` label on every metric. Default: `homeassistant`.
- **metrics_scrape_interval**: how often to scrape. Default: `60s`.

### What is collected

CPU, memory, disk I/O, load average, and network interface stats are **host-wide** — these
procfs counters are not container-isolated, and `host_network` exposes the real host interfaces.

### Filesystem caveat

HAOS add-ons cannot mount the host root filesystem, so whole-host `df` is not available.
Instead the add-on maps the `share`, `media`, and `backup` volumes (which live on the HAOS
data partition) and reports their usage — effectively the data-partition fill level.

### Example: logs + self-hosted Prometheus

```yaml
loki_url: "http://192.168.1.45:3100/loki/api/v1/push"
prometheus_url: "http://192.168.1.45:9090/api/v1/write"
instance_name: home-assistant
metrics_scrape_interval: 60s
```

### Example: Grafana Cloud (metrics only)

```yaml
loki_url: ""
prometheus_url: "https://prometheus-prod-XX.grafana.net/api/prom/push"
prometheus_username: "123456"
prometheus_password: "glc_your_access_policy_token"
instance_name: home-assistant
```

### Example: Grafana Cloud (logs + metrics)

Each backend has its own instance ID; the same access-policy token can be used for both,
provided its scopes cover `logs:write` and `metrics:write`.

```yaml
loki_url: "https://logs-prod-XX.grafana.net/loki/api/v1/push"
loki_username: "654321"
loki_password: "glc_your_access_policy_token"
prometheus_url: "https://prometheus-prod-XX.grafana.net/api/prom/push"
prometheus_username: "123456"
prometheus_password: "glc_your_access_policy_token"
instance_name: home-assistant
```

### Credential handling

Passwords are never written into the generated Alloy config. The add-on exports them as
environment variables and the config references `sys.env("LOKI_PASSWORD")`,
`sys.env("PROMETHEUS_PASSWORD")` and `sys.env("FLEET_PASSWORD")`, so `/etc/alloy/config.alloy` and
the debug UI stay safe to share when troubleshooting. The startup banner prints the username but
never the password.

Note that Home Assistant stores add-on options (including `password`-typed ones) in plain text
in the add-on's `/data/options.json` and in backups - the `password` schema type only masks the
field in the UI.

## Fleet Management

Set `fleet_url` to have the add-on register with [Grafana Fleet
Management](https://grafana.com/docs/grafana-cloud/send-data/fleet-management/). Alloy polls the
endpoint and runs the configuration it receives **alongside** the local pipelines generated from
the options above, in a separate controller. Nothing you configure locally is replaced or
disabled by remote configuration.

The fetched configuration is cached in the add-on's data directory, so it survives a restart and
a temporary loss of connectivity to Grafana Cloud.

### Options

- **fleet_url**: the Fleet Management service URL, e.g.
  `https://fleet-management-prod-001.grafana.net`. Copy it from the API tab of the Fleet
  Management page in your Grafana Cloud stack rather than constructing it by hand - some regions
  use a longer, nested hostname.
- **fleet_username** / **fleet_password**: your numeric instance ID and an access token. The
  predefined `set:alloy-data-write` scope covers both reading remote configuration and writing
  telemetry. The token is passed to Alloy via an environment variable, not written into the
  config file. Set both or neither.
- **fleet_collector_name**: human-readable name for this collector in the Fleet Management UI,
  e.g. `Home Assistant`. Optional.
- **fleet_attributes**: comma-separated `key=value` pairs, e.g. `env=home,role=hass`. Fleet
  Management uses these to decide which pipelines this collector receives. Optional, but without
  them matching can only be done by collector ID.
- **fleet_poll_frequency**: how often to check for configuration updates. Default: `5m`. Alloy
  requires at least `10s`.

The collector ID is not a separate option - it is `instance_name`, so the collector in Fleet
Management and the `instance` label on your metrics carry the same name.

`fleet_url` counts as a destination on its own: an add-on configured with only Fleet Management
starts normally and runs whatever pipelines the remote configuration supplies.

### Example: Grafana Cloud with logs, metrics and Fleet Management

```yaml
loki_url: "https://logs-prod-XX.grafana.net/loki/api/v1/push"
loki_username: "654321"
loki_password: "glc_your_access_policy_token"
prometheus_url: "https://prometheus-prod-XX.grafana.net/api/prom/push"
prometheus_username: "123456"
prometheus_password: "glc_your_access_policy_token"
fleet_url: "https://fleet-management-prod-001.grafana.net"
fleet_username: "987654"
fleet_password: "glc_your_access_policy_token"
fleet_collector_name: "Home Assistant"
fleet_attributes: "env=home,role=hass"
instance_name: home-assistant
```

### Example: Fleet Management only

```yaml
loki_url: ""
prometheus_url: ""
fleet_url: "https://fleet-management-prod-001.grafana.net"
fleet_username: "987654"
fleet_password: "glc_your_access_policy_token"
instance_name: home-assistant
```

## Labels

All journal entries are shipped to Loki with these labels:

| Label | Source |
|-------|--------|
| `job` | `systemd-journal` (static) |
| `unit` | systemd unit name |
| `hostname` | machine hostname |
| `syslog_identifier` | process identifier |
| `transport` | journal transport type |
| `container_name` | Docker container name (for add-ons) |
| `level` | log priority (debug, info, warning, error, etc.) |

## Debug UI

The Alloy debug UI is available at `http://<haos-ip>:12345` when the add-on is running. Use it to inspect component health, view the pipeline DAG, and troubleshoot issues.

## Advanced: Additional Config

The `additional_config` option lets you append raw Alloy config blocks. For example, to also scrape a file:

```
local.file_match "extra" { path_targets = [{__path__ = "/config/home-assistant.log"}] }
loki.source.file "extra" { targets = local.file_match.extra.targets forward_to = [loki.write.loki.receiver] }
```

Note: This is injected as-is into the config file. Syntax errors will prevent Alloy from starting.

## Troubleshooting

- **No logs in Loki**: Check that `loki_url` is reachable from HAOS. Try `ping <loki-host>` from the SSH add-on.
- **`401 Unauthorized` / `authentication error` in the add-on log**: the endpoint requires basic
  auth. Set `loki_username` + `loki_password` (or the `prometheus_*` equivalents). On Grafana
  Cloud the username is the numeric instance ID from the stack's connection details, not your
  email address.
- **Add-on refuses to start with `FATAL: ..._username is set but ..._password is empty`**: basic
  auth needs both values; fill in the missing one or clear both.
- **Add-on crashes on start**: Check the add-on log for Alloy config errors. Set `log_level: debug` for verbose output.
- **"timestamp too old" in Loki**: Normal on first start. Alloy reads the full journal history; Loki rejects entries outside its retention window. Resolves in 1-2 minutes.
- **Add-on refuses to start with `FATAL: fleet_attributes entry '...' is not key=value`**: every
  comma-separated segment of `fleet_attributes` must contain an `=` with a non-empty key. Write
  `env=home,role=hass`, not `env home, role`.
- **Collector does not appear in Fleet Management**: check the add-on log for the `Fleet` banner
  line, confirm `fleet_url` matches the API tab exactly, and confirm the token carries the
  `set:alloy-data-write` scope. A rejected token shows as a `401` from the Fleet Management host in
  the add-on log.
- **Collector appears but receives no pipelines**: matching is driven by `fleet_attributes` and the
  collector ID (`instance_name`). Compare the `Fleet attributes:` banner line against the matchers
  on your Fleet Management pipelines.

## Support

Report issues at: https://github.com/rknightion/ha-addon-alloy/issues
