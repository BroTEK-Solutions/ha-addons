---
id: HAB-0016
title: Build a Traefik state custom integration for Home Assistant
status: To Do
assignee: []
created_date: '2026-08-29 17:38'
labels:
  - 'unit:traefik'
dependencies: []
references:
  - 'https://doc.traefik.io/traefik/operations/api/'
priority: low
type: feature
ordinal: 16000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Observability-only HACS custom integration (new repository) reading the Traefik API (`/api/http/routers`, `/api/http/services`, `/api/overview`, `/api/entrypoints`): router and service health, provider errors, counts by status, certificate expiry where exposed. Gap verified 2026-08-29: no Traefik integration exists in core, HACS or GitHub - searches return only reverse-proxying HA behind Traefik. Read-only: surface state, take no action; alerting stays in Grafana. Multi-instance support matters (more than one Traefik deployment is in scope).
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Router and service health (status, error state) are exposed as entities per configured Traefik instance
- [ ] #2 An overview sensor exposes total/failing router and service counts
- [ ] #3 Multiple Traefik instances can be configured side by side
- [ ] #4 UI config flow supporting an API endpoint protected by basic auth or a header
- [ ] #5 The integration is read-only against the Traefik API
- [ ] #6 Installable through HACS with documentation covering enabling the API safely (no public exposure)
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
