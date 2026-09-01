# QA Run Report — 2026-09-01 — Loop route selection

- **Scope:** Honor same-pass route skips before evaluating later coordinator-owned gates or actions.
- **Cadence tier:** targeted
- **Base:** `34208e9990622ee62e9a5cf114386273ae6abfa0` · **Build:** `304059507bbeff0213b1d516cccbd5be7939bb03` · **Environment:** isolated integration harness using the real CLI, HTTP API, UDS, and Loop runtime with deterministic ACP fixtures
- **Started:** 2026-09-01T20:00:00Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Developer/operator | Local isolated runtime / en-US | Exclusive route history |

## Flows in Scope

- Exclusive route selection and durable route history.

## Session Matrix & Results

| # | Journey / Scenario | Persona | Status | Issue | Fix commit |
|---|---|---|---|---|---|
| 1 | `LP-exclusive-route-history` | Bruno | Pass | Unselected gate executed after the selected route | `30405950` |

## Session Debriefs

- The owning regression evaluates a selected `c_verify` gate before a lexically later unselected `z_reject` gate. Only `c_verify` ran, only the selected action was queued, and `z_reject` settled `route_not_taken`.
- The public route-history E2E created and ran the Loop through HTTP, read it through CLI, observed only the selected route and gate causes, and settled `done`.

## What Was Fixed

- **Root cause:** the planner iterated stale cloned generation outputs after route selection had marked canonical outputs skipped.
- **Fix:** reload each candidate from canonical state before deciding whether it remains pending.
- **Regression:** `TestCoordinatorRunnerShouldNotEvaluateUnselectedRouteGate`.

## Runtime Errors Observed

- None.

## Human Verifications Needed

- None.

## Final Status

- **Exit gate:** `make gate` passed; focused `go test -race ./internal/loop/...` passed; targeted public-path E2E passed.
- **Issues:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 targeted Loop route journey.
- **Verdict:** ready pending exact-head provider CI.
