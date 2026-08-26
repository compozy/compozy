---
id: LP-loop-contract-regime-adoption
area: LP
title: Keep Loop output contracts behaving after the contract regime moved
persona: Bruno
journey: J-complete-partial-loop
expected: A run-agent node's output_schema is still validated both when the result is produced and when it settles, an invalid payload still cannot settle as succeeded, the validator text matches what a call contract produces for the same payload, and no Loop node creates a call record.
entry_points: Loop DSL run-agent output_schema; compozy loop run --name contract-canary and compozy loop status run_01JBD9AAAA; compozy loop nodes --run run_01JBD9AAAA; web /loop-runs/run_01JBD9AAAA; compozy call list --caller ses_loop_root --limit 25 (expected empty); compozy config get calls.results.default_budget
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-call-return-contract-repair; TA-task-result-contract
---

This is the agent-comms cycle's adjacent canary. Loops were not supposed to change: they adopt the
unified contract regime but keep their own records and create no call records. What actually moved
beneath them is real — action validation, JSON extraction and repair-prompt rendering left
`internal/loop/action_schema.go` for `internal/contracts`, and the task-side blanket result ceiling
was replaced by the `[calls.results]` budget policy. The point of this walk is to test that
"unchanged" claim rather than assume it.

Run a `run-agent` node three ways: a payload that conforms to `output_schema`, one that violates it,
and one that exceeds the effective result budget. The conforming payload must be validated both when
produced and again when it settles. The violating one must be unable to settle as succeeded, and its
error text must be the validator's own. The oversized one must fail on the budget without leaking its
lease.

Then prove the regimes agree: feed the *same* payload and the *same* contract to a Loop node and to a
call's `expect`, and confirm identical verdicts and identical validator text — that equivalence is
the whole reason the pipelines were unified. Confirm the example-shape shorthand and its expanded
schema pin the same digest on both sides.

Finally confirm the boundary in both directions. After the run, `compozy call list` over that
workspace shows the Loop created no call records, and the Agents app Activity tree — which renders
session-origin calls — shows nothing for it, because the Loops app remains the owner of loop-run
visualization. Then invert it: have a Loop-bound worker session call an agent, and confirm *that*
does create a call like any other caller, so the boundary is about loop nodes rather than about
loops. The loop's own run, node and generation surfaces must still report the same truth they did
before.
