# BroTEK Solutions Home Assistant Add-ons

A Home Assistant add-on repository. Add it once and its add-ons appear in your Add-on Store.

1. Open **Settings** > **Add-ons** > **Add-on Store**
2. Click the overflow menu (three dots, top-right) > **Repositories**
3. Paste: `https://github.com/BroTEK-Solutions/ha-addons`
4. Click **Add** > **Close**

| Add-on | Architectures | Description |
|--------|---------------|-------------|
| [Grafana Alloy](alloy/) | `amd64`, `aarch64` | Ship HAOS journal logs to Loki and host metrics to Prometheus, optionally managed from Grafana Fleet Management |

---

# Grafana Alloy

Ship Home Assistant OS logs to [Grafana Loki](https://grafana.com/oss/loki/) **and host system metrics to Prometheus** using [Grafana Alloy](https://grafana.com/docs/alloy/latest/) — the modern replacement for the deprecated Promtail add-on, plus node_exporter-style host monitoring.

## Why?

The official Promtail add-on (v2.2.0) bundles Promtail 2.6.1, which cannot read the compact journal format introduced in systemd 252+ (HAOS 11+). This means **Promtail silently fails to ship logs on modern HAOS installations**.

Grafana Alloy is the official successor to Promtail, Grafana Agent, and Grafana Agent Flow. It uses a component-based pipeline architecture and has native systemd journal support that works with all journal formats.

## Installation

Add the repository as above, then find **Grafana Alloy** in the store and click **Install**.

Install and update pull a prebuilt image from `ghcr.io` (`brotek-solutions/{arch}-addon-alloy`),
built by GitHub Actions for `amd64` and `aarch64` on every push to `main`. Nothing is compiled on
your Home Assistant device.

## Configuration

Set `loki_url` to your Loki push endpoint, and/or `prometheus_url` to a Prometheus `remote_write` endpoint:

```yaml
loki_url: "http://192.168.1.45:3100/loki/api/v1/push"
prometheus_url: "http://192.168.1.45:9090/api/v1/write"
instance_name: home-assistant
log_level: info
```

At least one of `loki_url` (logs), `prometheus_url` (metrics) or `fleet_url` (Fleet Management)
must be set; the add-on refuses to start with all three empty. Each backend takes optional basic
auth (`*_username` + `*_password`, both or neither). See [the full
documentation](alloy/DOCS.md) for every option, the host-filesystem caveat, and how the `level`
label is derived.

## Host metrics

When `prometheus_url` is set, the add-on collects host CPU, memory, disk I/O, load average, and
network metrics via Alloy's `unix` exporter, under the job name `integrations/node_exporter`.
HAOS add-ons cannot mount the host root filesystem, so filesystem usage is reported for the
mapped HA volumes (`share`/`media`/`backup`, which sit on the data partition) rather than every
host mount.

## Fleet Management

Set `fleet_url` (plus `fleet_username` and `fleet_password`) to register the collector with
[Grafana Fleet Management](https://grafana.com/docs/grafana-cloud/send-data/fleet-management/).
Alloy then runs the pipelines it receives from Grafana Cloud alongside the local ones configured
above, never replacing them. Use `fleet_attributes` (`key=value` pairs) to control which pipelines
this collector receives. The collector ID is `instance_name`.

## What gets shipped

All systemd journal entries from HAOS, including:
- Home Assistant Core logs
- Add-on/app container logs
- Supervisor logs
- Host system logs (kernel, networkd, etc.)

Labels applied: `job`, `unit`, `hostname`, `syslog_identifier`, `transport`, `container_name`, and
`level` where the line matches Home Assistant's log format. Blank lines are dropped.

## Debug UI

Access the Alloy pipeline inspector at `http://<haos-ip>:12345`.

## Development

```bash
shellcheck alloy/rootfs/usr/share/alloy/generate-config.sh \
  alloy/rootfs/etc/s6-overlay/s6-rc.d/*/run \
  alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/finish alloy/tests/*.sh

# Add-on options and schema, the way Supervisor validates them
python3 -m pip install voluptuous pyyaml
python3 alloy/tests/config-schema.test.py

# Config generator, with `alloy fmt` / `alloy run` validation when Docker is present
bash alloy/tests/generate-config.test.sh

# The init-alloy service script, run inside the real add-on base image for bashio
docker run --rm -v "${PWD}:/w" -w /w \
  ghcr.io/home-assistant/amd64-base-debian:bookworm \
  bash alloy/tests/init-alloy.test.sh
```

Three suites, all run in CI on every push and pull request:

- **`config-schema.test.py`** validates `config.yaml` against Supervisor's own
  `RE_SCHEMA_ELEMENT` and voluptuous. It enforces the rule that a default in `options:` makes an
  option required regardless of the `?` in `schema:` — the trap that once made a Fleet-only
  install impossible to configure — and fails if an option has no `translations/en.yaml` entry.
- **`generate-config.test.sh`** drives the config generator through its option combinations and
  validates each generated config with `alloy fmt` and `alloy run`.
- **`init-alloy.test.sh`** runs the real service script against seeded add-on options, covering
  every way it can refuse to start. It needs bashio, so it runs inside the add-on base image;
  outside a container, point `BASHIO_BIN` at a checkout of `hassio-addons/bashio`.

The Alloy version is pinned in `alloy/Dockerfile` and nowhere else — the test suite parses it
out of that file, and Renovate tracks it there.

## License

MIT
