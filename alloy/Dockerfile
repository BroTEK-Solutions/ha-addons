ARG BUILD_FROM=ghcr.io/home-assistant/base-debian:bookworm@sha256:60cd882d5200157082e568a0b775cca0d9cb5704452ada3cf6f206c4dd6d83c5

FROM golang:1.27-bookworm@sha256:ded31c68586d2e49e760acc2e65a884b23d032e9bbbed0ae0c55abd3fcaf4452 AS ui-builder

WORKDIR /src
COPY ui/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/alloy-ui .

FROM golang:1.27-bookworm@sha256:ded31c68586d2e49e760acc2e65a884b23d032e9bbbed0ae0c55abd3fcaf4452 AS reporter-builder

WORKDIR /src
COPY reporter/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ha-reporter .

FROM ${BUILD_FROM}

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl bash jq unzip ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ARG TARGETARCH
# The Alloy version lives here and nowhere else. The test suite parses it out of
# this file so the validation container always matches the shipped binary.
RUN set -eux; \
    # renovate: datasource=github-releases depName=grafana/alloy
    ALLOY_VERSION="1.19.0"; \
    case "${TARGETARCH}" in \
        amd64)  ARCH="linux-amd64" ;; \
        arm64) ARCH="linux-arm64" ;; \
        *) echo "Unsupported arch: ${TARGETARCH}"; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/grafana/alloy/releases/download/v${ALLOY_VERSION}/alloy-${ARCH}.zip" -o /tmp/alloy.zip; \
    unzip /tmp/alloy.zip -d /tmp/alloy-extract; \
    mv /tmp/alloy-extract/alloy-${ARCH} /usr/bin/alloy; \
    chmod +x /usr/bin/alloy; \
    rm -rf /tmp/alloy.zip /tmp/alloy-extract

COPY rootfs /
COPY --from=ui-builder /out/alloy-ui /usr/bin/alloy-ui
COPY --from=reporter-builder /out/ha-reporter /usr/bin/ha-reporter

# Supervisor reads the container's Docker health status and surfaces it as the
# add-on's health, which is what the user-facing Watchdog toggle acts on. This
# replaces the `watchdog:` config key, which is obsolete.
HEALTHCHECK --start-period=30s --interval=30s --timeout=5s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8099/healthz >/dev/null || exit 1

EXPOSE 8099/tcp

ARG BUILD_DATE
LABEL \
    org.opencontainers.image.title="Home Assistant App: Grafana Alloy" \
    org.opencontainers.image.description="Ship HAOS logs to Loki and host metrics to Prometheus using Grafana Alloy" \
    org.opencontainers.image.source="https://github.com/BroTEK-Solutions/ha-addons" \
    org.opencontainers.image.documentation="https://github.com/BroTEK-Solutions/ha-addons/blob/main/alloy/DOCS.md" \
    org.opencontainers.image.licenses="MIT"
