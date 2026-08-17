---
id: HAB-0002
title: 'Land PR #58: resolve CodeRabbit findings and fix CI'
status: In Progress
assignee: []
created_date: '2026-08-17 13:24'
updated_date: '2026-08-17 13:37'
labels: []
dependencies: []
ordinal: 2000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
PR #58 ("Claude/addon improvements brainstorm") adds MQTT health entities, ingress status pages for Grafana PDC and Synthetic Monitoring, an AppArmor profile for Alloy, and a shared option-validator library. It is 104 files, +8006/-188, and cannot merge: two required checks fail and a CodeRabbit review left 9 actionable findings plus 3 nitpicks, none of them addressed.

Two CI failures block the merge. `Repository quality` fails because `alloy/config.yaml` declares `stage: stable`, which the Home Assistant App linter rejects as a redundant default. `Trivy` fails with 25 high and 25 medium alerts, all one root cause: `golang.org/x/net v0.43.0` pulled in transitively through Paho by all five reporter modules, against 10 CVEs whose highest fixed version is 0.56.0.

Those two failures share a cause with one of the review findings. `renovate.json` ignores `*/reporter/**`, which also matches the source module `shared/reporter/`, so Renovate never proposed the x/net bump that would have kept Trivy green.

Every finding must be judged against the current code before it is acted on, and a finding that does not hold must be dismissed in writing with its reason rather than silently skipped.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Every CodeRabbit finding on PR #58 is either fixed or dismissed with a written reason recorded in the task notes
- [ ] #2 The Repository quality check passes: the Home Assistant App linter accepts every App
- [ ] #3 The Trivy check reports no new high or medium alerts
- [ ] #4 renovate.json no longer ignores the shared reporter and launcher source modules
- [ ] #5 The repository gate and every per-App test suite named in AGENTS.md pass locally
- [ ] #6 Generated artefacts are in sync: sync_shared_lib.py --check and sync_synthetic_monitoring_variants.py leave no drift
- [ ] #7 PR #58 is mergeable and all required checks are green
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

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
0. Merge origin/main into the PR branch. DONE - the branch predated the Backlog tracker, so the tracker only exists on the branch after this merge.
1. Judge all 12 CodeRabbit findings against the code at head 7118ff6. The review was submitted against that exact SHA, so none of them are stale.
2. CI-blocking fixes first, because they gate everything else:
   a. Delete 'stage: stable' from alloy/config.yaml. Stable is the linter's default and it rejects the redundant key.
   b. Narrow renovate.json ignorePaths to the generated reporter and launcher copies, so the shared source modules are managed again.
   c. Bump golang.org/x/net to >= 0.56.0 in shared/reporter, then regenerate every copy.
3. Apply the remaining valid findings in the SOURCE tree only - shared/reporter and synthetic_monitoring_shared - then regenerate with scripts/sync_shared_lib.py and scripts/sync_synthetic_monitoring_variants.py. Editing a generated copy is rejected by CI.
4. Dismiss what does not hold, in writing, in the task notes.
5. Run the whole gate from AGENTS.md: yamllint, the five repository tests, the per-App Python and Go suites, both generators with --check.
6. Push to the PR branch and confirm every required check is green.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## CodeRabbit findings - disposition

All 12 were judged against head 7118ff6, which is the exact SHA the review was submitted against, so none were stale. Fixes land in the SOURCE trees only; the generated copies are regenerated by the two sync scripts.

### Fixed (11)

