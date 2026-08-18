---
id: HAB-0007
title: >-
  Export GCLOUD_FM_COLLECTOR_ID so Fleet-delivered pipelines can label the
  collector
status: Done
assignee: []
created_date: '2026-08-18 16:58'
updated_date: '2026-08-18 17:12'
labels:
  - alloy
  - fleet
dependencies: []
priority: medium
type: bug
ordinal: 7000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Grafana Fleet Management's generated self-monitoring pipeline labels every collector with

    replacement = sys.env("GCLOUD_FM_COLLECTOR_ID")

as its `collector_id`, which is what powers the Collector Health section of the Fleet UI. The App
never exports that variable, so on this App alone the label resolves to the empty string and the
series arrives unattributed. Every other collector in a fleet carries a `collector_id` equal to its
collector ID; the App is the only one that does not.

The variable belongs in `alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/run`, next to the existing
`sys.env()` exports, because the generated config that references it is fetched by remotecfg rather
than produced by `generate-config.sh`. `init-alloy` cannot supply it: it is a separate s6 service,
so its environment does not reach the Alloy process.

The value must equal what `init-alloy` writes as remotecfg's `id`, i.e. `config_or instance_name`
with the same `homeassistant` fallback. Reading the setting a second time in `alloy/run` via the
existing `setting()` helper keeps the two in step as long as the fallback is duplicated exactly; a
divergence would attribute the collector's own health metrics to a name Fleet does not know.

Related but deliberately out of scope: remote pipelines that set `instance` from
`constants.hostname` resolve it to the App's container name, whose prefix changes on reinstall. That
is a fleet-side config problem, fixed by using a literal in the pipeline, and no App change helps it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 alloy/run exports GCLOUD_FM_COLLECTOR_ID, derived from the instance_name setting with the same homeassistant fallback init-alloy uses
- [x] #2 A contract check fails if the fallback in alloy/run stops matching the one init-alloy writes as remotecfg's id
- [x] #3 The export is unconditional, so it survives local mode where remotecfg is absent
- [x] #4 shellcheck passes on alloy/run under the CI invocation and the repository gate is green
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

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Exported in `alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/run` beside the existing `sys.env()` exports, before any mode branching, so local mode keeps it too. Value is `setting instance_name` with a `homeassistant` fallback applied in the script rather than trusted to the helper, matching what init-alloy does.

Three checks added to `alloy/tests/image-contract.test.py`, which was already the static contract for this file. The load-bearing one reads *both* service scripts and compares the fallback literal; a behavioural test was not built because `alloy/run` ends in `exec /usr/bin/alloy` with no binary override, and adding one to a production entrypoint is more risk than the drift it would catch. Proved the check is not vacuous by changing the fallback to `hass` and watching only that check fail, then restoring.

Gate run locally, all green: shellcheck under CI's own `-x --source-path=SCRIPTDIR` invocation over all 21 listed files; yamllint --strict; the five repository tests plus app_contract and shared_validate_lib; all four Alloy suites including generate-config at 188/188 once Docker was up; both PDC suites; the SM variant sync and both Go modules. `voluptuous` and `yamllint` are not in the system Python, so those two ran from a venv.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
The App is now the only source of its own collector identity for remote pipelines. Fleet's generated self-monitoring config labels every series `collector_id = sys.env("GCLOUD_FM_COLLECTOR_ID")`; with the variable unset that resolved to an empty string, and this App was the only collector in the fleet whose health metrics arrived unattributed while every other one carried a collector_id equal to its collector ID.

Not fixed here, and no App change can fix it: remote pipelines that set `instance` from `constants.hostname` get the App's container name, whose prefix is the App slug hash and changes on reinstall. The fix for that is a literal in the pipeline, fleet-side.
<!-- SECTION:FINAL_SUMMARY:END -->
