---
id: HAB-0008
title: Migrate the repo task surface to just and retire Makefiles and ad-hoc scripts
status: To Do
assignee: []
created_date: '2026-08-28 19:27'
labels: []
dependencies: []
priority: medium
type: chore
ordinal: 8000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Fleet-wide migration of the developer and CI task surface to the `just` command runner. This repo's
share of it. The fleet standard is frozen; do not re-litigate the recipe vocabulary, the group
taxonomy or the header.

## 1. Outcome

`ha-addons` gains one self-contained top-level `justfile` that is the only place the repository's
build, lint, format, generate and test commands are written down. `just --list` becomes the answer to
"what can I do here". The five CI test lanes in `.github/workflows/builder.yaml` each collapse to a
single `run: just <recipe>` line behind a pinned `extractions/setup-just` step, keeping the five job
names, `ci-success`'s `needs:` list, every `permissions:` block, every SHA pin and every
`persist-credentials: false` byte-identical. `AGENTS.md`'s hand-maintained command block and
`backlog.config.yml`'s six `definition_of_done` entries are replaced by the `just` contract.
`tests/repository_workflow_contract_test.py` is retargeted so it asserts the same guarantees against
the justfile instead of against `run:` bodies that no longer exist. **There is no Makefile in this
repo** and no script is deleted: every shell script here is either a shipped runtime artifact, a
shell test suite, or a cloud-agent bootstrap, all of which §6 of the standard says to KEEP.

## 2. The complete justfile

Drop this in at the repository root as `justfile`. It has been parsed and validated against
`just 1.58.0`: `just --fmt --check` exits 0, `just --list` renders a doc comment and a group for
every public recipe, `just --dump --dump-format json` succeeds, and `just --dry-run check` expands to
exactly the command set the five CI lanes run today.

