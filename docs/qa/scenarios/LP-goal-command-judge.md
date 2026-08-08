---
id: LP-goal-command-judge
area: LP
title: Approve a Loop goal through a command-only judge
persona: Bruno
journey:
expected: A goal node whose judge list holds only command criteria runs the command on every settled turn; exit 0 approves the goal and the run advances to the next graph node, a non-zero exit routes one revision turn, and no judge attempt reports goal_judge_unavailable or pauses the run for human approval.
entry_points: compozy loop run; compozy loop status; compozy loop runs; HTTP/UDS loop-run routes; web /loop-runs/:run_id detail; loop_goal_judge_attempts diagnostics
qa_status: blocked-decision
bug_ids: BUG-20260808-goal-command-judge-unavailable
fix_status: fixed-pending-verification
retest_status:
fix_commits:
evidence: internal/daemon/loop_goal_executor_test.go::TestLoopGoalJudgeEvaluatorShouldEvaluateDeterministicJudges
last_report:
overlaps: LP-ratchet-climb-restore
---

Run a loop whose goal node declares a deterministic `command` judge (no agent-judge criterion) and
a downstream action node. The judge command must actually execute at each turn settlement: a passing
check approves the goal and the run proceeds past the goal node; a failing check produces a
`rejected` verdict and one revision turn, never `goal_judge_unavailable`.

2026-08-08 blocked-decision: the live reproduction (`looprun-0693ff2130ebb9ce`, workspace
imersao-aovivo) is paused at `needs-approval` and resuming it consumes the operator's real provider
tokens — walking this scenario needs the operator to approve the paused gate on the rebuilt daemon
or authorize a fresh provider-backed run. Unit regression already pins the gate construction.
