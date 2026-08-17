---
id: doc-0003
title: Closed GitHub issues (pre-Backlog history index)
type: other
created_date: '2026-08-17 12:03'
updated_date: '2026-08-17 12:45'
---
# Closed GitHub issues (pre-Backlog history index)

Every issue this repository closed before Backlog.md became its tracker, as one row each. The
original `#NNN` numbers stay the only ID space over this history — they are already cited in commit
messages, so importing them as `Done` tasks would have created a second, permanently non-matching
set of numbers over the same events.

**The issues still exist.** This is a pointer, not the record: read a body with
`gh issue view <N> --repo BroTEK-Solutions/ha-addons`. If the issues are ever deleted, this document
stops being a pointer and must be replaced by a redacted archive before that happens.

The GitHub tracker also stays enabled for external contributors, and Renovate's Dependency Dashboard
(`#8`) is a live open issue that is recreated on every run — neither is Backlog's business.

Two commits are given per row because they answer different questions. The **work commit** is what
changed; the **merge commit** is what shipped to `main`, which is the one the Wave operating model
asks a `Done` task to cite. Both are reachable from `main`.

| # | Closed | Title | Work commit | Shipped as |
|---|---|---|---|---|
| 29 | 2026-08-02 | Expose collector attributes (journal path, container name) in Fleet Management mode | `ac2ec07` feat(alloy): publish HAOS Fleet attributes | `0f42abd` (#32) |
| 28 | 2026-08-02 | `logs_exclude_addons` never matches: rule uses `addon_` prefix, Supervisor uses `app_` | `84c2096` fix(alloy): match HA Supervisor journal containers | `05ede94` (#31) |
| 17 | 2026-08-01 | Alloy: add a Fleet-managed Hybrid configuration mode | `9605eba` feat(alloy): generate Fleet starter pipelines | `17cecda` (#23) |

Three closed issues, one open (the Renovate dashboard), verified with
`gh issue list --state all --limit 1000` on 2026-08-17. All three are Alloy App work and all three
landed; nothing here is unfinished work that should have become a task.
