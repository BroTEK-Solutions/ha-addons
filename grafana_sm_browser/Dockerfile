FROM golang:1.27.4-alpine3.23@sha256:f70850ef6ce341e8b6427f677049145d63c0b7ce002d4d7e3d464828b1b96dcd AS launcher-builder

WORKDIR /src
COPY launcher/go.mod launcher/main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/ha-sm-launcher \
    .

FROM golang:1.27.4-alpine3.23@sha256:f70850ef6ce341e8b6427f677049145d63c0b7ce002d4d7e3d464828b1b96dcd AS ui-builder

WORKDIR /src
COPY ui/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/ha-sm-ui \
    .

FROM golang:1.27.4-alpine3.23@sha256:f70850ef6ce341e8b6427f677049145d63c0b7ce002d4d7e3d464828b1b96dcd AS reporter-builder

WORKDIR /src
COPY reporter/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/ha-reporter \
    .

FROM grafana/synthetic-monitoring-agent:v0.71.0-browser@sha256:7b8067ff7aaf08a322f8d65538250790bb0ebe66dd219158559d5323b2b9aea9

USER root
COPY --from=launcher-builder --chown=0:0 --chmod=0500 \
    /out/ha-sm-launcher /usr/local/bin/ha-sm-launcher
COPY --from=reporter-builder --chown=0:0 --chmod=0555 \
    /out/ha-reporter /usr/local/bin/ha-reporter
COPY --from=ui-builder --chown=0:0 --chmod=0555 \
    /out/ha-sm-ui /usr/local/bin/ha-sm-ui

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
