# BUG-20261005-loop-cancel-leaves-worker-running: Cancel run leaves its worker executing

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-01, cancel an active Loop and trust that its worker stops
- **Scenarios:** LP-003
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Bruno confirms Cancel run after an execution exceeds its time limit. The run immediately
becomes Canceled, but its managed worker remains active and continues prompting for at
least 59 seconds. The confirmation promised active sessions would stop automatically.
Bruno must separately stop the worker session to end the work.

## Reproduction

- **Charter:** CH-012 · **Tour:** Feature Tour
- **Environment:** desktop 1512×862 / DPR 2 / fast Wi-Fi / en-US / America/Los_Angeles.
- **Build:** 547027459 runtime behavior; isolated daemon PID 9029, port 50727,
  binary 42a1236d1a792a8bd9e96cc282692b672dd3417143dcfc60a70991c43a299b1a.

1. Start the provider-backed implement-tasks run described in the linked wall-limit finding.
2. In Web, choose Cancel run and confirm the dialog for looprun-4e6bb5f893b3b18f.
3. Read Loop why through CLI and the managed worker status independently through UDS.
4. After the canceled outcome is durable, verify whether the worker still has an active prompt.

**Expected:** Canceling the run settles its owned worker without requiring a separate session stop.

**Actual:** The run records canceled/operator_cancel at 09:44:55.374524Z. At 09:45:54Z worker
sess_c848f72460c729e1c69ec46cbe09bc79 still reports active, prompting, active_prompt=true.
A separate public session stop --wait succeeds with verified=true and forced escalation.
This differs from the false operator-attribution bug on already completed workers.

## Evidence

- docs/qa/evidence/2026-10-02-untested/
  loops-terminal-guardrails-bruno-{cancel-confirm-2,final-worker,canceled-why,stop-worker}.json; loops-terminal-guardrails-bruno-canceled.png.
- Recording: /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/loops-terminal-guardrails-bruno
  (closed, 20 frames).
- Exact owned resources and cleanup history: loops-terminal-owned.json.

## Fix

Root cause investigation pending. The persona session ended before implementation inspection.
Repair is within the authorized bug-fix scope; add regression coverage in the existing owning
suite after locating the boundary, then repeat the original public journey.

## Verification

Original failure confirmed through public surfaces. Repair and fresh-persona replay remain pending.
