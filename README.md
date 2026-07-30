# Home Assistant App: Grafana Alloy

Ship Home Assistant OS logs to [Grafana Loki](https://grafana.com/oss/loki/) **and host system metrics to Prometheus** using [Grafana Alloy](https://grafana.com/docs/alloy/latest/) — the modern replacement for the deprecated Promtail add-on, plus node_exporter-style host monitoring.

## Why?

The official Promtail add-on (v2.2.0) bundles Promtail 2.6.1, which cannot read the compact journal format introduced in systemd 252+ (HAOS 11+). This means **Promtail silently fails to ship logs on modern HAOS installations**.

Grafana Alloy is the official successor to Promtail, Grafana Agent, and Grafana Agent Flow. It uses a component-based pipeline architecture and has native systemd journal support that works with all journal formats.

## Installation

1. Open **Settings** > **Add-ons** > **Add-on Store**
2. Click the overflow menu (three dots, top-right) > **Repositories**
3. Paste: `https://github.com/rknightion/ha-addon-alloy`
4. Click **Add** > **Close**
5. Find **Grafana Alloy** in the store and click **Install**

Install and update pull a prebuilt image from `ghcr.io` (`{arch}-addon-alloy`), built by
GitHub Actions for `amd64` and `aarch64` on every push to `main`. Nothing is compiled on your
Home Assistant device.

## Configuration

Set `loki_url` to your Loki push endpoint, and/or `prometheus_url` to a Prometheus `remote_write` endpoint:

```yaml
loki_url: "http://192.168.1.45:3100/loki/api/v1/push"
prometheus_url: "http://192.168.1.45:9090/api/v1/write"
instance_name: home-assistant
log_level: info
```

At least one of `loki_url` (logs) or `prometheus_url` (metrics) must be set. See the add-on's **Documentation** tab for the full metrics options and the host-filesystem caveat.

## Host metrics

When `prometheus_url` is set, the add-on collects host CPU, memory, disk I/O, load average, and
network metrics via Alloy's `unix` exporter. HAOS add-ons cannot mount the host root filesystem,
so filesystem usage is reported for the mapped HA volumes (`share`/`media`/`backup`, which sit on
the data partition) rather than every host mount.

## What gets shipped

All systemd journal entries from HAOS, including:
- Home Assistant Core logs
- Add-on/app container logs
- Supervisor logs
- Host system logs (kernel, networkd, etc.)

Labels applied: `unit`, `hostname`, `syslog_identifier`, `transport`, `container_name`, `level`.

## Debug UI

Access the Alloy pipeline inspector at `http://<haos-ip>:12345`.

## License

MIT
