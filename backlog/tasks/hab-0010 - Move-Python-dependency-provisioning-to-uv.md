---
id: HAB-0010
title: Move Python dependency provisioning to uv
status: Done
assignee: []
created_date: 2026-08-29 16:20
updated_date: '2026-11-06T00:09:00Z'
labels:
  - unit:repo
dependencies: []
priority: medium
type: chore
ordinal: 10000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Python dependencies are installed three different ways today, all of them pip, and none of them reproducible from the script that needs them:

- `scripts/cloud-environment-setup.sh` runs `pip install --user pyyaml voluptuous yamllint`
- `.github/workflows/builder.yaml` runs `pip install --quiet ...` in three separate jobs
- a developer machine gets nothing, so `just check` fails on a missing import

On macOS the developer path is not merely undocumented, it is blocked: PEP 668 refuses `pip install --user` against the system Python, so the only way to run `just check` locally is to hand-roll a venv. That is what happened while fixing HAB-0009.

uv is the standard across Rob's projects and is already present in the Codex and Claude cloud images. Six of the fourteen Python scripts need a third-party import; the other eight are pure stdlib and need no environment at all.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 The six scripts that import voluptuous or pyyaml declare those dependencies inline (PEP 723), so each carries its own environment
- [x] #2 just check passes on a machine with no pyyaml or voluptuous installed and no venv prepared
- [x] #3 No pip install remains in the justfile, the CI workflow, or the cloud provisioning script
- [x] #4 CI installs uv rather than pip-installing test dependencies, with the action SHA-pinned and the version in a trailing comment
- [x] #5 The cloud provisioning script asserts uv and fails loudly if it is absent, rather than silently proceeding
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 just check
- [x] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Already delivered by PR #99, merge 9f42ad0. Its clean-machine proof and deliberately failing contract checks cover isolated uv provisioning; current source retains six PEP 723 scripts, locked uv recipes, pinned setup-uv and explicit cloud uv requirement. Board reconciled after repository assessment; no new implementation.
<!-- SECTION:FINAL_SUMMARY:END -->
