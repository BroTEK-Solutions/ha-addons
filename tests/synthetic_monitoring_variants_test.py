#!/usr/bin/env python3
"""Contracts for generated Grafana Synthetic Monitoring App variants."""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
VARIANTS = {
    "grafana_sm": {
        "name": "Grafana Synthetic Monitoring Probe",
        "image": "ghcr.io/brotek-solutions/app-grafana-sm",
        "tmpfs": False,
    },
    "grafana_sm_browser": {
        "name": "Grafana Synthetic Monitoring Probe with Browser Checks",
        "image": "ghcr.io/brotek-solutions/app-grafana-sm-browser",
        "tmpfs": True,
    },
}


def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    if (ROOT / "synthetic_monitoring_shared/template/CHANGELOG.md").exists():
        fail("release-managed changelogs must not be generator-owned")

    sync = ROOT / "scripts/sync_synthetic_monitoring_variants.py"
    result = subprocess.run(
        [sys.executable, str(sync), "--check"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        fail(f"generated variants are stale: {result.stdout}{result.stderr}")

    configs: dict[str, dict] = {}
    dockerfiles: dict[str, str] = {}
    upstream_versions: dict[str, str] = {}
    for slug, expected in VARIANTS.items():
        app_dir = ROOT / slug
        config = yaml.safe_load((app_dir / "config.yaml").read_text())
        configs[slug] = config
        for key in ("name", "image"):
            if config[key] != expected[key]:
                fail(f"{slug} {key} is {config[key]!r}, expected {expected[key]!r}")
        if config["slug"] != slug:
            fail(f"{slug} metadata must retain its directory slug")
        if config["arch"] != ["aarch64", "amd64"]:
            fail(f"{slug} must support exactly aarch64 and amd64")
        if config.get("privileged") != ["NET_RAW"]:
            fail(f"{slug} must request only NET_RAW for ICMP checks")
        for forbidden in ("full_access", "host_ipc", "host_network", "host_pid"):
            if config.get(forbidden):
                fail(f"{slug} must not enable {forbidden}")
        if bool(config.get("tmpfs", False)) != expected["tmpfs"]:
            fail(f"{slug} tmpfs must be {expected['tmpfs']}")
        if config.get("apparmor", True) is False:
            fail(f"{slug} must retain Supervisor AppArmor confinement")
        for deprecated in ("enable_adhoc_checks", "enable_traceroute"):
            if deprecated in config["schema"]:
                fail(f"{slug} must not expose obsolete agent option {deprecated}")

        dockerfile = (app_dir / "Dockerfile").read_text()
        dockerfiles[slug] = dockerfile
        upstream = re.search(
            r"^FROM grafana/synthetic-monitoring-agent:(v[^@\s]+)@sha256:([0-9a-f]{64})$",
            dockerfile,
            re.MULTILINE,
        )
        if upstream is None:
            fail(f"{slug} must pin a versioned upstream image by digest")
        upstream_versions[slug] = upstream.group(1)
        if "USER root" not in dockerfile:
            fail(f"{slug} launcher must start as root to read Supervisor-owned options")
        if "SM_AGENT_API_TOKEN" in dockerfile:
            fail(f"{slug} Dockerfile must not contain probe credentials")

        for filename in (
            "CHANGELOG.md",
            "DOCS.md",
            "README.md",
            "icon.png",
            "logo.png",
            "translations/en.yaml",
            "launcher/go.mod",
            "launcher/main.go",
        ):
            if not (app_dir / filename).is_file():
                fail(f"{slug} is missing generated file {filename}")

    shared_keys = ("options", "schema", "ports", "ports_description")
    standard = configs["grafana_sm"]
    browser = configs["grafana_sm_browser"]
    expected_browser_version = f"{upstream_versions['grafana_sm']}-browser"
    if upstream_versions["grafana_sm_browser"] != expected_browser_version:
        fail("standard and browser upstream images must use the same agent version")
    for key in shared_keys:
        if standard.get(key) != browser.get(key):
            fail(f"the two variants drifted in shared config key {key}")

    normalized: dict[str, str] = {}
    for slug, expected in VARIANTS.items():
        text = dockerfiles[slug]
        text = re.sub(
            r"FROM grafana/synthetic-monitoring-agent:v[^@\s]+@sha256:[0-9a-f]{64}",
            "FROM {{UPSTREAM_IMAGE}}@{{UPSTREAM_DIGEST}}",
            text,
            count=1,
        )
        replacements = {
            expected["name"]: "{{APP_NAME}}",
            expected["image"]: "{{APP_IMAGE}}",
            slug: "{{APP_SLUG}}",
        }
        entrypoint = (
            '["/usr/local/bin/ha-sm-launcher"]'
            if slug == "grafana_sm"
            else '["/usr/local/bin/ha-sm-launcher", "--with-tini"]'
        )
        replacements[entrypoint] = "{{ENTRYPOINT}}"
        for old, new in replacements.items():
            text = text.replace(old, new)
        text = text.replace(
            "Run a private Grafana Cloud Synthetic Monitoring probe from Home Assistant.",
            "{{APP_DESCRIPTION}}",
        ).replace(
            "Run a browser-capable private Grafana Cloud Synthetic Monitoring probe from Home Assistant.",
            "{{APP_DESCRIPTION}}",
        )
        normalized[slug] = text
    if normalized["grafana_sm"] != normalized["grafana_sm_browser"]:
        fail("variant-owned Dockerfiles drifted outside their image, identity and entrypoint")

    print("Synthetic Monitoring variants are generated and synchronized")


if __name__ == "__main__":
    main()
