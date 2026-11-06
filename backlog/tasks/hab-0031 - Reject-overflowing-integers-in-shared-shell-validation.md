---
id: HAB-0031
title: Reject overflowing integers in shared shell validation
status: In Progress
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T00:13:00Z'
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
- [ ] #1 Out-of-range arbitrarily long decimal strings are rejected before unsafe arithmetic.
- [ ] #2 Valid boundaries and supported leading-zero inputs preserve intended behavior.
- [ ] #3 Endpoint validation regression tests demonstrate the overflow rejection.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Normalize leading zeros and compare decimal lengths/lexical values before arithmetic. Cover overflowing endpoint strings, huge leading-zero inputs and inclusive bounds in existing shared validator suite; regenerate copies and run full gate with shared reporter batch.
<!-- SECTION:PLAN:END -->
