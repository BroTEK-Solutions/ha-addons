FROM golang:1.24.6-alpine3.22@sha256:c8c5f95d64aa79b6547f3b626eb84b16a7ce18a139e3e9ca19a8c078b85ba80d AS launcher-builder

WORKDIR /src
COPY launcher/go.mod launcher/main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/ha-sm-launcher \
    .

FROM grafana/synthetic-monitoring-agent:v0.62.1@sha256:d38b4940d747ec380d8e8ef512a1fe694412a763b1ca906a94ff76f4d4a3e560

USER root
COPY --from=launcher-builder --chown=12345:12345 --chmod=0500 \
    /out/ha-sm-launcher /usr/local/bin/ha-sm-launcher

# This checks the local HTTP server, not /ready. Grafana Cloud connectivity is
# readiness state and must not turn an upstream outage into a restart loop.
HEALTHCHECK --start-period=30s --interval=30s --timeout=5s --retries=3 \
    CMD ["/usr/local/bin/ha-sm-launcher", "healthcheck"]

USER sm
ENTRYPOINT ["/usr/local/bin/ha-sm-launcher"]

LABEL \
    org.opencontainers.image.title="Home Assistant App: Grafana Synthetic Monitoring Probe" \
    org.opencontainers.image.description="Run a private Grafana Cloud Synthetic Monitoring probe from Home Assistant." \
    org.opencontainers.image.source="https://github.com/BroTEK-Solutions/ha-addons" \
    org.opencontainers.image.documentation="https://github.com/BroTEK-Solutions/ha-addons/blob/main/grafana_sm/DOCS.md" \
    org.opencontainers.image.licenses="MIT"