```just
set shell := ["bash", "-euo", "pipefail", "-c"]

# Go modules this repository OWNS. Everything under alloy/reporter,
# grafana_pdc/reporter, grafana_sm/ and grafana_sm_browser/ is a generated copy
# written by scripts/sync_shared_lib.py or
# scripts/sync_synthetic_monitoring_variants.py; formatting or linting those
# reports every finding twice and is reverted by the next `just gen`.
go_source_modules := "alloy/ui grafana_pdc/ui shared/reporter synthetic_monitoring_shared/launcher synthetic_monitoring_shared/ui"

# show the task surface
default:
    @just --list

# download Go module dependencies and verify the host toolchain is complete
[script('bash')]
setup:
    set -euo pipefail
    for module in {{ go_source_modules }}; do
        (cd "$module" && go mod download)
    done
    missing=()
    for tool in python3 node go gofmt yamllint shellcheck; do
        command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
    done
    python3 -c 'import voluptuous, yaml' >/dev/null 2>&1 || missing+=("python3 modules pyyaml+voluptuous")
    if ((${#missing[@]})); then
        printf 'setup: missing from this machine: %s\n' "${missing[*]}" >&2
        printf 'setup: cloud agents run `bash scripts/cloud-environment-setup.sh` instead\n' >&2
        exit 1
    fi
    command -v docker >/dev/null 2>&1 || printf 'setup: docker absent - the image lanes in `just check` will fail\n' >&2

# format Go sources and this justfile in place, then refresh the generated copies
[group('check')]
fmt: && gen
    gofmt -w {{ go_source_modules }}
    just --fmt

# verify formatting; never mutates
[group('check')]
[no-exit-message]
fmt-check:
    just --fmt --check
    unformatted="$(gofmt -l {{ go_source_modules }})"; if [ -n "$unformatted" ]; then printf 'unformatted Go sources:\n%s\n' "$unformatted" >&2; exit 1; fi

# lint every YAML file and every tracked shell script, including the s6 services
[group('check')]
[no-exit-message]
lint:
    yamllint --strict .
    shellcheck -x --source-path=SCRIPTDIR \
        shared/lib/ha-validate.sh \
        tests/shared_validate_lib_test.sh \
        alloy/rootfs/usr/share/alloy/generate-config.sh \
        alloy/rootfs/etc/s6-overlay/s6-rc.d/init-alloy/run \
        alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/run \
        alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/finish \
        alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy-reporter/run \
        alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy-reporter/finish \
        alloy/tests/generate-config.test.sh \
        alloy/tests/init-alloy.test.sh \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/init-grafana-pdc/run \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/run \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/finish \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc-reporter/run \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc-reporter/finish \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc-ui/run \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc-ui/finish \
        grafana_pdc/tests/service.test.sh \
        grafana_pdc/tests/image-smoke.test.sh \
        grafana_pdc/tests/fixtures/bashio \
        grafana_pdc/tests/fixtures/fake-pdc

# rewrite every generated copy from its source tree
[group('gen')]
gen:
    python3 scripts/sync_shared_lib.py
    python3 scripts/sync_synthetic_monitoring_variants.py

# fail when a generated copy has drifted from its source
[group('gen')]
[no-exit-message]
gen-check:
    python3 scripts/sync_shared_lib.py --check
    python3 scripts/sync_synthetic_monitoring_variants.py --check

# run every test lane; needs a working Docker daemon
[group('check')]
test: test-repo test-pdc test-alloy test-alloy-init test-sm

# repository contract tests, shared validator library and shared MQTT reporter
[group('check')]
[no-exit-message]
test-repo:
    bash tests/shared_validate_lib_test.sh
    (cd shared/reporter && go test ./...)
    python3 tests/app_metadata_contract_test.py
    python3 tests/app_contract_test.py
    python3 tests/app_version_changed_test.py
    python3 tests/renovate_config_contract_test.py
    python3 tests/repository_workflow_contract_test.py
    python3 tests/synthetic_monitoring_variants_test.py

# Grafana PDC schema, status page, S6 service and complete-image tests
[group('check')]
[no-exit-message]
test-pdc:
    python3 grafana_pdc/tests/config-schema.test.py
    python3 grafana_pdc/tests/image-contract.test.py
    (cd grafana_pdc/ui && go test ./...)
    ha_base="$(awk '/^FROM ghcr.io\/home-assistant\/base:/ { print $2; exit }' grafana_pdc/Dockerfile)"; \
        test -n "$ha_base"; \
        docker run --rm -e BASHIO_BIN=/usr/bin/bashio -v "${PWD}:/w" -w /w \
        "$ha_base" bash grafana_pdc/tests/service.test.sh
    docker build --tag local/ha-grafana-pdc:smoke grafana_pdc
    bash grafana_pdc/tests/image-smoke.test.sh local/ha-grafana-pdc:smoke

# Alloy schema, ingress UI, browser state, generator and complete-image tests
[group('check')]
[no-exit-message]
test-alloy:
    python3 alloy/tests/config-schema.test.py
    python3 alloy/tests/image-contract.test.py
    (cd alloy/ui && go test ./...)
    node alloy/tests/ui-static.test.mjs
    bash alloy/tests/generate-config.test.sh
    docker build --tag local/ha-alloy:smoke alloy
    bash alloy/tests/image-smoke.test.sh local/ha-alloy:smoke

# Alloy init-service tests, inside the base image pinned by alloy/Dockerfile
[group('check')]
[no-exit-message]
test-alloy-init:
    alloy_base="$(awk -F= '/^ARG BUILD_FROM=ghcr.io\/home-assistant\/base-debian:/ { print $2; exit }' alloy/Dockerfile)"; \
        test -n "$alloy_base"; \
        docker run --rm -v "${PWD}:/w" -w /w \
        "$alloy_base" bash alloy/tests/init-alloy.test.sh

# shared Synthetic Monitoring launcher tests and both variant image smoke tests
[group('check')]
[no-exit-message]
test-sm:
    (cd synthetic_monitoring_shared/launcher && go test ./...)
    docker build --tag local/ha-grafana-sm:smoke grafana_sm
    docker build --tag local/ha-grafana-sm-browser:smoke grafana_sm_browser
    python3 tests/synthetic_monitoring_image_smoke_test.py local/ha-grafana-sm:smoke standard
    python3 tests/synthetic_monitoring_image_smoke_test.py local/ha-grafana-sm-browser:smoke browser

# THE GATE - everything CI enforces, in one command
[group('check')]
check: fmt-check lint gen-check test

# compile every source Go module without keeping artifacts
[group('build')]
build:
    for module in {{ go_source_modules }}; do (cd "$module" && go build ./...); done

# build one App's complete image locally
[group('build')]
image app tag="local/ha-dev:smoke":
    docker build --tag '{{ tag }}' '{{ app }}'

# delete the local `go build` output listed in .gitignore
[group('dev')]
clean:
    rm -f alloy/ui/alloy-ui grafana_pdc/ui/grafana-pdc-ui shared/reporter/ha-reporter synthetic_monitoring_shared/launcher/synthetic-monitoring-launcher
```

Notes on the deviations, so nobody "fixes" them:

- **No `[confirm]` recipe exists and none is missing.** Nothing in this repo mutates a remote from a
  developer's shell: image publication and Git tags belong to `home-assistant/builder` and
  release-please inside GitHub Actions. `clean` deletes only `go build` output already listed in
  `.gitignore`, which `just build` reproduces.
- **`test` takes no `filter` parameter.** The suite is four different runners (python3 scripts, `go
  test`, `bash` harnesses, `node`); there is no single filter flag to forward.
- **`setup` does not create a venv and does not `pip install`.** CI installs `pyyaml voluptuous
  yamllint` with `actions/setup-python` + `pip`, and every test invokes a bare `python3`. A repo-local
  venv would change every recipe body away from what CI runs. `setup` therefore does the safe,
  sudo-free half - `go mod download` per source module - and fails loudly listing what is missing.
- **`build` and `clean` are optional-vocabulary recipes** (§2) and are genuinely backed here: five
  compilable Go modules and four ignored binary outputs.
