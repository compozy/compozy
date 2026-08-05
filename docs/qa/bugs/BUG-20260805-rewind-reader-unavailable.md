# BUG-20260805-rewind-reader-unavailable: Rewind failed when the session was read through the pooled recorder

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Théo
- **Journey Step:** J-rewind-conversation, confirm rewind
- **Scenarios:** RT-conversation-rewind
- **Found:** 2026-08-05 · **Report:** docs/qa/reports/2026-08-04-session-rewind.md
- **Origin:** CH-rewind-conversation isolated live-provider walk

## Summary

Confirming a rewind failed before changing the conversation because the pooled read-only recorder did not expose the rewind reader contract implemented by its underlying session database.

## Reproduction

1. Run `CH-rewind-conversation` with the data tour against an idle user session containing two durable prompts.
2. Choose the second prompt as the rewind target.
3. Confirm the rewind through `compozy session rewind` or the HTTP endpoint.

**Expected:** The same session restarts from the retained prefix.
**Actual:** The operation failed with `session: event recorder does not support conversation rewind`.

## Evidence

- Run report: `docs/qa/reports/2026-08-04-session-rewind.md`.
- Retest proof: `/Users/pedronauck/dev/qa-labs/compozy-session-rewind-20260805-024938-234761-lab/qa-artifacts/qa/verification-report.md`.
- Clean teardown: `/Users/pedronauck/dev/qa-labs/compozy-session-rewind-20260805-024938-234761-lab/qa-artifacts/qa/teardown.json`.

## Fix

- **Root cause:** `readOnlyPoolLease` forwarded ordinary event queries but did not implement `store.ConversationRewindReader`, so manager preflight rejected the leased recorder even though the underlying recorder supported rewind.
- **Correction:** The lease forwards rewind target, state, and receipt reads to the underlying recorder.
- **Fix commit:** `6c8deff`
- **Regression test:** `internal/store/sessiondb/session_db_integration_test.go` verifies the pooled read-only lease exposes the complete rewind reader contract against a real session database.

## Verification

- The same CLI command passed after rebuilding the daemon, then default and archived HTTP reads proved the active/audit split.
- A fresh Codex ACP session answered from the retained prefix, and the browser confirmation showed the correct side-effect boundary.
