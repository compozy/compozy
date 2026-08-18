---
id: LP-web-editor-node-menu-selection
area: LP
title: Use the node context menu, multi-select and edge delete
persona: Bruno
journey: J-author-fanout-strategy
expected: Right-clicking a node offers duplicate, copy, paste, rename and delete on a workspace loop and hides or disables every mutating verb on a read-only source; marquee-selecting two nodes and deleting removes both nodes and all of their edges with no orphan edge left in the draft; a selected edge deletes from its midpoint affordance; and route edges carry their condition pill.
entry_points: /loops/$name/editor node context menu; canvas marquee; edge affordance
qa_status: untested
bug_ids: ""
fix_status: none
retest_status: pending
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop author, I can rearrange and prune a graph directly on the canvas without leaving it in a state that fails publish.

src: .compozy/tasks/graph-eng/task_08.md
