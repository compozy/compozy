---
id: LP-fan-out-filtering
area: LP
title: Filter fan-out source elements before creating lanes
persona: Bruno
journey: J-complete-partial-loop
expected: A fan-out filter can read item, source index, and authored aliases; only matching elements are batched, and max_fan_out counts the resulting lanes.
entry_points: compozy loop validate; compozy loop run; compozy loop status; /docs/loops/dsl-reference; skills/compozy/references/loops.md
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-runtime-graph-fixes-20260819-234724-890442-lab/qa-artifacts/qa/logs/bruno-filter-success-status.json; /Users/pedronauck/dev/qa-labs/compozy-loop-runtime-graph-fixes-20260819-234724-890442-lab/qa-artifacts/qa/logs/bruno-zero-filter-status.json; /Users/pedronauck/dev/qa-labs/compozy-loop-runtime-graph-fixes-20260819-234724-890442-lab/qa-artifacts/qa/logs/bruno-filter-error-exit-status.json; /Users/pedronauck/dev/qa-labs/compozy-loop-runtime-graph-fixes-20260819-234724-890442-lab/qa-artifacts/qa/logs/bruno-terminal-drain-v2-proof.json
last_report: docs/qa/reports/2026-08-19-loop-runtime-graph-fixes.md
overlaps:
---

acceptance-walk: Validate and run a Loop whose fan-out uses bind_as and index_as in a filter, has batch_size 2, and would exceed max_fan_out before filtering. Confirm validation succeeds, excluded source elements create no worker lanes, matching elements retain order in the batch, and an invalid predicate follows the authored on_eval_error policy.
