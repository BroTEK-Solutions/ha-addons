---
id: HAB-0012
title: Build a Grafana IRM / OnCall custom integration for Home Assistant
status: To Do
assignee: []
created_date: 2026-08-29 17:37
updated_date: '2026-11-06T00:09:00Z'
labels:
  - unit:grafana-irm
dependencies: []
references:
  - >-
    https://grafana.com/docs/grafana-cloud/observe-and-act/respond-to-incidents/reference/oncall-api/alertgroups/
  - https://github.com/pinpox/home-assistant-grafana-relay
priority: high
type: feature
ordinal: 12000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Surface Grafana Cloud IRM (OnCall) state inside Home Assistant so the house can react to paging state: wall-panel display, lights, announcement routing and quiet-hours policy. Gap verified 2026-08-29: no HA integration exists for Grafana IRM/OnCall anywhere (core, HACS or GitHub); the only prior art is a webhook-to-notify relay (pinpox/home-assistant-grafana-relay). Deliverable is a HACS-installable custom integration in a new repository (same publishing model as the Meraki Dashboard integration), not an App in this repo. Boundary decision: Grafana owns alerting and decides; HA only displays and reacts - the integration must stay read-mostly, with acknowledge/resolve as the only write actions. Candidate surface: on-call status per schedule/user, alert-group counts by state and severity, incident sensors, ack/resolve buttons, and an optional webhook receiver that turns alert-group lifecycle changes into HA events. Reference: Grafana IRM alert-group HTTP API and OnCall outgoing webhooks.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A binary sensor answers "is the configured user on call now" per schedule
- [ ] #2 Alert groups are exposed as sensors with counts by state (firing/acknowledged/resolved/silenced) and the newest alert group available as attributes or an event entity
- [ ] #3 Acknowledge and resolve are available as HA actions/buttons and verified against a live stack
- [ ] #4 Configuration is via UI config flow with a token; no YAML required and no credentials in logs or diagnostics
- [ ] #5 Installable through HACS with documentation covering token scope and rate limits
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
2026-11-06 delegated product disposition: Defer the separate Grafana IRM HACS product. It has a useful household automation case but needs a dedicated repository, token-scope design and live write-action validation. Reconsider as a separate product initiative with a named target and read-only first milestone. Archived as unbuilt/deferred, not delivered. User authorized autonomous product triage; historical market-gap claims have not been revalidated.
<!-- SECTION:NOTES:END -->
