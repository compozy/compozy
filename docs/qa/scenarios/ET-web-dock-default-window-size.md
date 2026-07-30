---
id: ET-web-dock-default-window-size
area: ET
title: Dock apps open at enlarged default window sizes
persona: Bruno
journey: J-operate-desktop-shell
expected: Opening Agents, Loops, Jobs, Triggers, Bridges, Knowledge, Vault, Sandbox, Marketplace, Dashboard, or Session from a fresh closed state lands a floating window at the enlarged registry defaultRect (≈920×640 list surfaces, ≈960×680 dashboards/marketplace/sandbox, Session ≈860×680); Network, Tasks, and Settings keep their existing large defaults; clampRect still fits the window inside the desktop gutters on smaller viewports; closing and reopening applies the registry defaults again.
entry_points: web desktop dock; app-registry defaultRect
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-web-window-routing-lifecycle; ET-web-desktop-shell-lifecycle; ET-web-catalog-navigation
---

story: As a builder I open a dock app and get a work surface large enough to read lists and toolbars without immediately resizing.

qa-impact: Enlarged defaultRect values for cramped prototype-sized dock apps (2026-07-22). Flag only; the next QA cycle owns live retesting.
