# Changelog

## 1.0.0

Initial release of Grafana Synthetic Monitoring Probe with Browser Checks.

- Runs Grafana Synthetic Monitoring Agent as a non-root Home Assistant App.
- Keeps the probe token out of the process argument list.
- Supports private probe registration, ICMP, k6, ad hoc, and traceroute checks.
- Uses Grafana's pinned multi-architecture upstream image for `amd64` and `aarch64`.
