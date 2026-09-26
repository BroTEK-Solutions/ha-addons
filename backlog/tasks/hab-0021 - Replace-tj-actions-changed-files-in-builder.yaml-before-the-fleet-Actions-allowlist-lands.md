---
id: HAB-0021
title: >-
  Replace tj-actions/changed-files in builder.yaml before the fleet Actions
  allowlist lands
status: To Do
assignee: []
created_date: '2026-09-26 15:37'
updated_date: '2026-09-26 15:37'
labels:
  - 'unit:repo'
dependencies: []
priority: high
type: chore
ordinal: 21000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The fleet repo-settings standard (rknightion/.github fleet/) switches every repo's Actions policy to an explicit allowlist, and tj-actions/changed-files is deliberately NOT on it: it is the action compromised in March 2025 (tag re-pointing that dumped CI secrets). The pinned v47 SHA here is post-incident, but the decision is to drop the dependency. Once the allowlist is applied to this repo, builder.yaml fails at the 'Read changed files' step until this lands.

Only one output is consumed: steps.changed_files.outputs.all_changed_files feeds the 'Select changed Apps' step.

Replacement, in order of preference:
1. Native git, no third-party action: checkout with enough history (fetch-depth: 0, or fetch the PR base) and compute the list with git diff --name-only <base>...<head> (pull_request: github.event.pull_request.base.sha; push: github.event.before, falling back to all files when before is the zero SHA on a new branch). Emit it in the same space-separated shape all_changed_files had.
2. step-security/changed-files: StepSecurity's hardened drop-in fork with identical outputs. step-security is already on the allowlist via harden-runner.
3. dorny/paths-filter with list-files: works, but it adds a new third-party dependency to the allowlist.

This is a BroTEK-Solutions repo, so it lands via a branch and PR, not a push to main.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 builder.yaml has no tj-actions reference and zizmor/actionlint pass
- [ ] #2 A PR that touches exactly one App builds only that App; a push to main behaves as before
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
