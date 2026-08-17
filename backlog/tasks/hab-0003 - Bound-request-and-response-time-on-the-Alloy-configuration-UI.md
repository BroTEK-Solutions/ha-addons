---
id: HAB-0003
title: Bound request and response time on the Alloy configuration UI
status: To Do
assignee: []
created_date: '2026-08-17 14:19'
labels: []
dependencies: []
priority: medium
type: bug
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`alloy/ui/main.go` builds its `http.Server` with `ReadHeaderTimeout: 5s` and `IdleTimeout: 60s` and nothing else. Neither bounds a single exchange: ReadHeaderTimeout stops once the headers are in, and Go then drains an unread request body before completing the response, so a client that declares a body and dribbles it holds the connection for as long as it likes. IdleTimeout only covers the gap BETWEEN requests.

This is the same defect PR #58 fixed on the two read-only status pages (`synthetic_monitoring_shared/ui/main.go` and `grafana_pdc/ui/main.go`), where 10s ReadTimeout and 10s WriteTimeout were added. The Alloy UI was left alone only because that PR does not touch the file - retro-fitting it would have widened an already large diff.

It is arguably the MORE exposed of the three. The status pages are read-only; this one has POST handlers that decode a JSON body (`main.go` lines 96, 140, 152, 166). `http.MaxBytesReader(w, r.Body, 1<<20)` caps the SIZE of that body but not the TIME taken to send it, so a slow 1 MiB upload is a held connection with a size limit, not a bounded request.

Exposure is limited: the listener is loopback-plus-ingress and `ingressOnly` rejects any source IP other than the Supervisor's. This is defence in depth against a compromised or misbehaving client inside that boundary, not a route open to the network.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 alloy/ui/main.go sets ReadTimeout and WriteTimeout alongside the existing ReadHeaderTimeout and IdleTimeout
- [ ] #2 The chosen values do not break a legitimate settings save, including a manual_config override at the 1 MiB MaxBytesReader ceiling
- [ ] #3 cd alloy/ui && go test ./... passes
- [ ] #4 The three UI servers agree on the timeout policy, or the file explains in a comment why the Alloy UI differs
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
