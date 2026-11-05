---
id: HAB-0023
title: Make MQTT health reporting configurable and correct device presentation
status: Done
assignee:
  - agent
created_date: '2026-11-05T23:34:00Z'
updated_date: '2026-11-05T23:51:00Z'
labels:
  - unit:alloy
  - unit:grafana-pdc
  - unit:sm-shared
  - unit:repo
dependencies: []
ordinal: 23000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
App health devices appear automatically whenever a Supervisor MQTT broker exists. The owner approved an explicit off switch, readable device models, and correcting Alloy health/configuration sensors whose problem class currently interprets success as a fault. Keep existing installations enabled by default and preserve entity identities.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 All four Apps expose mqtt_enabled, defaulting to true for existing and new installations; false stops health reporting and removes retained discovery when the broker is reachable.
- [x] #2 Alloy problem sensors report OFF for success and ON for observed failures; unavailable metrics remain unknown.
- [x] #3 Device models use readable product names while existing device identifiers and entity unique IDs remain stable.
- [x] #4 Configuration, migration effects, broker-unavailable cleanup behavior and monitoring limitations are documented; generated copies stay synchronized.
- [x] #5 Focused regression tests, the repository pre-commit gate, and dsreview pass; changes are submitted through a PR.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 just check
- [x] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Read mqtt_enabled from the mounted App options in the shared reporter, preserving true when the key is absent; disabled mode removes retained discovery/state using the existing identity, retries if the broker is unavailable, then idles without health polling. Restart applies the option. 2. Add default-true options, translations and user documentation in the three source App trees; keep Alloy option in its native Configuration tab. 3. Correct problem-class polarity and add stable product model names without changing IDs/topics. 4. Prove regressions red then green, exercise MQTT publication/cleanup against a local protocol fixture, regenerate copies, run just check, and review the final candidate with dsreview. 5. Submit and land a focused conventional-commit PR after required CI; record the merge identity.

Final step-1 correction: each launcher reads the native option and passes only REPORTER_MQTT_ENABLED. The reporter never reads the token-bearing options file. Missing s6 options files also preserve enabled behavior; existing malformed/unreadable files fail closed.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Runtime seam corrected after inspecting SM privilege drop: launchers pass only REPORTER_MQTT_ENABLED rather than allowing the reporter to read the token-bearing options file. Missing keys/flags preserve enabled behavior. Regression tests first failed for Alloy success/failure polarity, slug-as-model, and disabled mode still advertising a last will. All now pass using HTTP probe fixtures and actual MQTT client packets against a local protocol peer. Native configuration, translations and docs are implemented; launcher and s6 false/default wiring verified.

Final local gate: just check passed after generated-copy refresh; all 188 Alloy configuration checks passed. MQTT source tests passed three iterations under the race detector with fixture-only probe traffic (go test -race -timeout 90s -count=3). The enabled-flow test waits for initial state before cancellation and allows the existing five-second shutdown grace. No live HA instance modified. Final review pending.

dsreview identified a valid startup regression in the s6 scripts when the options file is absent (the Alloy image smoke test uses empty /data). Correcting missing-file behavior to enabled; unreadable or malformed existing files intentionally remain fail-closed. Independent review otherwise confirmed option plumbing, polarity, model naming and SM token separation. Test-timing investigation also identified pre-existing orderly-shutdown availability race, recorded separately in HAB-0024 (make MQTT offline publication reliable during orderly reporter shutdown); it is outside this change.

Both review findings resolved: absent options files preserve enabled behavior, while existing unreadable/malformed files and dangling symlinks remain fail-closed. Both original scripts failed the missing-file fixture; corrected scripts passed seven cases each (missing file, true, false, absent key, malformed, unreadable, dangling symlink), plus ShellCheck. No unresolved major/minor findings; trivial guard correction does not require repeating the full dsreview per review policy. Reviewer evidence: /tmp/ha-mqtt-review.json. Runtime tests use local MQTT/HTTP fixtures, not a live HA broker.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Merged PR #169 as 22adf8387d3b45718f97210c5a92b8f45511b470. All four Apps now expose default-enabled MQTT health reporting; disabling and restarting withdraws retained discovery/state and idles. Alloy problem polarity is corrected; readable product models preserve registry identity. Generated copies and operational docs are synchronized. just check passed, reporter tests passed three race-detector iterations, and CI run 42131870088 passed all App test legs, AppArmor startup, security checks and eight architecture builds against 51f012613798679363d3ed2d1832e2c9bba5627e. Merge tree is byte-identical to that tested candidate. dsreview initially found the missing-options-file startup regression in both s6 scripts; both findings were corrected and seven fixture cases per script plus the final gate passed, with no unresolved findings. No live Home Assistant validation or release performed. Existing raw-state automations need the documented Alloy polarity adjustment. Pre-existing orderly-shutdown availability race is tracked separately in HAB-0024 (make MQTT offline publication reliable during orderly reporter shutdown).
<!-- SECTION:FINAL_SUMMARY:END -->
