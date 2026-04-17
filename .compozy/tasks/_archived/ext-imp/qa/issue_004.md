# Issue 004: ACP helper test publishes connection without synchronization

## Summary

The new ACP regression test helper could start handling RPC requests before its `conn` field was safely published. Under `go test -race`, that showed up as a real data race in the helper subprocess and caused the package to fail with `exit status 66`.

## Reproduction

```bash
go test ./internal/core/agent -race \
  -run TestClientCreateSessionBuffersUpdatesArrivingBeforeNewSessionReturns \
  -count=1
```

Observed before the fix:

- the test failed in `client.Close()` with `wait for ACP agent process: exit status 66`
- the helper subprocess race report pointed to concurrent access to `helperAgent.conn`

## Expected

The ACP helper used by regression tests should publish its connection handle with proper synchronization before request handlers try to emit session updates.

## Root cause

`internal/core/agent/client_test.go` assigned `agent.conn` after `acp.NewAgentSideConnection(...)` had already started background receive handling. `helperAgent.emitUpdates` could read `a.conn` concurrently with that write.

## Fix

Gate access to the helper connection behind a readiness channel so request handlers wait until the connection has been published safely before emitting ACP session updates.
