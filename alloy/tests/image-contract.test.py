#!/usr/bin/env python3
"""Check the Alloy image and S6 wiring needed by the ingress UI."""

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
dockerfile = (ROOT / "Dockerfile").read_text()
alloy_run = (ROOT / "rootfs/etc/s6-overlay/s6-rc.d/alloy/run").read_text()
ui_run = ROOT / "rootfs/etc/s6-overlay/s6-rc.d/alloy-ui/run"
ui_type = ROOT / "rootfs/etc/s6-overlay/s6-rc.d/alloy-ui/type"
ui_dependency = ROOT / "rootfs/etc/s6-overlay/s6-rc.d/alloy-ui/dependencies.d/alloy"
ui_manifest = ROOT / "rootfs/etc/s6-overlay/s6-rc.d/user/contents.d/alloy-ui"
apparmor = (ROOT / "apparmor.txt").read_text()
apparmor_rules = [line.strip() for line in apparmor.splitlines() if not line.lstrip().startswith("#")]

checks = {
    "BUILD_FROM is declared before every build stage": dockerfile.index("ARG BUILD_FROM")
    < dockerfile.index("FROM golang:"),
    "pinned Go builder": "FROM golang:1.26-bookworm@sha256:" in dockerfile,
    "UI is built from its module": "COPY ui/ ./" in dockerfile and "go build" in dockerfile,
    "UI binary is copied into runtime": "/usr/bin/alloy-ui" in dockerfile,
    "health checks the unprivileged ingress health endpoint": "127.0.0.1:8099/healthz" in dockerfile,
    "health does not depend on Alloy readiness": "127.0.0.1:12345/-/ready" not in dockerfile,
    "Alloy serves its UI below the proxy prefix": "--server.http.ui-path-prefix=/alloy" in alloy_run,
    "Alloy debug listener is loopback-only behind ingress": "--server.http.listen-addr=127.0.0.1:12345" in alloy_run,
    # One source of truth for --stability.level: the Native App option, so it
    # covers Fleet pipelines and the manual override, and stays reachable when a
    # raised-stability configuration stops Alloy from starting.
    "stability level comes from the Native App option": "bashio::config 'alloy_stability_level'"
    in alloy_run,
    "stability level is not read from the ingress settings store": "setting alloy_stability_level"
    not in alloy_run,
    "UI service run file exists": ui_run.is_file(),
    "UI service is a longrun": ui_type.is_file() and ui_type.read_text().strip() == "longrun",
    "UI starts independently of Alloy": not ui_dependency.exists(),
    "UI service is in the user bundle": ui_manifest.is_file(),
    "AppArmor uses Home Assistant's profile preamble": "#include <tunables/global>" in apparmor,
    "AppArmor declares the alloy profile with the required flags": "profile alloy flags=(attach_disconnected,mediate_deleted)"
    in apparmor,
    # `file,` is shorthand for `/ rwmlk,`. AppArmor permissions are cumulative,
    # so one such rule makes every write rule below it decorative and there is
    # no way to claw the permission back. This profile has no inner profile to
    # fall back on, so the outer one is the whole confinement.
    "AppArmor grants no unrestricted file rule": "file," not in apparmor_rules,
    "AppArmor keeps reads and memory-mapping broad": "/{,**} rm," in apparmor_rules,
    # The writable set is the point of the profile: everything else is read-only.
    "AppArmor writes are confined to App state": {rule for rule in apparmor_rules if " rw" in rule or " w" in rule}
    == {
        "/etc/services.d/** rwix,",
        "/etc/cont-init.d/** rwix,",
        "/etc/cont-finish.d/** rwix,",
        "/run/{,**} rwk,",
        "/dev/tty rw,",
        "/dev/null rw,",
        "/tmp/** rwk,",
        "/data/ rw,",
        "/data/** rwk,",
        "/etc/alloy/ rw,",
        "/etc/alloy/** rwk,",
    },
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(f"  {'✓' if ok else '✗'} {name}")
if failed:
    raise SystemExit(f"{len(failed)} image contract checks failed: {', '.join(failed)}")
print(f"all {len(checks)} image contract checks passed")
