---
id: HAB-0027
title: Bound PDC diagnostics and distinguish unavailable configuration
status: Done
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T01:00:00Z'
labels:
  - unit:grafana-pdc
dependencies: []
priority: medium
type: bug
ordinal: 27000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Sequential endpoint checks can exceed the status HTTP write deadline during outages. An unreadable options file is incorrectly presented as an unrestricted allowlist.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Status collection completes within a whole-request budget under multiple slow endpoint failures.
- [x] #2 Failed configuration reads display unknown policy instead of unrestricted access.
- [x] #3 Real HTTP deadline and missing/malformed options regression cases pass.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 just check
- [x] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Bound total diagnostic collection below HTTP response deadline, check endpoints with bounded concurrency, and explicitly mark unfinished checks. Track successful options reads separately so missing/malformed configuration displays unknown rather than unrestricted access. PDC lane owns isolated UI candidate, red-green actual HTTP outage/config cases, full gate and review.
<!-- SECTION:PLAN:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Merged in PR #172 at 502c2305dba78e4a2539530c4cc2b2e720c1afe0. PDC diagnostics share a five-second request budget across four workers; cancelled or unfinished checks remain Not tested. Invalid/missing/null/trailing options produce unknown policy instead of unrestricted access. Actual HTTP outage/configuration and embedded Node renderer regressions proved red then green; parent-budget regression, race checks, just check and independent review passed. Exact combined head c1a50d86d703e5903de38566892f049b011ea980 passed CI42140343973 and all eight image builds.
<!-- SECTION:FINAL_SUMMARY:END -->
