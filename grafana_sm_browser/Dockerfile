FROM golang:1.26.4-alpine3.22@sha256:727cfc3c40be55cd1bc9a4a059406b28a059857e3be752aa9d09531e12c20c56 AS launcher-builder

WORKDIR /src
COPY launcher/go.mod launcher/main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/ha-sm-launcher \
    .

FROM golang:1.26.4-alpine3.22@sha256:727cfc3c40be55cd1bc9a4a059406b28a059857e3be752aa9d09531e12c20c56 AS reporter-builder

WORKDIR /src
COPY reporter/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/ha-reporter \
    .

FROM grafana/synthetic-monitoring-agent:v0.63.0-browser@sha256:2678de4f78a317b6b7eecfec7c5e0a8800d1f95931653a51ec8577c57969bb36

USER root
COPY --from=launcher-builder --chown=0:0 --chmod=0500 \
    /out/ha-sm-launcher /usr/local/bin/ha-sm-launcher
COPY --from=reporter-builder --chown=0:0 --chmod=0555 \
    /out/ha-reporter /usr/local/bin/ha-reporter

# Names the App this probe publishes Home Assistant entities as. Both
# variants can be installed at once, so they must not share a device.
ENV REPORTER_APP=grafana_sm_browser

# This checks the local HTTP server, not /ready. Grafana Cloud connectivity is
# readiness state and must not turn an upstream outage into a restart loop.
HEALTHCHECK --start-period=30s --interval=30s --timeout=5s --retries=3 \
    CMD ["/usr/local/bin/ha-sm-launcher", "healthcheck"]

# Read Supervisor-owned options, then permanently drop to upstream UID/GID 12345.
# hadolint ignore=DL3002
USER root
ENTRYPOINT ["/usr/local/bin/ha-sm-launcher", "--with-tini"]

LABEL \
    org.opencontainers.image.title="Home Assistant App: Grafana Synthetic Monitoring Probe with Browser Checks" \
    org.opencontainers.image.description="Run a browser-capable private Grafana Cloud Synthetic Monitoring probe from Home Assistant." \
    org.opencontainers.image.source="https://github.com/BroTEK-Solutions/ha-addons" \
    org.opencontainers.image.documentation="https://github.com/BroTEK-Solutions/ha-addons/blob/main/grafana_sm_browser/DOCS.md" \
    org.opencontainers.image.licenses="MIT"
