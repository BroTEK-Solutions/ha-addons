FROM golang:1.26.4-alpine3.22@sha256:727cfc3c40be55cd1bc9a4a059406b28a059857e3be752aa9d09531e12c20c56 AS reporter-builder

WORKDIR /src
COPY reporter/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -buildid=" -o /out/ha-reporter .

FROM golang:1.26.4-alpine3.22@sha256:727cfc3c40be55cd1bc9a4a059406b28a059857e3be752aa9d09531e12c20c56 AS ui-builder

WORKDIR /src
COPY ui/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -buildid=" -o /out/grafana-pdc-ui .

FROM grafana/pdc-agent:0.0.63@sha256:3d8346ee5ec30aeaaae5f82db2a77f8347405fbfc4e4a3248ae26d65dd8bf031 AS pdc-source

FROM ghcr.io/home-assistant/base:3.24@sha256:93ef607824e3f27e868f11b10938283a98bf880ed57bcf8eaa81c6c2d521f6f5

# The digest pins the HA base and Alpine release. Apply security updates from
# that release on every rebuild, and keep OpenSSH as a virtual package so its
# patched build is selected. tempio is inherited but unused by this App; remove
# it to avoid shipping its otherwise unreachable Go dependency surface.
# hadolint ignore=DL3017,DL3018
RUN apk upgrade --no-cache \
    && apk add --no-cache openssh-client \
    && rm -f /usr/bin/tempio \
    && addgroup -S -g 30000 pdc \
    && adduser -S -D -H -u 30000 -G pdc pdc

COPY --from=pdc-source /usr/bin/pdc /usr/bin/pdc
COPY --from=reporter-builder /out/ha-reporter /usr/bin/ha-reporter
COPY --from=ui-builder /out/grafana-pdc-ui /usr/bin/grafana-pdc-ui
COPY rootfs /

# This is deliberately a liveness probe. Startup and tunnel readiness are
# controlled by S6 and must not turn a transient upstream failure into a
# container restart loop.
HEALTHCHECK --start-period=30s --interval=30s --timeout=5s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8090/metrics >/dev/null || exit 1

EXPOSE 8090/tcp

LABEL \
    org.opencontainers.image.title="Home Assistant App: Grafana Private Data Source Connect" \
    org.opencontainers.image.description="Connect Grafana Cloud to private Home Assistant network resources" \
    org.opencontainers.image.source="https://github.com/BroTEK-Solutions/ha-addons" \
    org.opencontainers.image.documentation="https://github.com/BroTEK-Solutions/ha-addons/blob/main/grafana_pdc/DOCS.md" \
    org.opencontainers.image.licenses="MIT"
