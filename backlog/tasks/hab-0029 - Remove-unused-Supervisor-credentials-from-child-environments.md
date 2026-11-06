---
id: HAB-0029
title: Remove unused Supervisor credentials from child environments
status: In Progress
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T00:27:00Z'
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
- [ ] #1 PDC and SM agent/UI environments omit Supervisor credentials while reporters retain required access.
- [ ] #2 Probe token routing remains restricted to the intended SM agent.
- [ ] #3 Sentinel process-environment regression tests and generated source checks pass.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Remove Supervisor credentials from PDC/SM main-agent and UI exec environments, preserve reporter access, and preserve probe-token exclusions. Verify with sentinel environment tests at process construction/service exec boundaries; document isolation limits and regenerate variants.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
PR #172 head b8a046e. Sentinel agent environment test red/green and PDC service suite23/24 before to25/25 after. Review minor positive-control gap fixed: UI fixture must execute and emit its harmless sentinel before credential absence assertions. Reporters retain required Supervisor access; environment filtering is explicitly not full process isolation.
<!-- SECTION:NOTES:END -->
