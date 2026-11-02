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

### User scope (U3 integration leg)

Run `/goal` in a User session. Confirm the run keeps an empty workspace and the session's concrete
profile, its checkpoint is valid, and Goal status remains visible after the command. Repeat under
another profile and verify run/request lists and `@user&all_profiles=true` retain exact User scope
and owner labels. Worker coverage lives in the canonical Goal command and SQLite loop suites; the
full User-session transport journey waits for U2 integration.

### Verified external waits (#1149)

Use a provider-free acpmock worker that calls the real `compozy__monitor_create` tool during its
first Goal turn, then ends without a terminal claim. Keep a 180-second timer active and make the
judge reject unfinished work. Verify `status: waiting`, a flat started-turn count, and no judge
attempt for the yielded turn; after the wake, verify one queued wake turn followed by resumed Goal
work. Repeat with an asynchronous delegated child and with a terminal monitor. For the child,
retain the wait across terminal publication and queued/dispatched result delivery; resume only
after delivery, acknowledgement, or disposal.

Cancel the monitor, stop the child, and exercise a failed/undelivered wake: each must release the
Goal without operator resume. Restart the isolated daemon during a live wait and verify the
original wait deadline, stable turn audit, and eventual advance. Repeat with max-token compaction
pending, and with a prior queued input registering a monitor before the Goal ticket is claimed.
Neither may submit Goal work or compaction while the wait is live. Pause and Stop while waiting must
remain effective; an explicit complete claim must still reach the judge. Check Waiting and Pause
in the Web Goal strip and inspect the CLI/HTTP Goal snapshot. A stuck wait must pause with a reason.

2026-11-02 baseline: issue #1149 reproduced before the fix for timers and async children. Both
consumed 8/8 turns and exhausted before the wait ended. acpmock drove the real native-tool and
agent-judge paths; its judge always revised. The lab used a driver-only overlay allowing judge
sessions without hosted MCP.

Operator lab report at `304669183`: a 180.9-second monitor wait and a 149.5-second child wait
kept Goal turn counts flat with no waiting-state judge. The Goal completed at 3/8 turns with two
judges. CLI, HTTP, native, and Web exposed Waiting; 44 Goals had no duplicate turns or judges.
The child-only restart case failed because the stopped parent did not consume the queued wake.
The recovery, delivery-handoff, pending-compaction, queued-input deferral, pause deadline, and
narrow-screen fixes require a new-head runtime/rendered rewalk before delivery.
