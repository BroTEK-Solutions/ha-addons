#!/usr/bin/env python3
"""Generate the two Synthetic Monitoring Apps from one canonical source tree."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "synthetic_monitoring_shared"
TEMPLATE = SOURCE / "template"
VARIANTS = {
    "grafana_sm": {
        "APP_NAME": "Grafana Synthetic Monitoring Probe",
        "APP_DESCRIPTION": "Run a private Grafana Cloud Synthetic Monitoring probe from Home Assistant.",
        "APP_IMAGE": "ghcr.io/brotek-solutions/app-grafana-sm",
        "APP_SLUG": "grafana_sm",
        "TMPFS_LINE": "",
        "VARIANT_SENTENCE": "This is the smaller standard variant and does not include Chromium, so it cannot run browser checks.",
    },
    "grafana_sm_browser": {
        "APP_NAME": "Grafana Synthetic Monitoring Probe with Browser Checks",
        "APP_DESCRIPTION": "Run a browser-capable private Grafana Cloud Synthetic Monitoring probe from Home Assistant.",
        "APP_IMAGE": "ghcr.io/brotek-solutions/app-grafana-sm-browser",
        "APP_SLUG": "grafana_sm_browser",
        "TMPFS_LINE": "tmpfs: true\n",
        "VARIANT_SENTENCE": "This variant includes Chromium and can run every Synthetic Monitoring check type, including browser checks.",
    },
}


def render(template: str, values: dict[str, str]) -> str:
    template = template.replace("# {{TMPFS_LINE}}\n", values.get("TMPFS_LINE", ""))
    for key, value in values.items():
        if key == "TMPFS_LINE":
            continue
        template = template.replace("{{" + key + "}}", value)
    if "{{" in template or "}}" in template:
        raise ValueError("unresolved template marker")
    return template


def expected_files(slug: str, values: dict[str, str]) -> dict[Path, bytes]:
    result: dict[Path, bytes] = {}
    release_versions = json.loads((ROOT / ".release-please-manifest.json").read_text())
    values = {**values, "APP_VERSION": str(release_versions[slug])}
    for source in sorted(TEMPLATE.rglob("*")):
        if source.is_file():
            relative = source.relative_to(TEMPLATE)
            result[ROOT / slug / relative] = render(source.read_text(), values).encode()
    for source in sorted((SOURCE / "launcher").iterdir()):
        if source.is_file() and source.name != "main_test.go":
            result[ROOT / slug / "launcher" / source.name] = source.read_bytes()
    for source in sorted((SOURCE / "assets").iterdir()):
        if source.is_file():
            result[ROOT / slug / source.name] = source.read_bytes()
    return result


def synchronize(check: bool) -> int:
    stale: list[str] = []
    for slug, values in VARIANTS.items():
        app_dir = ROOT / slug
        expected = expected_files(slug, values)
        expected_paths = set(expected)
        existing_paths = {path for path in app_dir.rglob("*") if path.is_file()} if app_dir.exists() else set()
        # Dockerfiles remain variant-owned for Renovate. Changelogs remain
        # variant-owned because Release Please prepends package release notes.
        existing_paths.difference_update(
            {app_dir / "Dockerfile", app_dir / "CHANGELOG.md"}
        )
        for destination, content in expected.items():
            if destination.exists() and destination.read_bytes() == content:
                continue
            stale.append(str(destination.relative_to(ROOT)))
            if not check:
                destination.parent.mkdir(parents=True, exist_ok=True)
                destination.write_bytes(content)
        for unexpected in sorted(existing_paths - expected_paths):
            stale.append(str(unexpected.relative_to(ROOT)))
            if not check:
                unexpected.unlink()
        if not check:
            for directory in sorted(
                (path for path in app_dir.rglob("*") if path.is_dir()),
                reverse=True,
            ):
                if not any(directory.iterdir()):
                    directory.rmdir()

    if stale and check:
        print("Generated Synthetic Monitoring files are stale:", file=sys.stderr)
        for path in stale:
            print(f"  {path}", file=sys.stderr)
        print("Run: python3 scripts/sync_synthetic_monitoring_variants.py", file=sys.stderr)
        return 1
    if stale:
        print(f"Synchronized {len(stale)} generated files")
    else:
        print("Synthetic Monitoring variants already synchronized")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="fail instead of rewriting stale generated files")
    args = parser.parse_args()
    return synchronize(args.check)


if __name__ == "__main__":
    raise SystemExit(main())
