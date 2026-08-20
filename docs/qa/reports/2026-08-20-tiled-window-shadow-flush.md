# QA Run Report — 2026-08-20 — tiled-window-shadow-flush

- **Scope:** Zoomed/tiled window chrome no longer casts `--shadow-window` into the work-area gutters (clipped smear vs dock).
- **Cadence tier:** targeted (single scenario walk)
- **Build:** working tree on `ui-normies` over `origin/main`.
- **Environment:** live `make web-dev` at `http://127.0.0.1:3000` with local daemon `:2123`. Presentation chrome only; no data-path change.
- **Started:** 2026-08-20T22:21:00Z · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | operator | desktop / local / en | targeted walk of ET-web-tiled-window-shadow-flush |

## Flows in Scope

- `J-operate-desktop-shell` — operate windows through the desktop shell (`../journeys/J-operate-desktop-shell.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | — | J-operate-desktop-shell / ET-web-tiled-window-shadow-flush | Bruno | Visual | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Targeted walk — Bruno

- **Ran:** pending
- **Findings:**
- **Bugs filed/updated:** none yet
- **Scenarios settled:** pending
- **Paper cuts:**
- **Surprises:**
- **Suggested next charter:** full shell cycle already owns zoom gesture mechanics via ET-window-manager-layout-gestures

## What Was Fixed

### Clipped zoomed-window shadow
- **Symptom:** Zoomed Home left a hard-edged reddish smear in the bottom-left gutter against the dock.
- **Root cause:** Tiled/zoomed chrome still painted `--shadow-window` (~90px blur) into 8–10px gaps; desk overflow and `contain: strict` clipped it.
- **Fix:** Cast elevation only on floating frames (`OsWindowChrome` `kind="tiled"` → `shadow-none`).
- **Regression test:** `web/src/systems/os/components/__tests__/os-window-frame.test.tsx`, `web/src/systems/os/components/__tests__/os-window.test.tsx`
- **Retested:** pending this walk

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| | | | | |

## Runtime Errors Observed

- none yet

## Human Verifications Needed

- none

## Decisions for a Human

- none

## Learnings

- pending

## Final Status

- **Exit gate (full automated suite):** pending scoped `make bun-lint` / `make bun-typecheck` plus focused vitest
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** one scenario in scope
- **Verdict:** in-progress
