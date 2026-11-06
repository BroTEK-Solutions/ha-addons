---
id: HAB-0030
title: Keep release publication queued and reject unexpected skipped gates
status: Done
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T00:42:00Z'
labels:
  - unit:repo
dependencies: []
priority: high
type: bug
ordinal: 30000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Workflow cancellation can discard a version-changing push when a later tracker-only push selects no Apps. Default concurrency also replaces pending runs. ci-success currently accepts unexpected skipped mandatory jobs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 PR checks can cancel superseded work while every required main publication remains scheduled.
- [x] #2 Documented explicit recovery can publish a missed version safely.
- [x] #3 Required aggregation rejects every unexpected skipped prerequisite and permits build skips only for successful no-change selection.
- [x] #4 Outcome tests cover version push followed by tracker pushes, queued publications and all mandatory skip cases.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 just check
- [x] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Keep superseded PR cancellation, preserve queued main publication for every version change, and provide explicit missed-version recovery. Enforce mandatory successful outcomes and allow image skips only for verified no-change selection. CI lane researches current supported concurrency schema, proves outcome cases, validates actionlint/zizmor and full gate, then guarded review before landing.
<!-- SECTION:PLAN:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Merged PR #174 at f56b861d261538be0379f91416a1308f36e49914. PR runs cancel superseded checks; main/manual publication uses the supported 100-entry queue with cancellation disabled. Explicit manual recovery is documented with old-version ordering caveat. Mandatory result aggregation rejects unexpected skips. Verified real version/tracker selection fixtures, executed outcome matrix, actionlint/zizmor and just check; hosted run 42137921296 on 767f6f1041d338eb495da51afddfc5a72d61d37b passed required gate and eight image builds. Review claim that queue is unsupported was checked against current official GitHub documentation and disproved by hosted startup; no unresolved valid findings.
<!-- SECTION:FINAL_SUMMARY:END -->
