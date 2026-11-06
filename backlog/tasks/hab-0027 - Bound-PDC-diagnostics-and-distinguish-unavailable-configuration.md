---
id: HAB-0027
title: Bound PDC diagnostics and distinguish unavailable configuration
status: In Progress
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T00:25:00Z'
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
- [ ] #1 Status collection completes within a whole-request budget under multiple slow endpoint failures.
- [ ] #2 Failed configuration reads display unknown policy instead of unrestricted access.
- [ ] #3 Real HTTP deadline and missing/malformed options regression cases pass.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Bound total diagnostic collection below HTTP response deadline, check endpoints with bounded concurrency, and explicitly mark unfinished checks. Track successful options reads separately so missing/malformed configuration displays unknown rather than unrestricted access. PDC lane owns isolated UI candidate, red-green actual HTTP outage/config cases, full gate and review.
<!-- SECTION:PLAN:END -->
