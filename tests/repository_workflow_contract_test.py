#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = ["pyyaml"]
# ///
"""Regression checks for the multi-App build and test orchestration."""

from __future__ import annotations

import ast
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


def third_party_imports(source: str) -> set[str]:
    """Top-level imports this repository does not ship and Python does not bundle."""
    modules: set[str] = set()
    for node in ast.walk(ast.parse(source)):
        if isinstance(node, ast.Import):
            modules.update(alias.name.split(".", 1)[0] for alias in node.names)
        elif isinstance(node, ast.ImportFrom) and node.level == 0 and node.module:
            modules.add(node.module.split(".", 1)[0])
    return {module for module in modules if module not in sys.stdlib_module_names}


def check_python_invocation(justfile: str) -> None:
    """Nothing may depend on a package that happens to be installed on the machine.

    A script that imports outside the standard library declares those imports in
    a PEP 723 header and is run with `uv run`, so it carries its own environment.
    A pure-stdlib script stays on bare `python3` and needs nothing. The failure
    this pins is silent: adding `import yaml` to a `python3` script works on a
    machine that already has it and breaks everywhere else, and pip cannot fix
    that on macOS at all, because PEP 668 refuses `pip install --user` against
    the system Python.
    """
    for script in sorted(ROOT.glob("**/*.py")):
        if ".git" in script.parts:
            continue
        relative = script.relative_to(ROOT).as_posix()
        source = script.read_text()
        declared = "# /// script" in source
        imports = third_party_imports(source)
        if imports and not declared:
            fail(f"{relative} imports {sorted(imports)} without a PEP 723 header")
        if declared and not imports:
            fail(f"{relative} declares dependencies it does not import")
        if declared:
            # Only the text between the markers, so a dependency name that also
            # appears in the body cannot stand in for a missing declaration.
            header = source.split("# /// script", 1)[1].split("# ///", 1)[0]
            for module in imports:
                wanted = "pyyaml" if module == "yaml" else module
                if f'"{wanted}"' not in header:
                    fail(f"{relative} imports {module} without declaring {wanted}")
            lock = script.with_name(script.name + ".lock")
            if not lock.is_file():
                fail(f"{relative} declares dependencies without a committed lockfile")
        if relative not in justfile:
            continue
        wanted, unwanted = ("uv run --locked", "python3") if imports else ("python3", "uv run")
        if f"{wanted} {relative}" not in justfile:
            fail(f"the justfile must invoke {relative} with `{wanted}`")
        if f"{unwanted} {relative}" in justfile:
            fail(f"the justfile must not invoke {relative} with `{unwanted}`")

    for path in (
        "justfile",
        ".github/workflows/builder.yaml",
        "scripts/cloud-environment-setup.sh",
    ):
        if "pip install" in (ROOT / path).read_text():
            fail(f"{path} must provision Python through uv, not pip")


