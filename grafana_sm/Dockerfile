FROM golang:1.27.3-alpine3.23@sha256:5d5113301ffcac84e2bfb20c42648803fecaea6789c89bb9b66884e18cbb30a7 AS launcher-builder

WORKDIR /src
COPY launcher/go.mod launcher/main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/ha-sm-launcher \
    .

FROM golang:1.27.3-alpine3.23@sha256:5d5113301ffcac84e2bfb20c42648803fecaea6789c89bb9b66884e18cbb30a7 AS ui-builder

WORKDIR /src
COPY ui/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/ha-sm-ui \
    .

FROM golang:1.27.3-alpine3.23@sha256:5d5113301ffcac84e2bfb20c42648803fecaea6789c89bb9b66884e18cbb30a7 AS reporter-builder

WORKDIR /src
COPY reporter/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/ha-reporter \
    .

FROM grafana/synthetic-monitoring-agent:v0.71.0@sha256:6ffda60334b28e58c652edd1d2d1a777aaee030be05a78feae88b53bcad9adbe

USER root
COPY --from=launcher-builder --chown=0:0 --chmod=0500 \
    /out/ha-sm-launcher /usr/local/bin/ha-sm-launcher
COPY --from=reporter-builder --chown=0:0 --chmod=0555 \
    /out/ha-reporter /usr/local/bin/ha-reporter
COPY --from=ui-builder --chown=0:0 --chmod=0555 \
    /out/ha-sm-ui /usr/local/bin/ha-sm-ui

# Names the App this probe publishes Home Assistant entities as. Both
# variants can be installed at once, so they must not share a device.
ENV REPORTER_APP=grafana_sm

# This checks the local HTTP server, not /ready. Grafana Cloud connectivity is
# readiness state and must not turn an upstream outage into a restart loop.
HEALTHCHECK --start-period=30s --interval=30s --timeout=5s --retries=3 \
    CMD ["/usr/local/bin/ha-sm-launcher", "healthcheck"]

# Read Supervisor-owned options, then permanently drop to upstream UID/GID 12345.
# hadolint ignore=DL3002
USER root
ENTRYPOINT ["/usr/local/bin/ha-sm-launcher"]

LABEL \
    org.opencontainers.image.title="Home Assistant App: Grafana Synthetic Monitoring Probe" \
    org.opencontainers.image.description="Run a private Grafana Cloud Synthetic Monitoring probe from Home Assistant." \
    org.opencontainers.image.source="https://github.com/BroTEK-Solutions/ha-addons" \
    org.opencontainers.image.documentation="https://github.com/BroTEK-Solutions/ha-addons/blob/main/grafana_sm/DOCS.md" \
    org.opencontainers.image.licenses="MIT"
