---
id: RT-delegation-depth-and-caps
area: RT
title: Refuse over-deep and over-wide delegation in each wall's own shape
persona: Dora
journey: J-contain-and-audit-delegation
expected: At the depth wall the call tool is absent from the child's toolset rather than present and refusing, the per-parent children cap rejects with a typed error naming the cap and count, and the per-root execution budget admits and queues visibly instead of rejecting.
entry_points: compozy config set calls.max_depth 1, compozy config set calls.max_children 1, compozy config set calls.max_active_per_root 1, and compozy config set calls.max_batch 1; compozy__agent_call with {"agent":"reviewer","prompt":"nested work"} from the depth-walled child; compozy call list --state queued --limit 8; HTTP and UDS POST /api/workspaces/{workspace_id}/calls with {"tasks":[{"agent":"scout","prompt":"one"},{"agent":"reviewer","prompt":"two"}]}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-calls-config-effects; RT-agent-call-batch; RT-call-wake-delivery-exactly-once
---

Containment here is budget-based, and the three walls refuse in three deliberately different shapes.
Conflating them is the failure this scenario hunts.

**Depth** is a wall of absence. Delegate down to `calls.max_depth` and confirm the call tool is not
in the walled child's toolset at all — there must be no tool that exists only to return
`call_depth_exceeded` to its own caller. Confirm every child's context states its literal remaining
depth, and that depth is computed from durable lineage: plant a forged depth claim in the prompt and
confirm it changes nothing.

**`max_children`** is an admission wall: a call over the per-parent cap **rejects** with
`call_children_cap` naming the cap and the current count. It must never quietly queue.

**`max_active_per_root`** is an execution budget: admitted work over it **queues**, visibly, as a
durable `queued` call that a list read can see. It must never reject. Then let that queued work
through and confirm nothing about the budget interfered with an already-committed call's settlement
or delivery.

Finish on the batch wall — a batch over `calls.max_batch` is rejected as one request with
`call_batch_over_cap` and nothing partial ran — and confirm each limit change applies to new calls
only, leaving in-flight snapshots untouched.
