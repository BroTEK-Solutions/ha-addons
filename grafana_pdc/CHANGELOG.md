# Changelog

## 1.0.0

Initial release of the Grafana Private Data Source Connect Home Assistant App.

- Runs the Grafana PDC agent as an outbound reverse tunnel to Grafana Cloud.
- Supports optional destination restrictions, persistent SSH identity material, and Prometheus
  metrics on the internal App network.
- Documents Home Assistant networking, PDC endpoint configuration, operational recovery, and
  security boundaries.
- Stops the App after an unexpected terminating signal instead of treating every signal as an
  intentional shutdown and silently restarting the PDC agent. SIGTERM remains a clean stop.
