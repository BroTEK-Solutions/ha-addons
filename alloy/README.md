# Grafana Alloy for Home Assistant

![Supports amd64 Architecture](https://img.shields.io/badge/amd64-yes-green.svg)
![Supports aarch64 Architecture](https://img.shields.io/badge/aarch64-yes-green.svg)

Run Grafana Alloy on Home Assistant OS with a conditional ingress configuration
page and the proxied Alloy component graph.

- **Fleet Management mode** runs pipelines supplied by Grafana Cloud and makes
  the shared read/write key available to them as `GCLOUD_RW_API_KEY`.
- **Local mode** builds selectable HAOS/Core/App log pipelines, host/Home
  Assistant/Alloy metric pipelines, OTLP trace forwarding and Alloy
  self-profiling.

Alloy's UI and port 12345 are available only through Home Assistant ingress.
OTLP ports 4317 and 4318 remain loopback-only unless network access is
explicitly enabled for trusted senders.

See the App's **Documentation** tab for setup, defaults, security boundaries and
migration guidance.