- **`check` includes the Docker lanes.** §1 forbids `check` being a subset of what CI enforces, and
  `ci-success` gates on all five lanes. `just test-repo` remains the fast Docker-free subset for
  iteration, but it is not the gate.

## 3. Makefile disposition

**There is no Makefile, GNUmakefile or makefile anywhere in this repository.** Verified with
`git ls-files | grep -Ei '(^|/)(GNUmakefile|Makefile|makefile)(\.|$)'` - zero matches. Nothing to
translate, nothing to `git rm`. If one appears before this task is implemented, translate it in full
per §12 of the standard and `git rm` it.

## 4. Script disposition

Every tracked script, from `git ls-files | grep -E '\.(sh|bash|zsh|ps1)$'` plus the extensionless s6
service scripts and the Python helpers. **Nothing here is ABSORB.** That is the correct answer for
this repo, not an oversight - do not delete any of these files.

| File | Disposition | Recipe that reaches it | Why it survives |
| --- | --- | --- | --- |
| `shared/lib/ha-validate.sh` | KEEP | `just lint` (shellcheck), `just test-repo` (its unit tests) | Shipped runtime library, copied into two App rootfs images by `scripts/sync_shared_lib.py`; executes inside a container that has no `just` |
| `alloy/rootfs/usr/share/ha-lib/ha-validate.sh` | KEEP (generated) | `just gen` / `just gen-check` | Generated copy of the above; never edited by hand |
| `grafana_pdc/rootfs/usr/share/ha-lib/ha-validate.sh` | KEEP (generated) | `just gen` / `just gen-check` | Same |
| `alloy/rootfs/usr/share/alloy/generate-config.sh` | KEEP | `just lint`, `just test-alloy` | Shipped runtime artifact - the Alloy config generator that runs inside the App image |
| `alloy/rootfs/etc/s6-overlay/s6-rc.d/**/{run,finish,up}` | KEEP | `just lint`, `just test-alloy-init` | s6 service scripts executed by the supervision tree inside the image |
| `grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/**/{run,finish,up}` | KEEP | `just lint`, `just test-pdc` | Same |
| `alloy/tests/generate-config.test.sh` (361 lines) | KEEP | `just test-alloy` | Shell test suite with its own harness, counters and Dockerfile parsing |
| `alloy/tests/init-alloy.test.sh` (455 lines) | KEEP | `just test-alloy-init` | Shell test suite; runs inside the pinned Debian base with `BASHIO_BIN` |
| `alloy/tests/image-smoke.test.sh` (128 lines) | KEEP | `just test-alloy` | Shell test suite; takes an `IMAGE` argument, uses `trap`/`mktemp` cleanup |
| `grafana_pdc/tests/service.test.sh` (346 lines) | KEEP | `just test-pdc` | Shell test suite; runs inside the pinned HA base |
| `grafana_pdc/tests/image-smoke.test.sh` (70 lines) | KEEP | `just test-pdc` | Shell test suite; `trap` cleanup, health-poll loop, takes an `IMAGE` argument |
| `tests/shared_validate_lib_test.sh` (165 lines) | KEEP | `just test-repo` | Shell test suite with a `getent` stub and `trap` cleanup |
| `grafana_pdc/tests/fixtures/bashio`, `grafana_pdc/tests/fixtures/fake-pdc` | KEEP | `just lint` | Test fixtures executed inside containers as stand-ins for real binaries |
| `scripts/cloud-environment-setup.sh` (128 lines) | KEEP | not wrapped - see below | Invoked by Codex and Claude Code cloud provisioning, not by a developer or by CI. Functions, `EUID` branching, `apt-get`, global `npm --prefix`, `.bashrc` persistence. `just` is not installed when it runs, and `just setup` must not `sudo` |
| `scripts/app_version_changed.py` (107 lines) | KEEP | called from the `init` workflow job (unchanged) | A real program - `git show` inspection and YAML/JSON version parsing - and it is workflow-orchestration input, not a developer task |
| `scripts/sync_shared_lib.py` (134 lines) | KEEP | `just gen`, `just gen-check` | A real program: generated-copy renderer with banner injection and a `--check` drift mode |
| `scripts/sync_synthetic_monitoring_variants.py` (145 lines) | KEEP | `just gen`, `just gen-check` | Same, plus template rendering for both SM variants |
| `tests/*.py`, `alloy/tests/*.test.py`, `grafana_pdc/tests/*.test.py` | KEEP | `just test-repo`, `just test-alloy`, `just test-pdc` | Test programs |
| `alloy/tests/ui-static.test.mjs` | KEEP | `just test-alloy` | Test program |

`scripts/cloud-environment-setup.sh` deliberately gets **no** recipe. Wrapping it would put a
`sudo apt-get` / global `npm install` path behind `just setup`, which §1 forbids. It stays documented
in `README.md` as the cloud-provider setup command and `just setup` prints a pointer to it when a tool
is missing.

## 5. CI changes

Four workflow files. Only `builder.yaml` changes.

