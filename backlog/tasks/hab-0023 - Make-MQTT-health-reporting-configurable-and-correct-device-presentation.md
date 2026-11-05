---
id: HAB-0023
title: Make MQTT health reporting configurable and correct device presentation
status: In Progress
assignee:
  - agent
created_date: '2026-11-05T23:34:00Z'
updated_date: '2026-11-05T23:35:00Z'
labels:
  - unit:alloy
  - unit:grafana-pdc
  - unit:sm-shared
  - unit:repo
dependencies: []
ordinal: 23000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
App health devices appear automatically whenever a Supervisor MQTT broker exists. The owner approved an explicit off switch, readable device models, and correcting Alloy health/configuration sensors whose problem class currently interprets success as a fault. Keep existing installations enabled by default and preserve entity identities.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 All four Apps expose mqtt_enabled, defaulting to true for existing and new installations; false stops health reporting and removes retained discovery when the broker is reachable.
- [ ] #2 Alloy problem sensors report OFF for success and ON for observed failures; unavailable metrics remain unknown.
- [ ] #3 Device models use readable product names while existing device identifiers and entity unique IDs remain stable.
- [ ] #4 Configuration, migration effects, broker-unavailable cleanup behavior and monitoring limitations are documented; generated copies stay synchronized.
- [ ] #5 Focused regression tests, the repository pre-commit gate, and dsreview pass; changes are submitted through a PR.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Read mqtt_enabled from the mounted App options in the shared reporter, preserving true when the key is absent; disabled mode removes retained discovery/state using the existing identity, retries if the broker is unavailable, then idles without health polling. Restart applies the option. 2. Add default-true options, translations and user documentation in the three source App trees; keep Alloy option in its native Configuration tab. 3. Correct problem-class polarity and add stable product model names without changing IDs/topics. 4. Prove regressions red then green, exercise MQTT publication/cleanup against a local protocol fixture, regenerate copies, run just check, and review the final candidate with dsreview. 5. Submit and land a focused conventional-commit PR after required CI; record the merge identity.
<!-- SECTION:PLAN:END -->
