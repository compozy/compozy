# BUG-20260928-handoff-stream-error-says-retry: The prompt stream tells the operator to retry while the session prescribes handoff

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Théo
- **Journey Step:** J-15-operate-session-via-cli-api, rate-limited turn on a user session
- **Scenarios:** RT-provider-error-handoff
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b1.md

## Summary

`compozy session prompt <user-session> "rate limit this turn"` printed
`…; provider_failure_kind=rate_limited; next_action=retry; guidance=retry after the provider recovers` while the persisted
error event's `provider_error` and `failure.summary` said `next_action: handoff` with the `compozy session continue` guidance.
The operator reading the CLI got the wrong instruction.

## Root cause

`decorateHandoffAction` (`internal/session/derive_handoff.go`) rewrote `ProviderError` and `Failure.Summary` but not the
event's `Error` text, which carries the same recovery metadata and is what the prompt SSE `error` frame shows.

## Fix

The decoration rewrites the recovery metadata of `event.Error` and `Failure.Summary` through one helper. Regression:
`TestDecorateHandoffAction` (`internal/session/derive_test.go`) now asserts both texts (and that spawned sessions keep their action).

## Verification

Re-walk: rate limit and auth lapse on `handoff-agent` print `next_action=handoff; guidance=continue this session … compozy session continue <id> --agent <name>`
(`docs/qa/evidence/2026-09-28-session-continue-fork-b1/handoff-p2.txt`, `docs/qa/evidence/2026-09-28-session-continue-fork-b1/handoff-p3.txt`); a spawned child keeps `retry` / `inspect` (`docs/qa/evidence/2026-09-28-session-continue-fork-b1/spawned-p2.txt`, `docs/qa/evidence/2026-09-28-session-continue-fork-b1/spawned-p3.txt`).
