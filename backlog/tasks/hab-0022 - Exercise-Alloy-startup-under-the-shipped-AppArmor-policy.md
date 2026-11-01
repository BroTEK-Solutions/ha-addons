---
id: HAB-0022
title: Exercise Alloy startup under the shipped AppArmor policy
status: In Progress
assignee: []
created_date: '2026-11-01T22:29:00Z'
updated_date: '2026-11-01T22:41:00Z'
labels:
  - unit:alloy
  - unit:repo
dependencies: []
priority: high
type: task
ordinal: 22000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A base-image update passed CI while the s6 compatibility startup path attempted a write forbidden by the shipped Alloy AppArmor profile. The merged regression test now boots init with read-only service definitions and checks Alloy readiness. Extend coverage to actual confinement where the Linux runner supports it, keeping portable coverage explicit.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 CI boots the complete Alloy image with the shipped AppArmor policy enforced and checks Alloy readiness and supervised services, or records a concrete runner limitation without claiming confinement proof.
- [ ] #2 A negative control demonstrates that the confinement test detects prohibited writes and that startup failures fail the gate.
- [ ] #3 The portable read-only startup regression remains available on Docker hosts without AppArmor.
- [ ] #4 Runner requirements, test commands and evidence boundaries are documented and the changes pass repository checks and review.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 just check
- [ ] #2 Fast subset while iterating (not the gate): just fmt-check && just lint && just gen-check && just test-repo
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Verify the merged baseline and investigate hosted-runner AppArmor support and established add-on test patterns. 2. Add a Linux-only real-profile startup leg with explicit prerequisites and negative controls, retaining portable startup checks. 3. Prove the failure and success paths, run just check and scoped Docker tests, obtain CodeRabbit review, and open a new PR with exact CI evidence.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented on branch test/alloy-apparmor-startup in PR #165: https://github.com/BroTEK-Solutions/ha-addons/pull/165 . Candidate 6bc08cffe94e367a5a1d4effce2042490a0aacd2 passed just check, the portable arm64 image test, and a negative legacy-layout boot fixture that failed at the forbidden user/type write. Removing or disabling the AppArmor workflow step fails the contract test. CodeRabbit CLI reviewed all six files; one minor conditional-step issue was fixed and verified. GitHub run 41358092562 job 125002392237 successfully booted the actual profile on Ubuntu 24.04 with AppArmor parser 4.0.1 and Docker 28.0.4, verified enforce mode, readiness, service state and paired write controls. The PR remains open pending final CI/review and owner merge; do not mark Done before a merge commit exists. Research of current core, community Node-RED and AlexBelgium PR workflows found lint/build coverage but no confined runtime boot. Ubuntu proof does not establish every HAOS feature or host configuration.
<!-- SECTION:NOTES:END -->
