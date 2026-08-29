---
id: HAB-0018
title: Batch init-alloy's per-option jq lookups into one pass
status: Done
assignee: []
created_date: '2026-08-29 20:22'
updated_date: '2026-08-29 21:03'
labels:
  - 'unit:alloy'
  - 'unit:repo'
dependencies: []
priority: high
ordinal: 18000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`alloy/rootfs/etc/s6-overlay/s6-rc.d/init-alloy/run` read each persisted setting with two jq processes: `settings_has_value` then `settings_get`. `config_or` does one lookup per option for 31 options, so a single run spawned **46 jq processes and spent ~1.4s** in process startup. That cost is paid on **every real App start**, not only in CI - it was found while profiling the alloy-init CI lane but it is a shipped-behaviour improvement, not a test-only one.

Replaced with one jq pass into a bash associative array. The dump is NUL-separated because `manual_config` and `additional_config` are free text containing newlines, and jq emits an entry-count header that the read loop counts against, so a failed or desynchronised read aborts loudly instead of silently yielding an empty cache - which would turn every option into its default.

**The regression this must not reproduce:** `config_or`'s own comment records that hand-rolling the has-value check with jq is how `loki_url`'s default once made a supposedly optional option mandatory. Absent, explicit null and empty string must all behave as unset; `false` and `0` must behave as real values.

Measured: `just test-alloy-init` 86.9s -> 18s (concurrency, HAB-0011) -> **5.7s** (this change).
<!-- SECTION:DESCRIPTION:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## Merged as 98a677e (PR #106)

Adversarial review found four defects before merge; all fixed and regression-tested.

1. **Safe mode broke.** The batch ran before the safe-mode branch, so a corrupt settings file made
   the App unstartable even in safe mode - the one recovery lever when the settings store is what
   broke. Moved after it.
2. **An odd number of NUL bytes silently discarded every setting.** A JSON string may legally contain
   NUL; each adds a field, shifting all later fields by one while the entry count still agrees.
   Reproduced: `expected=3 read=3`, guard passes, `loki_url=LOST instance_name=LOST`, exit 0 - the
   App starts, passes its healthcheck and ships nothing. Reachable because the ingress save path
   copies submitted keys verbatim (`mergeOptions` has no allowlist). NUL is now stripped, matching
   the old behaviour where $( ) dropped it.
3. **An empty key aborted the script with a raw bash error.** `SETTINGS_CACHE[""]` is
   `bad array subscript`, fatal under set -e before any diagnostic prints. Now dropped.
4. **jq's exit status was unchecked.** jq can emit a complete first object then fail on trailing
   garbage; process substitution discards the status. Now prechecked with `jq -e 'type == "object"'`.

**Credentials cache a presence marker, never their value** (CodeRabbit, major). Nothing reads one by
value. Keep the list in step with `secretOptionNames` in `alloy/ui/config.go`.

**Two of my own tests were passing for the wrong reason.** `generate-config.sh` independently
re-applies `\${VAR:-default}` for all 31 options, so an empty and an absent value render identically
and a config-level assertion cannot fail. They now assert on the startup banner. Three mutations that
survived the first suite are killed: has_value counting present-but-empty as set, deleting the desync
guard, and the credential marker always being empty.

**Deliberately NOT done:** moving multi-document JSON rejection into the precheck (CodeRabbit, minor).
The entry-count guard already rejects it (`expected=1 read=2`, verified), and tightening the precheck
would make the guard unreachable from tests. An untested guard is how this class of bug survives.

135 checks, up from 117. `just test-alloy-init` 18s -> 5s, and the same saving lands on every real
App start.
<!-- SECTION:NOTES:END -->
