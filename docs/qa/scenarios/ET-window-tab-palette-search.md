---
id: ET-window-tab-palette-search
area: ET
title: Find and activate a tab from the command palette
persona: Bruno
journey: J-organize-tabbed-work
expected: Command-K lists live tabs with disambiguating app, desktop, leaf-title, attention, and minimized context; selecting a result restores and activates that exact window without changing its route depth, and an empty or closed result group disappears without stale rows.
entry_points: Command-K; command palette Go to tab; web desktop URL
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/keyboard-03-palette-tabs.png
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-window-tab-multi-instance; ET-web-window-routing-lifecycle
---

Derived from J-organize-tabbed-work step 3. Covers keyboard-first usability, truthful attention,
stale-result recovery, and compatibility with minimized and cross-desktop windows.

2026-08-20 qa-impact: Root matching now inserts the agent fallback only when the served threshold
allows it. Keep this scenario `untested`; task 12 owns the tab-result boundary re-walk.

Walk (task_11 plan):

1. Group several windows into tabs across two desktops, minimize one member.
2. ⌘K and type a tab's title — the tab section lists live tabs with app, desktop, leaf-title,
   attention, and minimized context. Capture `GET /api/cmd-palette/rank-signals`
   `fallback_weak_match_threshold` and the query's top score: score equal to the threshold keeps
   tab results plus Ask agent; score below the threshold is fallback-only.
3. Select a result — that exact window restores and activates without changing its route depth,
   including the minimized and cross-desktop members.
4. Close a listed tab from another surface — the stale row disappears on refresh; a closed-result
   group vanishes without dead rows.

Expected evidence: screenshots of the disambiguated tab results and the restored cross-desktop
window; the rank-signals threshold and top-score pair for the equality and below-threshold
branches.
