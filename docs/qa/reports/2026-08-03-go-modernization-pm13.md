# QA Run Report — 2026-08-03 — Go modernization PM13

- **Scope:** Remove the inert `memory.recall.signals.metrics_enabled` contract across config, settings API, structured tools, Web, OpenAPI, and docs while preserving the live signal queue/retry controls.
- **Cadence tier:** targeted
- **Build:** `f40c110c` + current working tree · **Environment:** isolated lab `compozy-go-modernization-ms026-20260804-001619-492273-lab`, daemon `:64855`, Web proxy target `http://127.0.0.1:64855`
- **Started:** 2026-08-03T21:16:39-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | isolated `go-modernization-ms026` workspace | desktop / wifi-fast / en-US | CH-memory-settings-live-truth |

## Flows in Scope

- `J-administer-runtime-settings` — inspect, change, abandon, reload, and independently confirm runtime settings (`../journeys/J-administer-runtime-settings.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-memory-settings-live-truth | J-administer-runtime-settings / MS-026 | Dora | Back-Button Tour | Skipped | Source changed after this pre-freeze lab; a fresh run owns the verdict | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

The pre-freeze lab was deliberately closed without walking the journey. Its manifest records a clean teardown; `docs/qa/reports/2026-08-04-go-modernization-closeout.md` owns the fresh-build re-walk.

## What Was Fixed

No QA-discovered fix yet.

## Paper Cuts

None; no session ran.

## Runtime Errors Observed

None; no session ran.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

Do not reuse a QA lab after source changes; open a fresh lab and preserve the skipped row as honest run history.

## Final Status

- **Exit gate (full automated suite):** not run; this pre-freeze run was superseded before behavior execution.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 0/1 journey walked; the only row was skipped because later source changes invalidated the build.
- **Verdict:** not ready — use the fresh source-frozen closeout run for the MS-026 verdict.
