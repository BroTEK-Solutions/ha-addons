FROM golang:1.27.4-alpine3.23@sha256:f70850ef6ce341e8b6427f677049145d63c0b7ce002d4d7e3d464828b1b96dcd AS reporter-builder

WORKDIR /src
COPY reporter/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -buildid=" -o /out/ha-reporter .

FROM golang:1.27.4-alpine3.23@sha256:f70850ef6ce341e8b6427f677049145d63c0b7ce002d4d7e3d464828b1b96dcd AS ui-builder

WORKDIR /src
COPY ui/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -buildid=" -o /out/grafana-pdc-ui .

FROM grafana/pdc-agent:0.0.69@sha256:32332a5d4605598d091ecaec7f977c2aecf3bdb89fb424328b3062f6c5d40561 AS pdc-source

FROM ghcr.io/home-assistant/base:3.24@sha256:5021aac8a4b7f2a1397e23a9749dfe9f736463ffa471df6b200216c68d530fe6

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
