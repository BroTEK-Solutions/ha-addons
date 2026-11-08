#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = ["pyyaml"]
# ///
"""Regression checks for the multi-App build and test orchestration."""

from __future__ import annotations

import ast
import json
import os
import re
import runpy
import struct
import subprocess
import sys
from datetime import date
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def pinned_action(steps: list[dict], action: str, context: str) -> dict:
    matches = [step for step in steps if str(step.get("uses", "")).split("@", 1)[0] == action]
    if len(matches) != 1:
        fail(f"{context} must run exactly one {action} step")
    step = matches[0]
    if not re.fullmatch(re.escape(action) + r"@[0-9a-f]{40}", step["uses"]):
        fail(f"{context} must pin {action} to a full commit SHA")
    return step


def pinned_version(step: dict, key: str, context: str) -> str:
    version = str((step.get("with") or {}).get(key, ""))
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        fail(f"{context} must pin an explicit {key} version")
    return version


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


def check_app_lint_results(steps: list[dict]) -> None:
    """Every App gets diagnostics, but continuing past a linter never hides failure."""
    linters = [step for step in steps if str(step.get("uses", "")).startswith("frenck/action-addon-linter@")]
    apps = {"alloy", "grafana_pdc", "grafana_sm", "grafana_sm_browser"}
    if {step.get("with", {}).get("path", "").removeprefix("./") for step in linters} != apps:
        fail("metadata lint must cover all four Apps")
    for step in linters:
        if step.get("continue-on-error") is not True or step.get("if") != "${{ !cancelled() }}":
            fail("every App linter must run after earlier failures and preserve its outcome")
    aggregate = next((step for step in steps if step.get("id") == "app_lint_result"), None)
    if not aggregate or aggregate.get("if") != "${{ !cancelled() }}" or aggregate.get("continue-on-error"):
        fail("App lint results must be aggregated even after other quality failures")
    references = {f"${{{{ steps.{step['id']}.outcome }}}}": step for step in linters}
    if set(aggregate.get("env", {}).values()) != set(references):
        fail("App lint aggregation must consume every linter's outcome, not its conclusion")
    # Execute the shipped aggregation script. Two failures must both be reported;
    # a skipped linter must not accidentally turn an incomplete run green.
    cases = (
        ({app: "success" for app in apps}, True),
        ({app: "failure" if app in {"alloy", "grafana_pdc"} else "success" for app in apps}, False),
        ({app: "skipped" if app == "grafana_sm" else "success" for app in apps}, False),
    )
    for outcomes, success in cases:
        environment = os.environ.copy()
        for key, reference in aggregate["env"].items():
            app = references[reference]["with"]["path"].removeprefix("./")
            environment[key] = outcomes[app]
        result = subprocess.run(
            ["bash", "-euo", "pipefail", "-c", aggregate["run"]],
            env=environment, capture_output=True, text=True, timeout=10, check=False,
        )
        if (result.returncode == 0) != success:
            fail(f"App lint aggregation returned {result.returncode} for {outcomes}")
        for app, outcome in outcomes.items():
            if outcome != "success" and f"{app}: {outcome}" not in result.stdout + result.stderr:
                fail(f"App lint aggregation hid {app}'s {outcome} result")


def check_required_job_outcomes(jobs: dict, prerequisites: set[str]) -> None:
    """Exercise the shipped required-check decision against real job-result shapes."""
    gate = jobs["ci-success"]
    steps = gate["steps"]
    if gate.get("if") != "always()" or len(steps) != 1:
        fail("the required gate must always evaluate all prerequisite results")
    step = steps[0]
    if step.get("if") or step.get("continue-on-error") or step.get("env") != {
        "NEEDS_JSON": "${{ toJSON(needs) }}",
        "EVENT_NAME": "${{ github.event_name }}",
    }:
        fail("the required gate must consume actual needs results without skipping or tolerating failure")
    successful = {name: {"result": "success"} for name in prerequisites}
    successful["init"]["outputs"] = {"changed": "true"}
    pull_request_only = {"ac-evidence", "privacy-sweep"}
    # Each case is (needs, event name, expected success).
    cases = [(successful, "pull_request", True), (successful, "push", True)]

    def changed_case(job: str, result: str, changed: str = "true") -> dict:
        case = json.loads(json.dumps(successful))
        case[job]["result"] = result
        case["init"]["outputs"]["changed"] = changed
        return case

    cases.append((changed_case("build-app", "skipped", "false"), "push", True))
    for job in prerequisites:
        for result in ("skipped", "failure", "cancelled"):
            for event in ("pull_request", "push"):
                # Only the pull-request-only jobs may be skipped, and only off a pull request.
                allowed = job in pull_request_only and result == "skipped" and event != "pull_request"
                cases.append((changed_case(job, result), event, allowed))
    for changed in ("", "unknown"):
        cases.append((changed_case("build-app", "success", changed), "push", False))
        cases.append((changed_case("build-app", "skipped", changed), "push", False))
    cases.append((changed_case("init", "skipped", "false"), "push", False))
    for needs, event, success in cases:
        result = subprocess.run(
            ["bash", "-euo", "pipefail", "-c", step["run"]],
            env={**os.environ, "NEEDS_JSON": json.dumps(needs), "EVENT_NAME": event},
            text=True, capture_output=True, timeout=10, check=False,
        )
        if (result.returncode == 0) != success:
            fail(f"required gate returned {result.returncode} for {event} {needs}: {result.stdout}{result.stderr}")
    for job in sorted(pull_request_only):
        if jobs[job].get("if") != "github.event_name == 'pull_request'":
            fail(f"{job} must run on pull requests only, because its reusable fails on any other event")
        if not re.fullmatch(r"rknightion/\.github/\.github/workflows/[a-z-]+\.yml@[0-9a-f]{40}", jobs[job].get("uses", "")):
            fail(f"{job} must call its reusable workflow by full commit SHA")


