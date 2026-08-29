---
id: HAB-0018
title: Batch init-alloy's per-option jq lookups into one pass
status: In Progress
assignee: []
created_date: '2026-08-29 20:22'
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
