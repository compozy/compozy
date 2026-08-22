# QA Run Report — 2026-08-22 — demo seed history

- **Scope:** Northstar Pay demo seed completeness, replacement safety, and live-daemon history preservation
- **Cadence tier:** targeted
- **Build:** working tree · **Environment:** isolated targeted lab, daemon `127.0.0.1:52229`, web `localhost:3000`
- **Started:** 2026-08-22T15:13:55-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Power User | desktop / wifi-fast / en-US | CH-demo-seed-boot-truth |

## Flows in Scope

- `J-prepare-demo-recording` — recreate one populated, truthful demo world (`../journeys/J-prepare-demo-recording.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-demo-seed-boot-truth | J-prepare-demo-recording / RT-demo-seed-replace-boot | Dora | Data Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-demo-seed-boot-truth — Dora

- **Ran:** 2026-08-22T15:13:55-03:00 → 2026-08-22T15:52:50-03:00 (box respected: no; implementation gaps were fixed before the clean replay)
- **Findings:** The clean replay passed. Two seeded workspaces retained the same IDs and the same 18 count fields after replace. Daemon boot returned all 17 Loop runs as history with zero live aggregates. The UI showed `Active now 0`, `Past 7`, `History`, and `Recorded state`; historical controls were absent and CLI mutation was rejected. Goal turns, the real Git worktree, scoped memory, disabled notification presets, task outcomes, automation, transcripts, and observability were populated through public reads. Replace refused an unowned workspace root and preserved its sentinel file.
- **Bugs filed/updated:** none; review and engineering checks found the implementation gaps before this formal clean replay.
- **Scenarios settled:** RT-demo-seed-replace-boot → pass
- **Paper cuts:** none
- **Surprises:** macOS exposes the temporary runtime through both `/var` and `/private/var`; the seed now compares canonical filesystem identity.
- **Suggested next charter:** record the video series against this seed and create live runs only for interactive transitions.

## What Was Fixed

No fix was applied during the formal persona walk. Before the clean replay, reviewer and engineering verification corrected historical rows being counted as active, live wording and elapsed clocks on historical details, and macOS path-alias ownership during replace. Canonical tests cover each contract.

## Paper Cuts

None.

## Runtime Errors Observed

None. Browser console noise was not used as evidence; CLI, HTTP, refresh, and deep links independently confirmed the state.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A stored status such as `running` does not imply live work; consumers must check `historical` before using live language, timers, streams, controls, or aggregates.
- Seed replacement needs physical filesystem identity on macOS, not lexical path equality.
- The offline seed is suitable for truthful history and populated surfaces; interactive transitions still start through public runtime surfaces.

## Final Status

- **Earlier lab verification:** `make verify` — PASS; evidence: `/Users/pedronauck/dev/qa-labs/compozy-demo-seed-history-20260822-181255-352261-lab/qa-artifacts/qa/final-make-verify.log`. Exact-head full verification is delegated to pull-request CI.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked; CLI, API, web, and runtime surfaces covered
- **Verdict:** ready — the Northstar Pay seed is populated, repeatable, ownership-safe, and truthful after daemon boot.
