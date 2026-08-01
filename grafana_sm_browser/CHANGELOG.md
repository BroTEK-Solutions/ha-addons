# Changelog

## [1.1.0](https://github.com/BroTEK-Solutions/ha-addons/compare/grafana-sm-browser-v1.0.0...grafana-sm-browser-v1.1.0) (2026-08-01)


### Features

* add Grafana Synthetic Monitoring private probe Apps ([762fae2](https://github.com/BroTEK-Solutions/ha-addons/commit/762fae2e21088db989a5faf71c21003dfa95bb78))
* add Grafana Synthetic Monitoring probes ([5349452](https://github.com/BroTEK-Solutions/ha-addons/commit/53494527d519dbbc99dd17fa5bd525317b75e64d))


### Bug Fixes

* read Supervisor options before dropping Synthetic Monitoring privileges ([01fba9b](https://github.com/BroTEK-Solutions/ha-addons/commit/01fba9b2dbedd95d0dde5a464b6e3541fc1a0753))
* read Supervisor probe options before dropping privileges ([825e5c3](https://github.com/BroTEK-Solutions/ha-addons/commit/825e5c3c59576b2765c71327970c7d6c8cc94cbb))

## 1.0.0

Initial release of Grafana Synthetic Monitoring Probe with Browser Checks.

- Runs Grafana Synthetic Monitoring Agent as a non-root Home Assistant App.
- Keeps the probe token out of the process argument list.
- Supports private probe registration, ICMP, k6, ad hoc, and traceroute checks.
- Uses Grafana's pinned multi-architecture upstream image for `amd64` and `aarch64`.
