# BUG-20260928-derive-retry-creates-second-child: Retrying a failed Continue creates a second child

- **Status:** fixed
- **Impact (user-side):** Data-Loss
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-14 derive branch: Continue with a first message whose admission fails after the commit
- **Scenarios:** ET-web-session-continue; ET-web-session-fork-from-here
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md

## Summary

Bruno continues to an agent whose model the provider refuses, with a first message. The daemon commits the child, then the first message's admission fails and the POST answers 422 with the bind error. The dialog shows the error. Bruno presses Continue again and a second, separate child is created. The catalog now shows two identical children, one of them orphaned.

## Root cause

`useSessionDeriveIdempotencyKey` rotated the key on any 4xx on the assumption that "a refusal created nothing". A 422 can follow the commit (spec §806: a retry with the same derive key completes an undispatched admission). Refusals before the commit record no receipt, so keeping the key is safe for them too (verified: `agent_not_found` then a different agent with the same key → 201).

## Fix

The key is now fixed for the dialog's lifetime (`web/src/systems/session/hooks/use-session-derive.ts`; `settleFailure` removed from the continue/fork dialog hooks). Regression: `session-continue-dialog.test.tsx` "Should retry with the same idempotency key after a failure". Re-walked: two submits after a post-commit 422 produced one child (`sess-3540e32ceffb49f3`).

## Evidence

docs/qa/evidence/2026-09-28-session-continue-fork-b2/postcommit-422-error.png; docs/qa/evidence/2026-09-28-session-continue-fork-b2/journey-log.jsonl (`continue_postcommit_failure_retry`).
