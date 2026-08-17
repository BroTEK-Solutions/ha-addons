---
id: doc-0002
title: Wave operating model
type: guide
created_date: '2026-08-17 12:03'
updated_date: '2026-08-17 12:45'
---
# Wave operating model — ha-addons

This repository's own rules. The campaign model itself — run contract, routing, lane briefs, the
goal-file template, the blocker contract, the pre-flight checklist — is the **Agent fan-out protocol
(canonical)** document; nothing here repeats it. Read both before designing a wave.

## The shape of the repo, as it affects lanes

Four Home Assistant Apps published as signed multi-arch images: `alloy/`, `grafana_pdc/`,
`grafana_sm/`, `grafana_sm_browser/`. The last two are **not source**. Cross-cutting machinery lives
in `.github/workflows/`, `scripts/` and `tests/`.

## Ownership map

**There are three independent work units, not four:** `alloy/`, `grafana_pdc/`, and
`synthetic_monitoring_shared/` — the last owning both generated variants with it. Do not read the
four published Apps as four lanes; `grafana_sm/` and `grafana_sm_browser/` share a source and a
launcher, so a lane on either is really a lane on the shared tree. The three units share no code with
each other, only CI orchestration. The files that are *not* unit-local are the contended ones:

| Path | Owner |
|---|---|
| `alloy/`, `grafana_pdc/` | one lane each, exclusive |
| `synthetic_monitoring_shared/` | one lane, and it owns **both** generated variants with it |
| `grafana_sm/`, `grafana_sm_browser/` | **never a lane of their own** — see below |
| `.github/workflows/`, `renovate.json`, `.yamllint`, `trivy.yaml` | a single wiring pass, never in parallel |
| `tests/`, `scripts/` | the lane whose change the test or script covers; if two lanes both need it, wiring pass |
| `.release-please-manifest.json`, any App `config.yaml` `version:` | **nobody** — release-please owns them |

**Escape hatch.** A lane that finds it must edit a path it does not own stops and returns the
question rather than reaching across the boundary. Two lanes editing `builder.yaml` in the same wave
is the failure this table exists to prevent, and it is silent — both edits apply, the workflow still
parses, and the second lane's job matrix quietly loses the first's.

## The two Synthetic Monitoring Apps are build output

`grafana_sm/` and `grafana_sm_browser/` are generated from `synthetic_monitoring_shared/` by
`python3 scripts/sync_synthetic_monitoring_variants.py`. CI runs the same script in check mode and
rejects drift, so a hand-edit to a generated file does not survive review — but it does waste a lane.

**Generated, never edit in the variant:** `config.yaml`, `DOCS.md`, `README.md`,
`translations/en.yaml`, `launcher/go.mod`, `launcher/main.go`, `icon.png`, `logo.png`.

**Variant-owned, edit in place:** `Dockerfile` and `CHANGELOG.md` only. The Dockerfiles are
deliberately not generated so Renovate can bump Grafana's standard and `-browser` images
independently; a contract test requires every line outside image, identity and entrypoint to stay
equivalent between the two, so an edit to one is almost always an edit to both.

**`main_test.go` is deliberately excluded from the copy.** `grafana_sm/launcher/` and
`grafana_sm_browser/launcher/` therefore contain no tests at all, and `go test ./...` inside either
one **passes vacuously**. The launcher is only really tested from
`synthetic_monitoring_shared/launcher`. A lane that reports the SM launcher green from a variant
directory has verified nothing.

## Versions belong to release-please, and the generator reads them

Never hand-edit an App's `config.yaml` `version:` or `.release-please-manifest.json`. The generated
release PR updates both together, and bypassing it can publish an image without matching release
state.

This has a second edge specific to this repo: `sync_synthetic_monitoring_variants.py` reads
`APP_VERSION` **out of `.release-please-manifest.json`**, so a hand-edited version silently
propagates into both generated variants on the next sync and looks like the generator's doing. To
force an exact version, use a `Release-As: X.Y.Z` footer on the App's commit and merge the release PR
normally.

## Conventional Commits are load-bearing, and PR titles are commits

`feat` selects a minor, fixes and other non-breaking changes a patch, `!` or a `BREAKING CHANGE`
footer a major. **When a PR is squash-merged its title becomes the commit subject on `main`**, so a
PR title without a Conventional Commit prefix silently costs a release. Scope by App
(`fix(alloy):`, `feat(alloy):`) — the history is consistently scoped and release-please routes on it.

## Recurring defects in this codebase

