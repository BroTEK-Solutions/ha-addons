---
id: HAB-0001
title: Finish BroTEK-Solutions onboarding to agent-docs drift detection
status: Done
assignee: []
created_date: '2026-08-17 12:11'
updated_date: '2026-08-29 14:54'
labels:
  - 'unit:repo'
dependencies: []
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The fan-out protocol doc (doc-0001) is rendered and byte-correct, but this repo is not yet covered by the daily drift check in m7kni/agent-docs. The agent-docs changes are committed and HELD on local branch feat/brotek-solutions-consumer (36b150e) and must not be pushed until both preconditions below hold, or the daily run fails for the whole fleet rather than for this repo alone.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 The drift permission set for this repo's owner exists in OpenBao with contents:read and NO repositories enumeration — installation-scoped by design, matching the existing owners; do not tighten it. Exact names and ids are in the agent-docs commit, which is private; this board is public
- [x] #2 The shared drift policy grants read on that permission set's token path
- [x] #3 The Backlog.md setup PR is merged to main, so the remote drift check can read backlog/docs through the API
- [x] #4 The held agent-docs branch is merged to main and pushed
- [x] #5 The agent-docs drift workflow reports current, with this repo counted and no NO TOKEN or MISSING rows
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

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Shipped. BroTEK-Solutions is now a fully wired agent-docs consumer.

OpenBao: a drift permission set for this owner, matching the existing three (contents:read, no repositories enumeration — installation-scoped by design), plus the shared drift policy granting its token path.

agent-docs: registry row, a fourth mint step, and the token_for case. That case matches registry casing literally and this is the only owner that is not all-lowercase, so a lowercase-only pattern would have returned an empty token and reported NO TOKEN rather than a bug.

Shipped as merge commits on main: 6dd9762 (#60, the board) and f130fb9 (#61, the protocol re-render). agent-docs at 13e0f40.

Verified end to end rather than asserted: the drift workflow reports current=42 stale=0 missing=0 unreadable=0, read through the GitHub API from CI. unreadable=0 is the load-bearing number — it proves the new owner's token minted and the board was actually read, not skipped.

Side effect worth knowing: reviewing #60 surfaced a real defect in the canonical protocol (the goal-file template could not express front-loaded run mode). Fixed at source as agent-docs#2 and re-rendered into all 36 consumer boards in the same sweep.
<!-- SECTION:FINAL_SUMMARY:END -->
