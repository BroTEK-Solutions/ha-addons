#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = ["pyyaml"]
# ///
"""Regression checks for the published Home Assistant App image contract."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[1]
APP_DIR = REPOSITORY_ROOT / "alloy"


def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    config = yaml.safe_load((APP_DIR / "config.yaml").read_text())
    if "{arch}" in config["image"]:
        fail("config.yaml image must use one generic multi-architecture image name")
    if config["image"] != "ghcr.io/brotek-solutions/app-alloy":
        fail("config.yaml image must publish ghcr.io/brotek-solutions/app-alloy")

    if (APP_DIR / "build.yaml").exists():
        fail("Apps must define their build base in Dockerfile, not alloy/build.yaml")

    dockerfile = (APP_DIR / "Dockerfile").read_text()
    if "io.hass.type=addon" in dockerfile:
        fail("Dockerfile must not label the image as the obsolete add-on type")
    if "ghcr.io/home-assistant/base-debian:bookworm@sha256:" not in dockerfile:
        fail("Alloy must retain its Debian HA base and pin the multi-arch digest")
    if "apk add" in dockerfile:
        fail("the build migration must not change Alloy from Debian to Alpine")
    for label in (
        "org.opencontainers.image.title",
        "org.opencontainers.image.description",
        "org.opencontainers.image.source",
        "org.opencontainers.image.documentation",
        "org.opencontainers.image.licenses",
    ):
        if label not in dockerfile:
            fail(f"Dockerfile must retain OCI label {label}")

    legacy_workflow = REPOSITORY_ROOT / ".github/workflows/build.yaml"
    if legacy_workflow.exists():
        fail("legacy builder workflow must be replaced by composite App build actions")

    workflow = REPOSITORY_ROOT / ".github/workflows/build-app.yaml"
    if not workflow.exists():
        fail("App build workflow is missing")
    workflow_text = workflow.read_text()
    for required_action in (
        "home-assistant/builder/actions/prepare-multi-arch-matrix@4de35182ce1e329181bffcbcc84d33db5e2c7e10",
        "home-assistant/builder/actions/build-image@4de35182ce1e329181bffcbcc84d33db5e2c7e10",
        "home-assistant/builder/actions/publish-multi-arch-manifest@4de35182ce1e329181bffcbcc84d33db5e2c7e10",
    ):
        if required_action not in workflow_text:
            fail(f"App build workflow must use {required_action}")
    if "io.hass.type=app" not in workflow_text:
        fail("App build workflow must label images as io.hass.type=app")
    if workflow_text.count('cosign: "true"') < 2:
        fail("App build workflow must explicitly enable Cosign for images and manifests")
    if "push: ${{ inputs.publish }}" not in workflow_text:
        fail("the reusable workflow must use the caller's publication decision")
    if "if: inputs.publish" not in workflow_text:
        fail("manifest publication must use the caller's publication decision")
    for output in ("image", "version", "name", "description", "url"):
        json_variable = f"{output.upper()}_JSON"
        if f'{output}=$(jq -r . <<< "${json_variable}")' not in workflow_text:
            fail(f"{output} must decode the helper's JSON scalar before reuse")
    for output in ("version", "name", "description", "url"):
        expected = f"{output}: ${{{{ steps.normalize.outputs.{output} }}}}"
        if expected not in workflow_text:
            fail(f"prepare output {output} must use normalized metadata")
    for action_reference in re.findall(r"^\s*uses:\s+([^\s#]+)", workflow_text, re.MULTILINE):
        if not re.search(r"@[0-9a-f]{40}$", action_reference):
            fail(f"GitHub Action must use an immutable commit SHA: {action_reference}")

    renovate = json.loads((REPOSITORY_ROOT / "renovate.json").read_text())
    if not any(
        "home-assistant/builder" in rule.get("matchPackageNames", [])
        for rule in renovate.get("packageRules", [])
    ):
        fail("Renovate must track the Home Assistant builder composite actions")

    devcontainer = json.loads((REPOSITORY_ROOT / ".devcontainer.json").read_text())
    if devcontainer.get("image") != "ghcr.io/home-assistant/devcontainer:5-apps":
        fail("the development container must use the official Home Assistant Apps image")


if __name__ == "__main__":
    main()
