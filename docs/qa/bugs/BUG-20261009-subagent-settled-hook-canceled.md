# BUG-20261009-subagent-settled-hook-canceled: subagent.settled hooks never run

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** see scenario
- **Scenarios:** RT-subagent-delegate
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents-r2.md

## Summary

A config hook on `subagent.settled` never executed. Each of 3 settled subagents logged `hook.dispatch.async_failed … error: hook canceled: context canceled` plus a failed telemetry write.

## Reproduction

- **Environment:** isolated lab, stock build `sa-qa` @ 980d51fbe, real Claude (claude-agent-acp) parent and Codex children.

1. Declare an async config hook on `subagent.settled` (subprocess writing stdin to a file).
2. Delegate and let the subagent settle.

**Expected:** The hook receives the documented payload once per settled subagent.
**Actual:** No run. `hook canceled: context canceled`.

## Evidence

- `publishTerminal` (`internal/session/subagent_publish.go:101-104`) dispatches inside `s.launch` with `hookCtx, cancel := context.WithTimeout(s.ctx, …)` and `defer cancel()`. The async dispatch returns at once and the deferred cancel kills the hook run. Introduced with 2bea65ef1 (async settlement observers). Async hooks need a context that outlives the dispatch call.
