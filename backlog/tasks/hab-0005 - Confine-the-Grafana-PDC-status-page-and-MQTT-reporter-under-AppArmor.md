---
id: HAB-0005
title: Confine the Grafana PDC status page and MQTT reporter under AppArmor
status: To Do
assignee: []
created_date: '2026-08-17 14:20'
updated_date: '2026-08-29 14:54'
labels:
  - 'unit:grafana-pdc'
dependencies: []
priority: high
type: enhancement
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`grafana_pdc/apparmor.txt` grants a bare `file,` in its OUTER profile. That is shorthand for `/ rwmlk,` - read, write, lock, memory-map and link on every path - and AppArmor permissions are cumulative, so no later rule can take any of it back.

That was defensible when the outer profile only supervised S6 and the real work happened in the nested `pdc` profile, which is tightly scoped. It is no longer the whole picture. PR #58 added two long-running programs to this App, and both land in the outer profile:

- `/usr/bin/grafana-pdc-ui` - an HTTP server, the ingress status page
- `/usr/bin/ha-reporter` - an MQTT client that talks to the Supervisor and the broker

Neither gets a nested profile. Only `/usr/bin/pdc` transitions, via `/usr/bin/pdc cx -> pdc,`. Everything else matches `/usr/bin/** ix,` and inherits the outer profile, so both new network-facing binaries run with write access to the entire filesystem and AppArmor contributes no containment if either is compromised.

There is now a proven pattern to copy. PR #58 replaced Alloy's identical `file,` with `/{,**} rm,` - reads and memory-mapping stay broad so nothing breaks silently, writes are confined to the App's own state - and pinned it with three checks in `alloy/tests/image-contract.test.py`, including an exact-set assertion over the writable rules. Alloy runs its own reporter and UI under that same tightened profile, so the shape is known to work for exactly this arrangement.

This is defence in depth, not an open route: the status page is ingress-only and rejects any source IP other than the Supervisor's, and the reporter makes only outbound connections. The gap is that if either is compromised, nothing contains it.

Untestable in CI, like the Alloy change: AppArmor only takes effect on a real Home Assistant OS install. Alloy's profile header documents `apparmor: false` as the recovery switch and names the symptoms (failure to start, or a metric family disappearing); this profile should carry the same note. Verify on real hardware before merging, and check the host's dmesg for denied operations.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The bare file, rule is gone from grafana_pdc/apparmor.txt and reads stay broad enough that no read is silently denied
- [ ] #2 Writes in the outer profile are confined to the App's own state, matching the shape Alloy uses
- [ ] #3 grafana_pdc/tests/image-contract.test.py asserts no unrestricted file rule and pins the writable set exactly, mirroring the Alloy checks
- [ ] #4 The status page, the MQTT reporter and the pdc agent all still start and work on a real Home Assistant OS install, with no denials in dmesg
- [ ] #5 The profile header documents the apparmor: false recovery switch and the symptoms of an over-tight rule
- [ ] #6 python3 grafana_pdc/tests/config-schema.test.py and python3 grafana_pdc/tests/image-contract.test.py pass
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 yamllint --strict .
- [ ] #2 python3 tests/app_metadata_contract_test.py && python3 tests/app_version_changed_test.py && python3 tests/renovate_config_contract_test.py && python3 tests/repository_workflow_contract_test.py && python3 tests/synthetic_monitoring_variants_test.py
- [ ] #3 SHELL only: shellcheck alloy/rootfs/usr/share/alloy/generate-config.sh alloy/rootfs/etc/s6-overlay/s6-rc.d/init-alloy/run alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/run alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/finish alloy/tests/generate-config.test.sh alloy/tests/init-alloy.test.sh grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/init-grafana-pdc/run grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/run grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/finish grafana_pdc/tests/service.test.sh grafana_pdc/tests/image-smoke.test.sh grafana_pdc/tests/fixtures/bashio grafana_pdc/tests/fixtures/fake-pdc
- [ ] #4 ALLOY only: python3 alloy/tests/config-schema.test.py && python3 alloy/tests/image-contract.test.py && bash alloy/tests/generate-config.test.sh && node alloy/tests/ui-static.test.mjs && (cd alloy/ui && go test ./...)
- [ ] #5 PDC only: python3 grafana_pdc/tests/config-schema.test.py && python3 grafana_pdc/tests/image-contract.test.py
- [ ] #6 SM only: python3 scripts/sync_synthetic_monitoring_variants.py && (cd synthetic_monitoring_shared/launcher && go test ./...)
<!-- DOD:END -->
