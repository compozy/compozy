# BUG-20260828-unbound-session-start-latency: Logical session creation waits on runtime discovery

- **Status:** fixed, pending remediation commit
- **Impact (user-side):** Blocks-Completion
- **Severity:** Major · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-17 Start a durable session and reach its composer
- **Scenarios:** RT-new-session-fast-feedback
- **Found:** 2026-08-28 · **Report:** docs/qa/reports/2026-08-27-acp-runtime-catalog.md

## Summary

Creating an unbound logical session took about 1.4 seconds because acceptance refreshed and validated Cursor's live catalog before any ACP process existed. After the response, the Web also waited for a React render before starting the route transition.

## Fix

- **Root cause:** model validation ran during logical acceptance instead of the first runtime bind, and the session-create state machine delegated its next transition to a render-driven effect.
- **Fix:** defer live model validation only for `CreateAccepted`; synchronous creation and the first runtime bind still validate. Start navigation directly from the durable-acceptance transition while preserving workspace activation before routing.
- **Fix commit:** pending; included in the single remediation commit
- **Regression tests:** `TestCreateAcceptedLogicalRuntimeLifecycle`; `use-session-create-dialog.test.tsx` in their canonical session suites.

## Verification

- **Focused automated result:** the Go logical-acceptance tests pass with `-race`; the Web session-create suite passes 14/14 through Turborepo.
- **API result:** direct `POST /api/sessions` fell from about 1.4 seconds to 148 ms.
- **Browser result:** pass — feedback in 14.8 ms, navigation in 207.4 ms, composer ready in 392 ms, and exactly one successful create request.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-create-fast-feedback.json`; `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-create-fast-feedback-pass.png`.
