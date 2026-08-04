# BUG-20260803-prompt-cancel-warning-noise: Successful prompt replacement showed a cancellation warning

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Charter / Tour:** CH-session-calm-transcript · Feature Tour
- **Journey Step:** J-14 supervise a live transcript, active-turn replacement
- **Scenarios:** ET-web-session-transcript-calm-grammar
- **Found:** 2026-08-03 · **Report:** docs/qa/reports/2026-08-03-session-input-coderabbit.md

## Summary

After a successful steer or interrupt, Théo still saw “Prompt canceled by operator.” as a warning row even though the replacement prompt had already been accepted and answered.

## Reproduction

- **Charter:** CH-session-calm-transcript · **Tour:** Feature Tour
- **Environment:** isolated current-source daemon and Web dev server / live Codex provider / desktop Chrome / local network / pt-BR

1. Open a live session with an active terminal command.
2. Steer or interrupt the active turn with a replacement prompt.
3. Wait for the replacement response and read the settled transcript.

**Expected:** The transcript shows the replacement conversation without normal lifecycle warnings.
**Actual:** A warning row showed the internal `transcript_marker.prompt_cancel` acknowledgement.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-session-input-coderabbit-20260804-003939-559639-lab/qa-artifacts/qa/evidence/prompt-cancel-warning-before.txt`
- `docs/qa/evidence/2026-08-03-session-input-coderabbit/01-working-queued-calm.png` — fresh load after the fix shows the same settled transcript without the warning row.

## Fix

- **Root cause:** The Web lifecycle filter recognized `prompt_canceled` and `prompt_cancelled`, but not the daemon’s canonical singular marker kind, `prompt_cancel`.
- **Fix commit:** PR #304 CodeRabbit remediation batch (this commit)
- **Regression test:** `web/src/systems/session/components/__tests__/runtime-activity-notice.test.tsx`

## Verification

- **Retested:** 2026-08-03, same persona/journey · **Report:** docs/qa/reports/2026-08-03-session-input-coderabbit.md
- **Result:** A fresh page load kept queue/steer/interrupt lifecycle history durable while rendering no cancellation warning; the focused canonical suite passed 17/17 tests.
