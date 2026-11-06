---
id: HAB-0030
title: Keep release publication queued and reject unexpected skipped gates
status: To Do
assignee: []
created_date: '2026-11-06T00:09:00Z'
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
- [ ] #1 PR checks can cancel superseded work while every required main publication remains scheduled.
- [ ] #2 Documented explicit recovery can publish a missed version safely.
- [ ] #3 Required aggregation rejects every unexpected skipped prerequisite and permits build skips only for successful no-change selection.
- [ ] #4 Outcome tests cover version push followed by tracker pushes, queued publications and all mandatory skip cases.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
