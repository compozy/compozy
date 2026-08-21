---
id: LP-fanout-progress-naming
area: LP
title: Author named fan-out variables and read live progress
persona: Bruno
journey: J-complete-partial-loop
expected: A nested fan-out round-trips bind_as and index_as without shadowing reserved roots, exposes the declared names only inside the body, and reports qualified nodes.<fanout>.progress plus the body-local progress alias consistently in routing, gating, CLI, HTTP, native, and Web reads.
entry_points: compozy loop validate|create|run|status; web Loop editor Graph/DSL views; GET /loop-runs/:id; compozy__loop_status; /docs/loops/dsl-reference; skills/compozy/references/loops.md
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: LP-best-effort-partial; LP-web-editor-route-ask; LP-web-strategy-progress
---

Walk the named variables through authoring, runtime evaluation, and fresh structured reads. Include a nesting-name collision and a reserved-root collision as recoverable authoring errors.
