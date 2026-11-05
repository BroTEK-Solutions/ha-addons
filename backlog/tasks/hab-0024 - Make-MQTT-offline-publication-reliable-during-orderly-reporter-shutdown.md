---
id: HAB-0024
title: Make MQTT offline publication reliable during orderly reporter shutdown
status: To Do
assignee: []
created_date: '2026-11-05T23:44:00Z'
labels:
  - unit:alloy
  - unit:grafana-pdc
  - unit:sm-shared
  - unit:repo
dependencies: []
ordinal: 24000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The reporter passes its process context to autopaho.NewConnection, then tries to publish offline only after that context is cancelled. Autopaho concurrently sends a normal DISCONNECT and closes the connection, racing the offline publication and suppressing the broker last will. Wire-level tests repeatedly observed the five-second shutdown publish timeout; this behavior predates the MQTT configuration changes. Retained availability may remain online after an orderly stop. Investigate and fix the connection lifetime separately from the current reporting-controls change.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A protocol-level regression proves an orderly stop publishes and acknowledges retained offline before disconnecting.
- [ ] #2 Cancellation during broker lookup or connection setup remains bounded and does not leak connection goroutines.
- [ ] #3 Disabled reporting still removes retained topics without recreating availability or entering a restart loop.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
