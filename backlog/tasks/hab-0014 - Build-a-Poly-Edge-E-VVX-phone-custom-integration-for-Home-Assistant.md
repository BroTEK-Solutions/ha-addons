---
id: HAB-0014
title: Build a Poly Edge E / VVX phone custom integration for Home Assistant
status: To Do
assignee: []
created_date: '2026-08-29 17:37'
labels:
  - 'unit:poly-phone'
dependencies: []
references:
  - >-
    https://docs.poly.com/bundle/edge-e-administrator-guide-8-2-2/page/support-for-rest-api.html
  - >-
    https://community.home-assistant.io/t/polycom-vvx-call-state-via-php-and-mqtt/101592
priority: medium
type: feature
ordinal: 14000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
HACS custom integration (new repository) for Poly desk phones running PVOS/UCS, which expose a local REST API on Edge E and VVX series. Gap verified 2026-08-29: nothing exists beyond homebrew PHP+MQTT call-state hacks and a microbrowser dashboard add-on; no config-flow integration anywhere. Candidate surface: per-line in-call binary sensor, incoming-call event entity, DND switch, SIP registration/line status sensors, device info from the phone. Primary automations this unlocks: pause media and suppress TTS announcements during calls, "on a call" indicator. Two Edge E350 units are available for dogfooding; the VVX install base makes this broadly useful. Write the shape, not the instance: no SIP account details, hostnames or credentials in this tracker.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A binary sensor reflects in-call state per registered line, updating within the polling interval
- [ ] #2 Incoming calls raise an HA event (or event entity) carrying caller display info
- [ ] #3 DND can be toggled from HA and reflects changes made on the phone
- [ ] #4 SIP registration state per line is exposed as a sensor
- [ ] #5 UI config flow (host + API credentials); works without cloud/Poly Lens connectivity
- [ ] #6 Installable through HACS with documentation covering enabling the REST API on the phone
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