**Alloy config generation is the hotspot: 28 of the last 100 commits are `fix(alloy)`.** The
recurring shape is not a typo — it is a config-generation feature that lands *incomplete* and needs a
follow-up fix, usually within one PR of the original:

- `82f4203 feat(alloy): move stability level to config.yaml as a native option` →
  `27b3a85 fix(alloy): complete the native stability option and stop reporting successful restarts as failures`
- `889ca64 feat(alloy): render Fleet starter pipelines with editable placeholders` →
  `fdf6c9b fix(alloy): drop starter endpoints for deselected signals`
- `557d5aa fix(alloy): pin metric and log identity to the instance name` →
  `5fcdb30 fix(alloy): scope identity rewrite to the host exporter`

The common cause is **mode interaction**. Alloy has local, Fleet-managed, hybrid and break-glass
configurations, and a change proved against one mode breaks another
(`d4b54e9 fix(alloy): keep Alloy startup flags reachable in Fleet and break-glass modes`). An Alloy
lane's acceptance criteria must name **every mode the change touches**, and
`bash alloy/tests/generate-config.test.sh` must be run, not just the schema test.

**The other one that has bitten CI: generated output formatting.**
`8336801 fix(ci): preserve generated config formatting in releases` — the release path reformatted
generated config. Anything that rewrites a generated artifact must round-trip byte-for-byte, and the
check is running the generator twice, not reading the diff.

## Rules this project adds

**Any directory holding a `config.yml` at depth 2 becomes an App.**
`home-assistant/actions/helpers/find-addons` discovers Apps with
`find ./ -maxdepth 2 -name config.json -o -name config.yaml -o -name config.yml`, and it has no
exclude mechanism. Adding *any* top-level directory with a `config.{json,yaml,yml}` in it silently
enlists that directory in the build matrix, which then fails on missing `name`, `slug`, `version`,
`arch` and `description` — five errors that read like a broken App rather than a misplaced file.

This is why the tracker config is `backlog.config.yml` at the repository root and not
`backlog/config.yml` (`backlog init --config-location root`). It looks like an untidy stray and must
not be "fixed": `backlog/docs/` has to stay at that exact path for the fleet's sync and drift tooling,
so the config is the part that moves. Found on the Backlog.md setup PR itself, after CI failed.

**`.yamllint` caps lines at 120 and ignores only `.git/`.** Every YAML file a lane adds anywhere in
the tree is linted by `yamllint --strict .`, including files that feel like tooling rather than
source. This also bit the Backlog.md setup: `backlog.config.yml`'s `definition_of_done` entries
exceeded 120 characters and had to be written as folded scalars. If the `backlog` CLI ever rewrites
that file it will re-flatten them, and the gate will fail — reapply the folding.

**Reusable workflows are pinned to a release SHA with the version in a trailing comment.** zizmor
fails a reintroduced floating `@main`, and the pin and the comment must move together or Renovate's
update flow breaks. `.github/workflows/security.yml` is the reference.

**This repository is `BroTEK-Solutions`, which is outside the org list that gets direct pushes to
`main`.** Every change lands as a branch and a pull request, no exceptions — the standing "push
straight to main on Rob's own repos" rule does not reach here. Branch naming follows the existing
history: `feat/`, `fix/`, `chore/`, or `codex/<topic>` for agent work.

## Exclusive resources

**The Docker daemon, and specifically the smoke-test image tags.** `local/ha-alloy:smoke`,
`local/ha-grafana-sm:smoke` and `local/ha-grafana-sm-browser:smoke` are fixed names. Lanes on
*different* Apps can build concurrently; **two lanes on the same App cannot**, because the second
build silently retags the first's image out from under a running smoke test. Serialize, or skip the
image smoke tests in the lane and run them once in the wiring pass.

Docker is also simply absent on some machines. `scripts/cloud-environment-setup.sh` says so out loud
and returns 0 anyway, so a cloud lane can report the gate green having skipped every image test. A
lane that could not run the image tests must say so rather than checking the criterion.

## Run-end against this tracker

Milestones are the wave (`backlog task list --plain -m <wave> -s "To Do"` is the queue). Beyond the
protocol's run-end contract, this repo's specifics:

- A `Done` task's final summary carries the **merge commit SHA on `main`**, not the branch SHA —
  work here lands through a PR, so the branch SHA is not what shipped. Use `--append-final-summary`.
- Work that shipped but has not released yet is still `Done`. The release PR is release-please's, not
  a task.
- A task discovered mid-run gets the `needs-triage` label. An Alloy task additionally gets a label
  naming the configuration modes it touches, because that is what the next lane needs to know before
  it can scope its own acceptance criteria.
