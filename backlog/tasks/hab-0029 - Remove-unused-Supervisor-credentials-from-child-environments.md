---
id: HAB-0029
title: Remove unused Supervisor credentials from child environments
status: Done
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T01:00:00Z'
labels:
  - unit:sm-shared
  - unit:grafana-pdc
dependencies: []
priority: medium
type: bug
ordinal: 29000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
PDC and SM main agents and status UIs inherit Supervisor credentials they do not use. Reporter access remains required. This is environment exposure reduction, not complete process isolation.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 PDC and SM agent/UI environments omit Supervisor credentials while reporters retain required access.
- [x] #2 Probe token routing remains restricted to the intended SM agent.
- [x] #3 Sentinel process-environment regression tests and generated source checks pass.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 just check
- [x] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Remove Supervisor credentials from PDC/SM main-agent and UI exec environments, preserve reporter access, and preserve probe-token exclusions. Verify with sentinel environment tests at process construction/service exec boundaries; document isolation limits and regenerate variants.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
PR #172 head b8a046e. Sentinel agent environment test red/green and PDC service suite23/24 before to25/25 after. Review minor positive-control gap fixed: UI fixture must execute and emit its harmless sentinel before credential absence assertions. Reporters retain required Supervisor access; environment filtering is explicitly not full process isolation.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Merged in PR #172 at 502c2305dba78e4a2539530c4cc2b2e720c1afe0. PDC and SM main-agent/UI environments omit Supervisor credentials; reporters retain required access. SM probe credentials reach only the intended agent. Sentinel environment and actual shell exec tests proved red then green, including positive UI execution control; generated variants remain synchronized. This reduces exposure without claiming process isolation. Independent review, just check, fresh combined PDC image/confinement checks, and exact-head CI42140343973 on c1a50d86d703e5903de38566892f049b011ea980 passed.
<!-- SECTION:FINAL_SUMMARY:END -->
