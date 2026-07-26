# L-001 — HTTP request lifetime ≠ prompt execution lifetime

**Class:** Concurrency / API
**Date discovered:** 2026-04-20 (prompt-stream-stall incident)
**Evidence sources:** 4 analyses concur

## Context

A user reported "prompt closes at first `tool_call`". Investigation surfaced FOUR compounding root causes inside one symptom:

1. HTTP request lifetime tied to prompt execution — when the AI client switched to streaming a tool, the writer cancelled the prompt context.
2. HTTP stream missing the AI SDK v6 `tool-input-available` framing — clients never saw the tool resolve.
3. Web "Stop" button used transport-level abort instead of an explicit cancel endpoint.
4. Inactive metadata repair classified `m.pending` startups as `crashed`, forcing re-init.

## Root cause

Tying execution lifetime to request lifetime. A `c.Request.Context()` is bound to the HTTP cycle and gets cancelled as soon as the client disconnects, the proxy times out, or the writer flushes a final chunk. Long-running work — prompts, network channel sends, automation jobs — must outlive the request that started it.

## Rule

> Any work that outlives an HTTP/UDS request MUST detach via `context.WithoutCancel(ctx)`. Never tie execution lifetime to request lifetime. Expose explicit cancel endpoints (e.g., `POST /api/workspaces/:workspace_id/sessions/:id/prompt/cancel`).

## Caveats

- **`context.WithoutCancel` does NOT preserve deadlines.** If the work needs a hard ceiling, re-attach a deadline with `context.WithDeadline(detached, ...)`.
- **The writer loop itself stays bound to the request context.** Detach the _prompt execution_, not the _response stream_. Client disconnect should stop streaming, not stop execution.
- **Inactive metadata repair must distinguish startup-pending from crashed.** Sessions in `m.pending` are still starting, not failed.

## Anti-pattern (don't do this)

- `time.Sleep` retries
- Client-side reconnect hacks
- Wrapping the original `c.Request.Context()` in another `WithCancel` and hoping order helps

## Source

- `.codex/plans/2026-04-20-prompt-stream-stall.md` — root-cause plan
- `.codex/ledger/2026-04-20-MEMORY-prompt-stream-stall.md` — incident timeline
- `../analysis/analysis_global_runs.md` lesson L1, `../analysis/analysis_codex_plans.md`, `../analysis/analysis_codex_ledger.md`, `../analysis/analysis_compozy_tasks.md`
