# BUG-20260827-session-create-first-message-regression: Start session duplicates the prompt composer

- **Status:** fixed, pending remediation commit
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Dora
- **Journey Step:** J-17 Launch a session and choose its next-prompt runtime
- **Scenarios:** MS-web-session-simple-advanced-launch; RT-063; ET-web-session-prompt-runtime-and-create-navigation; RT-new-session-fast-feedback
- **Found:** 2026-08-27 · **Report:** docs/qa/reports/2026-08-27-acp-runtime-catalog.md

## Summary

Start session rendered a First message textbox even though session creation and prompt submission are separate product actions. This duplicated the destination composer and contradicted the canonical launch contract.

## Reproduction

- **Charter:** CH-session-launch-composer-handoff · **Tour:** Feature Tour
- **Environment:** laptop / 1280px desktop viewport / wifi-fast / en-US; isolated local daemon

1. Open an agent detail page.
2. Click New session.
3. Inspect the Simple and Advanced launch modes.

**Expected:** The launch dialog contains only durable session identity and launch context. The destination composer owns the first prompt and its runtime.
**Actual:** The launch dialog also contains a First message textbox and queues its value after creation.

## Fix

- **Root cause:** the session-create draft had regained prompt state and the dialog rendered it as a launch field, merging two distinct lifecycles. The fallback path for a prompt already sent before agent selection was not modeled separately.
- **Fix:** remove prompt state and UI from the launch draft, keep ordinary creation prompt-free, and stage only an already-sent composer fallback prompt outside the draft until agent selection completes.
- **Fix commit:** pending; included in the single remediation commit
- **Regression tests:** `session-create-dialog.test.tsx`; `use-session-create-dialog.test.tsx`; `use-session-create.test.tsx`; `session-create-store.test.ts`

## Verification

- **Focused automated result:** 48 canonical Web tests pass; the Web typecheck passes.
- **Browser retest:** pass — Simple and Advanced contain no First message field; creation navigated to a separate composer, which accepted the first prompt once.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-create-no-first-message.png`; `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-create-advanced-no-first-message.png`; `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-first-prompt-grok45-fast-pass.png`.
