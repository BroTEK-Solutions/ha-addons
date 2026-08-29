---
id: HAB-0015
title: Build an NTP appliance observability custom integration for Home Assistant
status: To Do
assignee: []
created_date: '2026-08-29 17:37'
labels:
  - 'unit:ntp'
dependencies: []
references:
  - >-
    https://community.home-assistant.io/t/monitoring-ntp-server-status-on-local-pi/717525
priority: low
type: feature
ordinal: 15000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Observability-only HACS custom integration (new repository) for a GPS stratum-1 NTP appliance (CenterClick NTP250/NTP270 class) and generic NTP servers: stratum, offset, GPS fix/satellite count, per-upstream sync state, leap status. Gap verified 2026-08-29: nothing exists - a chrony App serves time and community threads only offer journald scraping; no integration monitors an NTP server. Deliberate boundary exception, decided 2026-08-29: telemetry normally belongs to Grafana, but this item was explicitly approved for HA "purely for observability purposes" - read-only sensors, no control surface, no alerting logic in HA. Design question for planning: appliance-specific API vs generic NTP/SNTP queries (mode 6 control queries where enabled) vs both, and what degrades when the appliance only speaks plain NTP.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Sensors expose stratum, offset and reachability for a configured NTP server
- [ ] #2 GPS lock / satellite / reference status is exposed where the appliance provides it
- [ ] #3 The integration is read-only: no entity can change appliance or time configuration
- [ ] #4 UI config flow; a plain NTP server that answers no status protocol degrades gracefully to the sensors it can serve
- [ ] #5 Installable through HACS with documentation stating the observability-only scope
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
