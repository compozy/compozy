# BUG-20261005-loop-wall-limit-ignored: Loop keeps working beyond the configured time limit

- **Status:** invalid
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-01, set and trust an unattended Loop time limit
- **Scenarios:** LP-003
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Bruno starts implement-tasks with a 20-second wall limit and expects the bounded job to stop.
The worker keeps executing while the Web meter shows 1m38s / 20s and the run remains Running.
An unattended operator cannot rely on the configured time boundary.

## Reproduction

- **Charter:** CH-012 · **Tour:** Feature Tour
- **Environment:** desktop 1512×862 / DPR 2 / fast Wi-Fi / en-US / America/Los_Angeles.
- **Build:** 547027459 runtime behavior; isolated daemon PID 9029, port 50727,
  binary 42a1236d1a792a8bd9e96cc282692b672dd3417143dcfc60a70991c43a299b1a.

1. Author the pending studio-weekly-handoff task and its v2 graph manifest in Studio Operations.
2. Start implement-tasks through the CLI with budget_wall_sec=20, budget_tokens=120000,
   iteration_cap=50, reattempt_strategy=halt, and auto_commit=false.
3. Open its returned Web URL, observe execute_default running, close its tab, and return
   through Loops > Runs. Reload the run and inspect the elapsed time.
4. Independently read Loop status/why and HTTP/UDS while its worker is still active.

**Expected:** The run reaches Exhausted at its time boundary and stops its worker; it is never Done.

**Actual:** Run looprun-4e6bb5f893b3b18f remains running more than 90 seconds after start with the
20-second limit still present in effective configuration. Worker
sess_c848f72460c729e1c69ec46cbe09bc79 remains active/prompting. The operator eventually
cancels the run at 09:44:55Z; this is not an observed exhausted outcome.

## Evidence

- docs/qa/evidence/2026-10-02-untested/
  loops-terminal-guardrails-bruno-{running-close,after-close-status,refound,wall-why,overrun-http,overrun-uds,overrun-worker}.json; loops-terminal-guardrails-bruno-overrun.png.
- Recording: /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/loops-terminal-guardrails-bruno
  (closed, 20 frames).
- Exact owned resources and cleanup history: loops-terminal-owned.json.

## Fix

No runtime repair is required for the reported overrun. The original expected result assumed
an in-turn timer, but the established budget contract checks before dispatch and at generation
boundaries (LOOPS-DESIGN-SPEC section 5.4). The owning
TestGlobalDBCompleteCoordinatorAndEnqueueNextShouldDeferBoundaryWhileGenerationInFlight suite
explicitly drains active work while preventing further reservations. The first walk canceled
before that boundary, so it could not establish a missing budget transition.

## Verification

Fresh CH-012 Bruno replay looprun-4f2b4617dbd32be1 uses the same 20-second wall budget,
120K token budget and pending weekly-handoff task, with the live-catalog GPT-6.1-Sol runtime.
The active worker settles at 10:18:51.734358Z; the run becomes exhausted/budget at
10:18:51.756209Z, before per_task_done is dispatched. That next action remains pending
without a task_run_id. Usage is 87,512 tokens, below the token cap, so this is the wall
budget boundary. Final duration is 2m25s. No operator cancellation or separate session stop
was issued. Web reload, CLI why, HTTP and UDS retain Exhausted; completed task work is kept.
The original immediate-stop expectation is invalid, and the finding is retained for audit.

Evidence: loops-budget-boundary-bruno-{working,observe-2,terminal-view,terminal-why,
terminal-http,worker-final,worker-history-final,task-run}.json and the visually inspected
loops-budget-boundary-bruno-exhausted.png. Recording
/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/loops-budget-boundary-bruno
is closed with five frames. Ownership is recorded in loops-budget-boundary-owned.json.

The worker's automatic stop still has the separately tracked false operator attribution.
This classification does not close that finding or the remaining stalled leg of LP-003.