### `.github/workflows/builder.yaml`

Insert this step immediately after `Checkout repository` in each of the five test jobs (`quality`,
`pdc-test`, `alloy-generator-test`, `alloy-init-test`, `synthetic-monitoring-test`), and **only**
those five. Exact YAML, SHA resolved live from `repos/extractions/setup-just` tag `v4`:

```yaml
      - name: Set up just
        uses: extractions/setup-just@53165ef7e734c5c07cb06b3c8e7b647c5aa16db3 # v4
        with:
          just-version: '1.58.0'
```

Pin `just-version` exactly: `just --fmt` output carries no backwards-compatibility guarantee, so an
unpinned bump can turn `fmt-check` red with no repo change. The action must be SHA-pinned or
`tests/repository_workflow_contract_test.py:260-265` fails - it walks every non-local `uses:` in
`builder.yaml`, `build-app.yaml` and `release.yaml` and requires a trailing 40-hex SHA. Renovate's
`helpers:pinGitHubActionDigests` (already extended in `renovate.json:5`) keeps it pinned afterwards.
If the SHA above is stale by implementation time, re-resolve it with
`gh api repos/extractions/setup-just/git/refs/tags --jq '.[] | select(.ref=="refs/tags/v4") | .object.sha'`.

Step-by-step edits:

| Job | Step (current line) | Becomes |
| --- | --- | --- |
| `quality` | `YAMLLint` - `run: yamllint --strict .` (:41) | deleted; folded into `run: just lint` |
| `quality` | `ShellCheck` - the 22-file `run: \|` block (:69-91) | deleted; folded into the same `run: just lint`, keeping its explanatory comment above the step |
| `quality` | `Shared validator library tests` (:94) | deleted; folded into `run: just test-repo` |
| `quality` | `Shared MQTT reporter tests` with `working-directory: shared/reporter` (:102-104) | deleted; folded into `run: just test-repo` |
| `quality` | `Repository contract tests` `run: \|` block (:107-114) | deleted; folded into `run: just test-repo` |
| `pdc-test` | `PDC schema and image contract tests` (:136-139) | deleted; folded into `run: just test-pdc` |
| `pdc-test` | `PDC status page tests` with `working-directory: grafana_pdc/ui` (:147-149) | deleted; folded into `run: just test-pdc` |
| `pdc-test` | `PDC service tests in the Home Assistant base` (:151-157) | deleted; folded into `run: just test-pdc` |
| `pdc-test` | `PDC complete-image smoke test` (:159-162) | deleted; folded into `run: just test-pdc` |
| `alloy-generator-test` | `Alloy schema and image contract tests` (:190-193) | deleted; folded into `run: just test-alloy` |
| `alloy-generator-test` | `Alloy ingress UI tests` (:195-197) | deleted; folded into `run: just test-alloy` |
| `alloy-generator-test` | `Alloy browser state tests` (:199-200) | deleted; folded into `run: just test-alloy` |
| `alloy-generator-test` | `Alloy generator tests` (:202-203) | deleted; folded into `run: just test-alloy` |
| `alloy-generator-test` | `Alloy complete-image ingress smoke test` (:205-208) | deleted; folded into `run: just test-alloy` |
| `alloy-init-test` | `Alloy initialization tests` `run: \|` block (:222-228) | `run: just test-alloy-init` |
| `synthetic-monitoring-test` | `Test the shared launcher` (:248-250) | deleted; folded into `run: just test-sm` |
| `synthetic-monitoring-test` | `Build and smoke-test both complete images` (:252-257) | deleted; folded into `run: just test-sm` |

After the edit each job's step list reads: checkout → set up just → (existing language setup steps)
→ one or two `run: just <recipe>` lines. The `quality` job ends with two: `run: just lint` first, then
`run: just test-repo`, so a lint failure is still reported before the tests run.

**Do not change any of the following in `builder.yaml`:**

- The `ci-success` job (:362-388), its `name: ci-success`, or its `needs:` list. The branch ruleset
  gates on that exact check name and `tests/repository_workflow_contract_test.py:218-220` asserts the
  set is `{init, quality, pdc-test, alloy-generator-test, alloy-init-test, synthetic-monitoring-test,
  build-app, security}`.
- The `release` job's `needs:` list and its `if:` expression (:390-421), asserted at
  `repository_workflow_contract_test.py:221-234`.
- The five test-job names. `repository_workflow_contract_test.py:120-128` requires exactly those five
  and explicitly fails if a job named `test` exists.
- `build-app`'s `needs:` (:336), asserted at :183-185.
- `permissions:` blocks (top-level `permissions: {}` at :10 and every per-job block),
  `concurrency:` (:15-17), `env: MONITORED_FILES` (:12-13), `timeout-minutes`, and every
  `persist-credentials: false`.
- The `init` job's entire `Select changed Apps` shell block (:283-332). That is GitHub-native matrix
  orchestration writing `$GITHUB_OUTPUT`, not build/test/lint logic. It must keep calling
  `python3 scripts/app_version_changed.py` verbatim (asserted at :190 of the contract test) and keep
  `fetch-depth: 0` (asserted at :209-211).
