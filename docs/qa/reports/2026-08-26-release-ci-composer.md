# QA Run Report — 2026-08-26 — Release CI composer

- **Scope:** Re-verify exact session composer text after dependency unification and imperative synchronization repair.
- **Cadence tier:** targeted
- **Build:** 11684c4fa + working tree · **Environment:** isolated real daemon and Web dev server; provider dispatch out of scope
- **Started:** 2026-08-26T05:21:19Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Builder | desktop / wifi-fast / en-US | CH-session-composer-text-entry |

## Flows in Scope

- `J-17` — launch a session and reach its durable next-prompt composer (`../journeys/J-17-session-create-unified-selector.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-composer-text-entry | J-17 / ET-web-session-composer-text-entry | Bruno | Feature Tour | Pass | — | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-session-composer-text-entry — Bruno

- **Ran:** 2026-08-26T05:24:44Z → 2026-08-26T05:31:00Z (box respected: yes)
- **Findings:** The composer preserved `  Revisão 😊 mantém   espaços ` through the runtime selector and a full reload. A clean-browser deep link rendered the same durable session and accepted `Second draft 日本語  with  spaces` exactly.
- **Bugs filed/updated:** No new bug. BUG-20260825-session-composer-window-fails remained verified; BUG-20260815-session-composer-draft-reload remained fixed and passed its canonical replay.
- **Scenarios settled:** ET-web-session-composer-text-entry → pass
- **Edge probes:** Blank launch, leading/repeated/trailing spaces, emoji, non-Latin text, runtime-selector open/close, full reload, and clean-browser deep link.
- **Paper cuts:** None in the targeted composer path.
- **Surprises:** Reopening the same URL inside the first driver process left that driver on a blank page without console errors; the one permitted clean-session retry loaded the public deep link normally, so no product finding was filed.
- **Suggested next charter:** CH-016 for the broader live queue and steer lifecycle.

## What Was Fixed

- Unified `@xstate/store` package identity and repaired draft hydration plus Lexical clearing order.

## Paper Cuts

None in the targeted composer path.

## Runtime Errors Observed

No product console errors. Vite emitted only its connection diagnostics and React DevTools notice.

## Experiential Lens Pass

- **Usability:** pass — the editor, runtime selector, and send affordance stayed understandable and responsive.
- **Accessibility:** pass for this quick check — the composer and runtime selector had stable accessible names and the editor remained keyboard-focusable.
- **Perceived performance:** pass on wifi-fast after initial dependency optimization; interactions updated immediately.
- **Compatibility:** limited to the manifest-selected Chromium driver; no layout or CSS change was in scope.
- **Error recoverability:** not exercised because the journey produced no product failure.
- **Production parity:** real current daemon and Web source, isolated runtime data, normal provider catalog, no provider dispatch; dev server rather than packaged desktop, wifi-fast only.
- **Scope note:** this targeted cycle contained one changed journey, so the lens pass re-walked J-17 once instead of inventing a second journey.

## Human Verifications Needed

None anticipated.

## Decisions for a Human

None.

## Learnings

- Draft persistence and editor context must be verified independently: a composer can mount correctly while still losing exact text at a synchronization boundary.

## Final Status

- **Exit gate:** `make gate` PASS with current codegen, Go lint/test, integration, Mage, and all-workspace Bun records.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 in-scope journey walked; no skips.
- **Verdict:** ready for the targeted composer surface; exact-head PR CI remains the delivery gate.
