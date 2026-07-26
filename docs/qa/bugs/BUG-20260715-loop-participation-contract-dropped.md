# BUG-20260715-loop-participation-contract-dropped: Loop public definition drops Network participation

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Nia
- **Journey Step:** J-network-local-default, inspect and start a Loop
- **Scenarios:** NB-execution-participation-defaults; NB-participation-controls-serialize
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

Loop definitions accepted typed Network participation internally, but the public definition document discarded it. Generated clients and Web therefore could not hydrate or preserve a definition-level participation choice.

## Reproduction

- **Charter:** CH-network-local-default · **Tour:** Feature Tour
- **Environment:** public HTTP/UDS contract plus daemon-served Web

1. Read a Loop definition that carries typed participation.
2. Inspect the public `LoopDefinitionDocument` or generated TypeScript client shape.

**Expected:** The document exposes the typed participation request.
**Actual:** The field disappeared at public conversion.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-network-local-default.md`
- Contract roundtrip and generated OpenAPI/TypeScript outputs.

## Fix

- **Root cause:** `LoopDefinitionDocument` had no `network_participation` member even though the owning runtime definition did.
- **Fix commit:** pending final whole-diff commit.
- **Regression tests:** the canonical API contract roundtrip suite requires participation to survive definition serialization; generated-contract drift is enforced by `make codegen-check`.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** public Go, OpenAPI, generated TypeScript, and Web hydration agree; Local remains the omission default and no legacy field reappears.
