---
id: HAB-0020
title: Harden Actions token and ci-success check
status: To Do
assignee: []
created_date: '2026-09-26 15:35'
labels: []
dependencies: []
priority: medium
ordinal: 20000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Found 2026-09-26 by the renovate-repair pilot security review (verified live): default_workflow_permissions=write and can_approve_pull_request_reviews=true on this repo, and the ruleset's required ci-success check has no integration_id pin (rknightion repos pin 15368), so any job or status named ci-success satisfies it. Combined with 0 required approvals, a pushed workflow file could approve and satisfy its own PR. Fix: set default workflow token to read (grant per-job permissions explicitly), disable Actions PR approval, pin ci-success to the GitHub Actions integration (15368). Check every workflow still has the permissions it needs before flipping the default.
<!-- SECTION:DESCRIPTION:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
