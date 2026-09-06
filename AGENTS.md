# ha-addons

Home Assistant App repository for BroTEK Solutions. Four Apps published as signed multi-arch images
from `alloy/`, `grafana_pdc/`, `grafana_sm/` and `grafana_sm_browser/`. `README.md` is the
user-facing description; this file is the contributor contract.

## Task interface

`just check` is the toolchain-only pre-commit gate and must pass before you commit. `just ci` is the
CI-equivalent gate and adds the Docker-backed test legs. Discover recipes with `just --list`;
`just --show <recipe>` shows what one actually runs.

- Prefer `just <recipe>` over an underlying tool. Run `just` with stdin from `/dev/null`.
- `just lint` owns the ShellCheck file list explicitly: the s6 service scripts have no extension, so
  they cannot be globbed. A new shell script must be added to that list by hand.
- A command that is not exposed gets a documented `[group(...)]` recipe rather than a direct
  invocation.
- `scripts/cloud-environment-setup.sh` is the cloud-provisioning command and is deliberately not a
  recipe: it uses sudo and installs tools globally.

## Rules that are easy to break by accident

**`grafana_sm/` and `grafana_sm_browser/` are generated.** Edit `synthetic_monitoring_shared/` and
run `just gen`. Only `Dockerfile` and `CHANGELOG.md` are variant-owned. CI rejects drift.

**Never hand-edit an App version.** `config.yaml` `version:` and `.release-please-manifest.json`
belong to release-please, and the SM generator reads the version out of the manifest, so a hand edit
propagates into generated files silently. Force a version with a `Release-As: X.Y.Z` commit footer.

**Conventional Commits select the release, and a squash-merged PR's *title* becomes the commit
subject on `main`** - so the title needs the prefix. Scope by App: `fix(alloy):`, `feat(alloy):`.

**`.yamllint` caps lines at 120 and ignores only `.git/`.** Every YAML file anywhere in the tree is
linted, tooling config included.

**Python dependencies come from uv, never pip.** A script importing outside the standard library
declares it in a PEP 723 `# /// script` header and runs under `uv run`; a pure-stdlib script stays on
bare `python3`. Adding a third-party import means adding the header and switching the justfile
invocation in the same change, or the script silently depends on whatever the machine happens to
have - and `pip install --user` cannot rescue it on macOS, where PEP 668 refuses it outright. `uvx`
covers Python CLI tools; `yamllint` is the only one and is deliberately not installed on the host.
`tests/repository_workflow_contract_test.py` enforces all of this, including that no `pip install`
returns to the justfile, the builder workflow or the cloud provisioning script.

**Reusable workflows are pinned to a release SHA with the version in a trailing comment.** zizmor
fails a floating `@main`; the pin and the comment move together.

**`actions/setup-go` needs both `cache: true` and its own `cache-dependency-path`.** The valuable
half is `GOCACHE`, not the module cache - compiling the stdlib cold costs 10-20s per module (measured
19.65s for `grafana_pdc/ui`'s `go test` cold against 0.65s warm), so "no external dependencies, so
nothing to cache" is true of the module cache and false of the build cache. `cache: true` alone is
inert here: `setup-go` globs for `go.mod` at the repository root, finds none because every module
lives in a subdirectory, and then **warns and caches nothing** rather than failing. Every step points
`cache-dependency-path` at its own `go.mod`.

**This repo is `BroTEK-Solutions` and takes no direct pushes to `main`.** Branch and PR, always. This
overrides the global push-straight-to-main rule, which covers only `rknightion` and `m7kni`. The one
exception is a tracker-only commit, below.

## Task tracking

Open work lives in `backlog/`, driven **only** through the `backlog` CLI. `backlog task list --plain`
is the queue; `backlog doc list --plain` lists the durable docs, which load on demand with
`backlog doc view <id> --plain`.

Read the **Agent fan-out protocol (canonical)** doc before designing a wave, and the **Wave operating
model** doc for this project's lane, ownership and defect conventions.

- **`backlog/` is committed, so no real identifiers in tasks or docs.** No email addresses, handles,
  usernames, account IDs, device or host names, addresses, coordinates, or Grafana Cloud stack and
  tenant IDs. Write the shape, not the instance: `<stack>/<tenant>`, "the browser variant's probe
  token". Aggregate counts, timings and structural findings are fine.
- **Never use the bare form of any flag that has an `--append-*` variant.** `--notes`, `--plan` and
  `--final-summary` *silently replace* the whole section, destroying another session's writes at exit
  0. This is an open upstream bug. Check `--help` for the append variant before using any
  field-setting flag; a new one is covered by this rule on sight.
- **A second `backlog task edit` on the same task with `--dep`, `--label`, `--assignee`, `--ref`,
  `--acceptance-criteria` or `--modified-file` REPLACES what the first set.** Repeating the flag
  *inside* one call is additive and correct; a second call is not.
- **Finalize in one call**, so an interrupted agent cannot leave finished work looking unfinished:
  `backlog task edit hab-0007 --check-ac 1 --check-ac 2 -s Done`.
- **Never hand-edit task, doc or decision markdown.** Section boundaries are HTML-comment markers;
  breaking one drops the section silently on read and makes the file unwritable by the CLI, with no
  repair command. `backlog.config.yml` is the one file edited by hand, because list-valued keys
  cannot be set through `backlog config set`.
- **A tracker-only commit goes straight to `main`. No branch, no PR, no review.** It applies when
  *every* path in the commit is under `backlog/` or is `backlog.config.yml`: nothing ships, no build
  can break, and a PR round-trip only delays the queue other sessions read to decide what to work on.
  Push it immediately - a tracker write left unpushed is invisible to every other session. One source
  file in the same commit forfeits the exception; split it instead.

## Deeper references

- `docs/tracker-conventions.md` - read before creating, labelling or filtering a Backlog task: the
  mandatory `unit:` label vocabulary, which prospective units ship from this repo and which do not,
  the label-replacement and filter traps, statuses, and why `backlog.config.yml` sits at the repo
  root.

<!-- BACKLOG.MD GUIDELINES START -->
<!-- backlog.md-instructions-version: 1.50.1 -->
<CRITICAL_INSTRUCTION>

## Backlog.md Workflow

This project uses Backlog.md for task and project management.

**For every user request in this project, run `backlog instructions overview` before answering or taking action.**

Use the overview to decide whether to search, read, create, or update Backlog tasks.

Before task lifecycle actions, read the matching detailed guide:
- `backlog instructions task-creation` before creating or splitting tasks
- `backlog instructions task-execution` before planning, changing status or assignee, adding a plan or implementation notes, or implementing task work
- `backlog instructions task-finalization` before checking acceptance criteria, writing final summaries, or moving tasks to terminal statuses

Use `backlog <command> --help` before running unfamiliar commands. Help shows options, fields, and examples.

Do not edit Backlog task, draft, document, decision, or milestone markdown files directly. Use the `backlog` CLI so metadata, relationships, and history stay consistent.

</CRITICAL_INSTRUCTION>
<!-- BACKLOG.MD GUIDELINES END -->
