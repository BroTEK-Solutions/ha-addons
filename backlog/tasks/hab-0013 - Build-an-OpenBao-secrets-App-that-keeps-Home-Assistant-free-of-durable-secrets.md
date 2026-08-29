---
id: HAB-0013
title: Build an OpenBao secrets App that keeps Home Assistant free of durable secrets
status: To Do
assignee: []
created_date: '2026-08-29 17:37'
labels:
  - 'unit:openbao-secrets'
dependencies: []
references:
  - 'https://github.com/rgruyters/addon-vault-secrets'
priority: medium
type: feature
ordinal: 13000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
New App in this repo: authenticate to an OpenBao (or Vault-API-compatible) server with short-lived credentials (AppRole or JWT), render `secrets.yaml` entries from templates, and optionally push rotated values into other Apps' options via the Supervisor API - so no durable secret lives on the HAOS box. Gap verified 2026-08-29: the only prior art is rgruyters/addon-vault-secrets, archived and Vault-only; nothing targets OpenBao or a no-durable-secrets model. OpenBao is Vault-API-compatible, so the App serves the wider Vault audience too. Design constraints to settle during planning: how HA reloads after a `secrets.yaml` rewrite, failure behaviour when the OpenBao server is unreachable at boot (must not brick HA startup), and never logging rendered values.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The App renders configured secrets from an OpenBao KV mount into `secrets.yaml` entries without a long-lived token stored in App options
- [ ] #2 A render cycle survives the OpenBao server being unreachable: existing secrets stay in place and the App reports the failure without blocking HA
- [ ] #3 Rendered secret values never appear in the App log at any log level
- [ ] #4 Works against stock Vault as well as OpenBao (API-compatible paths only)
- [ ] #5 Documentation covers auth setup (AppRole/JWT), template syntax and the reload story
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
