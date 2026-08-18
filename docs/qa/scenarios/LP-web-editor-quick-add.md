---
id: LP-web-editor-quick-add
area: LP
title: Add editor nodes by quick-add and connection drop
persona: Bruno
journey: J-complete-partial-loop
expected: Pressing a with canvas focus opens quick-add while the same key typed into an inspector input does not; choosing a kind places the node at the visible viewport centre, selects it, opens the inspector, and enables Save layout; double-clicking the pane opens the same dialog; jump-to-node reveals and centres a node; dropping a dragged connection on empty canvas opens the picker and creates the node already wired in one step, with Escape dismissing it with zero mutations.
entry_points: /loops/$name/editor canvas quick-add; connection drop picker
qa_status: untested
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop author, I can build a graph from the keyboard and the canvas without hunting through a rail.

src: .compozy/tasks/graph-eng/task_08.md
