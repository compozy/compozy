# BUG-20260804-stopped-prompt-restart-panic: A stopped session could not accept its first prompt after a daemon restart

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Théo
- **Journey Step:** J-13 follow, stop, and continue one durable ACP conversation
- **Scenarios:** RT-018
- **Found:** 2026-08-04 · **Report:** docs/qa/reports/2026-08-04-durable-acp-sessions.md

## Summary

Théo stopped a real Claude session, restarted the daemon, and submitted a normal prompt from the same Web composer. The request returned HTTP 500 because prompt admission dereferenced an in-memory session that correctly did not exist for the durable stopped record.

## Reproduction

- **Charter:** CH-stopped-session-prompt-continuity · **Tour:** Interrupt Tour
- **Environment:** isolated macOS lab / Web + HTTP + UDS / live Claude Fable 5 / en-US

1. Create a session, send one prompt, and wait for its answer.
2. Stop the session and confirm that its composer stays enabled.
3. Restart the daemon and reopen the same session permalink.
4. Submit a normal prompt.

**Expected:** the daemon resumes the durable session and dispatches the prompt under the same session id.
**Actual:** the first post-restart prompt returned HTTP 500 before dispatch.

## Evidence

- Isolated lab manifest: `/Users/pedronauck/dev/qa-labs/compozy-durable-acp-sessions-20260804-164200-701355-lab/qa-artifacts/qa/bootstrap-manifest.json`
- Failure note: `/Users/pedronauck/dev/qa-labs/compozy-durable-acp-sessions-20260804-164200-701355-lab/qa-artifacts/qa/evidence/pre-fix-restart-failure.md`

## Fix

- **Root cause:** prompt admission used `session.ID` as its serialized admission key even when the target resolved from durable stopped state and no active in-memory session existed.
- **Fix:** admission now uses the already validated target session id from the prepared prompt request.
- **Fix commit:** working tree; no commit requested
- **Regression test:** canonical `internal/session` `TestPromptErrorPaths`, case `Should resume a stopped persisted session after manager restart`.

## Verification

- **Retested:** 2026-08-04, same persona/journey · **Report:** `docs/qa/reports/2026-08-04-durable-acp-sessions.md`
- **Result:** Pass. The live provider recalled `FIRST-TURN-ALPHA` from the earlier turn under the same session id after the daemon restart.
