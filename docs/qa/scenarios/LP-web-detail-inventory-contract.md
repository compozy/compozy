---
id: LP-web-detail-inventory-contract
area: LP
title: Loop detail sections collapse and the node inventory states its truth
persona: Dora
journey: J-05
expected: Detail main sections (Contract, Body, Recent runs) are collapsible `LoopSection`s with their concept icons; rail cards carry leading icons; gate criteria render as icon + plain type text (bot/terminal/user-check) with bold id and mono method; DAG nodes are uniform-width cards that lead with class glyphs, neutral class labels, and arrow-right connectors, with long summaries ellipsized (full text on hover); declared start kinds are one mono line; there is no Cost row in Limits; recent runs show an origin glyph and a duration column. The node inventory leads every row/card with its tinted state glyph, offers no sort control (server order only), composes the standard listing toolbar with state pills, loop and run selects, and a Rows|Cards toggle; cards show one state pill and a two-line reason clamp; per-state empty icons render with filter-aware copy; switching state/filters replaces history instead of pushing.
entry_points: web /loops/:name; web /loop-runs?nodes=waiting
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits: f1e91fc5
evidence:
last_report:
overlaps: LP-toggle-loop-goal; TA-web-task-list-loop-subtask-nesting
---

Added by the loops visual-contract parity pass (2026-08-14). The sort-control deletion is behavioral: ordering is now server truth (the route has no sort param). Walk needs nodes parked in all four states; deferred to the next seeded QA cycle — detail and inventory suites plus stories are green at 9a694ff2.