1. **alloy/apparmor.txt - unrestricted `file,` rule.** VALID and material. `file,` is shorthand for `/ rwmlk,` and AppArmor permissions are cumulative, so it granted write, link and lock everywhere and made every write rule below it decorative - the profile's own header claims 'reads stay broad, writes and capabilities are tight', which was false. Replaced with `/{,**} rm,`: reads stay broad (a denied read costs a metric family silently) and `m` is retained because Go binaries memory-map their own executable segments and `ix` does not permit that. Execute is unaffected - `file,` never granted `x`. Grafana PDC keeps its `file,` deliberately: its outer profile only runs the S6 tree and the real confinement is the nested `pdc` profile. Alloy has no nested profile, so its outer profile is the whole confinement. Pinned by three new checks in alloy/tests/image-contract.test.py, including an exact-set assertion on the writable rules.
2. **renovate.json - `*/reporter/**` and `*/launcher/**` too broad.** VALID, and the root cause of the Trivy failure. Those globs also matched shared/reporter and synthetic_monitoring_shared/launcher, so Renovate stopped managing the module every App compiles from and never proposed the x/net bump. Replaced with the eight explicit generated-copy paths. grafana_sm/ui and grafana_sm_browser/ui are added - they were generated but NOT ignored, so Renovate could have opened a PR against a file the next sync reverts. Regression-guarded by check_ignore_paths in tests/renovate_config_contract_test.py, which asserts both directions: every generated copy ignored, every source module still managed.
3. **golang.org/x/net v0.43.0 (10 CVEs).** VALID. Bumped to v0.58.0 in shared/reporter and propagated to all four reporter copies. This is the whole Trivy failure: 25 high plus 25 medium was 10 CVEs times 5 modules.
4. **collect.go - ineffectual assignment to `name`.** VALID. Every path reaching the use reassigns it first. Changed to `var name, remainder string`.
5. **collect.go - local `close` shadows the builtin.** VALID. Renamed to `end`.
6. **collect.go - unchecked deferred `response.Body.Close()` (two sites).** VALID. Wrapped as `defer func() { _ = response.Body.Close() }()`, matching the existing ignored-error style on io.Copy.
7. **ui/main.go - no ReadTimeout or WriteTimeout.** VALID. ReadHeaderTimeout does not bound a request whose declared body never arrives, because Go drains the unread body before finishing the response; IdleTimeout only covers the gap between requests. Added 10s each to synthetic_monitoring_shared/ui/main.go AND grafana_pdc/ui/main.go, which has the identical gap and is new in this PR.
8. **page.go - stale diagnostics after a failed refresh.** VALID. The catch block changed only #agent, so cloud state, endpoint reachability, configuration and counters kept reading as current. Added markStale() to both status pages. 'Last checked' is deliberately left alone: it records the last SUCCESSFUL read and stays true.
9. **launcher/main_test.go - tautological token assertion.** VALID. The test asserted that os.Environ() does not contain the token, which the test process never sets, so it proved nothing. Rather than only rewriting the test, the guarantee was moved into the code: withoutToken() now strips SM_AGENT_API_TOKEN in startReporter and startUI, so it holds even if a future caller passes the agent's environment instead of the ambient one. The test now seeds the ambient environment WITH a token and asserts it is stripped, and also asserts buildAgentProcess does not mutate the caller's slice.
10. **Nitpick - launcher children unsupervised and unreaped.** VALID. Documented rather than fixed: requiring tini in the standard variant is an image change with its own risk, and the side channels are best-effort by design. DOCS.md now states that an exited reporter or status page stays down until the App is restarted while the probe keeps running, and names the symptom (entities unavailable while checks still publish) so it is not misread as a probe failure.
11. **Nitpick - no generated-file marker on the reporter copies.** VALID. Injected at copy time rather than written into the source, because the source is not generated. Both sync scripts now prepend `// Code generated by <script> from <path>. DO NOT EDIT.` to every .go copy. go.mod and go.sum are excluded: go.sum has no comment syntax and a marker in go.mod is at the mercy of `go mod tidy`. Extended to the SM launcher and ui copies too - they are generated by exactly the same mechanism and doing one and not the other would be arbitrary.

### Dismissed (1)

**shared/reporter/main.go - 'reject credentialed non-TLS MQTT connections'. DOES NOT HOLD in this deployment.** The broker, the credentials and the network are all supplied by the Home Assistant Supervisor: the App declares `services: - mqtt:want`, reads /services/mqtt, and receives a per-App username and password for a broker on the internal Docker network. The Mosquitto broker App is plaintext on that network by default, which is the documented and near-universal configuration, so refusing to attach credentials when `ssl` is false would disable MQTT health entities for essentially every user while protecting a hop that never leaves the host. Credentials are Supervisor-minted per App, not user secrets, and are revocable by removing the App. Whether the broker is TLS is the user's choice made in the broker App, not this App's to override.

### Deliberately not done

- **alloy/ui/main.go has the same missing ReadTimeout/WriteTimeout.** It is NOT touched by this PR - it is the pre-existing configuration UI, not a status page added here - so it is left alone rather than retro-fitted. Worth a follow-up task.
- **No test for markStale().** The house style for static assets is regex assertions over the source string (see alloy/tests/ui-static.test.mjs), which is exactly the refactor-brittle implementation-detail test to avoid, and a DOM test would mean adding jsdom for one branch with no logic in it. `node --check` proves the syntax; the behaviour is reviewed, not pinned.
<!-- SECTION:NOTES:END -->
