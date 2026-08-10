# QA Run Report — 2026-08-10 — loop-convergence

- **Scope:** One-pass Loop template materialization, Goal convergence, workspace-correct command judges, durable diagnostics, and truthful Web projection.
- **Cadence tier:** targeted
- **Build:** working tree · **Environment:** isolated local daemon/API/Web lab on port 65503 with a live Cursor Goal session
- **Started:** 2026-08-10T03:48:45-03:00 · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-goal-command-convergence |
| Ada | Autonomous Agent | structured CLI+API / wifi-fast / en-US | CH-loop-template-materialization |
| Lea | New User | laptop / wifi-fast / en-US | CH-untested-004-04-lea |

## Flows in Scope

- `J-26` — converge and control one Goal (`../journeys/J-26-converge-and-control-goal.md`)
- `J-01` — arrive and use an authored task graph (`../journeys/J-01-arrive-and-use-run.md`)
- `J-04` — inspect and control a live Run (`../journeys/J-04-operator-pause-resume.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-goal-command-convergence | J-26 / LP-goal-command-judge | Bruno | Feature Tour | Fixed | BUG-20260808-goal-command-judge-unavailable | 9eaaf30 |
| 2 | CH-loop-template-materialization | J-01 / LP-loop-template-snapshot-round-trip | Ada | Feature Tour | Pass | | |
| 3 | CH-untested-004-04-lea | J-04 / LP-run-detail-story-redesign | Lea | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Bruno — Goal command convergence

- The first command turn rejected with exit code 1 and durable stdout, stderr, criterion, warning,
  and blocker data.
- The second command turn approved with exit code 0 from the same isolated workspace.
- The catalog-origin Goal session recorded `compozy__goal_report`; the successor completed and the
  Run reached `done`.

### Ada — one-pass materialization

- Dry-run resolved `slug=weather-app`, kept `{{ .inputs.shadow }}` literal, and created no Run.
- The persisted Run returned the raw authored contract under `executed_definition` and the resolved
  narrative under `materialized_contract`.
- The Goal agent received resolved operator text without a second template pass.

### Lea — truthful run detail

- Progress rendered the resolved Goal and definition of done.
- The Goal timeline exposed rejected and approved command diagnostics.
- Inspect and the definition page preserved raw authored templates and operator facts; the browser
  console had no errors.

## What Was Fixed

- `BUG-20260808-goal-command-judge-unavailable` is fixed and verified in a fresh isolated Run.
- Goal parameters now materialize exactly once before agent binding, prompts, and judging.
- Command judges run from the selected workspace and persist reconstructible diagnostics.
- Catalog-origin Goal sessions can record `compozy__goal_report`.

## Paper Cuts

None.

## Runtime Errors Observed

None.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A raw definition and a materialized runtime projection are both necessary: one preserves audit
  truth, while the other keeps operator-facing text resolved.
- The durable Goal-turn projection must own diagnostics; relying on live process state makes restart
  and Web replay incomplete.
- The selected workspace is part of a command judge's behavioral contract, not an ambient process
  detail.

## Final Status

- **Targeted real-scenario audit:** PASS — 0 blockers, 0 warnings
- **Teardown:** PASS — `teardown.json` reports `clean: true` and no survivors
- **Repository full gate:** runs once after the final tracked mutation, outside this QA lab
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 3/3 scenarios walked
- **Verdict:** PASS
