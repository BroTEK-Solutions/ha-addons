# ha-addons

Home Assistant App repository for BroTEK Solutions. Four Apps published as signed multi-arch images:
`alloy/`, `grafana_pdc/`, `grafana_sm/`, `grafana_sm_browser/`. `README.md` is the user-facing
description; this file is the contributor and agent contract.

Claude Code and Codex both read this file. `CLAUDE.md` is a stub that imports it, so there is one
canonical copy and the two cannot drift.

## Task interface

This repository's task surface is a `justfile`. Discover it instead of guessing:

    just --list                        # human-readable
    just --dump --dump-format json     # machine-readable
    just --show <recipe>               # what a recipe actually runs

- `just check` is the toolchain-only pre-commit gate. It must pass before you commit.
- `just ci` is the full local CI-equivalent gate and adds the Docker-backed test legs.
- Prefer `just <recipe>` over an underlying tool. If you are typing `python3 tests/...`, use the
  matching `just` recipe.
- `just lint` owns the ShellCheck file list; the s6 service scripts have no extension so it cannot
  be globbed.
- Run `just` with stdin from `/dev/null`. Recipes marked `[confirm]` are destructive: stop and ask
  before running one; never pass `--yes` or `JUST_YES=1`. This repository currently has none.
- If a task needs a command that is not exposed, add a documented `[group(...)]` recipe rather than
  invoking the underlying command directly.
- `scripts/cloud-environment-setup.sh` stays the Codex and Claude Code cloud-provisioning command.
  It is deliberately not a recipe because it uses sudo and installs tools globally.

## Rules that are easy to break by accident

**`grafana_sm/` and `grafana_sm_browser/` are generated.** Edit `synthetic_monitoring_shared/` and
run `just gen`. Only `Dockerfile` and `CHANGELOG.md` are variant-owned. CI rejects drift.

**Never hand-edit an App version.** `config.yaml` `version:` and `.release-please-manifest.json`
belong to release-please, and the SM generator reads the version out of the manifest, so a hand edit
propagates into generated files silently. Force a version with a `Release-As: X.Y.Z` commit footer.

**Conventional Commits select the release.** A squash-merged PR's *title* becomes the commit subject
on `main`, so the title needs the prefix. Scope by App: `fix(alloy):`, `feat(alloy):`.

**`.yamllint` caps lines at 120 and ignores only `.git/`** — every YAML file anywhere in the tree is
linted, tooling config included.

**Reusable workflows are pinned to a release SHA with the version in a trailing comment.** zizmor
fails a floating `@main`; the pin and the comment move together.

**This repo is `BroTEK-Solutions` and takes no direct pushes to `main`.** Branch and PR, always.

## Task tracking — Backlog.md

Open work lives in `backlog/`, driven **only** through the `backlog` CLI. `backlog task list --plain`
is the queue; `backlog doc list --plain` lists the durable docs. GitHub Issues stays enabled for
external contributors and for Renovate's Dependency Dashboard, but it is not where this project's
own work is tracked — the three issues closed before the migration are indexed in
**Closed GitHub issues (pre-Backlog history index)**.

Read the **Agent fan-out protocol (canonical)** doc before designing a wave, and the
**Wave operating model** doc for this project's own lane, ownership and defect conventions. Docs load
on demand with `backlog doc view <id> --plain`, so a long one costs nothing until it is read.

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
  cannot be set through `backlog config set` — keep its `definition_of_done` entries as folded
  scalars or `yamllint --strict .` fails on line length.
- **The tracker config lives at the repo root as `backlog.config.yml`, NOT at `backlog/config.yml`.**
  This is not cosmetic and must not be tidied back. `home-assistant/actions/helpers/find-addons`
  discovers Apps with `find ./ -maxdepth 2 -name config.json -o -name config.yaml -o -name
  config.yml`, so a `config.yml` inside `backlog/` makes the whole build treat `backlog` as a fifth
  App and fail on missing `name`, `slug`, `version`, `arch` and `description`. The action has no
  exclude mechanism. `backlog/docs/` must stay where it is, so the config is what moves.
- **Statuses are `To Do`, `In Progress`, `Parked`, `Done`.** `Parked` means attempted, blocked, and
  left with a concrete resume boundary — it is not `To Do`.

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
