FROM golang:1.26.4-alpine3.22@sha256:727cfc3c40be55cd1bc9a4a059406b28a059857e3be752aa9d09531e12c20c56 AS launcher-builder

WORKDIR /src
COPY launcher/go.mod launcher/main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/ha-sm-launcher \
    .

FROM grafana/synthetic-monitoring-agent:v0.62.1-browser@sha256:6ebab67eab36e4a86d847929c7c7bc9712856cdda77a0721de7357c5634169ea

USER root
COPY --from=launcher-builder --chown=12345:12345 --chmod=0500 \
    /out/ha-sm-launcher /usr/local/bin/ha-sm-launcher

# This checks the local HTTP server, not /ready. Grafana Cloud connectivity is
# readiness state and must not turn an upstream outage into a restart loop.
HEALTHCHECK --start-period=30s --interval=30s --timeout=5s --retries=3 \
    CMD ["/usr/local/bin/ha-sm-launcher", "healthcheck"]

USER sm
ENTRYPOINT ["tini", "--", "/usr/local/bin/ha-sm-launcher"]

LABEL \
    org.opencontainers.image.title="Home Assistant App: Grafana Synthetic Monitoring Probe with Browser Checks" \
    org.opencontainers.image.description="Run a browser-capable private Grafana Cloud Synthetic Monitoring probe from Home Assistant." \
    org.opencontainers.image.source="https://github.com/BroTEK-Solutions/ha-addons" \
    org.opencontainers.image.documentation="https://github.com/BroTEK-Solutions/ha-addons/blob/main/grafana_sm_browser/DOCS.md" \
    org.opencontainers.image.licenses="MIT"
