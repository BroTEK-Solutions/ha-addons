#!/usr/bin/env python3
"""Static contract checks for the Grafana Private Data Source Connect image."""

from __future__ import annotations

import re
import hashlib
import struct
import sys
from pathlib import Path


APP_DIR = Path(__file__).resolve().parents[1]
DOCKERFILE = APP_DIR / "Dockerfile"
APPARMOR = APP_DIR / "apparmor.txt"
ICON = APP_DIR / "icon.png"
LOGO = APP_DIR / "logo.png"
BRANDING = APP_DIR / "BRANDING.md"

def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def require_file(path: Path) -> str:
    if not path.is_file():
        fail(f"missing required packaging file: {path.relative_to(APP_DIR)}")
    return path.read_text()


def require(text: str, needle: str, message: str) -> None:
    if needle not in text:
        fail(message)


def main() -> None:
    dockerfile = require_file(DOCKERFILE)
    apparmor = require_file(APPARMOR)
    branding = require_file(BRANDING)

    def png_size(path: Path) -> tuple[int, int]:
        data = path.read_bytes()
        if data[:8] != b"\x89PNG\r\n\x1a\n":
            fail(f"{path.name} must be a PNG")
        return struct.unpack(">II", data[16:24])

    if png_size(ICON) != (128, 128):
        fail("official Grafana Cloud icon must be packaged at 128x128")
    if png_size(LOGO) != (250, 100):
        fail("official Grafana Cloud logo must be packaged at 250x100")
    old_assets = {
        "54890ff9c250ba605782435968e99f4b9f8a41347c55bad9a9471bc1756f1ba7",
        "c45827bef691d26cc423b0b1e8227e7d1c7696d38f5f4cd3275c281307d3f65f",
    }
    for asset in (ICON, LOGO):
        digest = hashlib.sha256(asset.read_bytes()).hexdigest()
        if digest in old_assets:
            fail(f"{asset.name} must replace the generated artwork")
    require(
        branding,
        "Grafana Cloud",
        "branding provenance must identify the official Grafana Cloud asset set",
    )

    if not re.search(
        r"^FROM grafana/pdc-agent:[^@\s]+@sha256:[0-9a-f]{64} AS pdc-source$",
        dockerfile,
        re.MULTILINE,
    ):
        fail("PDC source image must use a tag and immutable digest")
    if not re.search(
        r"^FROM ghcr\.io/home-assistant/base:[^@\s]+@sha256:[0-9a-f]{64}$",
        dockerfile,
        re.MULTILINE,
    ):
        fail("HA Alpine runtime base must use a tag and immutable digest")
    require(
        dockerfile,
        "COPY --from=pdc-source /usr/bin/pdc /usr/bin/pdc",
        "runtime must copy only /usr/bin/pdc from the upstream image",
    )
    require(dockerfile, "openssh-client", "runtime must install OpenSSH client")
    require(
        dockerfile,
        "apk upgrade --no-cache",
        "runtime must apply Alpine security updates within the pinned release",
    )
    require(
        dockerfile,
        "rm -f /usr/bin/tempio",
        "runtime must remove the unused inherited tempio binary",
    )
    require(
        dockerfile,
        "addgroup -S -g 30000 pdc",
        "runtime must create the upstream-compatible pdc group (GID 30000)",
    )
    require(
        dockerfile,
        "adduser -S -D -H -u 30000 -G pdc pdc",
        "runtime must create the upstream-compatible pdc user without a home directory",
    )
    require(dockerfile, "COPY rootfs /", "runtime rootfs must be copied into the image")
    require(dockerfile, "EXPOSE 8090/tcp", "only the local metrics endpoint may be exposed")
    if re.search(r"^\s*USER\s+", dockerfile, re.MULTILINE):
        fail("Dockerfile must not set USER; S6 must remain the container entrypoint user")
    if "io.hass.type" in dockerfile:
        fail("Dockerfile must not set Home Assistant app metadata labels")
    build_setup = dockerfile.split("COPY --from=pdc-source", maxsplit=1)[0]
    if re.search(r"\b(curl|wget)\b[^\n]*(https?|ftp)://", build_setup, re.IGNORECASE):
        fail("runtime must not download binaries from the web")

    healthcheck = re.search(r"^HEALTHCHECK\b(?P<command>[\s\S]*?)(?=^\S|\Z)", dockerfile, re.MULTILINE)
    if not healthcheck:
        fail("Dockerfile must define a Docker HEALTHCHECK")
    health_command = healthcheck.group("command")
    require(
        health_command,
        "http://127.0.0.1:8090/metrics",
        "healthcheck must probe the loopback metrics endpoint",
    )
    if "ready" in health_command.lower() or "startup" in health_command.lower():
        fail("healthcheck must be liveness-only, not readiness or startup gating")

    for label in (
        "org.opencontainers.image.title",
        "org.opencontainers.image.description",
        "org.opencontainers.image.source",
        "org.opencontainers.image.documentation",
        "org.opencontainers.image.licenses",
    ):
        require(dockerfile, label, f"Dockerfile must define OCI label {label}")

    require(apparmor, "#include <tunables/global>", "AppArmor must use HA's profile preamble")
    require(
        apparmor,
        "profile grafana_pdc flags=(attach_disconnected,mediate_deleted)",
        "AppArmor must define the grafana_pdc profile using HA's required flags",
    )
    outer_profile = apparmor.split("profile pdc", maxsplit=1)[0]
    require(
        outer_profile,
        "network inet stream,",
        "outer AppArmor profile must permit the loopback Docker health probe",
    )
    for rule in (
        "/init ix,",
        "/run/{s6,s6-rc*,service}/** ix,",
        "/usr/lib/bashio/** ix,",
        "/usr/bin/pdc cx -> pdc,",
        "/usr/bin/pdc mr,",
        "/usr/bin/ssh ix,",
        "/proc/[0-9]*/{stat,limits,cgroup} r,",
        "/proc/[0-9]*/mountinfo r,",
        "/proc/[0-9]*/net/netstat r,",
        "/proc/[0-9]*/fd/ r,",
        "/proc/sys/net/core/somaxconn r,",
        "/etc/ssh/** r,",
        "/data/ssh/** rwk,",
        "network,",
        "#include <abstractions/nameservice>",
        "signal (receive) peer=*_grafana_pdc,",
    ):
        require(apparmor, rule, f"AppArmor must permit the required least-privilege rule: {rule}")
    capabilities = set(re.findall(r"^\s*capability\s+([a-z0-9_]+),", apparmor, re.MULTILINE))
    if capabilities != {"chown", "kill", "setgid", "setuid"}:
        fail("AppArmor must grant only chown/kill/setgid/setuid needed by init and S6")
    for forbidden in ("/dev/**", "/proc/** rw", "/sys/**", "/mnt/**", "/media/**"):
        if forbidden in apparmor:
            fail(f"AppArmor must not grant host-wide access: {forbidden}")


if __name__ == "__main__":
    main()
