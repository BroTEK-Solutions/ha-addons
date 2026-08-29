# Changelog

## [2.4.1](https://github.com/BroTEK-Solutions/ha-addons/compare/alloy-v2.4.0...alloy-v2.4.1) (2026-08-29)


### Bug Fixes

* **alloy:** stop the hidden Fleet card re-enabling its required fields ([72e1432](https://github.com/BroTEK-Solutions/ha-addons/commit/72e1432e515c2d6f38e4e024c4eb1ffae38822e5)), closes [#85](https://github.com/BroTEK-Solutions/ha-addons/issues/85)
* **deps:** update dependency grafana/alloy to v1.19.0 ([#75](https://github.com/BroTEK-Solutions/ha-addons/issues/75)) ([6cf69da](https://github.com/BroTEK-Solutions/ha-addons/commit/6cf69da4e0c5929377c0d72f9cae33160f4c3152))
* **deps:** update dependency grafana/alloy to v1.19.2 ([#83](https://github.com/BroTEK-Solutions/ha-addons/issues/83)) ([b3d2146](https://github.com/BroTEK-Solutions/ha-addons/commit/b3d2146eee9c72f30e9c19eecf30b539033d5dd1))

## [2.4.0](https://github.com/BroTEK-Solutions/ha-addons/compare/alloy-v2.3.0...alloy-v2.4.0) (2026-08-20)


### Features

* publish App health over MQTT and add ingress status pages ([999b229](https://github.com/BroTEK-Solutions/ha-addons/commit/999b229f7d8d1e323a3272c426ec22d0dcec2c49))


### Bug Fixes

* **alloy:** export GCLOUD_FM_COLLECTOR_ID for Fleet-delivered pipelines ([210f5f9](https://github.com/BroTEK-Solutions/ha-addons/commit/210f5f9d1ed62b3057a2e1add4be2ffee6ee99f5))
* **deps:** update dependency grafana/alloy to v1.18.1 ([#48](https://github.com/BroTEK-Solutions/ha-addons/issues/48)) ([7976e25](https://github.com/BroTEK-Solutions/ha-addons/commit/7976e25869bb7a4e40911d28c4e6c4ffecc7bdf2))
* **deps:** update ghcr.io/home-assistant/base-debian:bookworm docker digest to 60cd882 ([#68](https://github.com/BroTEK-Solutions/ha-addons/issues/68)) ([795a78f](https://github.com/BroTEK-Solutions/ha-addons/commit/795a78fb6a13c586721b0c0eeceb47d5d437f4b2))

## [2.3.0](https://github.com/BroTEK-Solutions/ha-addons/compare/alloy-v2.2.0...alloy-v2.3.0) (2026-08-03)


### Features

* **alloy:** move stability level to config.yaml as a native option ([05c018d](https://github.com/BroTEK-Solutions/ha-addons/commit/05c018d9c6044bfb537e0bc729a322ed010457ef))
* **alloy:** move stability level to config.yaml as a native option ([82f4203](https://github.com/BroTEK-Solutions/ha-addons/commit/82f4203d0c0466ea171c2544f2186ce8ae400c1f))


### Bug Fixes

* **alloy:** complete the native stability option and stop reporting successful restarts as failures ([27b3a85](https://github.com/BroTEK-Solutions/ha-addons/commit/27b3a858aa9c5a1e0e87375143c5115c39b9e4ef))
* **alloy:** keep Alloy startup flags reachable in Fleet and break-glass modes ([285f610](https://github.com/BroTEK-Solutions/ha-addons/commit/285f61008aff0afa22781c47d5a38d44d0271311))
* **alloy:** keep Alloy startup flags reachable in Fleet and break-glass modes ([d4b54e9](https://github.com/BroTEK-Solutions/ha-addons/commit/d4b54e93ba4f23f0e923bf1f9bd603dfdcd108b1))

## [2.2.0](https://github.com/BroTEK-Solutions/ha-addons/compare/alloy-v2.1.1...alloy-v2.2.0) (2026-08-03)


### Features

* **alloy:** render Fleet starter pipelines with editable placeholders ([889ca64](https://github.com/BroTEK-Solutions/ha-addons/commit/889ca644bd4961f2deba67398562befc843f46b6))


### Bug Fixes

* **alloy:** drop starter endpoints for deselected signals ([fdf6c9b](https://github.com/BroTEK-Solutions/ha-addons/commit/fdf6c9bba89729f665b7c8e75f3f98f1f778ba12))
* **alloy:** pin metric and log identity to the instance name ([557d5aa](https://github.com/BroTEK-Solutions/ha-addons/commit/557d5aa6c1930fbdb7804753084c2511ec7de784))
* **alloy:** scope identity rewrite to the host exporter ([5fcdb30](https://github.com/BroTEK-Solutions/ha-addons/commit/5fcdb30d6097da150c8b36751c7a4b7aff6da2bd))
* **alloy:** scope the identity relabel to the host exporter ([41392fb](https://github.com/BroTEK-Solutions/ha-addons/commit/41392fba9a00425027211618812f7b68bd8e202a))

## [2.1.1](https://github.com/BroTEK-Solutions/ha-addons/compare/alloy-v2.1.0...alloy-v2.1.1) (2026-08-02)


### Bug Fixes

* **alloy:** confirm preselected wizard mode ([9af9884](https://github.com/BroTEK-Solutions/ha-addons/commit/9af988421630d6e8dbcb642f026c190a40dd42c7))
* **alloy:** harden and stage the v2.1.0 onboarding wizard ([5635409](https://github.com/BroTEK-Solutions/ha-addons/commit/563540965c5234a17280ef0ae880be7edaf0f9c2))
* **alloy:** preserve unsaved wizard settings ([28f8340](https://github.com/BroTEK-Solutions/ha-addons/commit/28f8340ba8341c657b81082c345fdb21bb75c0e3))
* **alloy:** repair onboarding UI upgrades ([93f4cc4](https://github.com/BroTEK-Solutions/ha-addons/commit/93f4cc4e1079a7362f6b89d34110b2a372eae5d7))

## [2.1.0](https://github.com/BroTEK-Solutions/ha-addons/compare/alloy-v2.0.0...alloy-v2.1.0) (2026-08-02)


### Features

* **alloy:** guide Fleet onboarding ([edf6c0a](https://github.com/BroTEK-Solutions/ha-addons/commit/edf6c0ae78aa6a2c1afb7cff3dbd97e7f75de559))
* **alloy:** guide Fleet onboarding ([c10d27e](https://github.com/BroTEK-Solutions/ha-addons/commit/c10d27e595cfbc04f276cd3d092488ebc95021bb))
* **alloy:** isolate Fleet starter selections ([acff816](https://github.com/BroTEK-Solutions/ha-addons/commit/acff816c735491010ef6c3eb7d675330a3f4579d))
* **alloy:** publish HAOS Fleet attributes ([0f42abd](https://github.com/BroTEK-Solutions/ha-addons/commit/0f42abd424b7a7f688efe183ad761e98c74aa48c))
* **alloy:** publish HAOS Fleet attributes ([ac2ec07](https://github.com/BroTEK-Solutions/ha-addons/commit/ac2ec077db2c7b3249da53e4dd8c61cc67bb261a)), closes [#29](https://github.com/BroTEK-Solutions/ha-addons/issues/29)
* **alloy:** report component health to ingress ([f528833](https://github.com/BroTEK-Solutions/ha-addons/commit/f5288335ad318d91391b4a945745797c3bc01e1e))


### Bug Fixes

* **alloy:** address Fleet onboarding review ([6c1c550](https://github.com/BroTEK-Solutions/ha-addons/commit/6c1c550eefa6a180428a1444b3a579739c302410))
* **alloy:** block starters during manual override ([2817980](https://github.com/BroTEK-Solutions/ha-addons/commit/28179804856c8c67e5788ec43bac0e78e64d5324))
* **alloy:** guard Fleet starter controls ([34cb7b1](https://github.com/BroTEK-Solutions/ha-addons/commit/34cb7b1c79d119f0b4f6bf6f5551152056f2a0f9))
* **alloy:** match HA Supervisor journal containers ([05ede94](https://github.com/BroTEK-Solutions/ha-addons/commit/05ede94c02a0f9d213de3b1ef12ae5b77d6e537e))
* **alloy:** match HA Supervisor journal containers ([84c2096](https://github.com/BroTEK-Solutions/ha-addons/commit/84c20966ebbeb99e440293c3a0cf8161b210144d)), closes [#28](https://github.com/BroTEK-Solutions/ha-addons/issues/28)
* **alloy:** preserve Fleet wizard state ([a076510](https://github.com/BroTEK-Solutions/ha-addons/commit/a0765104d2f875ef9c883a62bcab848c7add5e68))
* **alloy:** reject starter generation outside Fleet mode ([08d75be](https://github.com/BroTEK-Solutions/ha-addons/commit/08d75bea3e040140a68a2c2e4b6e1b8dacbb81df))
* **alloy:** retain both Supervisor container names ([415b62a](https://github.com/BroTEK-Solutions/ha-addons/commit/415b62a1dee2315b2fae9ff00d74e9a0d7ae3d79))
* **alloy:** retain legacy journal exclusions ([5be0cdf](https://github.com/BroTEK-Solutions/ha-addons/commit/5be0cdfcb565e901807873f5659c3143911f7e9e))

## [2.0.0](https://github.com/BroTEK-Solutions/ha-addons/compare/alloy-v1.8.0...alloy-v2.0.0) (2026-08-01)


### ⚠ BREAKING CHANGES

* **alloy:** Alloy operational configuration is now managed exclusively through the ingress Web UI. Native App options are reduced to safe_mode and ui_log_level, and the filesystem config.alloy override is replaced by the ingress manual override.

### Features

* **alloy:** generate Fleet starter pipelines ([17cecda](https://github.com/BroTEK-Solutions/ha-addons/commit/17cecda19b1cb1cb38abb0615cc7d34b0f5581c7))
* **alloy:** generate Fleet starter pipelines ([9605eba](https://github.com/BroTEK-Solutions/ha-addons/commit/9605eba4584c8c0b006fc59b520abdf288310dc0)), closes [#17](https://github.com/BroTEK-Solutions/ha-addons/issues/17)
* **alloy:** make ingress the configuration source ([b46ee07](https://github.com/BroTEK-Solutions/ha-addons/commit/b46ee079d0f879a959d619a52e6dc5a35acdd1f0))


### Bug Fixes

* **alloy:** align Fleet reference validation ([e075a88](https://github.com/BroTEK-Solutions/ha-addons/commit/e075a8838a3757ed91555fd15c779c1e3b1b1b93))
* **alloy:** align recovery form and endpoint validation ([e1add0d](https://github.com/BroTEK-Solutions/ha-addons/commit/e1add0d24cb6d6d00e88f8857f56377fb0496d5d))
* **alloy:** block Fleet starter in safe mode ([53ac135](https://github.com/BroTEK-Solutions/ha-addons/commit/53ac135e10a897078e5696f3349b10e8a4a7b779))
* **alloy:** harden Fleet command generation ([f3d18a6](https://github.com/BroTEK-Solutions/ha-addons/commit/f3d18a69bc11d4b042a878bb75a719a1a5623ecf))
* **alloy:** harden Fleet instance identity ([51b2a8d](https://github.com/BroTEK-Solutions/ha-addons/commit/51b2a8d7772ccb5eea967eb80152f5ff73de6080))
* **alloy:** migrate reserved Fleet attribute ([64c855f](https://github.com/BroTEK-Solutions/ha-addons/commit/64c855f2183018e23e8c7e99e984b0193f27e64f))
* **alloy:** persist applied Fleet state ([49eda2a](https://github.com/BroTEK-Solutions/ha-addons/commit/49eda2acf674c1eac11a14cc84250759d8e04a13))
* **alloy:** preserve multiline startup flags ([fb2180b](https://github.com/BroTEK-Solutions/ha-addons/commit/fb2180bb3151faf4b662c5446a3fc628f4aeb579))
* **alloy:** protect Fleet starter downloads ([6f84fcd](https://github.com/BroTEK-Solutions/ha-addons/commit/6f84fcd70186a3f0e9dd7c6a28482f4bb2c307b7))
* **alloy:** require applied Fleet settings ([40ca38c](https://github.com/BroTEK-Solutions/ha-addons/commit/40ca38cafe6c7cbb2e508852caf2361e5efefa97))
* **alloy:** serialize settings application ([b90fc8c](https://github.com/BroTEK-Solutions/ha-addons/commit/b90fc8c123b2b6d8f68f76aed7e2b1a0c35a9c5c))
* **alloy:** validate startup-only settings before save ([f718ab3](https://github.com/BroTEK-Solutions/ha-addons/commit/f718ab36ed95a00187d3834248bfb9003287445c))

## [1.8.0](https://github.com/BroTEK-Solutions/ha-addons/compare/alloy-v1.7.0...alloy-v1.8.0) (2026-08-01)


### Features

* improve Alloy and PDC configuration UX ([c210c9c](https://github.com/BroTEK-Solutions/ha-addons/commit/c210c9c25de7551d02e55a38685f1b9bfc932eea))
* improve Alloy and PDC configuration UX ([2890008](https://github.com/BroTEK-Solutions/ha-addons/commit/289000840ad756e7a1e60f0bc5a89873bef3fc4f))


### Bug Fixes

* ignore inactive Fleet credentials in local mode ([26e20ad](https://github.com/BroTEK-Solutions/ha-addons/commit/26e20ade97d72f3a5789e15cf6d7bb91c5f8479a))
* keep Alloy ingress available before setup ([3a8b79f](https://github.com/BroTEK-Solutions/ha-addons/commit/3a8b79f01a331193135cae0fce41e131f4d60986))
* preserve Alloy settings across mode changes ([1af5d33](https://github.com/BroTEK-Solutions/ha-addons/commit/1af5d33c5750a26987f5c9a3f3b32af61ea4b1c6))
* rely on default ingress port ([cf77935](https://github.com/BroTEK-Solutions/ha-addons/commit/cf77935c2b874dc93f4becf4255ad1c8f01ba81b))
* validate Alloy options before restart ([4d281e3](https://github.com/BroTEK-Solutions/ha-addons/commit/4d281e388cfb5fc987a8532c84e587841c077239))

## 1.7.0 - 2026-07-31

### Fixed
- Alloy now stops the App after an unexpected terminating signal instead of treating every
  signal as an intentional shutdown and entering an invisible restart loop. SIGTERM remains a
  clean stop; crash signals are reported using the conventional `128 + signal` exit code

### Added
- **Home Assistant Core metrics.** New `homeassistant_metrics` option, off by default, scrapes
  Core's own Prometheus endpoint through the Supervisor proxy for entity states and Core
  internals. Requires the `prometheus` integration enabled in Home Assistant. The Supervisor
  token is read with `sys.env()` and never written into the generated config
- **`host_metrics`**, on by default, so the host exporter can be turned off for a
  Home Assistant-only or logs-only setup. Enabling either source without `prometheus_url` is now
  refused at start-up instead of silently collecting nothing
- **A full configuration override.** A file at `/config/config.alloy` replaces the generated
  configuration entirely, for anything the options cannot express. Options that shape the config
  are then ignored - announced in the log on every start - while the startup flags still apply
  and the passwords stay available as `sys.env()`
- **Alloy startup flags as options.** `alloy_stability_level` for loading public-preview or
  experimental components, `alloy_disable_telemetry` (on by default, passing
  `--disable-reporting`), and `alloy_additional_args` for anything else
- `.github/CODEOWNERS`, and yamllint in CI alongside the add-on linter

### Changed
- Alloy updated to **1.18.0** (from 1.17.0). The breaking changes in that release are all in
  `otelcol.*` components, none of which this add-on uses
- The endpoint options now reject missing authorities, non-numeric or out-of-range ports,
  invalid percent escapes, malformed bracketed IPv6 literals, whitespace, double quotes and
  backslashes. Valid RFC 6874 scoped literals such as `[fe80::1%25eth0]` and IPv4-embedded IPv6
  literals remain accepted. The same checks run at start-up because `options.json` can be edited
  outside the UI
- `metrics_scrape_interval` and `fleet_poll_frequency` accept positive Go duration forms such as
  `+1m30s`, `.5s`, `1.s`, `100us`, `100µs` and `100μs`. Zero and negative intervals are rejected.
  The 1.6.0 pattern allowed only one unsigned decimal and one unit, which would have rejected
  working configurations on upgrade
- `fleet_attributes` rejects quotes and backslashes in the UI, matching what start-up validation
  already refused. Values may contain additional equals signs, so query fragments and padded
  base64 values such as `query=a=b` and `token=YWJjZA==` remain valid
- The add-on's own configuration folder is mapped read-write again, to hold the override file

### Removed
- `codeowners` from `config.yaml` stays removed: it is not a Home Assistant add-on option and
  Supervisor strips unknown keys, so it never did anything. Ownership now lives in
  `.github/CODEOWNERS`, where the Community Add-ons repository keeps it too

## 1.6.0 - 2026-07-31

### Fixed
- **A metrics-only or Fleet Management-only install could not be configured.** `loki_url` carried
  a default in `options:`, and a default overrides the `?` in `schema:` and makes an option
  required. Supervisor validates a `url` with voluptuous, which rejects an empty string, so
  clearing the field failed validation and leaving it set pointed the add-on at a Loki that was
  not there. The default is gone, and the three endpoint options now use a pattern that accepts
  an empty value, so clearing one in the UI disables that destination

### Added
- `translations/en.yaml`, giving every option a readable name and description in the add-on
  configuration UI instead of a raw snake_case key
- `icon.png` and `logo.png`, so the add-on is no longer a blank tile in the store
- An `OPEN WEB UI` button for the Alloy debug UI, via the `webui` option
- Option validation in the UI: endpoints must look like URLs, `metrics_scrape_interval` and
  `fleet_poll_frequency` must carry a unit, `instance_name` cannot be empty, and
  `fleet_attributes` must be well-formed `key=value` pairs. Previously these were accepted and
  became a start-up failure
- `alloy/tests/config-schema.test.py`, which validates the options and schema against
  Supervisor's own `RE_SCHEMA_ELEMENT` and voluptuous, so this class of bug fails in CI
- `alloy/tests/init-alloy.test.sh`, covering the service script's start-up validation - every
  refuse-to-start path and each destination working on its own. It had no tests before
- A Renovate configuration that tracks the pinned Alloy release
- The add-on now stops, reporting the exit code, when Alloy exits on its own - most often an
  `additional_config` that does not parse. Previously s6 restarted it indefinitely while the
  add-on still reported itself as running
- CI runs the Home Assistant add-on linter, which validates `config.yaml` and `build.yaml`
  against the add-on JSON schemas
- OCI image labels (source, documentation, licence) on the published images

### Changed
- The service scripts now use `bashio` rather than hand-rolled `jq`. `bashio::config.has_value`
  distinguishes unset from null from empty, which is the distinction the `loki_url` bug turned on
- `/data/alloy` is excluded from Home Assistant backups. It holds Alloy's write-ahead log and its
  cached Fleet Management configuration, both rebuilt on start
- The Alloy version is now defined only in `alloy/Dockerfile`; the test suite parses it from
  there rather than keeping a second copy
- `map` uses the current list syntax with explicit `read_only`, and no longer requests write
  access to the add-on config folder, which nothing used
- `ports_description` moved into `translations/en.yaml`
- Health is now reported through a Docker `HEALTHCHECK` rather than the obsolete `watchdog`
  config key. Supervisor reads the container's health status, so the Watchdog toggle behaves as
  before

### Removed
- `codeowners`, which is a Community Add-ons extension rather than a Home Assistant add-on option
  and was inherited from the upstream fork
- `boot: auto`, which only restated the default

## 1.5.1 - 2026-07-31

### Changed
- The add-on now lives in `BroTEK-Solutions/ha-addons`. **Existing installations must remove the old
  `rknightion/ha-addon-alloy` repository from the add-on store and add
  `https://github.com/BroTEK-Solutions/ha-addons` instead** - the old repository no longer exists.
- The image moved to `ghcr.io/brotek-solutions/{arch}-addon-alloy`. Images already published under
  `ghcr.io/rknightion/` are left in place but receive no further updates.
- Add-on tests moved from `tests/` to `alloy/tests/`, so each add-on in the repository owns its own.

### Documentation
- Corrected the `level` label description: it is regex-extracted from Home Assistant's log line
  format, not the systemd priority field, and is absent on lines that do not match - so a query
  filtering on `level` silently drops kernel and Supervisor output
- Corrected the options reference, which listed `loki_url` as required and then contradicted
  itself two paragraphs later. At least one of `loki_url`, `prometheus_url` or `fleet_url` is
  required; none is required individually
- Corrected the `additional_config` example, which tailed `/config/home-assistant.log` - Home
  Assistant's config directory is not mapped into the add-on, so that path does not exist
- Documented the journal path fallback, the disabled collectors and excluded network interfaces,
  the `integrations/node_exporter` job name, the watchdog endpoint, and the basic-auth pairing
  rule on `prometheus_*` (previously only stated for `loki_*`)

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
