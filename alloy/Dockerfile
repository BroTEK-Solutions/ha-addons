ARG BUILD_FROM=ghcr.io/home-assistant/base-debian:bookworm@sha256:8c7a9e207425e79b6b2ed1628a2b6727fa6e518d9fdddcbe3b1ac20440e70492

FROM golang:1.26-bookworm@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651 AS ui-builder

WORKDIR /src
COPY ui/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/alloy-ui .

FROM ${BUILD_FROM}

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl bash jq unzip ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ARG TARGETARCH
# The Alloy version lives here and nowhere else. The test suite parses it out of
# this file so the validation container always matches the shipped binary.
RUN set -eux; \
    # renovate: datasource=github-releases depName=grafana/alloy
    ALLOY_VERSION="1.18.0"; \
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

# Supervisor reads the container's Docker health status and surfaces it as the
# add-on's health, which is what the user-facing Watchdog toggle acts on. This
# replaces the `watchdog:` config key, which is obsolete.
HEALTHCHECK --start-period=30s --interval=30s --timeout=5s --retries=3 \
    CMD curl -fsS http://127.0.0.1:12345/-/ready >/dev/null \
    && curl -fsS http://127.0.0.1:8099/healthz >/dev/null || exit 1

EXPOSE 8099/tcp

ARG BUILD_DATE
LABEL \
    org.opencontainers.image.title="Home Assistant App: Grafana Alloy" \
    org.opencontainers.image.description="Ship HAOS logs to Loki and host metrics to Prometheus using Grafana Alloy" \
    org.opencontainers.image.source="https://github.com/BroTEK-Solutions/ha-addons" \
    org.opencontainers.image.documentation="https://github.com/BroTEK-Solutions/ha-addons/blob/main/alloy/DOCS.md" \
    org.opencontainers.image.licenses="MIT"
