---
id: HAB-0031
title: Reject overflowing integers in shared shell validation
status: Done
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T00:33:00Z'
labels:
  - unit:repo
dependencies: []
priority: medium
type: bug
ordinal: 31000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Bash arithmetic wraps a sufficiently large decimal port back into the accepted range. PDC endpoint strings can reach this validator without a prior numeric bound.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Out-of-range arbitrarily long decimal strings are rejected before unsafe arithmetic.
- [x] #2 Valid boundaries and supported leading-zero inputs preserve intended behavior.
- [x] #3 Endpoint validation regression tests demonstrate the overflow rejection.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 just check
- [x] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Normalize leading zeros and compare decimal lengths/lexical values before arithmetic. Cover overflowing endpoint strings, huge leading-zero inputs and inclusive bounds in existing shared validator suite; regenerate copies and run full gate with shared reporter batch.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
PR #173 head b564bb5. Existing suite reproduced overflowing decimal endpoint acceptance:85/87 before,87/87 after. Normalized digit-length/lexical comparison avoids arithmetic on untrusted values. Generated copies and full gate pass; shared guarded review complete.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Merged PR #173; exact tested head1ed7c3d921a68f3876cef1c2d6de668d05568b73 passed required CI run42136974924 and all eight image builds. Source red-green, race, template/endpoint proofs and review dispositions are recorded above. Merge commit on main: 2b201d72eddad998d56557b9c9926b765013fb5b.
<!-- SECTION:FINAL_SUMMARY:END -->
