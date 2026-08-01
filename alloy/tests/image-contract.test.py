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
    "UI service run file exists": ui_run.is_file(),
    "UI service is a longrun": ui_type.is_file() and ui_type.read_text().strip() == "longrun",
    "UI starts independently of Alloy": not ui_dependency.exists(),
    "UI service is in the user bundle": ui_manifest.is_file(),
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(f"  {'✓' if ok else '✗'} {name}")
if failed:
    raise SystemExit(f"{len(failed)} image contract checks failed: {', '.join(failed)}")
print(f"all {len(checks)} image contract checks passed")
