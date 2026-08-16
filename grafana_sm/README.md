# Grafana Synthetic Monitoring Probe

Run a Grafana Cloud private Synthetic Monitoring probe from the network attached to Home
Assistant. The probe makes outbound connections to Grafana Cloud and can run checks against
services reachable from its App network namespace.

This is the smaller standard variant and does not include Chromium, so it cannot run browser checks.

## Install

1. In Grafana Cloud, open **Testing & synthetics > Synthetics > Probes** and add a private probe.
2. Copy its one-time **Probe Authentication Token**.
3. Find your stack's **backend address**, then select the corresponding gRPC API server from
   Grafana's [private probe documentation](https://grafana.com/docs/grafana-cloud/observe-and-act/testing/synthetic-monitoring/set-up/set-up-private-probes/#probe-api-server-url).
4. Enter the token and API server as App configuration. The API server must be `host:port`, with
   no `https://` prefix.
5. Start the App and confirm the probe is online in Grafana Cloud before assigning checks.

The App's **Probe status** ingress page provides connection state, a regional API reachability
test, publishing counters, and check counts by type without exposing tokens, targets, or IDs.

See [DOCS.md](DOCS.md) for browser selection, security, private-network k6 checks, resource sizing,
and troubleshooting.

## License

MIT
