# BUG-20261005-loop-wall-limit-ignored: Loop keeps working beyond the configured time limit

- **Status:** open
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

Root cause investigation pending. The persona session ended before implementation inspection.
Repair is within the authorized bug-fix scope; add regression coverage in the existing owning
suite after locating the boundary, then repeat the original public journey.

## Verification

Original failure confirmed through public surfaces. Repair and fresh-persona replay remain pending.
