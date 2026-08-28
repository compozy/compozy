# QA Run Report — 2026-08-28 — cursor onboarding runtime defaults

- **Scope:** PR #498 follow-up: first-open Cursor catalog recovery and onboarding Reasoning/Fast defaults
- **Cadence tier:** targeted
- **Build:** `cfe4b82a3d8d` + working tree · **Environment:** isolated local daemon/Web at `http://127.0.0.1:53552`; real operator Cursor login
- **Started:** 2026-08-28T16:59:20Z · **Status:** QA passed; engineering gate pending

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | New User | laptop / wifi-fast / en-US | CH-cursor-onboarding-runtime-defaults |
| Sol | Accessibility-Reliant | desktop / wifi-fast / en-US | CH-cold-provider-catalog-recovery |
| Ada | Power User | desktop / wifi-fast / en-US | CH-provider-model-default-speed |

## Flows in Scope

- `J-19` — choose a truthful default runtime during onboarding (`../journeys/J-19-onboarding-default-model.md`)
- `J-17` — first runtime selector recovers missing allowed-provider rows (`../journeys/J-17-session-create-unified-selector.md`)
- `J-20` — curate model defaults through structured surfaces (`../journeys/J-20-catalog-curation-agent-surfaces.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-cold-provider-catalog-recovery | J-17 / RT-model-catalog-cold-open | Sol | Feature Tour | Pass | | |
| 2 | CH-cursor-onboarding-runtime-defaults | J-19 / RT-071; ET-web-runtime-selector-minimal-slider | Lea | Feature Tour | Pass | | |
| 3 | CH-provider-model-default-speed | J-20 / MS-054 | Ada | Feature Tour | Fixed | BUG-20260828-provider-model-fast-capability | pending |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Sol:** A first model-picker open on a fresh daemon showed Cursor and its models without catalog
  refresh. This was repeated in a second fresh lab after the remediation rebuild.
- **Lea:** Cursor Grok 4.6 exposed Reasoning and Fast; Extra high plus Fast advanced to Workspace and
  persisted on the logical model.
- **Ada:** Valid defaults round-tripped through CLI/UDS, HTTP, native tool, Settings, and effective
  runtime. An authored agent retained its own low/normal values over model defaults.

## What Was Fixed

- Cold Web reads now perform one deduplicated aggregate refresh and catalog reread when an allowed
  provider has no rows.
- Onboarding persists model-level `default_speed` alongside default reasoning.
- Live models with physical bindings now require an explicit Fast binding before Fast curation is
  accepted. The first QA pass found the missing negative gate; the rebuilt second pass verified it.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

- Expected negative-path diagnostic: `speed_rejected` for Cursor `gemini-3-flash` with Fast.

## Human Verifications Needed

None recorded yet.

## Decisions for a Human

None recorded yet.

## Learnings

- Cursor's aliases remain private transport bindings. The catalog should expose one logical model
  with configuration combinations, not duplicate every alias as a selectable model.
- A physical binding with no Fast dimension is a normal-only binding; unknown capability applies
  only when the catalog has no physical bindings at all.

## Final Status

- **Exit gate (scoped local suite):** pending
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 1 fixed · Friction 0 · Cosmetic 0
- **Coverage:** 3 / 3 journeys walked
- **Verdict:** QA passed after remediation; completion waits for the scoped engineering gate and PR CI.
