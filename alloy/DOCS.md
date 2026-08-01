# Grafana Alloy for Home Assistant

This App runs [Grafana Alloy](https://grafana.com/docs/alloy/latest/) on Home
Assistant OS. It can either run pipelines supplied by Grafana Cloud Fleet
Management or build local pipelines for logs, metrics, traces and profiles.

Open **Web UI** to configure the App. The page shows only the controls relevant
to the selected operation mode, saves through the Supervisor API, and links to
Alloy's component graph. Saving does not interrupt collection; choose **Save and
restart** when you are ready to apply the new configuration.

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
appears and which pipelines target it. Alloy checks for updates every **1m** by
default; the minimum supported poll interval is 10 seconds.

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

Logs retain useful journal labels including unit, hostname, transport and
container name. Home Assistant-formatted messages also receive a parsed `level`
label. An empty Loki URL disables log shipping.

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
| Additional startup arguments | empty | Extra Alloy CLI flags; an invalid flag prevents startup. |
| Additional Alloy configuration | empty | River blocks appended to generated local configuration. |

A full override at `/config/config.alloy` replaces the generated configuration.
This is for configurations the form cannot express. Startup controls and secret
environment variables remain available, but pipeline options are ignored while
the override exists.

## Secrets and upgrades

Passwords and tokens are referenced from Alloy with `sys.env()` and are never
written into `/etc/alloy/config.alloy` or shown in the browser. Home Assistant
still stores App options in `/data/options.json` and includes them in backups;
the password control masks display, not storage.

Older versions allowed Fleet and local pipelines simultaneously. Such an
installation continues in a compatibility-only **legacy hybrid** state until a
mode is chosen, so upgrading does not silently remove a pipeline. The Web UI
does not allow saving that legacy combination. Select Fleet or Local to migrate.
The former `fleet_password` value is accepted as an upgrade bridge and is shown
as the shared Grafana Cloud key; new configurations use `gcloud_rw_api_key`.

## Alloy component graph and network ports

Alloy's component graph is proxied through Home Assistant ingress at **Open Web
UI > Alloy status**. The internal Alloy server listens only on loopback port
**12345** for health checks and self-monitoring and is not exposed directly.
The configuration control plane accepts only Supervisor ingress traffic; its
separate health endpoint contains no configuration or restart capability.

If Alloy exits because of invalid generated or additional configuration, the App
stops and reports the error. With Home Assistant's Watchdog enabled, an
unresponsive Alloy process is restarted after its readiness health check fails.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Fleet registration fails | Re-copy the regional URL, numeric username and shared write key from the Fleet API tab; check the token scopes. |
| Loki/Prometheus/Tempo/Pyroscope returns 401 | Confirm the numeric tenant ID and token are paired for that specific backend. |
| Home Assistant scrape returns 404 | Enable Home Assistant's `prometheus` integration. |
| No expected journal entries | Check the three log source switches, excluded App slugs and replay-age limit. |
| OTLP sender cannot connect | Enable traces and explicitly allow OTLP clients on the HAOS network. |
| Alloy will not start | Read the App log for the named option or River component; remove invalid additional arguments/configuration. |
