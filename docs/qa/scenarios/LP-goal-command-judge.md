---
id: LP-goal-command-judge
area: LP
title: Approve a Loop goal through a command-only judge
persona: Bruno
journey: J-26
expected: A command-only Goal judge runs from the selected workspace on every settled turn; exit 0 advances the Run, a non-zero exit routes one informed revision with durable criterion diagnostics, and neither path reports goal_judge_unavailable or pauses for human approval.
entry_points: compozy loop run; compozy loop status; compozy loop runs; HTTP/UDS loop-run routes; web /loop-runs/:run_id detail; loop_goal_judge_attempts diagnostics
qa_status: pass
bug_ids: BUG-20260808-goal-command-judge-unavailable
fix_status: fixed
retest_status: pass
fix_commits: 9eaaf30
evidence: docs/qa/evidence/2026-08-10-loop-convergence/run-detail-goal-diagnostics.png; /Users/pedronauck/dev/qa-labs/compozy-loop-convergence-20260810-034845-371840-lab/qa-artifacts/qa/evidence/run-proof.json; /Users/pedronauck/dev/qa-labs/compozy-loop-convergence-20260810-034845-371840-lab/qa-artifacts/qa/qa-audit-report.md; /Users/pedronauck/dev/qa-labs/compozy-loop-convergence-20260810-034845-371840-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-10-loop-convergence.md
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

QA impact 2026-08-10: reset after fixing command-judge policy, workspace-root execution, catalog-run
Goal reporting, and durable per-criterion diagnostics. The new walk uses a fresh isolated Run and
must not resume or mutate the historical operator Run.

QA result 2026-08-10: Bruno ran `looprun-63a9f3bae9fa958d` in a fresh isolated workspace. Turn 1
persisted a rejected command verdict with exit code 1, workspace stdout, stderr, blockers, criteria,
and warnings. Turn 2 persisted an approved verdict with exit code 0, the successor node completed,
and the Run reached `done`. The bound catalog-origin Goal session recorded `compozy__goal_report`.
