# QA Run Report — 2026-08-13 — issue-386-awaited-child

- **Scope:** Awaited `run-loop` children retain their durable liveness and authored ordering through daemon restart.
- **Cadence tier:** targeted
- **Build:** uncommitted worktree · **Environment:** isolated local daemon (`COMPOZY_HOME=/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-b03dffe66980/runtime`, UDS CLI)
- **Started:** 2026-08-13T18:38:49-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-loop-await-child-recovery |

## Flows in Scope

- `J-recover-loop-node-failure` — A Loop follows an authored path and reaches a truthful terminal state after interruption (`../journeys/J-recover-loop-node-failure.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-await-child-recovery | J-recover-loop-node-failure / LP-run-loop-await-child | Ada | Interrupt Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-loop-await-child-recovery — Ada

- **Ran:** 2026-08-13T18:38:49-03:00 → 2026-08-13T18:44:39-03:00 (box respected: yes)
- **Findings:**
  - The parent remained `running` with `z_first_child` at `awaiting_child` and `a_second_child` at `pending`; the public read carried `child_loop_run_id` `looprun-d67084dd1b2f9c62`.
  - After a daemon stop and start, the same parent and first-child ids remained live. No second child existed before the first child reached `done`.
  - The first child reached `done`; one second child (`looprun-7f6ec720fea52ca0`) began and was awaited. The parent became `done` only after that second child reached `done`.
- **Bugs filed/updated:** none
- **Scenarios settled:** LP-run-loop-await-child → pass
- **Paper cuts:** none observed through the structured CLI.
- **Surprises:** The initial custom definitions needed an explicit `uds` start binding because the CLI connected through the UDS transport; this is an authored start-policy check, not a runtime defect.
- **Suggested next charter:** Re-walk a failing child terminal outcome to confirm the parent carries the child failure route through the same restart boundary.

## What Was Fixed

The task-completion projector now records an await-mode `run-loop` result as live node state rather than generic worker success. The stored `child_loop_run_id` lets the existing coordinator refresh and recovery paths hold the parent until the child reaches its terminal outcome.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- none

## Human Verifications Needed

- none

## Decisions for a Human

- none

## Learnings

- The public `loop status` read is sufficient to prove durable node liveness, child identity, authored ordering, and the parent terminal boundary without inspecting SQLite.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` — `/Users/pedronauck/dev/qa-labs/compozy-issue-386-awaited-child-20260813-213849-412258-lab/qa-artifacts/qa/make-verify.log`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1 / 1 in-scope journey walked through the UDS CLI; Web is not in scope because this change does not alter its rendered contract.
- **Verdict:** PASS — awaited children remained durable and ordered through restart, with no duplicate child submission and no early parent completion.
