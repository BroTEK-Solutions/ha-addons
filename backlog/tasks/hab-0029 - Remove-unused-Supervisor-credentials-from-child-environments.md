---
id: HAB-0029
title: Remove unused Supervisor credentials from child environments
status: To Do
assignee: []
created_date: '2026-11-06T00:09:00Z'
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
