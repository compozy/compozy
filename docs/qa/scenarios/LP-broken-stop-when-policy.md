---
id: LP-broken-stop-when-policy
area: LP
title: Settle a Loop whose stop condition cannot be evaluated
persona: Bruno
journey: J-complete-partial-loop
expected: A Loop whose valid `stop_when` expression fails during evaluation exits `done` by default and records a `predicate_diagnostic` event instead of starting another generation. With `on_eval_error: fail`, the same failure ends the run `failed`. Branch and filtered watch-events predicates fail their node by default and honor `on_eval_error: exit`.
entry_points: compozy loop validate; compozy loop run; compozy loop status; GET /api/workspaces/:workspace_id/loop-runs/:run_id/events; web Loop run Inspect; /docs/loops/dsl-reference; skills/compozy/references/loops.md
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/loop_run_events_e2e_integration_test.go; internal/loop/coordinator_test.go; internal/loop/coordinator_watch_test.go
last_report:
overlaps:
---

Author a Loop with a successful action and a `stop_when` expression that type-checks but fails at
runtime. Run it through a public surface, confirm the default terminal state is `done`, confirm no
successor generation starts, and inspect the SSE event payload for the predicate name, error code,
cost, limit, and warning fields. Repeat with `on_eval_error: fail`, then exercise the matching branch
and filtered watch-events overrides.

QA impact 2026-08-16: task 01 replaced implicit predicate error handling with the strict scalar/object
`stop_when` contract, explicit node policies, durable cost diagnostics, and fail-open/fail-closed
defaults. Flag only during the task loop; the loop's QA tail owns the isolated public walk.
