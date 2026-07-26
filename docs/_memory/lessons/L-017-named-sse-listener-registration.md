# L-017 — Named SSE events require explicit `addEventListener` registration

**Class:** Frontend / SSE integration
**Date discovered:** 2026-05-05 (orch-improvs task 26 audit)
**Evidence sources:** Local Codex audit on the delegated `useTaskStream` implementation

## Context

`useTaskStream` was added in `web/src/systems/tasks/hooks/use-task-stream.ts` to consume the
cursor-seeded `/api/tasks/{id}/stream` SSE feed. The first delegated draft assigned only
`source.onmessage = handleMessage` and shipped with focused tests that only exercised an unnamed
`message` frame. `make verify` passed.

The audit caught that the hook would silently drop every real task event in production: AGH writes
named events through `internal/api/core/sse.go:54-60`, where `WriteTaskStreamEvent` sets
`SSEMessage.Name = event.Type`. Each frame goes out as `event: task.run_started` (or similar).
`EventSource` only routes named SSE events to listeners registered through
`addEventListener("<type>", handler)`; they never reach `onmessage`. The hook would have looked
healthy, kept the connection open, and never invalidated a single TanStack Query cache key.

## Root cause

The Web Platform `EventSource` interface treats `onmessage` as a fallback for unnamed `message`
frames only. Named SSE events — anything where the producer sets a `Name`/`event:` line — must be
bound by event type. Tests that mock a single anonymous `MessageEvent` and assert `onmessage` was
called are not evidence that the hook handles the producer's real frames.

## Rule

> When a producer emits named SSE events (`event: <name>`), consumers MUST register
> `addEventListener` for each canonical event type. `onmessage` covers only unnamed `message` frames
> and is not a substitute. Tests must assert listener registration AND payload routing through the
> named handler, not only an `onmessage` parse path.

## Operationalization

- Keep the canonical event-type list co-located with the consumer (see
  `TASK_STREAM_EVENT_TYPES` at `web/src/systems/tasks/hooks/use-task-stream.ts:33-72`) and align it
  with the producer's emit list (`internal/task/manager.go`, review/notification surfaces in
  `internal/api`). When the producer adds a new event type, the consumer's array updates in the
  same change.
- Register `addEventListener("<type>", listener)` for every canonical type and call
  `removeEventListener` on cleanup when the runtime exposes it. Keep `onmessage` as a defensive
  unnamed-frame fallback, not as the primary path.
- Test stubs MUST mimic both `addEventListener`/`removeEventListener` and `onmessage`. Assert that
  named-handler invocations route through the registered listener, not through `onmessage`.
- For other named-SSE feeds in AGH (network, automation, observability), repeat this pattern
  before shipping a TypeScript/web consumer.

## Anti-pattern

- Assigning only `source.onmessage = handler` for a named-event feed.
- Asserting `addEventListener` is called without asserting payload routing through the named
  handler.
- Treating `onmessage` as the contract because "the test passes".

## Source

- `web/src/systems/tasks/hooks/use-task-stream.ts` (named listener registration loop)
- `web/src/systems/tasks/hooks/use-task-stream.test.tsx` (named handler routing + cleanup tests)
- `internal/api/core/sse.go:54-60` (`WriteTaskStreamEvent` sets `Name = event.Type`)
- `.compozy/tasks/orch-improvs/memory/task_26.md` (Errors / Corrections — named-SSE audit)
