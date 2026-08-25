# QA Run Report — 2026-08-24 — ENG-131 command-palette direct entity routes

- **Scope:** Concrete entity rows in the OS command palette now open their detail/action route directly, with workspace ownership preserved.
- **Cadence tier:** targeted
- **Build:** `a7a89709d` + ENG-131 working tree · **Environment:** isolated CompozyOS lab, Vite `http://localhost:3031`, daemon `127.0.0.1:60018`
- **Started:** 2026-08-25T00:47:48Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Sol | isolated workspace `ws_8f9af3086c3bf927` | Chromium / local / pt-BR operator environment | web palette walk |

## Flows in Scope

- `J-command-os-from-palette` — open one concrete workspace loop from the command palette and land on its detail route (`docs/qa/journeys/J-command-os-from-palette.md`).
- `ET-palette-direct-entity-routes` — verify direct identity, workspace ownership, and deep-link readability (`docs/qa/scenarios/ET-palette-direct-entity-routes.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | targeted web lab | J-command-os-from-palette / ET-palette-direct-entity-routes | Sol | workspace loop direct route | Pass | — | working tree |

## Session Debriefs

### Targeted web walk — Sol

- **Ran:** 2026-08-25T00:47:48Z → 2026-08-25T00:54:00Z (box respected: yes)
- **Findings:** The workspace was selected from the shell scope picker. Searching for `palette-direct-loop` displayed the concrete loop row, and selecting it opened `/loops/palette-direct-loop?workspace=ws_8f9af3086c3bf927`. The loop detail rendered its name, contract, graph summary, and run state. A fresh direct navigation to the same route rendered the same detail.
- **Bugs filed/updated:** none
- **Scenarios settled:** `ET-palette-direct-entity-routes` → pass
- **Paper cuts:** none observed
- **Surprises:** The global palette correctly omitted the workspace loop until the workspace scope was selected; this confirms ownership is part of the search contract rather than a fallback filter.
- **Suggested next charter:** Walk the remaining concrete domain rows (jobs, triggers, bridges, marketplace, and vault) in a broader palette coverage pass.

## What Was Fixed

No runtime defect was found during the targeted walk. The implementation under test is the ENG-131 direct-route change.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Sol | J-command-os-from-palette scope selection | Workspace rows require an explicit workspace scope before they appear | dull | expected ownership behavior; no action |

## Runtime Errors Observed

- None in the walked flow. The lab initially hit the already-occupied default Vite port, so it was restarted on the manifest-safe isolated port `3031`; the first lab process was torn down before retrying.

## Human Verifications Needed

- [ ] None for this targeted scenario; the direct row and independent deep link both passed.

## Decisions for a Human

None.

## Learnings

- A concrete row must carry enough ownership context to select the correct workspace before opening its route; global search should not silently broaden into a catalog fallback.

## Final Status

- **Exit gate (focused automated evidence):** See `focused-verify.log` in the lab artifacts. The task explicitly forbids `make verify` and `make gate-full`; focused Turbo tests, lint, typecheck, codegen-check, and diff checks all passed.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1 targeted journey walked / 1 targeted journey in scope; adjacent broad palette scenarios remain untested and are not claimed by this focused run.
- **Verdict:** PASS — the walked workspace-owned entity opens its exact detail route from the palette and survives independent deep linking.
