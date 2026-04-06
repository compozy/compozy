---
status: completed
title: ACP ingress buffer strategy
type: refactor
complexity: medium
dependencies: []
---

# Task 02: ACP ingress buffer strategy

## Overview
Grow the ACP session update channel from 128 to 1024 entries, replace the silent `select-default` drop in `sessionImpl.publish` with a 5-second timed-backpressure path, and add explicit per-session drop counters with structured warn logging. This closes the most upstream lossy path before the new journal (task_03) can claim source-of-truth status.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<note>
**Greenfield approach**: This project is in alpha (`v0.x`). Prioritize clean architecture and code quality over backwards compatibility. Do not add compatibility shims, legacy adapters, or deprecation wrappers — replace existing code directly. Breaking changes are expected and acceptable.
</note>

<requirements>
- MUST grow `sessionImpl.updates` channel capacity from 128 to 1024 in `internal/core/agent/session.go`
- MUST replace existing `select { case s.updates <- update: default: }` silent-drop pattern with timed backpressure
- MUST block publisher for up to 5 seconds on full buffer via `time.NewTimer` + select with ctx-aware cases
- MUST increment a per-session `droppedUpdates atomic.Uint64` counter on timeout
- MUST increment a per-session `slowPublishes atomic.Uint64` counter when backpressure succeeds after waiting
- MUST emit a rate-limited `slog.Warn` on drop with `session_id`, `buffer_cap`, `dropped_total`, `kind` attributes
- MUST change `publish()` signature to accept `context.Context` and honor ctx cancellation
- MUST expose `Session.SlowPublishes()` and `Session.DroppedUpdates()` accessor methods
- MUST update all internal callers of `publish()` to pass `context.Context`
- MUST preserve existing `sessionImpl` fields (finished, suppressUpdates, updatesSeen, etc.) and behavior
</requirements>

## Subtasks
- [x] 2.1 Change buffer capacity from 128 to 1024 in `sessionImpl` constructor
- [x] 2.2 Add `droppedUpdates` and `slowPublishes` `atomic.Uint64` fields to `sessionImpl`
- [x] 2.3 Refactor `publish()` to take `ctx context.Context` parameter with fast/slow/timeout/cancel paths
- [x] 2.4 Add rate-limited drop logging via slog with structured attributes
- [x] 2.5 Expose accessor methods `SlowPublishes()` and `DroppedUpdates()` on Session interface or concrete type
- [x] 2.6 Update every caller of `publish()` in the agent package to pass ctx
- [x] 2.7 Write unit tests for fast path, backpressure path, timeout path, ctx cancel path

## Implementation Details
The refactor is localized to `internal/core/agent/session.go` and its callers within the same package. See TechSpec "Core Interfaces" note on ACP ingress and ADR-006 for the complete backpressure semantics (buffer size rationale, timeout duration, drop-vs-backpressure policy). The kernel, event bus, and journal do not depend on this internal change — they consume events from `Session.Updates()` with unchanged semantics.

### Relevant Files
- `internal/core/agent/session.go:37-60` — `sessionImpl` struct declaration including `updates chan model.SessionUpdate` at current 128 capacity
- `internal/core/agent/session.go:102-122` — current `publish()` method with select-default silent drop
- `internal/core/agent/session.go:17-26` — `Session` interface contract
- `internal/core/agent/client.go` — sites that construct `sessionImpl` via constructor

### Dependent Files
- ACP callers inside `internal/core/agent/` that invoke `publish()` must pass ctx
- `internal/core/run/execution.go` — consumes `Session.Updates()` channel (behavior unchanged, just capacity grew)
- `internal/core/run/logging.go:HandleUpdate` — reads `Session.Updates()` (behavior unchanged)

### Related ADRs
- [ADR-006: ACP Ingress Buffer with Grown Capacity and Timed Backpressure](adrs/adr-006.md) — specifies 1024 capacity, 5s timeout, drop logging policy

## Deliverables
- Updated `sessionImpl.updates` channel at capacity 1024
- Refactored `publish(ctx, update)` method with four code paths (fast/backpressure/timeout/ctx-done)
- New `droppedUpdates` and `slowPublishes` per-session counters with accessor methods
- Rate-limited structured warn logs on drops
- Unit tests with 80%+ coverage **(REQUIRED)**
- Regression tests proving existing Session interface callers still work unchanged **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Fast path: buffer under capacity, publish returns immediately without incrementing counters
  - [x] Backpressure success: slow consumer holds channel full, publish blocks briefly, then drains within 5s; `slowPublishes` counter increments once
  - [x] Timeout path: consumer permanently stalled, publish blocks full 5s then drops; `droppedUpdates` counter increments and warn log fires with correct attributes
  - [x] Context cancel during backpressure: `publish(ctx)` returns cleanly when ctx is cancelled before timeout; neither counter increments
  - [x] Drop log rate limiting: 100 sequential drops within 1 second produce at most 1 warn log
  - [x] `SlowPublishes()` and `DroppedUpdates()` accessors return accurate atomic counter values
  - [x] After `finished=true`, publish returns early without touching channel (preserves existing behavior)
  - [x] After `suppressUpdates=true`, publish returns early without touching channel (preserves existing behavior)
- Integration tests:
  - [x] Existing session lifecycle tests in `internal/core/agent/session_test.go` still pass unchanged
  - [x] Live ACP session test: 1000 updates at 100/sec through full pipeline, zero drops with 1024 buffer
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Zero golangci-lint issues
- Existing ACP integration tests in `internal/core/agent/` pass without modification to assertions beyond ctx parameter
- Drop counter and slow counter visible via accessor methods for observability
- `publish()` never blocks indefinitely on ctx cancellation
