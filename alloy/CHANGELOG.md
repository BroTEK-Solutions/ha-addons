# Changelog

## 1.5.1 - 2026-07-31

### Changed
- The add-on now lives in `BroTEK-Solutions/ha-addons`. **Existing installations must remove the old
  `rknightion/ha-addon-alloy` repository from the add-on store and add
  `https://github.com/BroTEK-Solutions/ha-addons` instead** - the old repository no longer exists.
- The image moved to `ghcr.io/brotek-solutions/{arch}-addon-alloy`. Images already published under
  `ghcr.io/rknightion/` are left in place but receive no further updates.
- Add-on tests moved from `tests/` to `alloy/tests/`, so each add-on in the repository owns its own.

## 1.5.0 - 2026-07-30

### Added
- Grafana Fleet Management support via Alloy's `remotecfg` block. New options: `fleet_url`,
  `fleet_username`, `fleet_password`, `fleet_collector_name`, `fleet_attributes`,
  `fleet_poll_frequency`. Remote configuration runs alongside the locally generated Loki and
  Prometheus pipelines, never replacing them.
- Startup validation of `fleet_attributes`, rejecting a segment that is not `key=value`, has an
  empty key, or contains a quote or backslash (which would break Alloy's River string syntax),
  instead of failing later as an Alloy syntax error
- Startup banner reports the Fleet Management endpoint, poll frequency and attributes

### Changed
- `fleet_url` now counts as a destination: the add-on starts with only Fleet Management configured,
  where previously at least one of `loki_url` or `prometheus_url` was required
- The collector ID reported to Fleet Management is `instance_name`, matching the `instance` label
  on metrics

### Security
- The Fleet Management token is passed to Alloy through an environment variable and referenced as
  `sys.env("FLEET_PASSWORD")`, so it never lands in `/etc/alloy/config.alloy` - matching the
  existing handling of the Loki and Prometheus passwords

## 1.4.0 - 2026-07-30

### Changed
- The add-on now ships as a prebuilt image (`ghcr.io/rknightion/{arch}-addon-alloy`) built by
  GitHub Actions, instead of being compiled on the Home Assistant device at install/update time.
  Updates are a registry pull and no longer depend on the device's disk space, build timeout, or
  network access to Debian/GitHub release mirrors.
- Repository URLs and codeowner now point at `rknightion/ha-addon-alloy`

### Added
- CI: ShellCheck and the generator test suite run on every push and pull request. The Docker-based
  `alloy fmt` / `alloy run` config validation, previously skipped in most environments, now runs.

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
