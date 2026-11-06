---
id: HAB-0006
title: Report every App linter failure in one Repository quality run
status: Done
assignee:
  - '@codex'
created_date: 2026-08-17 14:20
updated_date: '2026-11-06T00:20:00Z'
labels:
  - unit:repo
dependencies: []
priority: low
type: chore
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The Repository quality job runs four `frenck/action-addon-linter` steps in sequence, one per App, then ShellCheck, then the shared-library, reporter and repository contract tests. Any failing step aborts the job, so only the FIRST problem is ever visible.

On PR #58 that turned one fix into three round-trips. `alloy/config.yaml` declared `stage: stable`; removing it revealed `grafana_pdc/config.yaml` declaring `ingress_port: 8099` - the same rule, a key restating a linter default - and fixing that revealed a ShellCheck failure that had never run on the branch at all. Each discovery cost a full CI cycle, and the last one had been invisible for the whole life of the PR.

The linter is a Docker action, so it can be run locally: clone frenck/action-addon-linter, `docker build` its `src/` directory, then run the image with `-v $PWD:/github/workspace -w /github/workspace -e INPUT_PATH=./<app> -e INPUT_COMMUNITY=false`. That is the workaround, not the fix - it should not take knowing that to see two lint errors at once.

Options worth weighing: `continue-on-error: true` on each linter step plus a final step that fails if any `steps.<id>.outcome` was failure; or splitting the four linters into a matrix job so each reports independently; or leaving the ordering alone and accepting it. The tests after ShellCheck have the same property and are worth the same thought.

Low priority - it costs contributor time, not correctness.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A commit that breaks the linter for more than one App reports every failure in a single run
- [x] #2 The job still fails when any App fails; nothing is downgraded to a warning
- [x] #3 actionlint, zizmor and yamllint --strict all still pass on the workflow
- [x] #4 AGENTS.md or the workflow records how to run the App linter locally, so the Docker recipe is not rediscovered each time
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 yamllint --strict .
- [x] #2 python3 tests/app_metadata_contract_test.py && python3 tests/app_version_changed_test.py && python3 tests/renovate_config_contract_test.py && python3 tests/repository_workflow_contract_test.py && python3 tests/synthetic_monitoring_variants_test.py
- [x] #3 SHELL only: shellcheck alloy/rootfs/usr/share/alloy/generate-config.sh alloy/rootfs/etc/s6-overlay/s6-rc.d/init-alloy/run alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/run alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/finish alloy/tests/generate-config.test.sh alloy/tests/init-alloy.test.sh grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/init-grafana-pdc/run grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/run grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/finish grafana_pdc/tests/service.test.sh grafana_pdc/tests/image-smoke.test.sh grafana_pdc/tests/fixtures/bashio grafana_pdc/tests/fixtures/fake-pdc
- [x] #4 ALLOY only: python3 alloy/tests/config-schema.test.py && python3 alloy/tests/image-contract.test.py && bash alloy/tests/generate-config.test.sh && node alloy/tests/ui-static.test.mjs && (cd alloy/ui && go test ./...)
- [x] #5 PDC only: python3 grafana_pdc/tests/config-schema.test.py && python3 grafana_pdc/tests/image-contract.test.py
- [x] #6 SM only: python3 scripts/sync_synthetic_monitoring_variants.py && (cd synthetic_monitoring_shared/launcher && go test ./...)
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Run all four App metadata checks despite individual failures and aggregate their outcomes to fail the job. Exercise multiple simultaneous failures and retain diagnostics. CI lane owns together with HAB-0004 (Go formatting and vetting gate).
<!-- SECTION:PLAN:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Merged PR #170 as 192c79cd56ea92ca820b484dabf37fe5f5874a58. Exact candidate 7c49f716082058dd6d62b6fba863d6c3d9263ba6 passed CI run 42135401737, including required ci-success and all eight architecture builds. Local just check, actionlint and zizmor passed; dsreview PASS with zero findings. Actual recipe probes proved format/vet/UI failures and pinned Docker metadata linter reported two simultaneous invalid Apps while the remaining Apps ran; aggregate failed. Standard full go vet is the chosen baseline without a second linter toolchain; generated modules are checked through source and generation drift. Existing backlog gate just check now includes vet. Legacy bare-python DoD commands were executed through their current locked uv recipes, preserving the same tests.
<!-- SECTION:FINAL_SUMMARY:END -->
