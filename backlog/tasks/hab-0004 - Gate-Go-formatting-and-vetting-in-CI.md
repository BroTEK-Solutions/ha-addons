---
id: HAB-0004
title: Gate Go formatting and vetting in CI
status: To Do
assignee: []
created_date: '2026-08-17 14:19'
labels: []
dependencies: []
priority: medium
type: chore
ordinal: 4000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
CI runs `go test ./...` for each Go module and nothing else - no `gofmt`, no `go vet`, no golangci-lint. Confirmed by grepping .github/workflows for gofmt, go vet, golangci and staticcheck: no matches.

The cost showed up on PR #58. Three of the CodeRabbit findings were pure lint - an ineffectual assignment to `name`, a local variable shadowing the `close` builtin, and two unchecked deferred `response.Body.Close()` calls. All three are things a linter reports in under a second, and all three reached human-and-bot review instead, on a 104-file PR where attention was needed elsewhere.

There is also existing drift: `gofmt -l` reports `alloy/ui/config_test.go`. It is the only file, and it has been unformatted long enough that no gate has ever objected.

Scope note: adding golangci-lint means picking a linter set and dealing with whatever it reports across every module the first time. `gofmt -l` plus `go vet ./...` is the cheap floor and would have caught two of the three findings on its own. Decide which bar to set as part of doing this - do not assume the largest one.

The repository has many small Go modules (alloy/reporter, alloy/ui, grafana_pdc/reporter, grafana_pdc/ui, shared/reporter, synthetic_monitoring_shared/launcher, synthetic_monitoring_shared/ui, and the six generated copies). Generated copies must NOT be linted separately: they are byte-identical to their sources apart from the DO NOT EDIT banner, so linting them doubles the runtime and reports every finding twice.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 gofmt -l reports nothing across the repository, including alloy/ui/config_test.go
- [ ] #2 CI fails when a Go source file is unformatted
- [ ] #3 CI fails on the vet-or-stronger checks chosen for this task, and the task notes record which bar was set and why
- [ ] #4 The gate covers the source modules and deliberately skips the generated copies under grafana_sm, grafana_sm_browser, alloy/reporter and grafana_pdc/reporter
- [ ] #5 The gate is added to backlog.config.yml definition_of_done and to the command list in AGENTS.md
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