- Every existing SHA-pinned `uses:`, including `actions/checkout`, `actions/setup-python`,
  `actions/setup-go`, `tj-actions/changed-files`, the four `frenck/action-addon-linter` steps and the
  two `home-assistant/actions` helpers. **Never convert a `uses:` into a `run: just`.** The four App
  metadata linters stay exactly as they are.
- `uses: ./.github/workflows/build-app.yaml`, `uses: ./.github/workflows/security.yml`,
  `uses: ./.github/workflows/release.yaml` and the `secrets: RELEASE_PLEASE_TOKEN` mapping.
- The `actions/setup-python` and `actions/setup-go` steps and their `pip install` step. They provision
  the runner; `just` recipes inherit the resulting PATH.

### `.github/workflows/build-app.yaml`, `.github/workflows/release.yaml`, `.github/workflows/security.yml`

**No changes.** `build-app.yaml` is entirely `uses:` calls into `home-assistant/builder` plus one
`jq` normalization step that writes `$GITHUB_OUTPUT` - GitHub-native orchestration. `release.yaml` is
release-please. `security.yml` is three reusable-workflow calls into
`rknightion/.github` (actionlint, zizmor, docker-security). §8 of the standard puts all of these
out of scope.

### `tests/repository_workflow_contract_test.py` - mandatory companion edit

This is the single largest trap in this repo and the CI change **cannot** land without it. Lines
133-167 build an `expected_commands` dict of literal command strings and assert each appears in the
JSON dump of the named job. Every one of those strings moves out of the workflow and into the
justfile. Lines 168-182 do the same for the two `awk`-derived base-image expressions.

Retarget, do not weaken:

1. Replace `expected_commands` with a map of job name → the `just` recipe that job must invoke:
   `quality` → `just lint` and `just test-repo`; `pdc-test` → `just test-pdc`;
   `alloy-generator-test` → `just test-alloy`; `alloy-init-test` → `just test-alloy-init`;
   `synthetic-monitoring-test` → `just test-sm`. Keep the existing `job_text()` JSON-dump lookup.
2. Add a second assertion block that reads `justfile` from `ROOT` and requires the same command
   strings the old dict held, so the guarantee moves rather than evaporating. That includes the four
   `docker build --tag local/ha-*` lines and both `python3 tests/synthetic_monitoring_image_smoke_test.py`
   invocations with their `standard` / `browser` arguments.
