---
id: HAB-0017
title: Build an Arcane container-management custom integration for Home Assistant
status: To Do
assignee: []
created_date: '2026-08-29 17:38'
labels:
  - 'unit:arcane'
dependencies: []
references:
  - 'https://getarcane.app/api-reference'
priority: medium
type: feature
ordinal: 17000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
HACS custom integration (new repository) for Arcane (getarcane.app), the Docker/Compose management UI. Gap verified 2026-08-29: no HA integration exists anywhere for Arcane. It ships a documented REST API - interactive reference at `/api/docs`, OpenAPI 3.1 document at `/api/openapi.json` - covering containers, images, volumes, networks, Compose stacks and multi-host agents, so entity generation can lean on the spec. Candidate surface: per-stack/container state sensors, image-update-available as HA `update` entities, agent/host connectivity binary sensors; optional restart/redeploy buttons behind an explicit opt-in (default observability-only, consistent with the rule that orchestration lives outside HA). Scope the first release to read-only plus update visibility.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Containers and Compose stacks appear as entities with running/exited/unhealthy state per connected host/agent
- [ ] #2 Available image updates surface as HA update entities
- [ ] #3 Agent/host connectivity is exposed as a binary sensor
- [ ] #4 Control actions (restart/redeploy) exist only behind an explicit opt-in and are absent by default
- [ ] #5 UI config flow with an Arcane API token; multi-host Arcane deployments supported
- [ ] #6 Installable through HACS with documentation covering token creation and least-privilege scope
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