def main() -> None:
    caller_path = ROOT / ".github/workflows/builder.yaml"
    reusable_path = ROOT / ".github/workflows/build-app.yaml"
    release_workflow_path = ROOT / ".github/workflows/release.yaml"
    release_config_path = ROOT / "release-please-config.json"
    release_manifest_path = ROOT / ".release-please-manifest.json"
    justfile_path = ROOT / "justfile"
    if not caller_path.exists():
        fail("the repository needs a changed-App builder workflow")
    if not justfile_path.is_file():
        fail("the repository needs a justfile task surface")

    caller = caller_path.read_text()
    reusable = reusable_path.read_text()
    justfile = justfile_path.read_text()
    caller_config = yaml.safe_load(caller)
    jobs = caller_config["jobs"]

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
        for app in ("alloy", "grafana_pdc", "grafana_sm", "grafana_sm_browser")
    }
    if release_manifest != expected_versions:
        fail("Release Please manifest versions must match the published App metadata")
    if release_config.get("include-component-in-tag") is not True:
        fail("per-App Git tags must include the component name")
    if release_config.get("include-v-in-tag") is not True:
        fail("per-App Git tags must retain the conventional v prefix")
    if release_config.get("separate-pull-requests") is not False:
        fail("the independent App releases must share one release PR")
    if release_config.get("group-pull-request-title-pattern") not in (
        None,
        "chore: release ${branch}",
    ):
        fail("grouped release PR titles must remain parseable by Release Please")

    expected_components = {
        "alloy": "alloy",
        "grafana_pdc": "grafana-pdc",
        "grafana_sm": "grafana-sm",
        "grafana_sm_browser": "grafana-sm-browser",
    }
    packages = release_config.get("packages", {})
    if set(packages) != set(expected_components):
        fail("Release Please must manage exactly the current App directories")
    for app, component in expected_components.items():
        package = packages[app]
        if package.get("component") != component:
            fail(f"{app} must use release component {component}")
        if package.get("release-type") != "simple":
            fail(f"{app} must use the simple semantic release strategy")
        expected_extra_file = (
            {"type": "generic", "path": "config.yaml"}
            if app in {"grafana_sm", "grafana_sm_browser"}
            else {"type": "yaml", "path": "config.yaml", "jsonpath": "$.version"}
        )
        if package.get("extra-files") != [expected_extra_file]:
            fail(f"{app} release must update its config.yaml root version")

    for app in ("grafana_sm", "grafana_sm_browser"):
        config_text = (ROOT / app / "config.yaml").read_text()
        if not re.search(
            r"^version:\s+\S+\s+# x-release-please-version$",
            config_text,
            re.MULTILINE,
        ):
            fail(f"{app} version must use the formatting-preserving release annotation")

    release_action = (
        "googleapis/release-please-action@"
        "45996ed1f6d02564a971a2fa1b5860e934307cf7"
    )
    for required in (
        "workflow_call:",
        release_action,
        "token: ${{ steps.bao.outputs.token }}",
        "rknightion/.github/.github/actions/broker-token@"
        "79a72d215e806c12876526ff30fbd524250e1bf9 # v1.17.1",
        "permission-set: release-please-ha-addons",
        "id-token: write",
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
    test_lanes = {
        "quality",
        "pdc-test",
        "alloy-generator-test",
        "alloy-init-test",
        "synthetic-monitoring-test",
    }
    if "test" in jobs or not test_lanes.issubset(jobs):
        fail("independent repository and App tests must use five parallel jobs")

    def job_text(job_name: str) -> str:
        return json.dumps(jobs[job_name], sort_keys=True)

    expected_recipes = {
        "quality": (
            "just fmt-check",
            "just lint",
            "just gen-check",
            "just test-repo",
        ),
        "pdc-test": (
            "just test-pdc",
            "just test-pdc-image",
        ),
        "alloy-generator-test": (
            "just test-alloy",
            "just test-alloy-image",
        ),
        "alloy-init-test": ("just test-alloy-init",),
        "synthetic-monitoring-test": (
            "just test-sm",
            "just test-sm-image",
        ),
    }
    for job_name, recipes in expected_recipes.items():
        lane = job_text(job_name)
        for recipe in recipes:
            if recipe not in lane:
                fail(f"{job_name} must run: {recipe}")
        for step in jobs[job_name].get("steps", []):
            run = step.get("run")
            if isinstance(run, str) and run.startswith("just ") and not run.endswith(" </dev/null"):
                fail(f"{job_name} must run just with stdin from /dev/null")

    setup_just = (
        "uses: extractions/setup-just@"
        "53165ef7e734c5c07cb06b3c8e7b647c5aa16db3 # v4"
    )
    if caller.count(setup_just) != len(expected_recipes):
        fail("each test lane must install the pinned just v4 action")
    if caller.count("just-version: '1.58.0'") != len(expected_recipes):
        fail("each test lane must pin just 1.58.0")
    setup_uv = "astral-sh/setup-uv@20cfd1bf945f4377ade1205e4dbc17946fc9a30d"
    if caller.count(setup_uv) != caller.count("version: '0.12.7'"):
        fail("every setup-uv step must pin an explicit uv version, or CI floats to latest")

    expected_justfile_commands = (
        "uvx yamllint@{{ yamllint_version }} --strict .",
        'yamllint_version := "1.38.0"',
        "shellcheck -x --source-path=SCRIPTDIR",
        "bash tests/shared_validate_lib_test.sh",
        "(cd shared/reporter && go test ./...)",
        "uv run --locked tests/app_metadata_contract_test.py",
        "uv run --locked tests/app_contract_test.py",
        "python3 tests/app_version_changed_test.py",
        "python3 tests/renovate_config_contract_test.py",
        "uv run --locked tests/repository_workflow_contract_test.py",
        "uv run --locked tests/synthetic_monitoring_variants_test.py",
        "python3 scripts/sync_shared_lib.py --check",
        "python3 scripts/sync_synthetic_monitoring_variants.py --check",
        "uv run --locked grafana_pdc/tests/config-schema.test.py",
        "python3 grafana_pdc/tests/image-contract.test.py",
        "(cd grafana_pdc/ui && go test ./...)",
        "bash grafana_pdc/tests/service.test.sh",
        "just image grafana_pdc local/ha-grafana-pdc:smoke",
        "bash grafana_pdc/tests/image-smoke.test.sh local/ha-grafana-pdc:smoke",
        "uv run --locked alloy/tests/config-schema.test.py",
        "python3 alloy/tests/image-contract.test.py",
        "(cd alloy/ui && go test ./...)",
        "node alloy/tests/ui-static.test.mjs",
        "bash alloy/tests/generate-config.test.sh",
        "just image alloy local/ha-alloy:smoke",
        "bash alloy/tests/image-smoke.test.sh local/ha-alloy:smoke",
        "bash alloy/tests/init-alloy.test.sh",
        "(cd synthetic_monitoring_shared/launcher && go test ./...)",
        "just image grafana_sm local/ha-grafana-sm:smoke",
        "just image grafana_sm_browser local/ha-grafana-sm-browser:smoke",
        "python3 tests/synthetic_monitoring_image_smoke_test.py local/ha-grafana-sm:smoke standard",
        "python3 tests/synthetic_monitoring_image_smoke_test.py local/ha-grafana-sm-browser:smoke browser",
    )
    for command in expected_justfile_commands:
        if command not in justfile:
            fail(f"justfile must run: {command}")
    check_python_invocation(justfile)

    # Every image build routes through the `image` recipe, which is the only
    # place the layer cache is configured. Both halves of it are load-bearing.
    # The buildx half needs a docker-container builder and the two Actions cache
    # variables, none of which exist on a developer laptop; the `docker build`
    # half is what that laptop runs, and it must not acquire a buildx dependency.
    # Losing either one fails silently - an uncached CI build is still green, and
    # so is a laptop that happens to have buildx installed.
    for required in (
        'if [ -n "${ACTIONS_RUNTIME_TOKEN:-}" ] && [ -n "${ACTIONS_RESULTS_URL:-}" ]; then',
        "--cache-from 'type=gha,scope={{ app }}'",
        "--cache-to 'type=gha,mode=min,ignore-error=true,scope={{ app }}'",
        "docker buildx build --load",
        "docker build --tag '{{ tag }}' '{{ app }}'",
    ):
        if required not in justfile:
            fail(f"the cached `image` recipe must keep: {required}")
    # Line-anchored so a comment mentioning docker build cannot trip this.
    if len(re.findall(r"^\s+docker (?:buildx )?build ", justfile, re.MULTILINE)) != 2:
        fail("image builds must go through the `image` recipe, not a second docker build")

    # ACTIONS_RESULTS_URL and ACTIONS_RUNTIME_TOKEN reach `uses:` steps only, so
    # the action exporting them into GITHUB_ENV is what makes `type=gha`
    # reachable from a `run:` step at all, and the docker-container builder is
    # what makes the cache exportable. Both must sit in the lane that builds the
    # Alloy image, ahead of the build step, or the cache is silently skipped.
    cache_actions = (
        "crazy-max/ghaction-github-runtime@04d248b84655b509d8c44dc1d6f990c879747487 # v4.0.0",
        "docker/setup-buildx-action@37fe631027851001ddb9b187196cc803df7f5f0e # v4.3.0",
    )
    for pinned in cache_actions:
        if pinned not in caller:
            fail(f"the Actions layer cache must stay pinned to {pinned}")
    # Every lane that builds an image through the `image` recipe caches it. A
    # lane that loses either action still goes green, it just silently builds
    # uncached, so the pair is asserted per lane rather than once for the file.
    for job_id, build_command in (
        ("alloy-generator-test", "just test-alloy-image"),
        ("synthetic-monitoring-test", "just test-sm-image"),
        ("pdc-test", "just test-pdc-image"),
    ):
        lane_steps = jobs[job_id]["steps"]
        build_step = next(
            index
            for index, step in enumerate(lane_steps)
            if str(step.get("run", "")).startswith(build_command)
        )
        for pinned in cache_actions:
            action = pinned.split(" #", 1)[0]
            positions = [
                index
                for index, step in enumerate(lane_steps)
                if step.get("uses") == action
            ]
            if not positions:
                fail(f"{action} must run in {job_id}")
            elif positions[0] > build_step:
                fail(f"{action} must run before {build_command}, not after it")

    for required in (
        "BASHIO_BIN=/usr/bin/bashio",
        "grafana_pdc/Dockerfile",
        r"ghcr.io\/home-assistant\/base:",
        '"$ha_base"',
    ):
        if required not in justfile:
            fail("PDC service tests must derive the pinned HA base from the App Dockerfile")
    for required in (
        "alloy/Dockerfile",
        r"ARG BUILD_FROM=ghcr.io\/home-assistant\/base-debian:",
        '"$alloy_base"',
    ):
        if required not in justfile:
            fail("Alloy initialization tests must derive the pinned HA base from the App Dockerfile")
    complete_test_gate = {"init", *test_lanes}
    if set(jobs["build-app"].get("needs", [])) != complete_test_gate:
        fail("image builds must wait for all repository and App test lanes")
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
        "issues: write",
        "TS_WIF_CLIENT_ID: ${{ secrets.TS_WIF_CLIENT_ID }}",
        "TS_WIF_AUDIENCE: ${{ secrets.TS_WIF_AUDIENCE }}",
    ):
        if required not in caller:
            fail(f"builder is missing immutable-release contract: {required}")
    if "secrets: inherit" in caller:
        fail("the release workflow must receive only the identifiers it needs")
    # The release credential is minted per run from the OpenBao broker on camden
    # and scoped to this repository alone. RELEASE_PLEASE_TOKEN was revoked
    # estate-wide and is unrecoverable, so a workflow reaching for it again does
    # not fail loudly - release-please receives an empty token and the run dies
    # somewhere less obvious. Pin its absence instead.
    for workflow_text, label in ((caller, "builder"), (release_workflow, "release")):
        if "RELEASE_PLEASE_TOKEN" in workflow_text:
            fail(f"{label} workflow must mint a broker token, not read RELEASE_PLEASE_TOKEN")
    if 'EVENT_NAME: ${{ github.event_name }}' not in caller:
        fail("changed-App selection must receive the triggering event name")
    if '"$EVENT_NAME" == "workflow_dispatch"' not in caller:
        fail("manual workflow runs must build every App")
    init_job = caller.split("\n  init:\n", 1)[1].split("\n  build-app:\n", 1)[0]
    if "fetch-depth: 0" not in init_job:
        fail("version comparison must fetch the base revision in the init job")

    security = (ROOT / ".github/workflows/security.yml").read_text()
    security_triggers = security.split("\npermissions:", 1)[0]
    for direct_trigger in ("\n  push:", "\n  pull_request:"):
        if direct_trigger in security_triggers:
            fail("security checks must run once through the required Build Apps gate")
    aggregate_gate = {"init", *test_lanes, "build-app", "security"}
    if set(jobs["ci-success"].get("needs", [])) != aggregate_gate:
        fail("ci-success must aggregate every build, security, and test result")
    if set(jobs["release"].get("needs", [])) != aggregate_gate:
        fail("release automation must wait for the same complete gate")
    release_condition = jobs["release"].get("if", "")
    for result in (*sorted(test_lanes), "init", "security"):
        if f"needs.{result}.result == 'success'" not in release_condition:
            fail(f"release automation must require successful {result}")
    if not all(
        required in release_condition
        for required in (
            "needs.build-app.result == 'success'",
            "needs.build-app.result == 'skipped'",
        )
    ):
        fail("release automation must accept only successful or skipped App builds")

    for required in (
        "workflow_call:",
        "uses: ./.github/workflows/security.yml",
        "name: ci-success",
    ):
        if required not in security + caller:
            fail(f"required CI gate is missing security contract: {required}")
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
    expected_root_paths = {
        "alloy/Dockerfile",
        "grafana_pdc/Dockerfile",
        "grafana_sm/Dockerfile",
        "grafana_sm_browser/Dockerfile",
    }
    if set(trivy_ignores) != {"misconfigurations"}:
        fail("Trivy ignores must not suppress vulnerabilities or secrets")
    root_ignores = trivy_ignores["misconfigurations"]
    if len(root_ignores) != 1 or root_ignores[0].get("id") != "AVD-DS-0002":
        fail("Trivy may ignore only the intentional S6 root-bootstrap finding")
    if set(root_ignores[0].get("paths", [])) != expected_root_paths:
        fail("the root-startup exception must be scoped to the four current App Dockerfiles")
    if root_ignores[0].get("expired_at") != date(2026, 10, 31):
        fail("the S6 root exception must expire for review on 2026-10-31")
    if not root_ignores[0].get("statement"):
        fail("the S6 root exception must record its rationale")

    readme = (ROOT / "README.md").read_text()
    for release_documentation in (
        "generated release PR",
        "alloy-vX.Y.Z",
        "grafana-pdc-vX.Y.Z",
        "grafana-sm-vX.Y.Z",
        "grafana-sm-browser-vX.Y.Z",
        "Conventional Commits",
        "Do not edit an App version manually",
    ):
        if release_documentation not in readme:
            fail(f"README is missing release guidance: {release_documentation}")

    for app in ("alloy", "grafana_pdc", "grafana_sm", "grafana_sm_browser"):
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
