---
id: HAB-0025
title: Restore MQTT availability after reconnect and preserve unknown measurements
status: In Progress
assignee:
  - '@codex'
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T00:27:00Z'
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

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Refresh Supervisor broker credentials before connection attempts; signal reconnects into the main publication loop and retry retained discovery/online with bounded publish contexts. Keep orderly shutdown path unchanged. Render explicit unknown sentinel per Home Assistant MQTT semantics and prove transitions with wire and template fixtures. Source shared reporter only; regenerate copies.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
PR #173 head b564bb5. MQTT wire reconnect with credential rotation and Jinja explicit unknown proof pass; reporter race suite and integrated just check pass. dsreview PASS: corrected valid collection/publication budget issue by creating publication context after collection. Low-confidence reconnect-delay finding refuted by pinned autopaho v0.23 source: establishServerConnection starts attempt0 each session, NewConstantBackoff returns0 for attempt0. Orderly shutdown unchanged.
<!-- SECTION:NOTES:END -->
