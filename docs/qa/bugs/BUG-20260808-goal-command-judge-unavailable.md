# BUG-20260808-goal-command-judge-unavailable: Command-only Goal judges never evaluate and trap the Loop in approval pauses

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** n/a
- **Scenarios:** LP-goal-command-judge
- **Found:** 2026-08-08 · **Report:** n/a (live forensics, dev daemon)
- **Origin:** live run `looprun-0693ff2130ebb9ce` (workspace imersao-aovivo, loop software-factory)

## Summary

A Loop goal node whose `judge` list contains only `command` criteria passes `compozy loop validate`
(the linter explicitly supports command-only judges and rejects `human` there), but at runtime every
judge attempt fails in ~2ms with `goal_judge_unavailable` before the command ever executes. The goal
can never be approved: below the broken-streak limit each failure re-prompts the orchestrator for
another turn (re-executing already-finished work), and at the limit the run pauses for human
approval. Because approval does not clear the streak and the failure is deterministic, every resumed
turn immediately pauses again — an endless approval loop that never reaches downstream nodes.

## Reproduction

- **Environment:** `make dev` daemon, `COMPOZY_HOME=.tmp/compozy-dev-home`
- Loop: goal node with `judge: [{id, type: command, check: <exit-0 command>}]`, downstream action node.

1. Start the loop run and let the goal node complete its work turn.
2. Observe `loop_goal_judge_attempts`: `outcome: error`, blocking issue `goal_judge_unavailable`,
   note `loop gate: verdict_policy revise_until_clean requires judge or human`.
3. Observe turns 1–2 re-prompt the goal session; turn 3 pauses with `needs_approval`
   (`goal_judge_broken`); every approval buys exactly one more turn before pausing again.

**Expected:** the judge command runs; exit 0 approves the goal and the loop advances to the next
node; non-zero exit routes a revision turn.

**Actual:** the command never ran; the run oscillated between `running` and `needs-approval`
(`broken_streak` reached 5) and never reached the QA node.

## Evidence

- `loop_goal_judge_attempts` rows turn 1–5, all
  `{"id":"goal_judge_unavailable","note":"loop gate: verdict_policy revise_until_clean requires judge or human: gate \"implementar:goal-judge\""}`.
- `loop_run_events` seq 14–34: repeated `needs_approval` → human approval → one
  `goal_turn_started`/`goal_turn_completed` pair → `needs_approval` again.
- Checkpoint: `phase: awaiting_control`, `goal_status: paused`, `broken_streak: 5`, `turns_used: 5/12`.

## Fix

- **Root cause:** `internal/daemon/loop_goal_executor.go` hardcoded
  `VerdictPolicy: revise_until_clean` when building the goal-judge gate, while
  `gate.Evaluator.validateGate` requires an `agent-judge` or `human` criterion under that policy —
  and the goal linter forbids `human` and supports command-only judges. Contract gates already
  derive their policy from criteria; the goal gate did not.
- **Fix commit:** `9eaaf30` — `gate.GateFromGoalJudge` derives the verdict policy from the criteria
  (agent-judge present → `revise_until_clean`, deterministic-only → `fixed_passes`) and the daemon
  uses it instead of the hand-built gate.
- **Regression test:** `internal/daemon/loop_goal_executor_test.go::TestLoopGoalJudgeEvaluatorShouldEvaluateDeterministicJudges`

## Verification

- **Retested:** 2026-08-10 in fresh isolated Run `looprun-63a9f3bae9fa958d`; the historical operator
  Run was not resumed or mutated.
- **Result:** the deterministic command executed from the selected workspace, persisted one
  rejected turn with complete diagnostics, approved the next turn with exit code 0, advanced to the
  successor node, and terminalized the Run as `done`. The strict QA evidence audit passed with zero
  blockers and zero warnings.
