# Issue 003: ACP session updates can race ahead of session registration

## Summary

During real `fix-reviews` execution with `--ide codex`, the ACP client could receive a `session/update` notification before `CreateSession` finished storing the new session locally. That produced a noisy runtime error even though the agent run itself kept progressing.

## Reproduction

```bash
/tmp/compozy-qa-compozy fix-reviews \
  --name demo \
  --round 1 \
  --ide codex \
  --tui=false \
  --timeout 5m
```

Observed before the fix:

- the real end-to-end run emitted `received update for unknown session "<id>"` from the ACP client
- the warning appeared immediately after session creation, during early Codex updates

## Expected

Early ACP updates for a session being created should be accepted and delivered once the session is registered locally. Only truly unknown sessions should raise an error.

## Root cause

`internal/core/agent/client.go` treated every `SessionUpdate` as invalid unless the session already existed in `c.sessions`. In the real ACP flow, `NewSession` can emit updates before returning the `sessionId`, so the client observed a legitimate update during a short registration gap.

## Fix

Track pending session creations, buffer updates that arrive during that window, and drain them immediately after storing the session. Keep the explicit error for updates targeting sessions with no pending creation.
