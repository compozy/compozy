# BUG-20260730-session-provider-auth-availability: Signed-out provider models enabled in session composer

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-17, choose the runtime for the next prompt
- **Scenarios:** RT-067
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-30-session-runtime-selector.md

## Summary

The session composer scoped providers to the workspace correctly but discarded each provider's
global authentication state. Groq reported `missing_credential` from `/api/providers`, while its
model row remained enabled in the session selector.

## Reproduction

- **Charter:** CH-prompt-bound-runtime-transition · **Tour:** Feature Tour
- **Environment:** isolated current-source Web / desktop / en-US

1. Open an active session's Next prompt selector in a workspace that allows Groq.
2. Observe Groq's global auth state as `missing_credential`.
3. Inspect or navigate to the Groq model row.

**Expected:** The provider is labelled as needing sign-in and its model rows are disabled.
**Actual:** The provider and its model row were selectable.

## Evidence

- Retest: `docs/qa/evidence/2026-07-30-session-runtime-selector/13-needs-auth-disabled.png`
- Behavioral proof: `docs/qa/evidence/2026-07-30-session-runtime-selector/runtime-selector-proof.md`

## Fix

- **Root cause:** `useSessionPromptRuntime` mapped workspace providers to runtime options without
  merging the auth status from the global provider projection.
- **Fix:** Intersect the workspace allow-list with global provider authentication before mapping
  selector providers and catalog models; fail closed while auth availability is loading or errored.
- **Fix commit:** Working tree; this QA remediation is not committed separately.
- **Regression test:** `web/src/systems/session/hooks/__tests__/use-session-prompt-runtime.test.tsx`
  owns the workspace-scope plus global-auth availability invariant.

## Verification

- **Retested:** 2026-07-30 in the same isolated browser and daemon after hot reload.
- **Result:** Pass — the Groq rail entry announces `needs sign in`, its model row is disabled with
  reason `Sign in`, and Web lint, typecheck, and all 4,089 tests pass.

