---
id: LP-best-effort-partial
area: LP
title: Complete a fan-out with accepted partial coverage
persona: Bruno
journey: J-complete-partial-loop
expected: A best_effort fan-out admits its collect node at the declared coverage threshold, cancels unfinished lanes, exposes the structured coverage result, and reports completion_state partial consistently through CLI, HTTP, native tools, and terminal SSE.
entry_points: compozy loop validate|run|status|runs; GET /loop-runs/:id over HTTP and UDS; compozy__loop_status; compozy__loop_runs; Loop SSE; /docs/loops/guardrails; /docs/loops/dsl-reference
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps:
---

acceptance-walk: Publish a three-lane Loop with best_effort at 66% and missing acceptable. Let two lanes succeed while one remains live. Confirm collect becomes partial, the live lane is canceled by strategy, coverage reports 2/3, downstream work runs, and every structured run surface reports completion_state partial.
