# ha-addons

Home Assistant App repository for BroTEK Solutions. Four Apps published as signed multi-arch images:
`alloy/`, `grafana_pdc/`, `grafana_sm/`, `grafana_sm_browser/`. `README.md` is the user-facing
description; this file is the contributor and agent contract.

Claude Code and Codex both read this file. `CLAUDE.md` is a stub that imports it, so there is one
canonical copy and the two cannot drift.

## Commands

Everything below runs from the repository root and mirrors what `.github/workflows/builder.yaml`
runs, so a local pass means CI passes.

```bash
# Repository gate — run for any change
yamllint --strict .
python3 tests/app_metadata_contract_test.py
python3 tests/app_version_changed_test.py
python3 tests/renovate_config_contract_test.py
python3 tests/repository_workflow_contract_test.py
python3 tests/synthetic_monitoring_variants_test.py

# Alloy
python3 alloy/tests/config-schema.test.py
python3 alloy/tests/image-contract.test.py
bash alloy/tests/generate-config.test.sh
node alloy/tests/ui-static.test.mjs
(cd alloy/ui && go test ./...)

# Grafana PDC
python3 grafana_pdc/tests/config-schema.test.py
python3 grafana_pdc/tests/image-contract.test.py

# Synthetic Monitoring — regenerate both variants, then test the SHARED launcher.
# `go test ./...` inside grafana_sm/launcher or grafana_sm_browser/launcher passes
# vacuously: main_test.go is deliberately not copied into the variants.
python3 scripts/sync_synthetic_monitoring_variants.py
(cd synthetic_monitoring_shared/launcher && go test ./...)
```

ShellCheck covers a fixed file list — copy it from the `ShellCheck` step in
`.github/workflows/builder.yaml` rather than globbing, because the s6 service scripts have no
extension.

Image smoke tests need a working Docker daemon and are the slowest gate; run them when the change
touches a Dockerfile, rootfs or entrypoint.

## Rules that are easy to break by accident

**`grafana_sm/` and `grafana_sm_browser/` are generated.** Edit `synthetic_monitoring_shared/` and
run the sync script. Only `Dockerfile` and `CHANGELOG.md` are variant-owned. CI rejects drift.

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
  repair command. `backlog/config.yml` is the one file edited by hand, because list-valued keys
  cannot be set through `backlog config set` — keep its `definition_of_done` entries as folded
  scalars or `yamllint --strict .` fails on line length.
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
