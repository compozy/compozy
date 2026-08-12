# QA Run Report — 2026-08-12 — PR 351 review round 1

- **Scope:** Targeted validation of the Web session-rename rejection and recovery path added during CodeRabbit review round 1, with successful CLI/API rename as adjacent canaries.
- **Cadence tier:** targeted
- **Build:** `558654a7` plus review-round working tree · **Environment:** `http://127.0.0.1:55812`, isolated local daemon and Web dev server; local production code with no service mocks.
- **Started:** 2026-08-12T03:03:41Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Runtime Administrator | desktop / wifi-fast with one deliberate daemon interruption / en-US | CH-rename-session-parity |

## Flows in Scope

- `J-rename-session` — rename a session without changing its work (`../journeys/J-rename-session.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-rename-session-parity | J-rename-session / RT-session-rename-durable | Dora | Feature Tour | Fixed | BUG-20260812-rename-dialog-double-escape | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-rename-session-parity — Dora

- **Ran:** 2026-08-12T03:07:00Z → 2026-08-12T03:17:00Z (box respected: yes)
- **Findings:**
  - A temporary daemon interruption returned a visible inline 502 error while preserving the dialog and entered name; retry succeeded after the daemon returned.
  - The overflow menu initially consumed the first Escape behind the rename dialog (Friction); the same round fixed and re-walked it.
- **Bugs filed/updated:** BUG-20260812-rename-dialog-double-escape
- **Scenarios settled:** RT-session-rename-durable → pass
- **Paper cuts:** None remaining after the governed fix.
- **Surprises:** The disconnected shell correctly exposed its separate sync-retry state while the rename dialog kept its own recoverable error.
- **Suggested next charter:** Repeat error-recovery behavior for the row-level rename entry point during a future broader session-catalog pass.

## What Was Fixed

### BUG-20260812-rename-dialog-double-escape: Keyboard users needed two Escapes

- **Symptom:** The closing More actions menu consumed the first Escape behind the visible rename dialog.
- **Root cause:** Rename opened before Base UI completed the menu close transition.
- **Fix:** review-round-1 commit (this commit); the dialog now opens from `onOpenChangeComplete`.
- **Regression test:** `web/src/systems/session/hooks/__tests__/use-session-topbar-slot.test.tsx`
- **Retested:** J-rename-session in a fresh page load; one Escape closed the dialog.

## Paper Cuts

None remaining.

## Runtime Errors Observed

- Expected HTTP 502 and transcript fetch error during the deliberate daemon interruption; recovery and fresh reload were clean.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Menu-to-dialog transitions must complete in sequence; otherwise a visually hidden menu can keep keyboard ownership.

## Final Status

- **Exit gate (full automated suite):** `make gate` — PASS for the project-required intermediate review batch; evidence: `/Users/pedronauck/dev/qa-labs/compozy-pr-351-review-round-1-20260812-030348-308615-lab/qa-artifacts/final-make-verify.log`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 1 fixed · Cosmetic 0
- **Coverage:** 1/1 in-scope journey walked; successful rename, daemon-interrupted rejection/retry, blank and overlong validation, foreign-workspace denial, CLI/API parity, refresh durability, and keyboard cancellation covered. Managed-session rejection was unchanged by this review batch and was not repeated in this targeted walk.
- **Parity:** Latest local production code ran against the real daemon and Web dev server in Chrome with seeded demo data; no provider session, browser matrix, or production deployment artifact was required for this bounded local behavior pass.
- **Verdict:** ready — the review-round behavior and its QA-discovered keyboard friction are fixed, re-walked, and backed by public-surface evidence.
