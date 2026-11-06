---
id: HAB-0025
title: Restore MQTT availability after reconnect and preserve unknown measurements
status: To Do
assignee: []
created_date: '2026-11-06T00:09:00Z'
labels:
  - unit:repo
dependencies: []
priority: high
type: bug
ordinal: 25000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The reporter publishes online once, so an unexpected disconnect can leave recovered devices unavailable. Broker credentials are captured once. Missing binary metrics render empty payloads which Home Assistant ignores, retaining stale state. Orderly shutdown publication is explicitly excluded.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An unexpected disconnect and reconnect restores discovery and online state using current broker credentials.
- [ ] #2 Missing binary measurements explicitly become unknown rather than retaining the last state; numeric semantics stay correct.
- [ ] #3 Wire-level reconnect, credential refresh and missing-state regression cases pass.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
