---
id: HAB-0001
title: Finish BroTEK-Solutions onboarding to agent-docs drift detection
status: To Do
assignee: []
created_date: '2026-08-17 12:11'
updated_date: '2026-08-17 12:36'
labels: []
dependencies: []
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The fan-out protocol doc (doc-0001) is rendered and byte-correct, but this repo is not yet covered by the daily drift check in m7kni/agent-docs. The agent-docs changes are committed and HELD on local branch feat/brotek-solutions-consumer (36b150e) and must not be pushed until both preconditions below hold, or the daily run fails for the whole fleet rather than for this repo alone.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 OpenBao permission set agent-docs-drift-brotek-solutions exists with contents:read and NO repositories enumeration — installation-scoped by design, matching the other three; do not tighten it
- [x] #2 Policy gha-agent-docs-drift grants read on that permission set's token path
- [ ] #3 The Backlog.md setup PR is merged to main, so doctor-remote can read backlog/docs through the API
- [ ] #4 agent-docs branch feat/brotek-solutions-consumer merged to main and pushed
- [ ] #5 gh workflow run drift.yml in agent-docs reports current, with this repo counted and no NO TOKEN or MISSING rows
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 yamllint --strict .
- [ ] #2 python3 tests/app_metadata_contract_test.py && python3 tests/app_version_changed_test.py && python3 tests/renovate_config_contract_test.py && python3 tests/repository_workflow_contract_test.py && python3 tests/synthetic_monitoring_variants_test.py
- [ ] #3 shellcheck over the file list in builder.yaml's ShellCheck step (only if a shell script changed)
- [ ] #4 ALLOY only: python3 alloy/tests/config-schema.test.py && python3 alloy/tests/image-contract.test.py && bash alloy/tests/generate-config.test.sh && node alloy/tests/ui-static.test.mjs && (cd alloy/ui && go test ./...)
- [ ] #5 PDC only: python3 grafana_pdc/tests/config-schema.test.py && python3 grafana_pdc/tests/image-contract.test.py
- [ ] #6 SM only: python3 scripts/sync_synthetic_monitoring_variants.py && (cd synthetic_monitoring_shared/launcher && go test ./...)
<!-- DOD:END -->
