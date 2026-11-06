---
id: HAB-0028
title: Preserve unknown and failed Synthetic Monitoring metrics
status: In Progress
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T00:15:00Z'
labels:
  - unit:sm-shared
dependencies: []
priority: medium
type: bug
ordinal: 28000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Empty, partial or truncated successful metrics responses currently become valid false/zero observations in the status UI.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Missing series remain unknown and observed zero values remain distinguishable.
- [ ] #2 Read and parse failures are surfaced without exposing credentials.
- [ ] #3 Empty, partial and truncated response regression cases cover the source UI and generated variants.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Represent absent diagnostic fields as null/unknown, preserve observed zeros, and reject truncated/invalid metrics reads. Exercise the source HTTP status handler and frontend formatting using existing test surfaces; regenerate variants.
<!-- SECTION:PLAN:END -->
