---
id: doc-0004
title: Autonomous maintenance execution state
type: other
created_date: '2026-11-06T00:09:00Z'
updated_date: '2026-11-06T01:16:00Z'
---
Revision 7, 2026-11-06. COMPLETE: all authorized maintenance, concrete defects and release publication finished. HAB-0024 (orderly MQTT shutdown availability) remains explicitly excluded and is the only open backlog item; no tasks In Progress, no drafts or open PRs. Six unbuilt product proposals were archived with reasons and reopening conditions, not marked delivered.

Source landings: PR #170 CI maintenance; #171 Alloy request deadlines; #173 MQTT reconnect/unknown-state and integer validation; #174 publication queue/strict required gates; #176 settings validity/concurrency/recovery at54bedb49e57c0120da5dc753096f954edb79f56d; #172 combined PDC confinement/diagnostics, SM unknown metrics and child credential hygiene at502c2305dba78e4a2539530c4cc2b2e720c1afe0. Draft #175 superseded by #172. Task records carry acceptance, review and exact merge/CI evidence. Final source CI42140343973 passed all eight images; local check, fresh PDC image/startup and enforcing confinement passed.

Released through generated PR #168 at e3060b617670b05d205d3b25e706abad5a316b36: Alloy2.5.0; PDC, SM and SM Browser1.3.0. Exact release candidate87e88cd passed local check, nine-file/version/generation/publication preflight and CI42141344547. Publication CI42141834190 succeeded, including eight architecture images, four signed generic manifests and four stable GitHub releases. Independent Cosign v3.0.6 verification passed all twelve digest references with exact expected workflow identity, issuer, repository, main ref, push trigger and release SHA. A deliberately wrong SHA was explicitly rejected. Version/latest digests match, generic images contain linux/amd64 and linux/arm64, architecture/config labels and child digests agree, and all four release tags resolve to the exact release commit. Verification evidence: /tmp/ha-release-e3060b6-evidence.14vtbc12/verification.json.

Real HAOS validation used the unchanged reviewed policy digest0ca50bbc001ac20797852a991cc285e91465d603022baebeeb8d104e83ece28a: protected custom profile loaded, PDC connected, reporter cleanup worked, authenticated ingress status returned200 with agent responding, continuous kernel capture through stop had zero denials. Source evidence /tmp/haos-pdc-validation-20261106/result.md. The temporary validation App remains installed but stopped/manual, with watchdog/auto-update and MQTT disabled. Its source archives and verified partial data backup remain on HAOS; original App options/version unchanged and still running; temporary login revoked. No production App update was performed. No pending live mutation or agent work remains. Root owns final tracker closeout. Recovery mechanism unknown; re-query live state before any future action.

Published generic image digests:
- app-alloy 2.5.0: sha256:3744ead2702d28fab7d6010eac36ab2ec8d1f9dc2e4dc56262affcfc720aa0e8
- app-grafana-pdc 1.3.0: sha256:ce445c7e9fa23ca37f32352f08fe47c30115b98656aafc638853a4f9f7d092e0
- app-grafana-sm 1.3.0: sha256:c5c9f00a79d1d87b9652d94a494da57c75d6ea6abfdf10b078db83a1834140b5
- app-grafana-sm-browser 1.3.0: sha256:88a52491151d25dcf6aa1265c0c7b3dbf342734551360875ed322f92bd1f3c04

Verification evidence SHA256: 0cfdad9df0437c5222f032ad5195c57123a184c92c1965873093ef734f3feee4.
