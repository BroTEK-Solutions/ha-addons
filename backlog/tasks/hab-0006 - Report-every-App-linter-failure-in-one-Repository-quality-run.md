---
id: HAB-0006
title: Report every App linter failure in one Repository quality run
status: To Do
assignee: []
created_date: '2026-08-17 14:20'
updated_date: '2026-08-29 14:54'
labels:
  - 'unit:repo'
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
- [ ] #1 A commit that breaks the linter for more than one App reports every failure in a single run
- [ ] #2 The job still fails when any App fails; nothing is downgraded to a warning
- [ ] #3 actionlint, zizmor and yamllint --strict all still pass on the workflow
- [ ] #4 AGENTS.md or the workflow records how to run the App linter locally, so the Docker recipe is not rediscovered each time
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 yamllint --strict .
- [ ] #2 python3 tests/app_metadata_contract_test.py && python3 tests/app_version_changed_test.py && python3 tests/renovate_config_contract_test.py && python3 tests/repository_workflow_contract_test.py && python3 tests/synthetic_monitoring_variants_test.py
- [ ] #3 SHELL only: shellcheck alloy/rootfs/usr/share/alloy/generate-config.sh alloy/rootfs/etc/s6-overlay/s6-rc.d/init-alloy/run alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/run alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/finish alloy/tests/generate-config.test.sh alloy/tests/init-alloy.test.sh grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/init-grafana-pdc/run grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/run grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/finish grafana_pdc/tests/service.test.sh grafana_pdc/tests/image-smoke.test.sh grafana_pdc/tests/fixtures/bashio grafana_pdc/tests/fixtures/fake-pdc
- [ ] #4 ALLOY only: python3 alloy/tests/config-schema.test.py && python3 alloy/tests/image-contract.test.py && bash alloy/tests/generate-config.test.sh && node alloy/tests/ui-static.test.mjs && (cd alloy/ui && go test ./...)
- [ ] #5 PDC only: python3 grafana_pdc/tests/config-schema.test.py && python3 grafana_pdc/tests/image-contract.test.py
- [ ] #6 SM only: python3 scripts/sync_synthetic_monitoring_variants.py && (cd synthetic_monitoring_shared/launcher && go test ./...)
<!-- DOD:END -->
