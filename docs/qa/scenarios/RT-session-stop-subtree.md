---
id: RT-session-stop-subtree
area: RT
title: Drain a governed session subtree before stopping its root
persona: Bruno
journey: J-supervise-delegation-trees
expected: Subtree stop fences new work, closes every open descendant call, stops each child once, and reports stopped children, closed calls, and preserved results.
entry_points: compozy session stop ses_01JBD7ZZAAAA --subtree --reason "superseded"; HTTP and UDS POST /api/workspaces/{workspace_id}/sessions/{id}/stop with {"subtree":true,"reason":"superseded"}; compozy__session_stop with {"session_id":"ses_01JBD7ZZAAAA","subtree":true,"reason":"superseded"}; web /agents/activity stop-subtree action; compozy call result call_01JBD8G2K7Q9
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-agent-call-cancel; RT-delegation-activity-tree; RT-parked-child-idle-ttl
---

Build a three-level call tree with one completed result, drain it, repeat the drain, and verify the report and processes.

The drain is fence-first, and that ordering is the thing to attack: the root-closing fence persists
*before* descendants are enumerated, and call and spawn admission re-validate the fence, so try to
slip a new call into the tree mid-drain and confirm it is refused. Kill the daemon partway through
and confirm boot resumes from the persisted fence rather than restarting or abandoning the drain.
The report must name stopped children, closed calls and preserved results; the repeat must be
idempotent; queued messages for drained targets must terminalize with the drain reason; and the
child processes must actually be gone, not merely marked stopped.
