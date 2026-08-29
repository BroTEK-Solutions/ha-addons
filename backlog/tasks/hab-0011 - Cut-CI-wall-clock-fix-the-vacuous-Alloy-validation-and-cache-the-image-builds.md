---
id: HAB-0011
title: 'Cut CI wall clock: fix the vacuous Alloy validation and cache the image builds'
status: In Progress
assignee: []
created_date: '2026-08-29 17:18'
updated_date: '2026-08-29 17:29'
labels:
  - 'unit:alloy'
  - 'unit:repo'
dependencies: []
priority: high
type: chore
ordinal: 11000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
CI wall clock is 233-306s across recent PR runs. The DAG is three phases - seven parallel jobs, then `build-app`, then `ci-success` - so only the critical path matters:

| Stage | Time |
|---|---|
| `alloy-generator-test` | 182-191s |
| ⤷ "Run Alloy toolchain tests" | 136s |
| ⤷ "Run Alloy Docker tests" | 42s |
| `build-app` (alloy amd64) | up to 94s |
| `ci-success` | 3s |

Every other lane (quality 48-78s, pdc-test 73s, alloy-init 69-89s, sm-test 78-97s, security 33s) finishes well before the Alloy lane and is off the critical path. Optimising them reduces runner minutes, not wall clock.

Inside the 136s step the four non-Docker components measure 0.1-0.2s each locally (config-schema, image-contract, `go test ./...`, ui-static). `alloy/tests/generate-config.test.sh` is effectively the whole step: 24 fixtures, each running `docker run ... fmt` and then `docker run ... run` wrapped in a hard-coded `sleep 4`. That is 96 seconds of pure sleeping.

**The sleep is buying nothing, because the check it waits on is vacuous.** The invocation passes `--server.http.listen-addr=0.0.0.0:0`. Port 0 makes memberlist fail with `failed to create cluster node: ... missing real listen port`, so Alloy exits at ~0.5s having never loaded the config. That error text is absent from the regex allowlist at generate-config.test.sh:53, so the script reports `alloy run loaded config` as a pass and then sleeps out the remaining 3.5s against a dead container.

Verified live, not inferred. A config containing `prometheus.exporter.thiscomponentdoesnotexist` passes today. With a concrete port instead of `:0` the same config fails immediately with `cannot find the definition of component name`, and a valid config starts cleanly. `alloy fmt` is syntax-only and does not cover this, so 24 of the suite's 188 checks currently assert nothing.

**Rejected with evidence:** per-app gating of `build-app` so the SM and PDC images stop waiting on the Alloy lane. It is not a win - the critical path is alloy-test then alloy-image, so unblocking the other apps earlier does not shorten it, and it would contradict the existing contract assertion that image builds wait for all test lanes.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 generate-config.test.sh validates semantics against a concrete listen address and decides pass/fail on the exit code, not a stderr regex allowlist
- [ ] #2 A config naming a nonexistent component fails the suite; the current suite passes it
- [ ] #3 The hard-coded 4s per-fixture sleep is gone and the 24 fixture validations run concurrently
- [ ] #4 setup-go caching is decided on measured evidence - cache hit rate and upload cost on a branch - and the outcome is recorded in AGENTS.md either way
- [ ] #5 The smoke image builds use buildx with a GitHub Actions layer cache
- [ ] #6 CI wall clock on a PR run is materially below the current 233-306s band, with before and after run IDs recorded in the final summary
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## Locked decisions (2026-08-29)

1. **Correctness and speed in one PR.** Fix the `:0` port and the exit-code check together with removing the sleep and parallelising. If the now-working checks surface genuine bugs in the generated configs, STOP and report them rather than fixing them silently inside the same change - a perf PR must not quietly become a bug-fix PR.
2. **setup-go `cache: false`: investigate before changing.** Every `setup-go` step currently passes `cache: false` and Go compile costs 18-21s per lane cold versus 0.2s warm locally. Do not flip it on assumption. Measure cache hit rate and cache upload cost on a branch first, then decide, and record the outcome in AGENTS.md whichever way it goes so a future session does not re-litigate it.
3. **Scope: critical path plus image build caching.** Items 1-4 of the research: the Alloy validation fix, fixture concurrency, the setup-go decision, and buildx with a GitHub Actions layer cache for the smoke image builds. Off-critical-path lanes are explicitly OUT - trimming them saves runner minutes but not wall clock.

## Sequencing

PR #99 (uv provisioning) touches `.github/workflows/builder.yaml` and must merge first, or this work conflicts with it.

## Baseline for the acceptance check

Measured PR runs before any change: 33264214115 = 233s, 33263500768 = 306s, 33263049511 = 271s, 33262200294 = wall not captured, alloy lane 182s. Use `gh api repos/BroTEK-Solutions/ha-addons/actions/runs/<id> --jq '(.updated_at|fromdateiso8601) - (.run_started_at|fromdateiso8601)'` for the after figure.

## Kickoff 2026-08-29

Branch `perf/ci-wall-clock` off main (84ca51e). PR #99 merged as 9f42ad0, so the builder.yaml sequencing conflict is cleared.

Three parallel lanes dispatched, disjoint file ownership:
- Lane A owns `alloy/tests/generate-config.test.sh` - the vacuous validation fix plus fixture concurrency (AC 1-3).
- Lane B is read-only - the setup-go cache measurement (AC 4).
- Lane C owns `justfile`, `.github/workflows/builder.yaml`, `tests/repository_workflow_contract_test.py` - buildx layer cache (AC 5).
<!-- SECTION:NOTES:END -->
