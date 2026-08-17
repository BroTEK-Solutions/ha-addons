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

    if config.get("automerge") is not True:
        fail("supported dependency updates must automerge after required CI passes")
    if config.get("platformAutomerge") is not True:
        fail("GitHub must complete automerge as soon as branch protection passes")
    if config.get("automergeType") != "pr":
        fail("Renovate must merge through pull requests")
    if config.get("rebaseWhen") != "behind-base-branch":
        fail("Renovate branches must refresh whenever main advances")
    if config.get("internalChecksFilter") != "strict":
        fail("Renovate must hold updates until internal checks pass")
    if "minimumReleaseAge" in config or "minimumReleaseAgeBehaviour" in config:
        fail("time-only release-age checks must not delay green dependency updates")
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

    devcontainer = matching_rule(
        rules,
        "Keep the development container on the Home Assistant supported major",
    )
    if devcontainer.get("matchManagers") != ["devcontainer"]:
        fail("the development-container constraint must not affect other managers")
    if devcontainer.get("matchPackageNames") != [
        "ghcr.io/home-assistant/devcontainer"
    ]:
        fail("the development-container constraint must target the official image")
    if devcontainer.get("allowedVersions") != "<6":
        fail("the development container must stay on the officially supported v5 major")

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

    app_dependencies = matching_rule(rules, "Classify App dependency updates")
    expected_packages = {
        "grafana/alloy",
        "grafana/pdc-agent",
        "grafana/synthetic-monitoring-agent",
        "ghcr.io/home-assistant/base",
        "ghcr.io/home-assistant/base-debian",
    }
    if set(app_dependencies.get("matchPackageNames", [])) != expected_packages:
        fail("the App dependency automerge allowlist has drifted")
    if "matchUpdateTypes" in app_dependencies:
        fail("App dependency classification must include major updates for CI-gated automerge")
    if app_dependencies.get("addLabels") != ["dep:app"] or "labels" in app_dependencies:
        fail("App dependency classification must preserve the standard labels")
    if app_dependencies.get("semanticCommitType") != "fix":
        fail("App dependency updates must trigger a Release Please patch release")
    if app_dependencies.get("semanticCommitScope") != "deps":
        fail("App dependency release commits must retain the deps scope")

    runtimes = matching_rule(rules, "Classify CI runtime updates")
    if set(runtimes.get("matchDepNames", [])) != {"python", "ubuntu"}:
        fail("CI runtime classification must cover Python and runner images")
    if "matchPackageNames" in runtimes:
        fail("CI runtime rules must match extracted dependency names, not registry package names")
    if runtimes.get("addLabels") != ["dep:toolchain"] or "labels" in runtimes:
        fail("CI runtime classification must preserve the standard labels")

    if any(rule.get("automerge") is False for rule in rules):
        fail("package rules must not bypass green-CI automerge")

    check_ignore_paths(config.get("ignorePaths", []))


# The generated App copies must be ignored, because the synchronizers rewrite
# them and would revert any Renovate PR. The modules those copies are generated
# FROM must not be: a wildcard that swallows one takes the code every App
# compiles from out of Renovate's reach, and the only symptom is a transitive
# CVE that is never proposed.
GENERATED_MODULES = (
    "alloy/reporter",
    "grafana_pdc/reporter",
    "grafana_sm/reporter",
    "grafana_sm/launcher",
    "grafana_sm/ui",
    "grafana_sm_browser/reporter",
    "grafana_sm_browser/launcher",
    "grafana_sm_browser/ui",
)
SOURCE_MODULES = (
    "shared/reporter",
    "synthetic_monitoring_shared/launcher",
    "synthetic_monitoring_shared/ui",
    "alloy/ui",
    "grafana_pdc/ui",
)


def check_ignore_paths(patterns: list[str]) -> None:
    from fnmatch import fnmatch

    def ignored(module: str) -> bool:
        # Renovate matches ignorePaths against the manifest path, not the
        # directory, so test the file Renovate would actually extract from.
        return any(fnmatch(f"{module}/go.mod", pattern) for pattern in patterns)

    for module in GENERATED_MODULES:
        if not (ROOT / module / "go.mod").exists():
            fail(f"the generated module {module} no longer exists; update this test")
        if not ignored(module):
            fail(f"Renovate must ignore the generated copy {module}")
    for module in SOURCE_MODULES:
        if not (ROOT / module / "go.mod").exists():
            fail(f"the source module {module} no longer exists; update this test")
        if ignored(module):
            fail(f"Renovate must keep managing the source module {module}")


if __name__ == "__main__":
    main()
