---
id: HAB-0009
title: >-
  Fix the silent Save no-op when a nested wizard card re-enables hidden required
  fields
status: Done
assignee: []
created_date: '2026-08-29 15:22'
updated_date: '2026-08-29 15:22'
labels:
  - 'unit:alloy'
dependencies: []
references:
  - 'https://github.com/BroTEK-Solutions/ha-addons/issues/85'
modified_files:
  - alloy/ui/static/app.js
  - alloy/ui/static/index.html
  - alloy/tests/ui-static.test.mjs
priority: high
type: bug
ordinal: 9000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Reported externally as GitHub issue #85 against Alloy App v2.4.0. Save and Save & restart silently no-opped in Local configuration mode and under the full manual override: no request was sent, no message was shown, and the generated config never picked up the entered Prometheus and Loki endpoints. The only signal was two browser console warnings naming fleet_url and fleet_username as "not focusable".

Root cause: `refreshWizardVisibility` in `alloy/ui/static/app.js` evaluated every section in isolation and iterated `[data-mode]` before `[data-wizard-step]`. The Fleet card at `alloy/ui/static/index.html:51` is nested inside the `data-mode="fleet"` container at `:50` but carries only `data-wizard-step="config"` of its own, so the container disabled fleet_url and fleet_username and the nested card re-enabled them on the same pass. Two `required` controls then sat enabled inside a hidden ancestor, which native constraint validation cannot focus, so `reportValidity()` rejected the form and `save()` returned early.

The reporter proposed stripping `required` or setting `disabled` on the Fleet fields. That treats the symptom - the fields were already meant to be disabled and the container already disabled them - so the fix corrects the bookkeeping instead.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A section nested inside an inactive section is inactive, so nothing inside a hidden mode container can re-enable itself
- [x] #2 Choosing Fleet still restores the Fleet card and its required fields
- [x] #3 A Local save omits the inactive mode's fields rather than submitting them
- [x] #4 A regression test fails against the pre-fix code and passes against the fix
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Shipped on `main` as 72e1432 (pushed direct, no PR, on Rob's explicit instruction for this one).

`refreshWizardVisibility` now computes each section's own mode/step activeness into a Map, then walks `parentElement` so a section nested inside an inactive one is inactive whatever its own attributes say. Sections are still iterated in document order via a single `[data-mode],[data-wizard-step]` query, so a narrower inactive wrapper still disables the field its active ancestor has just enabled. The `?v=` cache-bust string on both static assets moved so operators are not served the old script.

Verification, not assertion: the regression assertions were run against the pre-fix `app.js` with the original two-selector plumbing restored in a throwaway copy of the harness, and failed with `fleet_url disabled: false, expected: true`; they pass against the fix. `just check` exits 0. Note `voluptuous` is not installable with `pip --user` on macOS (PEP 668) - the two config-schema tests need a venv on `PATH`.

CodeRabbit returned one `major` finding on the new test, claiming an awaited promise resolves through a timer mock that never fires. False positive: `globalThis.setTimeout` is restored at ui-static.test.mjs:354, well before the new block. Checking it did surface a smaller real point - the assertions popped the last `api/config` POST out of a list that already held earlier saves - so the test now pins that this submit issued its own POST. That count assertion holds even with the awaits removed, because the mocked `fetch` records the request synchronously before `save()` suspends.

Server side needed no change: `validateModeRequirements` in `alloy/ui/config.go` already gates the Fleet requirements on `operation_mode == "fleet"`, and `mergeOptions` preserves options the client omits, which is what a Local save now relies on.
<!-- SECTION:FINAL_SUMMARY:END -->
