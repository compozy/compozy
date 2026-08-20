---
id: ET-web-dock-default-window-size
area: ET
title: Dock apps open at enlarged default window sizes
persona: Bruno
journey: J-operate-desktop-shell
expected: Opening Agents, Loops, Jobs, Triggers, Connections, Knowledge, Vault, Permissions, Marketplace, Dashboard, or Session from a fresh closed state lands a floating window at the enlarged registry defaultRect (≈920×640 list surfaces, ≈960×680 dashboards/marketplace/permissions, Session ≈860×680); Network, Tasks, and Settings keep their existing large defaults; clampRect still fits the window inside the desktop gutters on smaller viewports; closing and reopening applies the registry defaults again.
entry_points: web desktop dock; app-registry defaultRect
qa_status: skipped
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/keyboard-01-empty-desktop.png; docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-08-20-ui-normies-retry.md
overlaps: ET-web-window-routing-lifecycle; ET-web-desktop-shell-lifecycle; ET-web-catalog-navigation
---

2026-08-20 retry: skipped by explicit user instruction. No 1280/768 default-window fit pass is claimed.

story: As a builder I open a dock app and get a work surface large enough to read lists and toolbars without immediately resizing.

qa-impact: Enlarged defaultRect values for cramped prototype-sized dock apps (2026-07-22). Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 the dock now aggregates multiple app instances, cycles them in MRU order,
and offers tab destinations. Reset to retain the default-size canary.

2026-08-20 qa-impact: reset by the normie-friendly UI foundation pass. Two things changed under this
file's contract. First, the registry labels it enumerates were renamed — Bridges → Connections and
Sandbox → Permissions (`os/lib/app-catalog.ts:118,132`) — and `expected:` was rewritten to match, so
the prior `pass` was recorded against app names that no longer exist on screen. Second, and the real
reason to re-walk: the `defaultRect` values did not change, but what they have to hold did. The body
baseline moved 13.5px → 15px at line-height 1.55, `--text-small-body` 12.5px → 13.5px, and control
heights and pill sizes went up a tier. Shell chrome geometry was excluded from that lift, which means
the windows stayed the same size while their contents grew.

So the question this file now asks is not "did the numbers change" but "are the enlarged defaults
still enlarged enough" — a list surface at ≈920×640 that fit a toolbar and readable rows at 13.5px
may clip, scroll, or wrap at 15px. Walk each app from a fresh closed state and judge fit, not just
dimensions; a default that now needs an immediate resize fails this scenario's story even with an
unchanged `defaultRect`. `clampRect` behavior on small viewports and reopen-applies-defaults are
unchanged and should still hold.

`ET-web-catalog-navigation` owns the label change itself and was reset for it too.
