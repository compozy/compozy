---
id: LP-web-editor-chrome-rails
area: LP
title: Collapse and persist the editor rails
persona: Bruno
journey: J-complete-partial-loop
expected: A fresh editor opens with both rails collapsed and a full-bleed canvas; the toolbar toggles and the canvas-scoped bracket keys open and close them; selecting a node auto-opens the inspector on its Node tab; the linter dock fold persists; and a reload restores the whole chrome state from compozy:loops:editor-chrome:v1.
entry_points: /loops/$name/editor toolbar rail toggles; canvas keys
qa_status: pass
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: ""
---

story: As a Loop author, I get the whole canvas by default and the chrome I choose stays chosen across reloads.

src: .compozy/tasks/graph-eng/task_08.md
