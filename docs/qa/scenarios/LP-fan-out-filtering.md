---
id: LP-fan-out-filtering
area: LP
title: Filter fan-out source elements before creating lanes
persona: Bruno
journey: J-complete-partial-loop
expected: A fan-out filter can read item, source index, and authored aliases; only matching elements are batched, and max_fan_out counts the resulting lanes.
entry_points: compozy loop validate; compozy loop run; compozy loop status; /docs/loops/dsl-reference; skills/compozy/references/loops.md
qa_status: pass
bug_ids: BUG-20260901-filtered-fanout-phantom-rows
fix_status: fixed
retest_status: pass
fix_commits: e96962c
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-506-filtered-fanout-roster-20260901-131013-477371-lab/qa-artifacts/qa/logs/cli-single-worker-roster.json
last_report: docs/qa/reports/2026-09-01-issue-506-filtered-fanout.md
overlaps:
---

acceptance-walk: Validate and run a Loop whose fan-out uses bind_as and index_as in a filter, has batch_size 2, and would exceed max_fan_out before filtering. Confirm validation succeeds, excluded source elements create no worker lanes, matching elements retain order in the batch, and an invalid predicate follows the authored on_eval_error policy.

QA impact 2026-09-01: reset after sparse fan-out roster projection changed. The re-walk must prove
filtered source gaps create no phantom pending rows and every public read reports the same denominator.

QA result 2026-09-01: passed. A real filtered fan-out retained source index `2`; the terminal
roster exposed that worker only and no pending row for rejected indexes `0` or `1`.
