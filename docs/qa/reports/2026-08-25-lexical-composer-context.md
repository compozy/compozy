# QA Run Report — 2026-08-25 — Lexical composer context

- **Scope:** Restore the Web session composer after duplicate Lexical packages split its React context.
- **Cadence tier:** targeted
- **Build:** a7d6ce49e + working tree · **Environment:** http://localhost:3000, real local daemon and Web dev server; provider dispatch out of scope
- **Started:** 2026-08-25T23:42:46Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Builder | desktop / wifi-fast / en-US | CH-session-composer-text-entry |

## Flows in Scope

- `J-17` — launch a session and reach its durable next-prompt composer (`../journeys/J-17-session-create-unified-selector.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-composer-text-entry | J-17 / ET-web-session-composer-text-entry | Bruno | Feature Tour | Pass | BUG-20260825-session-composer-window-fails | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-session-composer-text-entry — Bruno

- **Ran:** 2026-08-25T23:42:46Z → 2026-08-25T23:47:00Z (box respected: yes)
- **Findings:** The corrected session window mounted its Lexical editor and preserved `  Revisão 😊 mantém   espaços ` through the runtime-selector interaction and a fresh deep-link return. A second draft, `Second draft 日本語  with  spaces`, remained exact after remount.
- **Bugs filed/updated:** BUG-20260825-session-composer-window-fails → verified
- **Scenarios settled:** ET-web-session-composer-text-entry → pass
- **Paper cuts:** None in the targeted composer path.
- **Surprises:** The stale Vite dependency graph required one development-server restart before the corrected graph could be exercised.
- **Suggested next charter:** CH-session-inline-slash-commands for broader command-menu coverage.

## What Was Fixed

### BUG-20260825-session-composer-window-fails: Session window fails to render

- **Symptom:** Opening a session replaces the window with a missing Lexical composer context error.
- **Root cause:** The app and `@assistant-ui/react-lexical` loaded different Lexical minor versions, creating different React context instances.
- **Fix:** Working tree aligns Lexical at 0.49 and synchronizes imperative composer updates at their React boundary.
- **Regression test:** `web/src/components/assistant-ui/__tests__/session-thread.test.tsx` failed at editor mount before the fix and now exercises the real composer integration.
- **Retested:** J-17 and its exact-text composer canary passed in a fresh session and after deep-link return.

## Paper Cuts

None in the targeted composer path.

## Runtime Errors Observed

No session-window runtime errors were observed after the corrected dependency graph loaded. An unrelated pre-existing Agents window error (`Cannot read properties of undefined (reading 'kind')`) remained outside this run's scope.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A package-level Lexical mismatch can produce a valid-looking component tree whose provider and consumer still use different context identities.

## Final Status

- **Exit gate (full automated suite):** `make gate` failed in the repository-wide `js-all` lane before reaching this diff's tests: `@compozy/ui` reported 12 pre-existing React Doctor errors, including `only-export-components` and one `anchor-has-content`. The changed Web files pass targeted zero-warning Oxlint, and the canonical SessionThread suite passes 89/89.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 in-scope journey walked; no skips.
- **Verdict:** not ready — the Lexical fix passed targeted browser and automated verification, but the mandatory repository gate remains red on unrelated baseline errors.
