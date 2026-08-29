---
id: HAB-0019
title: Move release-please off RELEASE_PLEASE_TOKEN to the OpenBao broker
status: In Progress
assignee: []
created_date: '2026-08-29 23:19'
labels:
  - 'unit:repo'
dependencies: []
priority: high
ordinal: 19000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
This repo still carries a `RELEASE_PLEASE_TOKEN` repo secret (created 2026-08-01). That PAT was revoked estate-wide and is unrecoverable; the 2026-08-08 sweep covered rknightion and m7kni only, so this third owner was missed. **Sweeping two owners proves nothing about a third.**

PR #108 (DRAFT) has the workflow change. It must NOT merge before the two admin steps below, or the next push to main fails its release job.

**Prerequisite already met, contrary to the runbook:** the App is installed on this owner.

| | |
|---|---|
| Owner id | 311350022 |
| Installation id | 152180795 (`rknightion-token-broker`) |
| ha-addons repo id | 1318146054 |

**Blocked on two admin actions, both needing credentials an agent does not hold:**

1. **Tailscale federated credential for this owner.** The repo runs on `ubuntu-latest`, so it takes the GitHub-hosted path and joins the tailnet as an ephemeral `tag:gha` node. Listing `keyType: \"federated\"` shows subjects `repo:rknightion*` and `repo:brewmdm*` only - this owner has none. Needs one pinned to claim 311350022, then `TS_WIF_CLIENT_ID` and `TS_WIF_AUDIENCE` set as repo secrets. They are identifiers, not credentials.
2. **OpenBao permission set, policy and JWT role.** Exact commands are in the PR body. Admin token required; an agent token returns 403.

**Then:** un-draft, merge, watch the first push to main, and only once green run `gh secret delete RELEASE_PLEASE_TOKEN`. The release job only runs on push to main, so a PR cannot exercise it - the first post-merge run IS the test.

**Accepted trade:** releases become dependent on camden being up, unsealed and on the tailnet. A mint failure is infrastructure, not a bad commit. Stated in a comment in the workflow.

The runbook at `camden/openbao/runbooks/CI-SECRETS.md` said two owners; corrected in place to three.
<!-- SECTION:DESCRIPTION:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->