3. Move the four `BASHIO_BIN=/usr/bin/bashio` / `grafana_pdc/Dockerfile` /
   `ghcr.io\/home-assistant\/base:` / `"$ha_base"` checks (:168-175) and the three
   `alloy/Dockerfile` / `ARG BUILD_FROM=ghcr.io\/home-assistant\/base-debian:` / `"$alloy_base"`
   checks (:176-182) from `caller` to the justfile text. Their failure messages ("must derive the
   pinned HA base from the App Dockerfile") stay accurate.
4. Add an assertion that `builder.yaml` contains `uses: extractions/setup-just@` so the runner cannot
   be dropped silently, and that the justfile exists at all.
5. Leave everything else in that file alone - the release-please contract (:36-113), the
   changed-App monitoring keys (:117-119), the `secrets: inherit` prohibition (:203-204), the
   `security.yml` trigger checks (:213-217), the Trivy config checks (:272-292), the README release
   guidance (:294-305) and the per-App asset checks (:307-318).

## 6. Docs and agent-contract changes

### `AGENTS.md`

Delete lines 10-47 - the `## Commands` heading, the 26-line bash block, the ShellCheck paragraph and
the image-smoke paragraph. Replace with the §9 contract, adjusted only where this repo genuinely
differs:

```markdown
## Task interface

This repo's task surface is a `justfile`. Discover it, don't guess it:

    just --list                        # human-readable
    just --dump --dump-format json     # machine-readable
    just --show <recipe>               # what a recipe actually runs

- `just check` is the full gate and is exactly what CI enforces. It must pass before you commit.
- Prefer `just <recipe>` over the underlying tool. If you are typing `python3 tests/...`, you want
  `just test-repo`.
- Run `just` with stdin from /dev/null. Recipes marked `[confirm]` are destructive - stop and ask
  before running one; never pass `--yes` or `JUST_YES=1`. (This repo currently has none.)
- If a task you need does not exist, add a recipe with a `#` doc comment and a `[group(...)]`
  rather than running a bare command.
- `just check` runs the Docker image lanes and needs a working daemon. `just test-repo` is the
  Docker-free subset for fast iteration, but it is not the gate.
- `scripts/cloud-environment-setup.sh` stays the Codex / Claude Code cloud provisioning command. It
  is deliberately not a recipe: it uses sudo and installs globally, which `just setup` must not.
```

**Do not paste the recipe list into `AGENTS.md`.** It rots on the next recipe and the agent will then
trust the stale copy.

Keep `AGENTS.md` lines 49-107 (the "Rules that are easy to break by accident" and "Task tracking"
sections) unchanged except: the sentence at :51-52 telling the reader to "run the sync script" should
say `just gen`, and the ShellCheck-file-list paragraph is replaced by "`just lint` owns the ShellCheck
file list; the s6 service scripts have no extension so it cannot be globbed."

### `CLAUDE.md`

No change. It is a five-line stub that `@AGENTS.md`-imports the contract.

### `README.md`

- Line 34: `Run \`python3 scripts/sync_synthetic_monitoring_variants.py\` after changing ...` →
  `Run \`just gen\` after changing ...`. Keep the following sentence about CI running the same
  command in check mode; it is still true via `just gen-check`.
- Lines 51-64 (`## AI cloud environments`): unchanged. `bash scripts/cloud-environment-setup.sh` at
  :59 stays exactly as written - it is the string a provider's settings box holds.
- `repository_workflow_contract_test.py:294-305` asserts seven release-guidance phrases are present in
  `README.md`; none of them are touched by the above.

### App-level docs

`alloy/DOCS.md`, `grafana_pdc/DOCS.md`, `grafana_sm/DOCS.md`, `grafana_sm_browser/DOCS.md` and the
per-App `README.md` files are end-user documentation for Home Assistant users and contain no `make`
or repo-script references. Leave them alone. `grafana_sm*/DOCS.md` and `README.md` are generated from
`synthetic_monitoring_shared/template/` anyway.

## 7. `backlog.config.yml`

**The tracker config is at the repository root as `backlog.config.yml`, not `backlog/config.yml`.**
That is deliberate and load-bearing: `home-assistant/actions/helpers/find-addons` globs
`find ./ -maxdepth 2 -name config.json -o -name config.yaml -o -name config.yml`, so a
`backlog/config.yml` would be discovered as a fifth App and fail the build. Do not move it.

Replace the whole `definition_of_done:` block (:7-41) with:

```yaml
definition_of_done:
  - "just check"
  - >-
    FAST subset while iterating (not the gate):
    just fmt-check && just lint && just gen-check && just test-repo
```

Both entries stay under 120 columns so `yamllint --strict .` still passes - the folded scalar form is
required for the second, per the comment at :4-6 which stays. Every other key in the file is
unchanged.

The six current entries name commands that will no longer be the repo's interface, and the fourth
one is already drifted: it omits `python3 tests/app_contract_test.py`, which the `quality` job does
run (`builder.yaml:109`). `just check` closes that gap by construction.

Existing open tasks `hab-0001` through `hab-0007` carry the old six-line DoD copied into their
markdown at creation time. **Do not hand-edit those files** - `backlog.config.yml` only affects
newly created tasks, and rewriting the HTML-comment-delimited sections by hand breaks them
irreparably. Leave them.

## 8. Order of work

1. Install `just 1.58.0` locally (`brew install just`; confirm with `just --version`).
2. Write the `justfile` from §2 verbatim at the repo root. Nothing else changes yet.
3. `just --fmt --check` → must exit 0. `just --list` → must show a doc comment and a group for every
   recipe. `just --dump --dump-format json > /dev/null` → must exit 0.
4. `just fmt`. This is the one intentionally mutating step: `gofmt -l` currently reports
   `alloy/ui/config_test.go` (recorded in `hab-0004`), so `just fmt` reformats it and then re-runs
   `just gen`. Inspect the diff - it must be whitespace in that one file plus, if `shared/reporter`
   moved, its four generated copies. Commit this separately from the justfile.
5. `just check` locally, with Docker running. Everything must pass before a single workflow line
   changes. This proves the recipes are faithful transcriptions.
6. Edit `tests/repository_workflow_contract_test.py` per §5. Run `just test-repo` - it will fail
   until step 7 lands, because the retargeted assertions now look for `just` in the workflow. Land
   steps 6 and 7 in the same commit.
7. Edit `.github/workflows/builder.yaml` per §5: add the five `setup-just` steps, collapse the
   `run:` bodies. Re-run `just check` - `lint` (yamllint over the changed workflow) and `test-repo`
   (the retargeted contract test) both have to pass.
8. Update `AGENTS.md` and `README.md` per §6, and `backlog.config.yml` per §7. Re-run
   `yamllint --strict .` via `just lint`.
9. Push the branch and confirm on GitHub that all five test jobs are green, `ci-success` is green,
   and actionlint + zizmor in the `security` job are green. `actionlint` runs ShellCheck over
   embedded `run:` bodies, so collapsing them can only reduce its findings.
10. There are **no deletions in this migration.** No Makefile exists and every script is a KEEP.
    Nothing is `git rm`'d.

## 9. Traps specific to this repo

- **`just --fmt` sorts stacked attributes alphabetically.** `[group('check')]` must be written
  *above* `[no-exit-message]`, and `[private]` above `[script('bash')]`. Getting this backwards is
  the only thing that made `just --fmt --check` fail while validating §2's file. Verified on 1.58.0.
- **`just --fmt` and `just --fmt --check` need no `--unstable` on 1.58.0.** Verified directly - both
  exit 0 on a formatted file. Do not add the flag.
- **`tests/repository_workflow_contract_test.py` asserts on the literal text of `builder.yaml`.**
  Editing the workflow without editing that test turns the `quality` job red at :133-182. See §5.
- **Each recipe line is its own shell.** `set shell := ["bash", "-euo", "pipefail", "-c"]` applies to
  those lines. The two `awk`-derived base-image blocks are therefore written as one logical line with
  `;` separators and `\` continuations, and `(cd dir && go test ./...)` uses a subshell rather than a
  bare `cd`. Do not "tidy" either into separate lines.
- **`[script('bash')]` does NOT inherit `set shell`.** The `setup` recipe's body starts with an
  explicit `set -euo pipefail` for exactly that reason. Any new `[script(...)]` recipe must do the
  same.
- **`gofmt` must not touch the generated modules.** `alloy/reporter`, `grafana_pdc/reporter`,
  `grafana_sm/{reporter,launcher,ui}` and `grafana_sm_browser/{reporter,launcher,ui}` are written by
  the two sync scripts with a `// Code generated ... DO NOT EDIT.` banner. The `go_source_modules`
  variable is the allowlist; adding a generated path to it double-reports every finding and the next
  `just gen` reverts the write.
- **`just fmt` uses a post-dependency (`fmt: && gen`)** so `gofmt -w shared/reporter` propagates into
  the four App build contexts. Without it, `just fmt` leaves the tree failing `just gen-check`.
- **Known pre-existing gofmt drift.** `gofmt -l` reports `alloy/ui/config_test.go` today - it has
  never been gated. Step 8.4 fixes it. Use plain `gofmt`, **not** `gofmt -s`: `-s` applies
  simplifications beyond formatting and would widen the diff past what `hab-0004` scoped.
- **`hab-0004` overlaps this task and is not superseded by it.** Landing this satisfies its AC #1
  (`gofmt -l` clean), #2 (CI fails on unformatted Go, via `fmt-check` inside `check`), #4 (generated
  copies skipped) and #5 (in `backlog.config.yml` and `AGENTS.md`). Its AC #3 - choosing a
  vet-or-stronger bar - is deliberately **not** done here. `go vet` and `golangci-lint` are new gates
  with unknown findings across eleven modules; adding them would stop this migration being a pure
  refactor. When `hab-0004` is worked, it adds `go vet` to `just lint` and nothing else changes.
