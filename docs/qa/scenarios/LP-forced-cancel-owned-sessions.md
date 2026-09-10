---
id: LP-forced-cancel-owned-sessions
area: LP
title: Cancel a Loop and stop every owned session
persona: Bruno
journey: J-04
expected: Cancel immediately commits canceled(operator_cancel), fences new work, stops every run-owned Goal and worker task session, preserves the borrowed origin session, survives retry and restart, and leaves Rerun as the only continuation path.
entry_points: web /loop-runs/:id Cancel; `compozy loop cancel`; `compozy loop node cancel`; POST /loop-runs/:id/cancel; native compozy__loop_cancel and compozy__loop_node_cancel
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-500-forced-loop-cancel-20260831-195541-194552-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-31-issue-500-forced-loop-cancel.md
overlaps: LP-016; LP-cancel-vs-kill; LP-cancel-restart-recovers
---

Acceptance walk: start a Loop with both a run-owned Goal session and a worker task session, plus a
borrowed origin session. Cancel through one public surface and immediately fresh-read through another.
Confirm terminal canceled truth and the work fence appear before cleanup, every owned session stops,
the borrowed and foreign-workspace sessions remain active, repeated Cancel is idempotent, failed stops
retry after daemon restart, every Kill surface is absent, Resume is unavailable, and Rerun starts a new
generation with new sessions.

Issue #595 acceptance: explicitly stop and remove separate session-origin Goals, including one before its first work turn. Confirm cancellation before removal, no active/queued work after cancellation, and idempotent Run cancellation when the session no longer exists. Distinguish window-only dismissal and daemon reconnect from actual session stop. Retain historical attempts and verify that catalog Loop informational origins still have independent lifecycles.

Targeted #595 retest evidence and limits: [Goal lifecycle report](../reports/2026-09-10-issue-595-goal-lifecycle.md). This slice does not replace earlier evidence or claim an unrun full-scenario sweep.
