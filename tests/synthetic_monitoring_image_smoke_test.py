#!/usr/bin/env python3
"""Smoke-test a complete generated Synthetic Monitoring App image."""

from __future__ import annotations

import json
import os
import socketserver
import subprocess
import sys
import tempfile
import threading
import time
import uuid
from pathlib import Path


def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def docker(*args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["docker", *args],
        text=True,
        capture_output=True,
        check=check,
    )


class HoldOpenTCPServer(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


class HoldOpenHandler(socketserver.BaseRequestHandler):
    def handle(self) -> None:
        time.sleep(15)


def smoke_launcher(image: str, variant: str) -> None:
    token = "smoke-test-token"
    container = f"ha-sm-{variant}-{uuid.uuid4().hex[:12]}"
    api_server = HoldOpenTCPServer(("0.0.0.0", 0), HoldOpenHandler)
    server_thread = threading.Thread(target=api_server.serve_forever, daemon=True)
    server_thread.start()
    api_address = f"host.docker.internal:{api_server.server_address[1]}"
    with tempfile.TemporaryDirectory(prefix="ha-sm-options-") as directory:
        options_path = Path(directory) / "options.json"
        options_path.write_text(
            json.dumps(
                {
                    "api_token": token,
                    "api_server_address": api_address,
                    "log_level": "warn",
                    "allow_private_networks": False,
                    "disable_usage_reports": True,
                }
            )
        )
        os.chmod(options_path, 0o644)
        try:
            docker(
                "run",
                "--detach",
                "--name",
                container,
                "--add-host",
                "host.docker.internal:host-gateway",
                "--volume",
                f"{directory}:/data:ro",
                image,
            )
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                health = docker(
                    "exec",
                    container,
                    "/usr/local/bin/ha-sm-launcher",
                    "healthcheck",
                    check=False,
                )
                if health.returncode == 0:
                    break
                time.sleep(0.25)
            else:
                fail(f"{variant} launcher did not expose its liveness endpoint")

            process_list = docker("top", container).stdout
            logs = docker("logs", container).stdout
            if token in process_list or token in logs:
                fail(f"{variant} launcher exposed its API token")
            if f"--api-server-address={api_address}" not in process_list:
                fail(f"{variant} launcher did not pass the configured API address")
        finally:
            docker("rm", "--force", container, check=False)
            api_server.shutdown()
            api_server.server_close()


def main() -> None:
    if len(sys.argv) != 3 or sys.argv[2] not in {"standard", "browser"}:
        fail(f"usage: {sys.argv[0]} IMAGE standard|browser")
    image, variant = sys.argv[1:]
    inspect = json.loads(docker("image", "inspect", image).stdout)[0]["Config"]

    if inspect["User"] != "sm":
        fail(f"image user is {inspect['User']!r}, expected non-root sm")
    expected_entrypoint = (
        ["/usr/local/bin/ha-sm-launcher"]
        if variant == "standard"
        else ["tini", "--", "/usr/local/bin/ha-sm-launcher"]
    )
    if inspect["Entrypoint"] != expected_entrypoint:
        fail(f"entrypoint is {inspect['Entrypoint']!r}, expected {expected_entrypoint!r}")
    expected_health = ["CMD", "/usr/local/bin/ha-sm-launcher", "healthcheck"]
    if inspect["Healthcheck"]["Test"] != expected_health:
        fail(f"healthcheck is {inspect['Healthcheck']['Test']!r}, expected {expected_health!r}")

    environment = set(inspect.get("Env") or [])
    browser_env = "K6_BROWSER_ARGS=no-sandbox,disable-dev-shm-usage"
    if (browser_env in environment) != (variant == "browser"):
        fail(f"{variant} image has unexpected browser environment: {sorted(environment)!r}")

    version = docker(
        "run",
        "--rm",
        "--entrypoint",
        "/usr/local/bin/synthetic-monitoring-agent",
        image,
        "--version",
    )
    if "synthetic-monitoring-agent version=" not in version.stdout:
        fail(f"agent version output was unexpected: {version.stdout!r}")

    missing_options = docker(
        "run",
        "--rm",
        "--entrypoint",
        "/usr/local/bin/ha-sm-launcher",
        image,
        check=False,
    )
    combined = missing_options.stdout + missing_options.stderr
    if missing_options.returncode == 0 or "open App options" not in combined:
        fail(f"launcher did not fail safely without /data/options.json: {combined!r}")

    smoke_launcher(image, variant)

    print(f"{variant} Synthetic Monitoring image smoke checks passed")


if __name__ == "__main__":
    main()