- **`synthetic_monitoring_shared/ui` has `main_test.go` that CI never runs.** The
  `synthetic-monitoring-test` job tests only `synthetic_monitoring_shared/launcher`. §2 keeps that
  parity: `test-sm` does not add the `ui` module, because `check` matching CI exactly is the contract
  and a newly-run test suite of unknown state would make this migration land red. The module is
  still in `go_source_modules` so it is formatted and built. Adding its tests is a separate task.
- **The `alloy-generator-test` job's `setup-go` uses `go-version-file: alloy/ui/go.mod` (go 1.26)**
  while `synthetic-monitoring-test` uses `synthetic_monitoring_shared/launcher/go.mod` (go 1.24).
  Keep both steps where they are; `just` does not manage Go toolchains. Locally, one Go ≥1.26 covers
  every module.
- **Docker is required by three of the five lanes.** `just check` will fail on a machine without a
  running daemon. `just setup` warns about this rather than failing, since `just lint`, `just fmt`
  and `just test-repo` all work without it.
- **`.yamllint` lints every YAML file in the tree except `.git/`, capping lines at 120.** The
  `setup-just` step and the rewritten `run:` lines are all short, but `backlog.config.yml`'s new
  `definition_of_done` must keep its folded-scalar form.
- **`yamllint` does not lint the justfile**, and no ShellCheck target covers it either - the recipe
  bodies that were previously inside `run:` blocks lose actionlint's embedded-ShellCheck coverage.
  `just --fmt --check` plus `just --dry-run check` are the only structural gates on it. Read the
  dry-run output once after step 7 and compare it against the pre-migration workflow.
- **`just` is not installed on the Home Assistant devcontainer image** (`.devcontainer.json` uses
  `ghcr.io/home-assistant/devcontainer:5-apps`). This task does not add it; note it if a devcontainer
  user reports `just: command not found`.
- **`{{` is `just`'s interpolation sigil.** None of §2's recipe bodies contain a literal `{{`, but
  `grafana_pdc/tests/image-smoke.test.sh` does (a Go template in `docker inspect --format`). That is
  why the smoke tests stay as script files and are never inlined.

## 10. Out of scope

Do not touch, do not "improve while you are in there":

