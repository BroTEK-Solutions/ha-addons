# Changelog

## [1.0.1](https://github.com/BroTEK-Solutions/ha-addons/compare/grafana-pdc-v1.0.0...grafana-pdc-v1.0.1) (2026-08-01)


### Bug Fixes

* **deps:** update grafana/pdc-agent docker tag to v0.0.63 ([#6](https://github.com/BroTEK-Solutions/ha-addons/issues/6)) ([2554b1c](https://github.com/BroTEK-Solutions/ha-addons/commit/2554b1c55f41da3355870b54824b309db7ec8887))
* **grafana_pdc:** initialise SSH permissions without fowner ([d975ec8](https://github.com/BroTEK-Solutions/ha-addons/commit/d975ec8964da9e38db6d19ce7e0ad3f0c6152860))
* unblock automated dependency releases ([11d0ed1](https://github.com/BroTEK-Solutions/ha-addons/commit/11d0ed16c265b0dbec902de90178e0dd435924a0))

## 1.0.0

Initial release of the Grafana Private Data Source Connect Home Assistant App.

- Runs the Grafana PDC agent as an outbound reverse tunnel to Grafana Cloud.
- Supports optional destination restrictions, persistent SSH identity material, and Prometheus
  metrics on the internal App network.
- Documents Home Assistant networking, PDC endpoint configuration, operational recovery, and
  security boundaries.
- Stops the App after an unexpected terminating signal instead of treating every signal as an
  intentional shutdown and silently restarting the PDC agent. SIGTERM remains a clean stop.
- Permits the outer AppArmor profile to make the IPv4 loopback connection used by the container
  health check; the PDC service continues to run under its separate constrained profile.