def check_publication_concurrency(caller: dict) -> None:
    """Constrain the one queue key the pinned actionlint cannot yet validate."""
    expected = {
        "group": "${{ github.event_name == 'pull_request' && format('build-apps-pr-{0}', github.ref) || 'app-publication' }}",
        "cancel-in-progress": "${{ github.event_name == 'pull_request' }}",
        "queue": "${{ github.event_name == 'pull_request' && 'single' || 'max' }}",
    }
    if caller.get("concurrency") != expected:
        fail("PRs must supersede their own checks; publications must share a non-cancelling max queue")
    for path in sorted((ROOT / ".github/workflows").glob("*")):
        if path.suffix not in {".yml", ".yaml"}:
            continue
        workflow = yaml.safe_load(path.read_text())
        concurrency = workflow.get("concurrency")
        if path.name != "builder.yaml" and isinstance(concurrency, dict) and "queue" in concurrency:
            fail(f"unsupported additional queue context in {path.name}")
        for job in workflow.get("jobs", {}).values():
            concurrency = job.get("concurrency")
            if isinstance(concurrency, dict) and "queue" in concurrency:
                fail(f"unsupported job-level queue context in {path.name}")
    security = yaml.safe_load((ROOT / ".github/workflows/security.yml").read_text())
    lint_job = security["jobs"]["actionlint"]
    steps = lint_job.get("steps", [])
    guard_index = next((i for i, step in enumerate(steps) if step.get("run") == "just test-workflows </dev/null"), -1)
    lint_index = next((i for i, step in enumerate(steps) if step.get("name") == "Run actionlint"), -1)
    if lint_job.get("if") or lint_job.get("continue-on-error") or not 0 <= guard_index < lint_index:
        fail("the mandatory queue guard must precede the local actionlint compatibility exception")
    for index in (guard_index, lint_index):
        if steps[index].get("if") or steps[index].get("continue-on-error"):
            fail("queue validation and actionlint must not skip or tolerate failures")
    lint_script = steps[lint_index]["run"]
    exact_queue_exception = (
        '''known_queue='^unexpected key "queue" for "concurrency" section\\. expected one of '\n'''
        '''known_queue+='"cancel-in-progress", "group"$' '''.rstrip()
    )
    if exact_queue_exception not in lint_script:
        fail("actionlint may exempt only its exact obsolete queue-key diagnostic")
    if set(re.findall(r'-ignore\s+(\S+)', lint_script)) != {'"$known_runner"', '"$known_queue"'}:
        fail("actionlint must not ignore additional diagnostics")


