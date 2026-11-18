---
id: ET-window-tab-navigation-stack
area: ET
title: Preserve independent route history for every tab
persona: Théo
journey: J-organize-tabbed-work
expected: Push, pop, and replace navigation update only the addressed window; tab switches, splitting and unsplitting, moving a tab to another desktop, reload, daemon restart, palette activation, and dock cycling preserve each window's pathname, search state, and navigation depth without borrowing another tab's URL; the layout upgrade keeps every window's route and navigation history.
entry_points: task and marketplace drill-in routes; topbar back; Command-[; compozy window navigate --mode replace|push|pop
qa_status: untested
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

qa-impact: 2026-11-13 window-tabs-split S10. Grouping no longer exists; splitting, unsplitting and the upgrade now carry the same guarantee. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. In a Tasks window drill into a task (push) and a run; in a second window do the same for another task. Split them, unsplit one with Move to new tab, move a tab to another desktop and back, reload, restart the daemon: each window's back control returns along its own history and `compozy window list` NAV DEPTH stays the same.
