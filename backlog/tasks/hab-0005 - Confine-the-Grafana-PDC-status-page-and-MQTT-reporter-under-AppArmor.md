---
id: HAB-0005
title: Confine the Grafana PDC status page and MQTT reporter under AppArmor
status: Done
assignee:
  - '@codex'
created_date: 2026-08-17 14:20
updated_date: '2026-11-06T01:00:00Z'
labels:
  - unit:grafana-pdc
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
- [x] #1 The bare file, rule is gone from grafana_pdc/apparmor.txt and reads stay broad enough that no read is silently denied
- [x] #2 Writes in the outer profile are confined to the App's own state, matching the shape Alloy uses
- [x] #3 grafana_pdc/tests/image-contract.test.py asserts no unrestricted file rule and pins the writable set exactly, mirroring the Alloy checks
- [x] #4 The status page, the MQTT reporter and the pdc agent all still start and work on a real Home Assistant OS install, with no denials in dmesg
- [x] #5 The profile header documents the apparmor: false recovery switch and the symptoms of an over-tight rule
- [x] #6 python3 grafana_pdc/tests/config-schema.test.py and python3 grafana_pdc/tests/image-contract.test.py pass
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 yamllint --strict .
- [x] #2 python3 tests/app_metadata_contract_test.py && python3 tests/app_version_changed_test.py && python3 tests/renovate_config_contract_test.py && python3 tests/repository_workflow_contract_test.py && python3 tests/synthetic_monitoring_variants_test.py
- [x] #3 SHELL only: shellcheck alloy/rootfs/usr/share/alloy/generate-config.sh alloy/rootfs/etc/s6-overlay/s6-rc.d/init-alloy/run alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/run alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/finish alloy/tests/generate-config.test.sh alloy/tests/init-alloy.test.sh grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/init-grafana-pdc/run grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/run grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/finish grafana_pdc/tests/service.test.sh grafana_pdc/tests/image-smoke.test.sh grafana_pdc/tests/fixtures/bashio grafana_pdc/tests/fixtures/fake-pdc
- [x] #4 ALLOY only: python3 alloy/tests/config-schema.test.py && python3 alloy/tests/image-contract.test.py && bash alloy/tests/generate-config.test.sh && node alloy/tests/ui-static.test.mjs && (cd alloy/ui && go test ./...)
- [x] #5 PDC only: python3 grafana_pdc/tests/config-schema.test.py && python3 grafana_pdc/tests/image-contract.test.py
- [x] #6 SM only: python3 scripts/sync_synthetic_monitoring_variants.py && (cd synthetic_monitoring_shared/launcher && go test ./...)
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Tighten PDC outer writable set and exact policy contract; document recovery; build enforcing Ubuntu startup/denied-write proof analogous Alloy. Retain real HAOS acceptance criterion as a pre-merge requirement and report it unverified until actual evidence exists. PDC lane owns its isolated candidate.
<!-- SECTION:PLAN:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Merged in combined PR #172 at 502c2305dba78e4a2539530c4cc2b2e720c1afe0; draft #175 superseded. Outer writes restricted to App state with exact writable-set contract and documented recovery. Fresh combined image/startup, enforcing Ubuntu AppArmor allowed/denied writes, unrestricted-policy negative mutation, just check and guarded review passed. Real HAOS proof used reviewed source d1cdd999b5df9ff14a42fe7215c14500802e3397 and unchanged policy SHA256 0ca50bbc001ac20797852a991cc285e91465d603022baebeeb8d104e83ece28a: protected custom profile loaded, real PDC signing/SSH connection, reporter cleanup, ingress status HTTP200 with agent responding. Continuous final kernel capture through stop had63 lines and zero denials. Original App stayed started with unchanged options/version. Separate test App stopped/manual with verified retained partial backup; temporary login revoked. Exact combined head c1a50d86d703e5903de38566892f049b011ea980 passed CI42140343973 and all eight images.
<!-- SECTION:FINAL_SUMMARY:END -->
