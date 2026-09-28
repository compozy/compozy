---
id: LP-web-detail-inventory-contract
area: LP
title: Loop detail sections collapse and the node inventory states its truth
persona: Dora
journey: J-05
expected: Detail main sections (Goal and finish line, Steps, Recent runs) are collapsible `LoopSection`s with their concept icons; the rail keeps three cards (Inputs, Automations, Limits) with leading icons, while the version rides the lede tag and 30-day success rides the lede meta; check criteria render as icon + plain type label (Agent review/Command/You approve) with bold id and a sans method line; how a run can end and overlapping-run policy sit in a folded Details row in plain words; DAG nodes are uniform-width cards that lead with class glyphs, a humanized step name, one faint detail line, and arrow-right connectors, with long text ellipsized (full text on hover); raw declared start kinds are not shown; Limits use plain labels (Max rounds, Time limit, When a budget runs out…) with ceilings on hover and no Cost row; recent runs show status, start time, and duration only. The node inventory leads every row/card with its tinted state glyph, offers no sort control (server order only), composes the standard listing toolbar with state pills, loop and run selects, and a Rows|Cards toggle; cards show one state pill and a two-line reason clamp; per-state empty icons render with filter-aware copy; switching state/filters replaces history instead of pushing.
entry_points: web /loops/:name; web /loop-runs?nodes=waiting
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits: f1e91fc5
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/task07-scenario-walks.md; .compozy/tasks/loop-task-legibility/evidence/visual/task_05/VC-25; .compozy/tasks/loop-task-legibility/evidence/visual/task_05/VC-32
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: LP-toggle-loop-goal; TA-web-task-list-loop-subtask-nesting
---

Added by the loops visual-contract parity pass (2026-08-14). The sort-control deletion is behavioral: ordering is now server truth (the route has no sort param). Walk needs nodes parked in all four states; deferred to the next seeded QA cycle — detail and inventory suites plus stories are green at 9a694ff2.
