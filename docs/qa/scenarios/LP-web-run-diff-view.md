---
id: LP-web-run-diff-view
area: LP
title: Compare two generations or two runs of one Loop
persona: Bruno
journey: J-replay-loop-history
expected: The diff view groups node rows by change kind using the CLI vocabulary, summarizes large values as size plus content hash with a link to full content, shows the divergence banner only when the two runs pin different definition versions, labels a still-executing side, renders an honest empty state when nothing differs, and never offers a cross-loop comparison.
entry_points: /loop-runs/$runId/diff deep link; inspect sheet Compare action
qa_status: blocked-verify
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: docs/qa/evidence/2026-08-18-loop-ui-polish/diff-route.png;docs/qa/evidence/2026-08-18-loop-ui-polish/diff-run-compare.png;docs/qa/evidence/2026-08-18-loop-ui-polish/diff-generation-compare.png;docs/qa/evidence/2026-08-18-loop-ui-polish/e2e-scoped.txt
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: ""
---

story: As a Loop operator, I can see exactly what changed between two attempts without reading two raw payloads side by side.

src: .compozy/tasks/graph-eng/task_08.md

2026-08-18 loop-ui-polish: page moved onto the canonical ListingPage gutter, pickers moved to styled Selects with per-side status pills (single header band), group eyebrows neutral, redundant per-row change pill removed. E2E-025 re-anchored and passing; blocked-verify because the walk contract (/qa-execution) is operator-invoked — walk pending.
