# Changelog

## 1.3.0 - 2026-07-30

### Added
- HTTP basic auth for Loki via new `loki_username` / `loki_password` options (Grafana Cloud:
  numeric Loki instance ID + access-policy token)
- Startup validation rejecting a half-configured auth pair (username without password, or the
  reverse) for both Loki and Prometheus, instead of failing silently with HTTP 401
- Startup banner now reports whether each backend uses basic auth, showing the username only

### Changed
- Both backends now share one `basic_auth` emitter in the config generator; the Prometheus
  output is unchanged

### Security
- The Loki password is passed to Alloy through an environment variable and referenced as
  `sys.env("LOKI_PASSWORD")`, so it never lands in `/etc/alloy/config.alloy` - matching the
  existing handling of the Prometheus password

## 1.2.1 - 2026-06-14

### Fixed
- Config generator ran under `with-contenv`, which reset the environment and wiped the
  exported `loki_url`/`prometheus_url` options, producing an empty Alloy config (no logs
  or metrics). The generator now runs under plain bash and inherits the exported options.

## 1.2.0 - 2026-06-14

### Added
- Host system metrics via Alloy `prometheus.exporter.unix` (CPU, memory, disk I/O, load, network)
- Filesystem usage for mapped HA volumes (`share`, `media`, `backup` — the HAOS data partition)
- New options: `prometheus_url`, `prometheus_username`, `prometheus_password`, `instance_name`, `metrics_scrape_interval`
- `host_network` enabled for accurate host network-interface metrics (keeps Protection mode intact)

### Changed
- `loki_url` is now optional; configure at least one of `loki_url` (logs) or `prometheus_url` (metrics)
- Alloy config generation extracted into a tested generator script
- Updated Grafana Alloy to v1.17.0

## 1.0.0 - 2026-02-21

### Added
- Initial release
- Grafana Alloy v1.13.1
- Systemd journal log shipping to Loki
- Journal field relabeling (unit, hostname, syslog_identifier, transport, container_name, level)
- Debug UI on port 12345
- Configurable Loki URL, log level, and additional config
- Watchdog health check via Alloy's `/-/ready` endpoint
- Support for amd64 and aarch64 architectures
