---
id: HAB-0028
title: Preserve unknown and failed Synthetic Monitoring metrics
status: Done
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T01:00:00Z'
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
- [x] #1 Missing series remain unknown and observed zero values remain distinguishable.
- [x] #2 Read and parse failures are surfaced without exposing credentials.
- [x] #3 Empty, partial and truncated response regression cases cover the source UI and generated variants.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 just check
- [x] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Represent absent diagnostic fields as null/unknown, preserve observed zeros, and reject truncated/invalid metrics reads. Exercise the source HTTP status handler and frontend formatting using existing test surfaces; regenerate variants.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
PR #172 head b8a046e plus tested small display follow-up pending. HTTP empty/partial/truncated/invalid regressions and actual embedded Node renderer test red/green; source UI race suite and integrated just check pass. Review major finding resolved by merging PR #170 source-UI gate; missing version/uptime display follow-up also uses Unknown.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Merged in PR #172 at 502c2305dba78e4a2539530c4cc2b2e720c1afe0. Absent metrics remain null/Unknown while measured zero remains zero. Failed/truncated reads reject partial observations; nonfinite values are omitted. Source HTTP and embedded Node rendering regressions proved red then green, race checks and generation checks passed. The source UI suite is required by the task surface and CI. Independent review findings corrected; final just check and exact-head CI42140343973 on c1a50d86d703e5903de38566892f049b011ea980 passed with all eight images.
<!-- SECTION:FINAL_SUMMARY:END -->
