# Grafana Alloy for Home Assistant

Ship Home Assistant OS logs to a remote [Loki](https://grafana.com/oss/loki/) instance and host system metrics to a remote Prometheus instance using [Grafana Alloy](https://grafana.com/docs/alloy/latest/).

This add-on replaces the deprecated Promtail add-on, which is incompatible with modern HAOS versions (11+) due to systemd 252+ compact journal format changes.

It bundles a fixed Alloy release - the exact version is pinned in `alloy/Dockerfile`, and add-on
updates are what move it. Only `amd64` and `aarch64` are built, and the add-on is flagged
`stage: experimental`.

## Configuration

### Destinations

**At least one of `loki_url` (logs), `prometheus_url` (metrics) or `fleet_url` (Fleet
Management) must be set.** No single one is required on its own; with all three empty the
add-on refuses to start with `FATAL: set at least one of loki_url (logs), prometheus_url
(metrics) or fleet_url (Fleet Management).` Each destination generates its own pipelines
independently, so a logs-only, metrics-only or Fleet-only deployment is equally valid.

None of the three has a default, so a fresh install starts with all of them empty and you fill
in the ones you want. **Clearing a field disables that destination** - the add-on regenerates its
Alloy config on every start, so removing a URL removes the corresponding pipelines.

Options are validated when you save, before the add-on starts. A malformed URL, a duration
without a unit (`60` rather than `60s`), an empty instance name, or a `fleet_attributes` entry
that is not `key=value` is rejected in the UI rather than becoming a start-up failure.

### Logs

- **loki_url**: the full URL of your Loki push endpoint, e.g.
  `http://192.168.1.45:3100/loki/api/v1/push`. Leave empty to disable log shipping.
- **loki_username** / **loki_password**: optional HTTP basic auth for the Loki endpoint. For
  Grafana Cloud, username is your numeric Loki instance ID and password is an access-policy token.
  The password is passed to Alloy via an environment variable, not written into the config file.
  Set both or neither - a half-configured pair is rejected at startup.

### General

- **log_level**: Alloy's own log verbosity (`debug`, `info`, `warn`, `error`). Default: `info`.
  This controls what the add-on log shows, not which log lines get shipped - the journal is
  shipped in full regardless.
- **instance_name**: used both as the `instance` label on every metric and as the collector ID
  reported to Fleet Management. Default: `homeassistant`.
- **additional_config**: extra Alloy config blocks to append (advanced users).

## Metrics (host monitoring)

Set `prometheus_url` to a Prometheus/Mimir `remote_write` endpoint to ship host metrics
collected by Alloy's `unix` exporter (the node_exporter equivalent).

### Options

- **prometheus_url**: remote_write endpoint, e.g. `http://192.168.1.45:9090/api/v1/write`
  (self-hosted) or `https://prometheus-prod-XX.grafana.net/api/prom/push` (Grafana Cloud).
- **prometheus_username** / **prometheus_password**: optional HTTP basic auth. For Grafana
  Cloud, username is your numeric metrics instance ID and password is an access-policy token.
  The password is passed to Alloy via an environment variable, not written into the config file.
  Set both or neither - as with Loki, a half-configured pair is rejected at startup.
- **instance_name**: value of the `instance` label on every metric. Default: `homeassistant`.
- **metrics_scrape_interval**: how often to scrape. Default: `60s`. Passed to Alloy verbatim, so
  it takes any Go duration string (`30s`, `1m`, `5m`).

### What is collected

CPU, memory, disk I/O, load average, and network interface stats are **host-wide** — these
procfs counters are not container-isolated, and `host_network` exposes the real host interfaces.

Metrics are scraped under the job name `integrations/node_exporter`, so Grafana Cloud's
node_exporter integration dashboards work against them unmodified.

The storage-oriented collectors that cannot see anything useful from inside an add-on container
are switched off: `ipvs`, `btrfs`, `infiniband`, `xfs`, `zfs`, `nfs`, `nfsd` and `mdadm`.
Virtual and container network interfaces (`veth*`, `docker*`, `br-*`, `lo`, `cali*`, `hassio*`)
are excluded from the network stats, leaving the real host NICs.

### Filesystem caveat

HAOS add-ons cannot mount the host root filesystem, so whole-host `df` is not available.
Instead the add-on reports usage for the three volumes it maps read-only - `share`, `media` and
`backup` - which all live on the HAOS data partition. In practice that is the data-partition
fill level. Pseudo-filesystems and the container's own overlay rootfs are excluded, so they do
not show up as phantom mounts.

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

Journal entries are shipped to Loki with these labels:

| Label | Source | Always present |
|-------|--------|----------------|
| `job` | the static value `systemd-journal` | yes |
| `unit` | systemd unit name (`__journal__systemd_unit`) | when the entry has one |
| `hostname` | machine hostname (`__journal__hostname`) | yes |
| `syslog_identifier` | process identifier (`__journal_syslog_identifier`) | when the entry has one |
| `transport` | journal transport type (`__journal__transport`) | yes |
| `container_name` | Docker container name, i.e. the add-on (`__journal_container_name`) | container entries only |
| `level` | parsed out of the message text, see below | no |

`level` is **not** the systemd priority field. It is extracted with a regex that looks for
Home Assistant's own log format - a `HH:MM:SS` timestamp followed by a level word:

```
2026-03-18 19:02:30.204 INFO (MainThread) [homeassistant.setup] Setup of domain sensor took 0.1s
```

Recognised words are `DEBUG`, `INFO`, `NOTICE`, `WARNING`, `WARN`, `ERROR`, `CRITICAL` and
`FATAL`, and the label carries them **verbatim in upper case**. Lines that do not match that
shape - most kernel, Supervisor and third-party container output - get no `level` label at all,
so a Loki query filtering on `level` silently excludes them. Filter with `level=~".+"` if you
want to check which entries were parsed.

Blank and whitespace-only journal lines are dropped before they reach Loki.

### Journal path

The add-on reads `/var/log/journal` (the persistent journal) when that directory exists and is
non-empty, and falls back to `/run/log/journal` (the volatile journal) otherwise. Which one it
settled on is printed in the startup banner. On a host with no persistent journal, history is
limited to what survives in the volatile journal across a reboot - which is nothing.

## Debug UI

The Alloy debug UI is available at `http://<haos-ip>:12345` when the add-on is running. Use it to
inspect component health, view the pipeline DAG, and troubleshoot issues. The same server backs
the container health check (`/-/ready`), which Home Assistant surfaces as the add-on's health -
so with the **Watchdog** toggle on, it restarts the add-on if Alloy stops responding.

If Alloy exits on its own rather than hanging - most often because `additional_config` does not
parse - the add-on stops outright and reports the exit code, instead of quietly restart-looping.

The generated config is written to `/etc/alloy/config.alloy` and is safe to read - no secret is
ever interpolated into it. Alloy's own state, including the cached Fleet Management
configuration, lives under `/data/alloy`, which is excluded from Home Assistant backups: it is
rebuilt on start and the write-ahead log would otherwise grow your backups for no benefit.

## Advanced: Additional Config

The `additional_config` option lets you append raw Alloy config blocks. For example, to also
tail a log file from the `share` folder:

```
local.file_match "extra" { path_targets = [{__path__ = "/share/some-app/app.log"}] }
loki.source.file "extra" { targets = local.file_match.extra.targets forward_to = [loki.write.loki.receiver] }
```

Two constraints worth knowing before you write one:

- **Only mapped paths are visible.** The add-on maps `share`, `media` and `backup`, all
  read-only. Home Assistant's own configuration directory is **not** mapped, so
  `home-assistant.log` cannot be tailed from here.
- **Referenced components must exist.** `loki.write.loki.receiver` is only generated when
  `loki_url` is set, and `prometheus.remote_write.metrics.receiver` only when `prometheus_url`
  is - forwarding to one that was not generated is a config error.

This is injected as-is into the config file. Syntax errors will prevent Alloy from starting.

## Troubleshooting

- **Add-on refuses to start with `FATAL: set at least one of loki_url ...`**: all three
  destinations are empty. Set `loki_url`, `prometheus_url` or `fleet_url`.
- **No logs in Loki**: Check that `loki_url` is reachable from HAOS. Try `ping <loki-host>` from the SSH add-on.
- **No metrics in Prometheus**: metrics are generated only when `prometheus_url` is set - check
  the startup banner for the `Metrics ->` line. If it is absent, the option did not take effect.
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
- **Alloy exits complaining about `poll_frequency`**: the add-on passes `fleet_poll_frequency`
  through without checking it, and Alloy rejects anything below `10s`.
- **Log lines have no `level` label**: expected for anything that is not in Home Assistant's log
  format - see [Labels](#labels). Nothing is dropped, the label is simply absent.

## Support

Report issues at: https://github.com/BroTEK-Solutions/ha-addons/issues
