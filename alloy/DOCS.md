# Grafana Alloy for Home Assistant

This App runs [Grafana Alloy](https://grafana.com/docs/alloy/latest/) on Home
Assistant OS. It can either run pipelines supplied by Grafana Cloud Fleet
Management or build local pipelines for logs, metrics, traces and profiles.

Open **Web UI** to configure the App. The page shows only the controls relevant
to the selected operation mode, validates the complete candidate with Alloy,
and links to Alloy's component graph. Saving does not interrupt collection;
choose **Save and restart** when you are ready to apply the new configuration.

The Home Assistant **Configuration** tab deliberately contains only two
recovery controls: **Safe mode** and **Configuration UI log level**. All pipeline
settings live in the Web UI, so the same option is never presented in two
places.

## Choose one operation mode

### Fleet Management

Fleet mode registers Alloy with Grafana Cloud and runs the configuration
delivered by Fleet Management. Locally generated log, metric, trace and profile
pipelines are disabled in this mode, even if old local endpoint values remain
stored after an upgrade.

Copy all three required values from the **API** tab of Fleet Management:

- the complete regional Fleet Management URL;
- the numeric Fleet username/instance ID; and
- a Grafana Cloud read/write API key.

The key is used to authenticate remote configuration and is exported to remote
pipelines as `GCLOUD_RW_API_KEY`. A Fleet pipeline can therefore use the same
key for Loki, Prometheus/Mimir, Tempo or Pyroscope without embedding another
token in its configuration. Grant only the scopes those pipelines need.

The optional collector name and `key=value` attributes control how the collector
appears and which pipelines target it. By default, the App also publishes
`haos=true`, `journal_path`, `alloy_container_name`,
`alloy_legacy_container_name`, and `ha_addon_slug`, so one Fleet pipeline can
target portable HAOS settings rather than hard-coding an installation. The
current and legacy container-name attributes cover the `app_` and `addon_`
Supervisor naming schemes respectively. Disable **Expose Home Assistant Fleet
attributes** in the Fleet wizard if those built-in attributes are not wanted;
the App always retains its own `ha_addon_instance` targeting attribute. Alloy
checks for updates every **1m** by default; the minimum supported poll interval
is 10 seconds.

#### Optional Fleet starter pipeline

Fleet mode remains consume-only: the App never calls the Fleet write API. Once
Alloy is ready and healthy, the Web UI offers a separate starter-pipeline step.
Its telemetry destinations are used only to render one Fleet `Pipeline`
manifest; they do not configure local Alloy pipelines or persist as local
settings.

Save and restart with the Fleet settings. The UI waits until the saved
configuration is applied and Alloy is ready and healthy before offering the
starter step.

**No telemetry destination has to be configured.** Select the signals you want
and choose **Generate Fleet pipeline manifest**. Every endpoint or tenant ID
left blank is written as a `REPLACE-ME` placeholder. Those placeholder hostnames
sit under the reserved `.invalid` domain, so a manifest published unedited fails
to resolve rather than shipping telemetry somewhere unintended.

The manifest appears in an editable box. Replace each placeholder with the
endpoint and numeric tenant ID from your Grafana Cloud stack, then choose
**Download manifest** - the download always carries what is in the box, not
what the App rendered. Edits there are not saved and do not change this App's
own configuration.

Install and authenticate `gcx`, then create the pipeline from the downloaded
file:

```sh
brew install grafana/grafana/gcx
gcx login home-assistant --server https://YOUR-STACK.grafana.net --oauth
gcx config check
gcx fleet pipelines create -f home-assistant-fleet-pipeline.yaml
```

The token in that local `gcx` context needs
permission to create Fleet Management pipelines. It is separate from the App's
stored shared key: the stored key authenticates Alloy's Fleet polling and the
generated telemetry writes, while `gcx` performs the one-time Fleet mutation.

The manifest contains backend URLs and numeric tenant IDs, but no credentials.
Backend authentication remains a runtime `sys.env("GCLOUD_RW_API_KEY")`
reference. It is returned through the same ingress-restricted API as every other
configuration call; nothing about the starter pipeline is reachable without Home
Assistant ingress.

The generated pipeline name starts with `home-assistant-<instance-name>` and
ends with a deterministic hash suffix so distinct installation names cannot
collide after Fleet-safe normalization. It targets the collector attribute
`ha_addon_instance=<instance-name>`, which the App adds automatically. Give
each installation a distinct **Instance name** when more than one reports to
the same Fleet stack.

`gcx fleet pipelines create` is deliberately create-only. Once created, the
pipeline belongs to the operator: edit, disable, update or remove it in Fleet
Management or with `gcx`. Re-saving, disabling or uninstalling the App does not
reconcile or delete it. Generate a new starter manifest only when you explicitly
want another initial configuration to apply.

### Local configuration

Local mode generates Alloy configuration from this App's options. Configure at
least one destination:

- a Loki push URL for logs;
- a Prometheus/Mimir remote-write URL for metrics;
- a Tempo OTLP/HTTP URL for traces; or
- a Pyroscope URL for continuous profiles.

Self-hosted endpoints can omit credentials. Grafana Cloud endpoints normally
need the numeric tenant or instance ID as the username and an access-policy token
as the password. Set both halves of a credential pair or neither.

## Local logs

The App reads the HAOS systemd journal and can independently include:

- Home Assistant OS and Supervisor services;
- the Home Assistant Core container; and
- other Home Assistant App containers.

All three are enabled by default. **Excluded Apps** accepts comma-separated App
slugs such as `alloy,mqtt`; Alloy excludes its own container by default to avoid
feeding its logs back into itself. **Journal replay age** defaults to **24h** and
limits how far Alloy reads backwards when no saved journal position exists.

Logs retain useful journal labels including unit, transport and container name.
Home Assistant-formatted messages also receive a parsed `level` label. The
`hostname` and `instance` labels are overwritten - see
[Instance identity labels](#instance-identity-labels). An empty Loki URL disables
log shipping.

## Local metrics

All enabled metric sources share one Prometheus remote-write destination:

- **Host metrics** (on by default): CPU, memory, load, disk I/O and network data
  from Alloy's Unix exporter.
- **Home Assistant metrics** (off by default): entity and Core metrics from the
  Supervisor-authenticated Home Assistant Prometheus endpoint. Enable the
  [`prometheus` integration](https://www.home-assistant.io/integrations/prometheus/)
  in Home Assistant first.
- **Alloy self-monitoring metrics** (on by default): Alloy's own component and
  process metrics from the internal server on port 12345.

The scrape interval defaults to **60s**. The instance label defaults to
`homeassistant`. Both controls are under **Advanced & optional configuration
options** because their defaults suit most installations.

HAOS does not expose its host root filesystem to Apps. Filesystem usage therefore
covers the mapped `share`, `media` and `backup` paths rather than a host-wide
`df`. Host CPU, memory, disk-I/O and network counters remain available.

## Instance identity labels

Alloy runs as a container, so anything it reports about itself carries a
container hostname such as `a141124a-alloy`. Grafana's stock dashboards key their
**Instance** and **Hostname** controls off those labels, which makes one Home
Assistant install show up under a name nobody chose.

The generated configuration therefore pins identity to the **Instance name**
(default `homeassistant`) rather than to whatever the container is called:

- Every built-in metric scrape sets `instance` on its own targets.
- Host-exporter series additionally pass through a relabel step that rewrites
  `nodename` and `hostname` - but only where those labels already exist, so no
  series gains a label it did not carry. `node_uname_info`'s `nodename` is the
  one the Linux node dashboard renders as "Hostname".
- Logs get both `instance` and `hostname` set from the same value, replacing the
  journal's own hostname field.

The real kernel nodename is not preserved under another label. Give each install
a distinct **Instance name** when more than one reports to the same stack.

**Additional Alloy configuration is left alone.** The rewrite sits in the host
exporter's own pipeline, not on the shared remote-write endpoint, so extra
scrape targets you forward to `prometheus.remote_write.metrics.receiver` keep
their own `instance` labels. Set those labels yourself on any target you add.

This applies to metrics and logs. Traces carry OTLP resource attributes set by
the sending application, which the App does not rewrite.

## Local traces

Enable traces after entering a Tempo OTLP/HTTP destination. Alloy then accepts
OTLP on ports **4317** (gRPC) and **4318** (HTTP) and forwards spans to Tempo.
Both receivers are loopback-only by default. Enable **Allow OTLP clients on the
HAOS network** only when a trusted external client must send traces; that binds
both unauthenticated receivers to every HAOS interface, so restrict them with
your network firewall.

## Local profiles

Enter a Pyroscope destination and enable **Profile Alloy itself** to send CPU and
memory profiles for the Alloy process. This is self-profiling, not whole-host or
Home Assistant Core profiling.

## Advanced & optional configuration options

| Control | Default | Purpose |
| --- | --- | --- |
| Instance name | `homeassistant` | Metric instance label and Fleet collector identity. |
| Metrics scrape interval | `60s` | Frequency for local metric collection. |
| Fleet poll frequency | `1m` | Frequency for remote-configuration checks. |
| Disable Alloy usage reporting | on | Passes `--disable-reporting`; no anonymous usage report is sent. |
| Stability level | generally available | Allows preview components when deliberately raised. |
| Additional startup arguments | empty | Extra `--flag` or `--flag=value` Alloy CLI arguments, checked before saving. |
| Additional Alloy configuration | empty | River blocks appended to generated local configuration. |
| Full manual configuration override | off | Replaces every generated pipeline with the supplied complete Alloy configuration. |

The full manual override is the break-glass path for configurations the guided
form cannot express. Enable it under **Advanced & optional configuration
options** and paste the complete `config.alloy` contents. The Web UI runs the
same Alloy binary shipped in the App to validate the candidate before it writes
anything. Generated Fleet and Local pipeline settings are ignored while the
override is enabled, but stored secrets remain available through their
documented environment variables.

## Secrets and upgrades

Passwords and tokens are referenced from Alloy with `sys.env()` and are never
written into `/etc/alloy/config.alloy` or returned to the browser. Operational
settings are stored atomically in `/data/settings.json` with mode `0600` and are
included in Home Assistant backups. The password control masks display, not
storage.

On first version 2 startup, recognized settings are imported once from the old
Home Assistant App options. The former `fleet_password` value becomes the shared
`gcloud_rw_api_key`; unrelated or unknown keys are not imported. An old mixed
Fleet-and-local configuration is shown as **legacy hybrid** until Fleet or Local
is selected, and cannot be saved unchanged.

If Alloy cannot start, enable **Safe mode** in the Home Assistant Configuration
tab and restart the App. Safe mode starts Alloy with only logging configured,
while leaving the Web UI available so the stored operational configuration can
be repaired. Disable safe mode and restart after the candidate saves cleanly.

## Alloy component graph and network ports

Alloy's component graph is proxied through Home Assistant ingress at **Open Web
UI > Alloy status**. The internal Alloy server listens only on loopback port
**12345** for health checks and self-monitoring and is not exposed directly.
The configuration control plane accepts only Supervisor ingress traffic, with no
exceptions. Its separate health endpoint contains no configuration or restart
capability. App health follows this recovery UI rather than Alloy readiness. If Alloy exits,
S6 can restart it and the Web UI remains available; **Alloy status & component
graph** reports an upstream error until Alloy is ready again.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Fleet registration fails | Re-copy the regional URL, numeric username and shared write key from the Fleet API tab; check the token scopes. |
| Loki/Prometheus/Tempo/Pyroscope returns 401 | Confirm the numeric tenant ID and token are paired for that specific backend. |
| Home Assistant scrape returns 404 | Enable Home Assistant's `prometheus` integration. |
| No expected journal entries | Check the three log source switches, excluded App slugs and replay-age limit. |
| OTLP sender cannot connect | Enable traces and explicitly allow OTLP clients on the HAOS network. |
| Alloy will not start | Open the Web UI, correct the rejected setting or manual configuration, and use Safe mode from the Home Assistant Configuration tab if recovery is needed. |
