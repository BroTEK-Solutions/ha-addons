# Grafana Alloy for Home Assistant

![Supports amd64 Architecture](https://img.shields.io/badge/amd64-yes-green.svg)
![Supports aarch64 Architecture](https://img.shields.io/badge/aarch64-yes-green.svg)

Run Grafana Alloy on Home Assistant OS with a conditional ingress configuration
page and the proxied Alloy component graph.

- **Fleet Management mode** runs pipelines supplied by Grafana Cloud and makes
  the shared read/write key available to them as `GCLOUD_RW_API_KEY`. Its guided
  setup can generate a secret-free starter manifest - with `REPLACE-ME`
  placeholders for anything not configured yet - to edit, download and create
  with `gcx`; the App itself never writes or reconciles Fleet.
- **Local mode** builds selectable HAOS/Core/App log pipelines, host/Home
  Assistant/Alloy metric pipelines, OTLP trace forwarding and Alloy
  self-profiling.

Other Alloy add-ons run the collector with a fixed configuration that forwards
Home Assistant metrics to Grafana Cloud. This App treats Alloy as a managed
collector instead: Fleet Management mode hands pipeline control to Grafana
Cloud, and Local mode assembles log, metric, trace and profiling pipelines from
the ingress UI without hand-written Alloy configuration.

Alloy's UI and port 12345 are available only through Home Assistant ingress.
OTLP ports 4317 and 4318 remain loopback-only unless network access is
explicitly enabled for trusted senders.

Version 2 uses the ingress page as the single operational configuration source.
The native Home Assistant Configuration tab contains only the startup controls
that must stay reachable when Alloy will not start: safe mode, UI log level and
minimum component stability. Advanced users can enable a validated, full manual
Alloy configuration override from the ingress page.

See the App's **Documentation** tab for setup, defaults, security boundaries and
migration guidance.
