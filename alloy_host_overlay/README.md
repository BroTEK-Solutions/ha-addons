# Grafana Alloy with Host Processes

![Supports amd64 Architecture](https://img.shields.io/badge/amd64-yes-green.svg)
![Supports aarch64 Architecture](https://img.shields.io/badge/aarch64-yes-green.svg)

The Grafana Alloy App with host process visibility. With its protection mode
turned off, Alloy runs in the Home Assistant OS host's process namespace, so
per-process metrics (`prometheus.exporter.process`) cover every process on the
host instead of only the App's own. With protection mode on it behaves like the
standard Grafana Alloy App.

Install the standard **Grafana Alloy** App unless you need per-process metrics:
turning protection mode off lets Alloy see every host process and its command
line. Both Apps can run side by side.

See the App's **Documentation** tab for the security trade-off, setup and the
full Alloy documentation.
