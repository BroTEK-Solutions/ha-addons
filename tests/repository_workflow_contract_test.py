#!/usr/bin/env python3
"""Regression checks for the multi-App build and test orchestration."""

from __future__ import annotations

import json
import re
import struct
import sys
from datetime import date
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    caller_path = ROOT / ".github/workflows/builder.yaml"
    reusable_path = ROOT / ".github/workflows/build-app.yaml"
    release_workflow_path = ROOT / ".github/workflows/release.yaml"
    release_config_path = ROOT / "release-please-config.json"
    release_manifest_path = ROOT / ".release-please-manifest.json"
    if not caller_path.exists():
        fail("the repository needs a changed-App builder workflow")

    caller = caller_path.read_text()
    reusable = reusable_path.read_text()

    for release_file in (
        release_workflow_path,
        release_config_path,
        release_manifest_path,
    ):
        if not release_file.is_file():
            fail(f"release automation is missing {release_file.relative_to(ROOT)}")

    release_workflow = release_workflow_path.read_text()
    release_config = json.loads(release_config_path.read_text())
    release_manifest = json.loads(release_manifest_path.read_text())

    expected_versions = {
        app: str(yaml.safe_load((ROOT / app / "config.yaml").read_text())["version"])
        for app in ("alloy", "grafana_pdc")
    }
    if release_manifest != expected_versions:
        fail("Release Please manifest versions must match the published App metadata")
    if release_config.get("include-component-in-tag") is not True:
        fail("per-App Git tags must include the component name")
    if release_config.get("include-v-in-tag") is not True:
        fail("per-App Git tags must retain the conventional v prefix")
    if release_config.get("separate-pull-requests") is not False:
        fail("the two independent App releases must share one release PR")

    expected_components = {"alloy": "alloy", "grafana_pdc": "grafana-pdc"}
    packages = release_config.get("packages", {})
    if set(packages) != set(expected_components):
        fail("Release Please must manage exactly the current App directories")
    for app, component in expected_components.items():
        package = packages[app]
        if package.get("component") != component:
            fail(f"{app} must use release component {component}")
        if package.get("release-type") != "simple":
            fail(f"{app} must use the simple semantic release strategy")
        if package.get("extra-files") != [
            {"type": "yaml", "path": "config.yaml", "jsonpath": "$.version"}
        ]:
            fail(f"{app} release must update its config.yaml root version")

    release_action = (
        "googleapis/release-please-action@"
        "45996ed1f6d02564a971a2fa1b5860e934307cf7"
    )
    for required in (
        "workflow_call:",
        release_action,
        "token: ${{ secrets.RELEASE_PLEASE_TOKEN }}",
        "config-file: release-please-config.json",
        "manifest-file: .release-please-manifest.json",
        "contents: write",
        "issues: write",
        "pull-requests: write",
    ):
        if required not in release_workflow:
            fail(f"release workflow is missing required contract: {required}")

    if "pull_request:" not in caller or "branches:" in caller.split("pull_request:", 1)[1].split("push:", 1)[0]:
        fail("stacked pull requests must run the builder regardless of base branch")
    for app_file in ("config.yaml", "Dockerfile", "rootfs", "apparmor.txt"):
        if app_file not in caller:
            fail(f"changed-App detection must monitor {app_file}")
    for command in (
        "python3 tests/app_metadata_contract_test.py",
        "python3 tests/app_version_changed_test.py",
        "python3 tests/renovate_config_contract_test.py",
        "python3 tests/repository_workflow_contract_test.py",
        "python3 grafana_pdc/tests/config-schema.test.py",
        "bash grafana_pdc/tests/service.test.sh",
        "bash grafana_pdc/tests/image-smoke.test.sh",
        "python3 grafana_pdc/tests/image-contract.test.py",
    ):
        if command not in caller:
            fail(f"repository test gate must run: {command}")
    for required in (
        "BASHIO_BIN=/usr/bin/bashio",
        "grafana_pdc/Dockerfile",
        r"ghcr.io\/home-assistant\/base:",
        '"$ha_base"',
    ):
        if required not in caller:
            fail("PDC service tests must derive the pinned HA base from the App Dockerfile")
    for required in (
        "alloy/Dockerfile",
        r"ghcr.io\/home-assistant\/base-debian:",
        '"$alloy_base"',
    ):
        if required not in caller:
            fail("Alloy initialization tests must derive the pinned HA base from the App Dockerfile")
    if "needs: [init, test]" not in caller:
        fail("image builds must wait for the complete repository test gate")
    blanket_publish_guard = "github.event_name == 'push' && github.ref == 'refs/heads/main'"
    if blanket_publish_guard in caller:
        fail("an ordinary main push must not overwrite an existing stable App version")
    for required in (
        "python3 scripts/app_version_changed.py",
        "BASE_SHA: ${{ github.event.before }}",
        "HEAD_SHA: ${{ github.sha }}",
        "TRIGGER_REF: ${{ github.ref }}",
        '"$TRIGGER_REF" == "refs/heads/main"',
        "app: ${{ matrix.app.app }}",
        "publish: ${{ matrix.app.publish }}",
        "uses: ./.github/workflows/release.yaml",
        "needs: [init, test, build-app]",
        "needs.build-app.result == 'success' || needs.build-app.result == 'skipped'",
        "issues: write",
        "RELEASE_PLEASE_TOKEN: ${{ secrets.RELEASE_PLEASE_TOKEN }}",
    ):
        if required not in caller:
            fail(f"builder is missing immutable-release contract: {required}")
    if "secrets: inherit" in caller:
        fail("the release workflow must receive only its named release token")
    if 'EVENT_NAME: ${{ github.event_name }}' not in caller:
        fail("changed-App selection must receive the triggering event name")
    if '"$EVENT_NAME" == "workflow_dispatch"' not in caller:
        fail("manual workflow runs must build every App")
    init_job = caller.split("\n  init:\n", 1)[1].split("\n  build-app:\n", 1)[0]
    if "fetch-depth: 0" not in init_job:
        fail("version comparison must fetch the base revision in the init job")

    security = (ROOT / ".github/workflows/security.yml").read_text()
    trusted_pr_guard = (
        "github.event_name != 'pull_request' || "
        "github.event.pull_request.head.repo.full_name == github.repository"
    )
    if trusted_pr_guard not in security:
        fail("container security scans must run before merge on trusted pull requests")

    if "workflow_call:" not in reusable:
        fail("build-app.yaml must be a reusable per-App workflow")
    for input_name in ("app", "publish"):
        if re.search(rf"^\s{{6}}{input_name}:\s*$", reusable, re.MULTILINE) is None:
            fail(f"reusable build workflow must accept {input_name}")
    if 'context: "./${{ inputs.app }}"' not in reusable:
        fail("reusable build workflow must build its selected App directory")
    if "if: inputs.publish" not in reusable:
        fail("manifest publication must honor the caller's publish decision")

    for workflow in (caller, reusable, release_workflow):
        for action in re.findall(r"^\s*uses:\s+([^\s#]+)", workflow, re.MULTILINE):
            if action.startswith("./"):
                continue
            if re.search(r"@[0-9a-f]{40}$", action) is None:
                fail(f"GitHub Action must use an immutable commit SHA: {action}")

    # Ensure both files are valid YAML despite the special `on` key.
    yaml.safe_load(caller)
    yaml.safe_load(reusable)
    yaml.safe_load(release_workflow)

    trivy_config = yaml.safe_load((ROOT / "trivy.yaml").read_text())
    if trivy_config != {"ignorefile": ".trivyignore.yaml"}:
        fail("Trivy must load only the repository's scoped YAML ignore file")
    trivy_ignores = yaml.safe_load((ROOT / ".trivyignore.yaml").read_text())
    expected_root_paths = {"alloy/Dockerfile", "grafana_pdc/Dockerfile"}
    if set(trivy_ignores) != {"misconfigurations"}:
        fail("Trivy ignores must not suppress vulnerabilities or secrets")
    root_ignores = trivy_ignores["misconfigurations"]
    if len(root_ignores) != 1 or root_ignores[0].get("id") != "AVD-DS-0002":
        fail("Trivy may ignore only the intentional S6 root-bootstrap finding")
    if set(root_ignores[0].get("paths", [])) != expected_root_paths:
        fail("the S6 root exception must be scoped to the two current App Dockerfiles")
    if root_ignores[0].get("expired_at") != date(2026, 10, 31):
        fail("the S6 root exception must expire for review on 2026-10-31")
    if not root_ignores[0].get("statement"):
        fail("the S6 root exception must record its rationale")

    readme = (ROOT / "README.md").read_text()
    for release_documentation in (
        "generated release PR",
        "alloy-vX.Y.Z",
        "grafana-pdc-vX.Y.Z",
        "Conventional Commits",
        "Do not edit an App version manually",
    ):
        if release_documentation not in readme:
            fail(f"README is missing release guidance: {release_documentation}")

    for app in ("alloy", "grafana_pdc"):
        app_dir = ROOT / app
        for filename in ("README.md", "DOCS.md", "CHANGELOG.md", "translations/en.yaml"):
            if not (app_dir / filename).is_file():
                fail(f"{app} is missing presentation file {filename}")
        for filename, expected in (("icon.png", (128, 128)), ("logo.png", (250, 100))):
            data = (app_dir / filename).read_bytes()
            if data[:8] != b"\x89PNG\r\n\x1a\n":
                fail(f"{app}/{filename} must be a PNG")
            dimensions = struct.unpack(">II", data[16:24])
            if dimensions != expected:
                fail(f"{app}/{filename} is {dimensions}, expected {expected}")


if __name__ == "__main__":
    main()
