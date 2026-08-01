#!/usr/bin/env python3
"""Regression checks for the multi-App build and test orchestration."""

from __future__ import annotations

import re
import struct
import sys
from datetime import date
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
HA_BASE = (
    "ghcr.io/home-assistant/base:3.23@"
    "sha256:445077433f08d1e352285ff22ee2594f6b93ac3260dcf84433c79cd849a946a0"
)


def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    caller_path = ROOT / ".github/workflows/builder.yaml"
    reusable_path = ROOT / ".github/workflows/build-app.yaml"
    if not caller_path.exists():
        fail("the repository needs a changed-App builder workflow")

    caller = caller_path.read_text()
    reusable = reusable_path.read_text()

    if "pull_request:" not in caller or "branches:" in caller.split("pull_request:", 1)[1].split("push:", 1)[0]:
        fail("stacked pull requests must run the builder regardless of base branch")
    for app_file in ("config.yaml", "Dockerfile", "rootfs", "apparmor.txt"):
        if app_file not in caller:
            fail(f"changed-App detection must monitor {app_file}")
    for command in (
        "python3 tests/app_metadata_contract_test.py",
        "python3 tests/repository_workflow_contract_test.py",
        "python3 grafana_pdc/tests/config-schema.test.py",
        "bash grafana_pdc/tests/service.test.sh",
        "bash grafana_pdc/tests/image-smoke.test.sh",
        "python3 grafana_pdc/tests/image-contract.test.py",
    ):
        if command not in caller:
            fail(f"repository test gate must run: {command}")
    if "BASHIO_BIN=/usr/bin/bashio" not in caller or HA_BASE not in caller:
        fail("PDC service tests must run with real Bashio in the pinned HA base")
    if "needs: [init, test]" not in caller:
        fail("image builds must wait for the complete repository test gate")
    publish_guard = "github.event_name == 'push' && github.ref == 'refs/heads/main'"
    if publish_guard not in caller:
        fail("only pushes to main may publish App images")
    if 'EVENT_NAME: ${{ github.event_name }}' not in caller:
        fail("changed-App selection must receive the triggering event name")
    if '"$EVENT_NAME" == "workflow_dispatch"' not in caller:
        fail("manual workflow runs must build every App")

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

    for workflow in (caller, reusable):
        for action in re.findall(r"^\s*uses:\s+([^\s#]+)", workflow, re.MULTILINE):
            if action.startswith("./"):
                continue
            if re.search(r"@[0-9a-f]{40}$", action) is None:
                fail(f"GitHub Action must use an immutable commit SHA: {action}")

    # Ensure both files are valid YAML despite the special `on` key.
    yaml.safe_load(caller)
    yaml.safe_load(reusable)

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
