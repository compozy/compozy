---
id: LP-web-editor-route-ask
area: LP
title: Author ask, route, strategy and review nodes in the visual editor
persona: Bruno
journey: J-complete-partial-loop
expected: The palette offers ask and route alongside every existing kind with none removed; route field edits, connections, renames, pastes, and deletions keep the graph edges and DSL targets synchronized; inspectors write DSL-valid shapes; the DSL view round-trips losslessly; and the linter dock surfaces the new codes under the Routing and Human requests chips.
entry_points: /loops/$name/editor palette, inspector, linter dock, DSL view
qa_status: pass
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: /Users/pedronauck/dev/qa-labs/compozy-graph-eng-review-20260818-141718-102629-lab/qa-artifacts/qa/screenshots/loop-editor-authored-published.png
last_report: docs/qa/reports/2026-08-18-graph-eng-review.md
overlaps: ""
---

story: As a Loop author, I can build a routed, human-in-the-loop graph in the editor and publish it without hand-editing YAML.

src: .compozy/tasks/graph-eng/task_08.md
