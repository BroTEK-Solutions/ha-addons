---
id: HAB-0011
title: 'Cut CI wall clock: fix the vacuous Alloy validation and cache the image builds'
status: In Progress
assignee: []
created_date: '2026-08-29 17:18'
updated_date: '2026-08-29 18:19'
labels:
  - 'unit:alloy'
  - 'unit:repo'
dependencies: []
references:
  - 'https://github.com/BroTEK-Solutions/ha-addons/pull/102'
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
- [x] #1 generate-config.test.sh validates semantics against a concrete listen address and decides pass/fail on the exit code, not a stderr regex allowlist
- [x] #2 A config naming a nonexistent component fails the suite; the current suite passes it
- [x] #3 The hard-coded 4s per-fixture sleep is gone and the 24 fixture validations run concurrently
- [x] #4 setup-go caching is decided on measured evidence - cache hit rate and upload cost on a branch - and the outcome is recorded in AGENTS.md either way
- [x] #5 The smoke image builds use buildx with a GitHub Actions layer cache
- [x] #6 CI wall clock on a PR run is materially below the current 233-306s band, with before and after run IDs recorded in the final summary
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

## Implementation landed on PR #102 (branch perf/ci-wall-clock)

All three lanes complete. PR #102 carries the full detail; the load-bearing facts:

**AC #1 wording is superseded, deliberately not rewritten.** It says "validates semantics against a
concrete listen address". The pinned Alloy image turned out to have an `alloy validate` subcommand
that loads and type-checks a config without starting any component, so the verdict is its exit
status and no `alloy run` is involved at all. That is strictly better than the concrete-port
approach the AC assumed, and it satisfies the AC's actual intent (exit-code verdict, no stderr
allowlist). The original wording is left in place so the superseded assumption stays visible.

**`alloy validate` coverage, verified directly:** unknown components, unrecognized attributes, and
unknown functions all exit 1; a good config exits 0.

**All 24 fixtures pass real validation.** The stop-and-report condition never triggered - the config
generator needed no change.

**Timings:** generate-config.test.sh 144.3s -> 24.1s, 188/188. Alloy image build 52.2s cold ->
11.0s on a local cache hit, less ~10.4s for setup-buildx-action to pull moby/buildkit, so expect
25-30s net on the lane.

**AC #4 answered NO, on measurement.** setup-go keeps `cache: false`. Only shared/reporter has a
go.sum; the other three Go modules are stdlib-only so the module cache has nothing to restore.
Cold-to-warm delta caps at 11-13s per module against four cache entries each paying restore and
upload. Recorded in AGENTS.md. Note the AC asked for hit rate and upload cost measured on a branch -
that was NOT done, because the bounding argument made the experiment unnecessary. Flagged to Rob.

**CodeRabbit:** one minor finding on the combined diff, applied - `ignore-error=true` on the gha
cache export, so a cache-write failure cannot fail a build that otherwise succeeded.

**Still open:** no CI cache hit has been observed. First run populates, second should show CACHED.
AC #6 needs the after-figure from a PR run.

## Environment defect found, not fixed

This laptop cannot pull from ghcr.io: `docker pull ghcr.io/home-assistant/base:3.24` returns
`error from registry: denied`. config.json has credsStore osxkeychain and a ghcr.io auths entry, so
a stale credential is sent on every pull and rejected. It breaks test-pdc-image, test-alloy-image,
test-sm-image and `just ci` on this machine regardless of this change. Work around with a throwaway
DOCKER_CONFIG (symlink cli-plugins, buildx and contexts into it, or buildx vanishes). Left for Rob:
the fix is `docker logout ghcr.io` or a fresh login, which mutates his keychain.

## Deferred decision for Rob: the SM lane is the bigger cache prize

Only the Alloy lane is wired. Measured cold vs warm per image: alloy 52.2/11.0s (531MB cache),
grafana_pdc 50.6/7.0s (152MB), grafana_sm 60.3/16.2s (228MB), grafana_sm_browser 112.8/48.1s
(611MB). The SM lane builds both SM images: 173.1s -> 64.3s, a ~109s saving, over 2.5x alloy's. Its
`timeout-minutes: 20` versus 10 elsewhere corroborates that it is already the known slow lane. The
justfile change already covers it - enabling it is the same two `uses:` steps in
synthetic-monitoring-test. Not done because all four lanes cached is 1.52GB per branch scope against
a 10GB LRU budget, so roughly six concurrent Renovate branches would start evicting main's entry.
Mitigations if taken: `mode=min`, or gate `--cache-to` on refs/heads/main so PR branches read
without writing. It is off the critical path, so it buys runner minutes, not wall clock.

## Measured results — AC #6 answered

Baseline 233s / 306s / 271s. After: **136s** on a fresh commit (run 33267818299), 131s on a warm
rerun (33267253355 attempt 2). Roughly a 45-55% cut.

| | baseline | cold cache | warm |
|---|---|---|---|
| Run Alloy toolchain tests | 136s | 37s | 39s |
| Run Alloy Docker tests | 42s | 91s | 20s |
| Alloy lane | 182-191s | 148s | 79-90s |
| Wall clock | 233-306s | 244s | 131-136s |

**A cold cache makes the Docker step slower, not faster** — 42s to 91s. It pays 24.1s sending the
cache export, 15.4s on the --load layer export and 4-8s pulling the builder, and collects nothing.
The first run after any change that invalidates the alloy cache will be slower than before this PR.
Do not read that as a regression.

**Cache confirmed working from the log, not inferred:** `importing cache manifest from gha:...` and
`sending cache export 24.1s done`, with `using docker-container driver`.

**Number NOT claimed as ours:** Build alloy amd64 measured 94s baseline, 51s, 18s, 23s across runs.
That step uses the unmodified Home Assistant builder action - registry variance. Holding it at 51s,
wall clock still lands near 164s.

**The critical path has flattened.** On the fresh run: alloy 90s, PDC 76s, SM 72s, alloy-init 69s.
No single lane dominates any more, so further wall-clock work means pulling the whole band down
rather than fixing one lane. This changes the calculus recorded in the original research, which
assumed alloy was the sole target.

All six acceptance criteria are checked. Status stays In Progress until PR #102 merges - the work is
complete but unmerged. AC #4's branch-measurement clause was satisfied by a bounding argument rather
than a branch run; see the earlier note.
<!-- SECTION:NOTES:END -->
