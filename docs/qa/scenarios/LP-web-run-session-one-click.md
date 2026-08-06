---
id: LP-web-run-session-one-click
area: LP
title: Run detail opens the live agent session in one click and reports usage honestly
persona: Dora
journey: J-05
expected: While a run-agent node is working, the Happening now card shows "Open session" directly on the hero step ("Working on execute task") and on any per-node lifecycle row whose cell has a bound ACP session; one click lands in the live session view (`/session/$id` → agent session). No session affordance renders for nodes without a bound session. The run detail API now exposes `session_id` per generation output (joined from the real task-run session binding — the synthetic `loop-action:*`/`daemon-loop-coordinator` ids no longer exist), so "Open session" from a task run also resolves a real session instead of a 404 blank page. The Usage rail never claims a confident `0 / ~$0.00`: when the agent reported no usage it reads Tokens "not reported" and Cost "—"; when tokens exist the estimate returns.
entry_points: web loop run detail (looprun-*) -> Happening now; web task run detail -> Open session
qa_status: blocked-decision
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-web-task-list-loop-subtask-nesting; LP-web-run-attention-quarantine-routing
---

story: Watching the live session is the single most important act while supervising a run; it must be one click from the run page, and the page must never invent a zero-cost story when the provider reported nothing.

The daemon persists the executing ACP session on the leased task run at bind time (`BindLeasedRunSession`, fenced by the claim token) and the run detail read joins it onto `generations[].outputs[].session_id`; the page view projects `nodeSessions` and the Happening-now card links hero + node rows. Token usage falls back to the session `token_usage` projection at node terminal when the live stream reported nothing.

blocked-decision: walking this requires a fresh software-delivery dogfood run with live agent sessions (spends the operator's ACP account); pending the operator starting one.

src: web/src/systems/loops/components/run-page/loop-run-now-card.tsx; web/src/systems/loops/lib/loop-node-lifecycle.ts; web/src/systems/loops/lib/loop-run-usage.ts; internal/daemon/loop_action_liveness.go; internal/task/lease_manager.go; internal/store/globaldb/queries/loop_runtime.sql

inventory: Needs QA
