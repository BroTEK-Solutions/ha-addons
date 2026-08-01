# Grafana Private Data Source Connect

Use Grafana Cloud Private Data Source Connect (PDC) from Home Assistant without exposing a
data source to the internet. This Home Assistant App opens an **outbound** reverse SSH tunnel to
Grafana Cloud; it does not require port forwarding or an inbound firewall rule.

## Install

1. Add this repository from **Settings > Apps > App store** if it is not already installed.
2. Open **Grafana Private Data Source Connect** and select **Install**.
3. In Grafana Cloud, open **Connections > Private data source connections > Configuration
   Details**. Copy these three values exactly:
   - a signing token with the `pdc-signing:write` scope;
   - the hosted Grafana ID; and
   - the cluster.
4. Paste them into the App configuration and start the App.

```yaml
# Deliberately fake examples: replace every value with Configuration Details values.
signing_token: "not-a-real-pdc-signing-token"
hosted_grafana_id: "123456789"
cluster: "example-cluster-copied-from-grafana"
```

The cluster is free text. Paste the value Grafana Cloud gives you; this App does not discover
clusters and does not maintain a region map. The common form contains only credentials,
destination restrictions and log level. Uncommon controls use pdc-agent's upstream defaults and
appear under Home Assistant's optional-configuration expander.

See [DOCS.md](DOCS.md) for endpoint restrictions, Home Assistant networking, metrics, persistence,
and troubleshooting.

## Security boundary

`allowed_endpoints` is optional. When set, it restricts which `host:port` destinations Grafana
Cloud may reach through the tunnel. When empty (the default), the tunnel can reach any destination
that is reachable from the App's network namespace. Start with a small allowlist where practical.

## License

MIT
