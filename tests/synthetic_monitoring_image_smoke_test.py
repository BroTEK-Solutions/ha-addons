#!/usr/bin/env python3
"""Smoke-test a complete generated Synthetic Monitoring App image."""

from __future__ import annotations

import json
import socketserver
import subprocess
import sys
import threading
import time
import uuid


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
    data_volume = f"ha-sm-data-{variant}-{uuid.uuid4().hex[:12]}"
    api_server = HoldOpenTCPServer(("0.0.0.0", 0), HoldOpenHandler)
    server_thread = threading.Thread(target=api_server.serve_forever, daemon=True)
    server_thread.start()
    if sys.platform == "linux":
        api_host = "127.0.0.1"
        network_args = ["--network", "host"]
    else:
        api_host = "host.docker.internal"
        network_args = ["--add-host", "host.docker.internal:host-gateway"]
    api_address = f"{api_host}:{api_server.server_address[1]}"
    options_json = json.dumps(
        {
            "api_token": token,
            "api_server_address": api_address,
            "log_level": "warn",
            "allow_private_networks": False,
            "disable_usage_reports": True,
        }
    )
    try:
        docker("volume", "create", data_volume)
        docker(
            "run",
            "--rm",
            "--user",
            "root",
            "--env",
            f"OPTIONS_JSON={options_json}",
            "--volume",
            f"{data_volume}:/data",
            "--entrypoint",
            "/bin/sh",
            image,
            "-c",
            'chmod 0700 /data && printf "%s" "$OPTIONS_JSON" > /data/options.json && chmod 0600 /data/options.json',
        )
        try:
            docker(
                "run",
                "--detach",
                "--name",
                container,
                "--cap-drop",
                "ALL",
                "--cap-add",
                "NET_RAW",
                "--cap-add",
                "SETGID",
                "--cap-add",
                "SETUID",
                *network_args,
                "--volume",
                f"{data_volume}:/data:ro",
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
                state = docker(
                    "inspect",
                    "--format",
                    "{{json .State}}",
                    container,
                    check=False,
                )
                logs = docker("logs", container, check=False)
                fail(
                    f"{variant} launcher did not expose its liveness endpoint; "
                    f"state={state.stdout.strip()!r}; "
                    f"logs={(logs.stdout + logs.stderr).strip()!r}"
                )

            process_list = docker("top", container).stdout
            logs = docker("logs", container).stdout
            if token in process_list or token in logs:
                fail(f"{variant} launcher exposed its API token")
            if f"--api-server-address={api_address}" not in process_list:
                fail(f"{variant} launcher did not pass the configured API address")
            for process in process_list.splitlines()[1:]:
                uid = process.split(maxsplit=1)[0]
                if uid in {"0", "root"}:
                    fail(f"{variant} left a runtime process as root: {process!r}")
            capabilities = docker(
                "exec",
                "--user",
                "12345:12345",
                container,
                "/bin/sh",
                "-c",
                'for proc in /proc/[0-9]*; do '
                'case "$(cat "$proc/comm" 2>/dev/null)" in synthetic-monit*) '
                'grep "^CapEff:" "$proc/status";; esac; done; true',
            ).stdout.split()
            if capabilities != ["CapEff:", "0000000000002000"]:
                fail(
                    f"{variant} agent effective capabilities are {capabilities!r}, "
                    "expected only CAP_NET_RAW"
                )
        finally:
            docker("rm", "--force", container, check=False)
    finally:
        docker("volume", "rm", "--force", data_volume, check=False)
        api_server.shutdown()
        api_server.server_close()


def main() -> None:
    if len(sys.argv) != 3 or sys.argv[2] not in {"standard", "browser"}:
        fail(f"usage: {sys.argv[0]} IMAGE standard|browser")
    image, variant = sys.argv[1:]
    inspect = json.loads(docker("image", "inspect", image).stdout)[0]["Config"]

    if inspect["User"] not in {"", "root"}:
        fail(f"image must start its configuration launcher as root, got {inspect['User']!r}")
    expected_entrypoint = (
        ["/usr/local/bin/ha-sm-launcher"]
        if variant == "standard"
        else ["/usr/local/bin/ha-sm-launcher", "--with-tini"]
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
