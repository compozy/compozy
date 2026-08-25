# QA Run Report — 2026-08-24 — eng-145-session-copy

- **Scope:** ENG-145 Session message copy visibility and Clipboard API outcome feedback
- **Cadence tier:** targeted
- **Build:** working tree before review · **Environment:** local Storybook story with MSW transcript fixtures; production-parity daemon/auth not available in this worktree
- **Started:** 2026-08-24T21:20:00-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Rafa | Casual User | desktop / wifi-fast / en-US | CH-message-actions-copy-timestamp |

## Flows in Scope

- `J-14` — Read a finished transcript (`../journeys/J-14-read-a-finished-transcript.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-message-actions-copy-timestamp | J-14 / RT-053 | Rafa | Feature Tour | Blocked (needs human verify) | Production-parity daemon/auth unavailable; Storybook preview stalled before the transcript rendered | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-message-actions-copy-timestamp — Rafa

- **Ran:** 2026-08-24T21:20:00-03:00 → 2026-08-24T21:24:00-03:00 (box respected: yes)
- **Findings:**
  - The Storybook manager loaded and exposed the `Hover Toolbar` story entry. Opening the isolated preview did not reach a stable rendered transcript within the browser command timeout; the story uses MSW fixtures and cannot prove production parity.
- **Bugs filed/updated:** []
- **Scenarios settled:** RT-053 → blocked-verify
- **Paper cuts:** None observed before the preview stall.
- **Surprises:** The existing charter still describes hover-only and hidden streaming behavior; the implementation and scenario contract now intentionally supersede those stale instructions.
- **Suggested next charter:** Re-run this same charter from an isolated authenticated Session daemon, not Storybook.

## What Was Fixed

### ENG-145: Session copy action visibility and reliability

- **Symptom:** Session copy actions were hidden behind hover and clipboard failures were silent.
- **Root cause:** MessageActions applied opacity/pointer-event gating; the shared primitive had no user-facing outcome notification; action derivation hid streaming text.
- **Fix:** Persistent action row, settled-so-far streaming copy, guarded Clipboard API write, and success/failure toast feedback.
- **Regression test:** `web/src/components/assistant-ui/__tests__/session-thread.test.tsx`; `packages/ui/src/components/custom/__tests__/code-block.test.tsx` — focused suites pass.
- **Retested:** Automated component and primitive suites pass; browser parity walk remains pending the isolated daemon/auth environment.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Rafa | J-14 step 3 | "The action should not disappear while I move the pointer." | sharp | fixed in ENG-145 |

## Runtime Errors Observed

- None in the focused automated suites. Storybook/MSW parity is not an independent production read path.

## Human Verifications Needed

- [ ] From a production-parity authenticated Session, walk `CH-message-actions-copy-timestamp` on desktop: verify plain user text, user text with attachments, long clamped user text, settled assistant markdown, streaming assistant partial text, and tool-only turns. Confirm actions are visible without hover, copy exact text, success/failure feedback is announced, timestamp/rewind placement is unchanged, and the result survives reload. Record screenshots in `docs/qa/evidence/2026-08-24-eng-145-session-copy/` and update `RT-053` to `pass` with the report path. (row #1)

## Final Status

- **Exit gate (full automated suite):** Not run by instruction; focused Turbo tests and typecheck passed.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 1 · Friction 0 · Cosmetic 0
- **Coverage:** 0/1 production-parity journeys walked; Storybook manager smoke attempted, preview stall and parity gap disclosed.
- **Verdict:** ready with blocked items — merge the implementation only with the targeted production-parity Session walk completed by a human or isolated QA lab.