def check_queued_publication_selection(caller: dict) -> None:
    """Version pushes keep publishing work even when intervening pushes select nothing."""
    repository_class = runpy.run_path(str(ROOT / "tests/app_version_changed_test.py"))["TemporaryRepository"]
    repository = repository_class()
    try:
        repository.write("scripts/app_version_changed.py", (ROOT / "scripts/app_version_changed.py").read_text())
        repository.write("example/config.yaml", 'version: "1.0.0"\n')
        base = repository.commit("initial")
        selector = next(step for step in caller["jobs"]["init"]["steps"] if step.get("id") == "filter")
        # A running version push, a queued tracker push, another queued version
        # push, and another tracker push. Default single-pending concurrency
        # would discard work; check_publication_concurrency forbids that policy.
        for name, content, publish in (
            ("example/config.yaml", 'version: "1.0.1"\n', True),
            ("tracker", "first update\n", False),
            ("example/config.yaml", 'version: "1.0.2"\n', True),
            ("tracker", "second update\n", False),
        ):
            repository.write(name, content)
            head = repository.commit("fixture update")
            output = repository.path / "outputs"
            output.write_text("")
            environment = {
                **os.environ,
                "APPS": "example", "CHANGED_FILES": name, "EVENT_NAME": "push",
                "TRIGGER_REF": "refs/heads/main", "BASE_SHA": base, "HEAD_SHA": head,
                "MONITORED_FILES": caller["env"]["MONITORED_FILES"], "GITHUB_OUTPUT": str(output),
            }
            result = subprocess.run(
                ["bash", "-euo", "pipefail", "-c", selector["run"]],
                env=environment, cwd=repository.path, text=True, capture_output=True, timeout=10, check=False,
            )
            if result.returncode:
                fail(f"queued App selection failed: {result.stderr}")
            values = dict(line.split("=", 1) for line in output.read_text().splitlines())
            if publish:
                if values.get("changed") != "true" or json.loads(values.get("changed_apps", "[]")) != [{"app": "example", "publish": True}]:
                    fail("a queued version-changing push lost its required image publication")
            elif values.get("changed") != "false":
                fail("a following tracker push must not independently republish a stable version")
            base = head
    finally:
        repository.close()


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
    check_publication_concurrency(caller_config)
    check_queued_publication_selection(caller_config)
    check_app_lint_results(jobs["quality"]["steps"])

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

    release_steps = yaml.safe_load(release_workflow)["jobs"]["release"]["steps"]
    for action in (
        "googleapis/release-please-action",
        "rknightion/.github/.github/actions/broker-token",
    ):
        pinned_action(release_steps, action, "release")
    for required in (
        "workflow_call:",
        "token: ${{ steps.bao.outputs.token }}",
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
            "just vet",
            "just lint",
            "just gen-check",
            "just test-repo",
        ),
        "pdc-test": (
            "just test-pdc",
            "just test-pdc-image",
            "just setup-apparmor-tests",
            "just test-pdc-apparmor",
        ),
        "alloy-generator-test": (
            "just test-alloy",
            "just test-alloy-image",
            "just setup-apparmor-tests",
            "just test-alloy-apparmor",
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

    for job_name, recipe in (
        ("alloy-generator-test", "test-alloy-apparmor"),
        ("pdc-test", "test-pdc-apparmor"),
    ):
        job = jobs[job_name]
        if job.get("continue-on-error") or "if" in job:
            fail(f"{job_name} confinement job must be unconditional and must propagate failures")
        step = next(step for step in job["steps"] if step.get("run") == f"just {recipe} </dev/null")
        if step.get("continue-on-error") or "if" in step:
            fail(f"{recipe} must be an unconditional, required runtime check")

    # Renovate owns the values; the contract owns pinning and consistency.
    for action, key, lanes in (
        ("extractions/setup-just", "just-version", test_lanes),
        ("astral-sh/setup-uv", "version", {"quality", "pdc-test", "alloy-generator-test"}),
    ):
        pins = set()
        for lane in sorted(lanes):
            step = pinned_action(jobs[lane]["steps"], action, lane)
            pins.add((step["uses"], pinned_version(step, key, lane)))
        if len(pins) != 1:
            fail(f"all lanes must use the same {action} SHA and {key} version")

    expected_justfile_commands = (
        "uvx yamllint@{{ yamllint_version }} --strict .",
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
        "(cd synthetic_monitoring_shared/ui && go test ./...)",
        "just image grafana_sm local/ha-grafana-sm:smoke",
        "just image grafana_sm_browser local/ha-grafana-sm-browser:smoke",
        "python3 tests/synthetic_monitoring_image_smoke_test.py local/ha-grafana-sm:smoke standard",
        "python3 tests/synthetic_monitoring_image_smoke_test.py local/ha-grafana-sm-browser:smoke browser",
    )
    for command in expected_justfile_commands:
        if command not in justfile:
            fail(f"justfile must run: {command}")
    if not re.search(r'^yamllint_version := "[0-9]+\.[0-9]+\.[0-9]+"$', justfile, re.MULTILINE):
        fail("the justfile must pin an explicit yamllint version")
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
        "crazy-max/ghaction-github-runtime",
        "docker/setup-buildx-action",
    )
    cache_pins = {action: set() for action in cache_actions}
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
        for action in cache_actions:
            step = pinned_action(lane_steps, action, job_id)
            cache_pins[action].add(step["uses"])
            if lane_steps.index(step) > build_step:
                fail(f"{action} must run before {build_command}, not after it")
    for action, pins in cache_pins.items():
        if len(pins) != 1:
            fail(f"all image lanes must use the same {action} SHA")

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
    aggregate_gate = {"init", *test_lanes, "build-app", "security", "ac-evidence", "privacy-sweep"}
    if set(jobs["ci-success"].get("needs", [])) != aggregate_gate:
        fail("ci-success must aggregate every build, security, and test result")
    check_required_job_outcomes(jobs, aggregate_gate)
    if jobs["release"].get("needs") != ["ci-success"]:
        fail("release automation must wait for the complete required gate")
    release_condition = jobs["release"].get("if", "")
    if not all(
        required in release_condition
        for required in (
            "always()",
            "github.event_name == 'push'",
            "github.ref == 'refs/heads/main'",
            "needs.ci-success.result == 'success'",
        )
    ):
        fail("release automation must require a passing main-push gate")

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
