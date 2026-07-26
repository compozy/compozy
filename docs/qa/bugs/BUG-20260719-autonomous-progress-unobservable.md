# BUG-20260719-autonomous-progress-unobservable: One-kickoff progress appears stalled while agents complete work

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** QA operator; registered collaborator agents
- **Journey Step:** RT-073 one-kickoff autonomous collaboration, runtime observation
- **Scenarios:** RT-073
- **Found:** 2026-07-19 · **Report:** docs/qa/reports/2026-07-19-hermes-comparison.md
- **Origin:** n/a

## Summary

After Priya's single kickoff, the registered team completed 10 of the 11 declared tasks, but the
runtime journey log never recorded task completion, agent activity, review, or channel progress.
The release observer therefore reported every task as unstarted and every task-owning agent as
silent. An operator cannot use the required observation surface to distinguish real autonomous
progress from a stalled team.

## Reproduction

- **Charter:** consumer-saas-growth behavioral scenario · **Tour:** autonomous collaboration
- **Environment:** isolated `hermes-comparison-consumer-saas-growth-20260719-190252-199062` lab;
  desktop Web at `127.0.0.1:61528`; isolated daemon at `127.0.0.1:61527`; one confirmed provider
  kickoff and no follow-up provider prompt.

1. Materialize the `consumer-saas-growth` playbook and create its seven agents, four channels, and
   eleven declared tasks.
2. Queue the eleven runs behind the scheduler barrier, deliver the Head of Growth kickoff once,
   and release dispatch.
3. Run `observe-runtime.py` for 1,800 seconds with a 300-second stall threshold while agents work.
4. After the window, compare `qa/observation-summary.json` with
   `agh task list --workspace lumen-notes -o toon` and the exact task-run lists.

**Expected:** Runtime-owned task, session, and Network progress keeps the journey log growing so the
observer identifies the actual silent agent or stalled task, if any.

**Actual:** The journey log stopped at 14 bootstrap/controller rows. The observer marked all 11
declared tasks unstarted and six task-owning agents silent, while the independent public CLI showed
10 declared tasks completed and one task returned to ready after a failed run.

## Evidence

- Observation summary:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/observation-summary.json`
- Journey log:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/journey-log.jsonl`
- Independent CLI comparison:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/notes/autonomous-progress-observer-mismatch.md`
- Task activation and one-kickoff evidence:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/task-activation.json`;
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/operator-kickoff.jsonl`
- Fresh attempt-2 reproduction:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-r2-20260719-202601-971723-lab/qa-artifacts/qa/observer-result.json`;
  the observer again reported ten declared tasks as unstarted and all eleven runs as lacking
  completion, while the public Task catalog showed eight completed tasks and three ready tasks.
- Attempt-2 journey/public-surface evidence:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-r2-20260719-202601-971723-lab/qa-artifacts/qa/journey-log.jsonl`;
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-r2-20260719-202601-971723-lab/qa-artifacts/qa/web-task-catalog.png`.

## Fix

- **Root cause:** `observe-runtime.py` exclusively tails `qa/journey-log.jsonl`, but the AGH daemon,
  agent sessions, task scheduler, and Network runtime have no writer that projects their durable
  lifecycle events into that log. Only bootstrap/controller helpers wrote rows in this run. The
  observer then derived task and agent state from that incomplete log instead of a runtime-owned
  public progress stream.
- **Fix commit:** none
- **Regression test:** pending a runtime-to-observer progress contract and a fresh one-kickoff replay
  proving the log grows through task completion without controller-authored runtime actions.

## Verification

- **Retested:** 2026-07-19 in a fresh isolated attempt after correcting the playbook review channel
  and CLI run-status rendering.
- **Result:** Reproduced. Controller-authored observations kept the journey log non-empty across all
  five required surfaces, but the observer still raised `stall_detected=true` and derived stale
  task state because runtime-owned lifecycle events were absent. No second provider prompt was sent
  to conceal the observer stall.
