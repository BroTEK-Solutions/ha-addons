# Grafana Alloy for Home Assistant

![Supports amd64 Architecture](https://img.shields.io/badge/amd64-yes-green.svg)
![Supports aarch64 Architecture](https://img.shields.io/badge/aarch64-yes-green.svg)

Ship Home Assistant OS systemd journal logs to Grafana Loki **and host system metrics to
Prometheus** using Grafana Alloy.

Replaces the deprecated Promtail add-on which fails on HAOS 11+ due to systemd 252+ compact
journal format incompatibility, and adds node_exporter-style host monitoring (CPU, memory,
disk, network). Set `prometheus_url` to enable metrics; see the **Documentation** tab. Set
`fleet_url` to manage this collector from Grafana Fleet Management.

For full documentation, see the **Documentation** tab after installing.
