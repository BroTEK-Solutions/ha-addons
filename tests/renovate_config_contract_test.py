#!/usr/bin/env python3
"""Regression checks for safe, CI-gated dependency automation."""

from __future__ import annotations

import json
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def matching_rule(rules: list[dict], description: str) -> dict:
    for rule in rules:
        if rule.get("description") == description:
            return rule
    fail(f"Renovate rule is missing: {description}")


def main() -> None:
    config = json.loads((ROOT / "renovate.json").read_text())

    if config.get("platformAutomerge") is not False:
        fail("Renovate must perform automerge itself so optional CI checks finish first")
    if config.get("automergeType") != "pr":
        fail("Renovate must merge through pull requests")
    if config.get("internalChecksFilter") != "strict":
        fail("Renovate must hold updates until internal checks pass")
    if config.get("minimumReleaseAge") != "12 hours":
        fail("dependencies need the fleet-standard 12-hour release cooldown")
    if config.get("minimumReleaseAgeBehaviour") != "timestamp-required":
        fail("dependencies without reliable timestamps must not be held indefinitely")
    if set(config.get("labels", [])) != {"dependencies", "renovate"}:
        fail("Renovate PRs need the standard dependency labels")

    rules = config.get("packageRules", [])
    builder = matching_rule(
        rules,
        "Keep the App build, manifest, and multi-architecture matrix actions on one Home Assistant builder release.",
    )
    if builder.get("matchPackageNames") != ["home-assistant/builder"]:
        fail("Home Assistant builder sub-actions must match Renovate's normalized package name")
    helpers = matching_rule(
        rules,
        "Keep Home Assistant metadata and App discovery helpers on one actions revision.",
    )
    if helpers.get("matchPackageNames") != ["home-assistant/actions"]:
        fail("Home Assistant helper sub-actions must match Renovate's normalized package name")

    if any(
        "rknightion/.github" in rule.get("matchDepNames", [])
        and rule.get("enabled") is False
        for rule in rules
    ):
        fail("release-pinned shared security workflows must remain updateable")

    actions = matching_rule(rules, "Automerge GitHub Actions after CI")
    if actions.get("matchManagers") != ["github-actions"] or actions.get("automerge") is not True:
        fail("GitHub Action updates must automerge after CI")
    if actions.get("addLabels") != ["dep:gha"] or "labels" in actions:
        fail("GitHub Action classification must preserve the standard labels")

    app_dependencies = matching_rule(rules, "Automerge non-major App dependencies after CI")
    expected_packages = {
        "grafana/alloy",
        "grafana/pdc-agent",
        "ghcr.io/home-assistant/base",
        "ghcr.io/home-assistant/base-debian",
    }
    if set(app_dependencies.get("matchPackageNames", [])) != expected_packages:
        fail("the App dependency automerge allowlist has drifted")
    if set(app_dependencies.get("matchUpdateTypes", [])) != {"digest", "pin", "patch", "minor"}:
        fail("only non-major App dependency updates may automerge")
    if app_dependencies.get("automerge") is not True:
        fail("allowlisted non-major App dependencies must automerge after CI")
    if app_dependencies.get("addLabels") != ["dep:low-risk"] or "labels" in app_dependencies:
        fail("App dependency classification must preserve the standard labels")

    runtimes = matching_rule(rules, "Review CI runtime updates manually")
    if set(runtimes.get("matchPackageNames", [])) != {"python", "ubuntu"}:
        fail("Python and runner-image updates must remain manual")
    if runtimes.get("automerge") is not False:
        fail("CI runtime updates must not automerge")
    if runtimes.get("addLabels") != ["dep:toolchain"] or "labels" in runtimes:
        fail("CI runtime classification must preserve the standard labels")


if __name__ == "__main__":
    main()
