# BUG-20260801-web-prompt-missing-message-id: Session prompt cannot be sent from the Web thread

- **Status:** invalid
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Théo
- **Journey Step:** J-11, send the first authored prompt
- **Scenarios:** RT-session-prompt-idempotency; RT-session-message-reload
- **Found:** 2026-08-01 · **Report:** docs/qa/reports/2026-07-31-session-prompt-idempotency-post-review.md

## Summary

After setup and attachment to a live Codex session, Théo can type and submit a message in the Web thread, but the request is rejected with `message_id is required`. The optimistic row remains visible without any provider response, so the primary session journey cannot continue.

## Reproduction

- **Charter:** CH-session-prompt-identity · **Tour:** Interrupt Tour
- **Environment:** desktop / 1440×900 / wifi-fast / en-US

1. Complete runtime setup with Codex and select the isolated workspace.
2. Open and attach the live `identity_assistant` session.
3. Enter `Please reply with exactly ONE-ROW-OK.` in the session prompt and send it.

**Expected:** The request carries the Assistant UI message identity, starts one provider turn, and reconciles the optimistic row with the durable transcript.
**Actual:** The daemon rejects the public Web request with `message_id is required`; no provider turn starts.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-post-review-20260801-072745-435459-lab/qa-artifacts/qa/web-missing-message-id.png`
- `/Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-post-review-20260801-072745-435459-lab/qa-artifacts/qa/teardown.json` (`clean=true`)

## Fix

- **Root cause:** The isolated daemon served its versioned embedded release bundle instead of the current repository's `web/dist`. The current API correctly rejected that stale client's legacy request. Local production-parity QA must set the daemon's supported `COMPOZY_WEB_DIST_DIR` override to the freshly built repository bundle.
- **Fix commit:** not applicable — environment artifact
- **Regression test:** not applicable — the daemon's local static-bundle override is already covered by `internal/api/httpapi/static_test.go`; the session journey will be re-walked with the correct bundle.

## Verification

- **Retested:** 2026-08-01 environment diagnosis · **Report:** docs/qa/reports/2026-07-31-session-prompt-idempotency-post-review.md
- **Result:** Invalid product finding. The failed browser loaded the release asset module, while current transport/provider integration tests and `web/dist` contain the required identity promotion. A fresh isolated lab will run against the supported local bundle override.
- **Production-parity recheck:** A fresh daemon using `COMPOZY_WEB_DIST_DIR` admitted the same Web prompt, completed one real provider turn, and preserved exactly one authored row through cold reload. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-current-web-20260801-081126-391752-lab/qa-artifacts/qa/session-prompt-reloaded-centered.png`.
