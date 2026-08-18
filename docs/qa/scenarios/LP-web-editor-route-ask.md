---
id: LP-web-editor-route-ask
area: LP
title: Author ask, route, strategy and review nodes in the visual editor
persona: Bruno
journey: J-complete-partial-loop
expected: The palette offers ask and route alongside every existing kind with none removed; their inspectors write DSL-valid shapes (a route keeps declaration order and a required default, a strategy collapses to shorthand unless best_effort, a review writes or clears the whole block); the DSL view round-trips losslessly; and the linter dock surfaces the new codes under the Routing and Human requests chips.
entry_points: /loops/$name/editor palette, inspector, linter dock, DSL view
qa_status: pass
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: ""
---

story: As a Loop author, I can build a routed, human-in-the-loop graph in the editor and publish it without hand-editing YAML.

src: .compozy/tasks/graph-eng/task_08.md