- **Every KEEP script in §4.** Specifically: `scripts/cloud-environment-setup.sh`,
  `scripts/app_version_changed.py`, `scripts/sync_shared_lib.py`,
  `scripts/sync_synthetic_monitoring_variants.py`, `shared/lib/ha-validate.sh`,
  `alloy/rootfs/usr/share/alloy/generate-config.sh`, every s6 `run`/`finish`/`up` script under both
  `rootfs/` trees, `alloy/tests/generate-config.test.sh`, `alloy/tests/init-alloy.test.sh`,
  `alloy/tests/image-smoke.test.sh`, `grafana_pdc/tests/service.test.sh`,
  `grafana_pdc/tests/image-smoke.test.sh`, `tests/shared_validate_lib_test.sh`, and both
  `grafana_pdc/tests/fixtures/` executables.
- **`.github/workflows/release.yaml`** - release-please.
- **`.github/workflows/security.yml`** - actionlint, zizmor and docker-security, all reusable
  `uses:` calls into `rknightion/.github` pinned at `ff89dc29cee6fbe49128a19715ee3f60390be0dc`.
- **`.github/workflows/build-app.yaml`** - the container build and publish workflow
  (`home-assistant/builder`, Cosign, GHCR manifest).
- **The four `frenck/action-addon-linter` steps** in `builder.yaml` and the two
  `home-assistant/actions` helper steps. `uses:` never becomes `run: just`.
- **The `init` job's changed-App selection shell** and `MONITORED_FILES`.
- **`ci-success`**, its name and its `needs:` list; the `release` job's `needs:` and `if:`.
- **`release-please-config.json`, `.release-please-manifest.json`,** and every App's
  `config.yaml` `version:`. Release-please owns them; a hand edit propagates into generated files.
- **`renovate.json`, `trivy.yaml`, `.trivyignore.yaml`, `.yamllint`, `.devcontainer.json`,
  `repository.yaml`, `.github/CODEOWNERS`.**
- **Generated trees**: `grafana_sm/`, `grafana_sm_browser/` (except their variant-owned `Dockerfile`
  and `CHANGELOG.md`, which nothing here touches), `alloy/reporter/`, `grafana_pdc/reporter/`, both
  `rootfs/usr/share/ha-lib/ha-validate.sh` copies. Change the source and run `just gen`.
- **`backlog/tasks/*.md` and `backlog/docs/*.md`.** Only `backlog.config.yml` is hand-edited, and
  only its `definition_of_done` block.
- **Adding `go vet`, `golangci-lint` or `staticcheck`** - that is `hab-0004`.
- **Making the four App metadata linters report together** - that is `hab-0006`.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A top-level justfile defines all seven mandatory recipes (default, setup, fmt, fmt-check, lint, test, check) plus gen, gen-check, build, image and clean, and just --dump --dump-format json exits 0
- [ ] #2 just --list shows a # doc comment and a [group(...)] drawn from check/build/dev/gen for every public recipe, with only default and setup ungrouped
- [ ] #3 just --fmt --check exits 0, with [group(...)] written above [no-exit-message] on every recipe that stacks both
- [ ] #4 just check passes locally with a running Docker daemon and its --dry-run expansion is exactly the command set the five builder.yaml lanes run today: fmt-check, lint, gen-check, test-repo, test-pdc, test-alloy, test-alloy-init, test-sm
- [ ] #5 gofmt -l over alloy/ui, grafana_pdc/ui, shared/reporter, synthetic_monitoring_shared/launcher and synthetic_monitoring_shared/ui reports nothing (alloy/ui/config_test.go reformatted), and no generated module under alloy/reporter, grafana_pdc/reporter, grafana_sm or grafana_sm_browser is formatted or linted
- [ ] #6 There is no Makefile or GNUmakefile in the repository and none was introduced; no script file is deleted by this task
- [ ] #7 Each of the five builder.yaml test jobs carries uses: extractions/setup-just@<40-hex SHA> # v4 with just-version: '1.58.0' and runs only just lint / just test-* lines, while ci-success's name and needs list, the release job's needs and if, every permissions, concurrency and persist-credentials setting, and every other SHA-pinned uses: are unchanged
- [ ] #8 tests/repository_workflow_contract_test.py asserts the just recipe names in builder.yaml AND asserts the moved literals (the four docker build --tag local/ha-* lines, both synthetic_monitoring_image_smoke_test.py invocations, and both awk base-image expressions) against the justfile, and just test-repo passes
- [ ] #9 Every KEEP script is reachable through a recipe - shared/lib/ha-validate.sh, the s6 run/finish scripts, alloy/rootfs/usr/share/alloy/generate-config.sh and the pdc fixtures via just lint; the six shell test suites via just test-repo/test-pdc/test-alloy/test-alloy-init; both sync scripts via just gen and just gen-check - while scripts/cloud-environment-setup.sh deliberately has no recipe and stays documented in README.md
- [ ] #10 AGENTS.md's Commands block is replaced by the Task interface section with no per-recipe list pasted in, README.md tells the reader to run just gen instead of the sync script, and backlog.config.yml's definition_of_done names just check while yamllint --strict . still passes
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
