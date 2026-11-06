---
id: HAB-0026
title: Keep Alloy saved settings valid, recoverable and concurrency safe
status: In Progress
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T00:20:00Z'
labels:
  - unit:alloy
dependencies: []
priority: high
type: bug
ordinal: 26000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Individually accepted requests can merge into a document larger than the loader limit. Concurrent read/merge/validate/save can restore stale secrets. Wrongly typed manual_config is validated differently from startup. Safe mode still consumes broken saved arguments and corrupt stores cannot be repaired through the UI.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Oversized merged documents are rejected without replacing valid settings.
- [ ] #2 Concurrent saves cannot silently revert another update.
- [ ] #3 Allowed option keys/types and trailing JSON are validated consistently with startup.
- [ ] #4 Safe mode uses known-good arguments and an ingress-protected recovery flow archives damaged settings.
- [ ] #5 Regression tests exercise HTTP saves, concurrent updates, persisted data and recovery.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Bound encoded persisted documents before replacement; serialize the full read/merge/validate/write transaction or reject revision conflicts; enforce supported option keys/types and single JSON document; make safe mode ignore saved runtime flags and expose ingress-protected recovery that archives damaged settings. Exercise HTTP saves, deterministic concurrent credentials, exact validated startup types and recovery with preserved originals. Alloy lane owns separate candidate after request-budget handoff.
<!-- SECTION:PLAN:END -->
