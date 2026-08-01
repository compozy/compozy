---
id: ET-window-tab-navigation-stack
area: ET
title: Preserve independent route history for every tab
persona: Théo
journey: J-organize-tabbed-work
expected: Push, pop, and replace navigation update only the addressed window; tab switches, grouping, reload, daemon restart, palette activation, and dock cycling preserve each member's pathname, search state, and navigation depth without borrowing another tab's URL.
entry_points: task and marketplace drill-in routes; topbar back; Command-[; compozy window navigate --mode replace|push|pop
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/keyboard-06-route-reload-general.png; /Users/pedronauck/dev/qa-labs/compozy-window-tabs-live-apply-status-retest-20260801-115716-306628-lab/qa-artifacts/qa/evidence/nav-stack-limit.json
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-web-window-routing-lifecycle; ET-window-tab-agent-parity
---

Derived from J-organize-tabbed-work steps 3-4. Covers ADR-009 active-member semantics, route
abandon-and-resume, reload/restart continuity, and TechSpec invariant 4.
