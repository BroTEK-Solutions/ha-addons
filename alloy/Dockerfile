ARG BUILD_FROM
FROM ${BUILD_FROM}

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl bash jq unzip ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ARG BUILD_ARCH
# The Alloy version lives here and nowhere else. The test suite parses it out of
# this file so the validation container always matches the shipped binary.
RUN set -eux; \
    # renovate: datasource=github-releases depName=grafana/alloy
    ALLOY_VERSION="1.17.0"; \
    case "${BUILD_ARCH}" in \
        amd64)  ARCH="linux-amd64" ;; \
        aarch64) ARCH="linux-arm64" ;; \
        *) echo "Unsupported arch: ${BUILD_ARCH}"; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/grafana/alloy/releases/download/v${ALLOY_VERSION}/alloy-${ARCH}.zip" -o /tmp/alloy.zip; \
    unzip /tmp/alloy.zip -d /tmp/alloy-extract; \
    mv /tmp/alloy-extract/alloy-${ARCH} /usr/bin/alloy; \
    chmod +x /usr/bin/alloy; \
    rm -rf /tmp/alloy.zip /tmp/alloy-extract

COPY rootfs /

# Supervisor reads the container's Docker health status and surfaces it as the
# add-on's health, which is what the user-facing Watchdog toggle acts on. This
# replaces the `watchdog:` config key, which is obsolete.
HEALTHCHECK --start-period=30s --interval=30s --timeout=5s --retries=3 \
    CMD curl -fsS http://127.0.0.1:12345/-/ready || exit 1

ARG BUILD_DATE
ARG BUILD_VERSION
LABEL \
    io.hass.name="Grafana Alloy" \
    io.hass.description="Ship HAOS logs to Loki and host metrics to Prometheus via Grafana Alloy" \
    io.hass.arch="${BUILD_ARCH}" \
    io.hass.type="addon" \
    io.hass.version=${BUILD_VERSION}
