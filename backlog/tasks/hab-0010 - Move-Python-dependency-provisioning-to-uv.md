---
id: HAB-0010
title: Move Python dependency provisioning to uv
status: In Progress
assignee: []
created_date: '2026-08-29 16:20'
labels:
  - 'unit:repo'
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
- [ ] #1 The six scripts that import voluptuous or pyyaml declare those dependencies inline (PEP 723), so each carries its own environment
- [ ] #2 just check passes on a machine with no pyyaml or voluptuous installed and no venv prepared
- [ ] #3 No pip install remains in the justfile, the CI workflow, or the cloud provisioning script
- [ ] #4 CI installs uv rather than pip-installing test dependencies, with the action SHA-pinned and the version in a trailing comment
- [ ] #5 The cloud provisioning script asserts uv and fails loudly if it is absent, rather than silently proceeding
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
