# BroTEK Solutions Home Assistant Apps

Home Assistant App repository from BroTEK Solutions.

## Install this repository

1. In Home Assistant, open **Settings > Apps > App store**.
2. Open the menu in the top right, choose **Repositories**, and add:
   `https://github.com/BroTEK-Solutions/ha-addons`
3. Close the repository dialog, select an App below, then choose **Install**.

| Home Assistant App | Architectures | Purpose |
| --- | --- | --- |
| [Grafana Alloy](alloy/) | `amd64`, `aarch64` | Ships Home Assistant OS logs to Loki and host metrics to Prometheus; can also be managed with Grafana Fleet Management. |
| [Grafana Private Data Source Connect](grafana_pdc/) | `amd64`, `aarch64` | Creates an outbound Grafana Cloud Private Data Source Connect tunnel to data sources reachable from Home Assistant. |

Each App has its own configuration and operational documentation in its directory.

## License

MIT
