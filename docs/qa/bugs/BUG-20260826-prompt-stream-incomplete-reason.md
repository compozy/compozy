# BUG-20260826-prompt-stream-incomplete-reason: Remote prompt interruptions lost their stable incomplete-stream reason

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-cross-workspace-access — resume work after a remote prompt stream disconnects
- **Scenarios:** ET-workspace-access-prompt-outcomes
- **Found:** 2026-08-26 · **Report:** `docs/qa/reports/2026-08-26-agent-comms.md`

## Summary

When an ACP prompt ended before a terminal event, the daemon emitted only a transport-failure
summary. The SSE response and CLI therefore lost the stable incomplete-stream reason, so a remote
client received a generic gateway interruption and could not choose the intended reconnect path.

## Reproduction

1. Start profile-scoped work through a remote gateway prompt stream.
2. End the ACP stream after a non-terminal event and before a result.
3. Observe the CLI error classification while reconnecting the same profile.

**Expected:** The gateway interruption retains `prompt_stream_incomplete` so the client can safely
resume or reconnect without treating the work as an unrelated transport failure.
**Actual:** The public SSE event contained only text, and the CLI lost the domain classification.

## Fix

- **Root cause:** Session failures had no stable reason-code field, and prompt error SSE payloads did
  not project their structured failure record.
- **Production fix:** The store, API contract, SSE encoder, and CLI now carry and validate
  `prompt_stream_incomplete`; gateway translation preserves both the gateway interruption and the
  underlying prompt classification.
- **Regression:** `TestGatewayClientStreamsReportStableInterruption` owns the remote stream error
  translation invariant and verifies both classifications survive the public SSE payload.
- **Fix commit:** `60883a0e5`.

## Verification

- Focused post-rebase regression:
  `go test -race ./internal/cli ./internal/api/core -run 'TestGatewayClientStreamsReportStableInterruption|TestDeliverPromptEventStream' -count=1` — passed.
- The completed runtime E2E pass exercised the remote gateway disconnect/reconnect flow before the
  operator moved remaining full/E2E validation to exact-head PR CI.
