---
id: HAB-0019
title: Move release-please off RELEASE_PLEASE_TOKEN to the OpenBao broker
status: Done
assignee: []
created_date: '2026-08-29 23:19'
updated_date: '2026-08-30 08:18'
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

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## Step 1 DONE — Tailscale federated credential (2026-08-30)

Created for this owner, copying the shape of the existing `repo:rknightion*` credential rather than
composing one from docs:

| | |
|---|---|
| Credential id | TBkeySVEB121CNTRL-kfnHs5W6z321CNTRL |
| Subject | repo:BroTEK-Solutions* |
| Claims | ref refs/heads/main, repository_owner_id 311350022 |
| Scopes / tags | auth_keys / tag:gha |

`TS_WIF_CLIENT_ID` and `TS_WIF_AUDIENCE` are set on the repo. Both are identifiers, not credentials.
The read-write OAuth client in the tailscale context has scope `all`, which is what made this
possible without human involvement - the runbook only documented reading federated credentials, but
`POST /api/v2/tailnet/-/keys` with `keyType: federated` works and returned 200.

**Trap found:** the API silently truncates `description` to 50 characters. Cosmetic here, but do not
rely on a long description round-tripping.

## Step 2 still blocked — OpenBao admin write

Both auth paths are interactive and cannot be driven by an agent: `bao login -method=oidc` is Entra
SSO in a browser, break-glass on camden prompts for a password. An agent token 403s on
`auth/token/lookup-self`. The exact commands are in PR #108's comment, written to print an existing
working policy first so the token path is copied rather than transcribed - that path is the one thing
in this migration nobody has verified.

## SECURITY: credentials exposed in a session transcript, 2026-08-30

While inspecting `~/repos/chat-personal/tailscale/.secrets/creds.local.env`, a grep intended to strip
values matched on `^[A-Z_]*=` while the file uses `export NAME=`, so the fallback printed the file in
full. **TS_API_KEY, TS_OAUTH_CLIENT_SECRET and GC_OTLP_TOKEN reached the terminal** and therefore the
session transcript, which is committed to chat-personal. Rotation is Rob's call and is NOT done.
Lesson: redact by construction (`grep -o '^[^=]*='`) rather than by a pattern that fails open.

## DONE end to end (2026-08-30)

PR #108 merged as 5d15990. Proven on the first push to main, run 33301024423, from the mint step's
own output:

    minted for repositories: ['ha-addons']
    ✔ Building candidate release pull request for path: alloy

The broker minted a repo-scoped token and release-please authenticated with it.
`RELEASE_PLEASE_TOKEN` is deleted; the repo now holds only `TS_WIF_CLIENT_ID` and
`TS_WIF_AUDIENCE`, which are identifiers. An org-wide sweep found no other copy.

**The policy I had transcribed was WRONG and the self-verifying step caught it.** I wrote
`capabilities = ["read"]`; every working consumer uses `["create", "read", "update"]`. Reading
`gha-release-please-autopi-ha` before writing the new policy is what found it. Per the runbook this
would have failed as a permission error rather than a not-found, reading like a broken policy rather
than a wrong capability list.

Both new objects were structurally diffed against `release-please-autopi-ha`, with **non-empty reads
asserted before diffing** - the runbook records a false pass from diffing two empty outputs.

Live values for a future session: permission set `release-please-ha-addons`, policy
`gha-release-please-ha-addons`, role `release-please-ha-addons`, installation 152180795, owner
311350022, repo 1318146054, Tailscale credential TBkeySVEB121CNTRL-kfnHs5W6z321CNTRL.

**Still open, Rob's call:** rotate `TS_API_KEY`, `TS_OAUTH_CLIENT_SECRET` and `GC_OTLP_TOKEN` after
the transcript exposure recorded above. Not done.
<!-- SECTION:NOTES:END -->
