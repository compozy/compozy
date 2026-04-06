---
status: pending
title: Journal writer upstream of fanout
type: refactor
complexity: high
dependencies:
  - task_01
---

# Task 03: Journal writer upstream of fanout

## Overview
Create the per-run Journal package that owns the `events.jsonl` file, assigns monotonic seq numbers in a single writer goroutine, batch-flushes to disk on size/interval thresholds, and forwards enriched events to the event bus. The journal sits UPSTREAM of fanout so live subscribers never see events that have not yet been persisted.

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
- MUST create package `internal/core/run/journal/`
- MUST open the `events.jsonl` file with `O_APPEND | O_CREATE | O_WRONLY`, mode 0644
- MUST wrap the file in `bufio.Writer` with 16KB buffer and `json.Encoder` on top
- MUST spawn ONE writer goroutine per journal instance that owns `seq` as a plain `uint64` local variable (no atomics)
- MUST expose `Submit(ctx context.Context, ev events.Event) error` with bounded inbox channel (default cap 1024) and timed backpressure
- MUST batch-flush on reaching 32 events OR 100ms interval, whichever first
- MUST force immediate `Flush()` + `f.Sync()` on terminal events (`run.completed`, `run.failed`, `run.cancelled`)
- MUST publish the enriched event (with seq assigned) to the provided `*events.Bus[events.Event]` AFTER successful append
- MUST expose `Close(ctx context.Context) error` that drains queued events, performs final flush+sync, and closes file
- MUST expose metrics via accessors: `EventsWritten()`, `DropsOnSubmit()`, `CurrentBufferDepth()`
- MUST tolerate a partial final line on reader side (injectable test seam for crash simulation)
</requirements>

## Subtasks
- [ ] 3.1 Create `internal/core/run/journal/journal.go` with `Journal` struct, constructor `Open`, and public API surface
- [ ] 3.2 Implement writer goroutine `writeLoop` with loop-local seq counter and batch flush policy
- [ ] 3.3 Implement `Submit(ctx, ev)` with bounded inbox channel and timed backpressure (same pattern as ADR-006)
- [ ] 3.4 Implement `Close(ctx)` that signals writer, waits for done, performs final flush+fsync
- [ ] 3.5 Add injectable `flushHook func()` test seam for crash-recovery tests
- [ ] 3.6 Add metric accessors and structured log statements with `run_id`, `seq`, `flush_latency_ms`
- [ ] 3.7 Write unit and integration tests covering ordering, batching, crash recovery, ctx cancel

## Implementation Details
See TechSpec "Core Interfaces" section for the Journal struct skeleton, and ADR-004 for the full writer-upstream-of-fanout rationale and batch flush thresholds. The journal is constructed by `plan.Prepare` (in task_08 wiring) alongside other run artifacts; the constructor takes the artifact path (`RunArtifacts.EventsPath`) and the event bus reference. Exec mode already writes `events.jsonl` via `exec_flow.go` — that custom writer will be replaced by the shared journal in task_05.

### Relevant Files
- `internal/core/model/model.go:155` — `RunArtifacts.EventsPath` field (target path for journal file)
- `internal/core/run/exec_flow.go` — existing exec-mode events.jsonl writer (precedent to replace later)
- `pkg/compozy/events/event.go` (task_01) — defines `events.Event` envelope the journal encodes
- `pkg/compozy/events/bus.go` (task_01) — defines `events.Bus[events.Event]` the journal publishes to

### Dependent Files
- `internal/core/plan/prepare.go` (task_05) — will construct the journal alongside RunArtifacts
- `internal/core/run/execution.go` (task_05) — will call `journal.Submit` at every event emission site
- `internal/core/run/logging.go` (task_05) — `HandleUpdate` will call `journal.Submit` for session.update events
- `pkg/compozy/runs/` (task_07) — reader library consumes the `events.jsonl` files this journal writes

### Related ADRs
- [ADR-004: Journal Upstream of Fanout with Single-Writer Per-Run Model](adrs/adr-004.md) — defines seq assignment, batch flush, terminal fsync, crash recovery semantics
- [ADR-003: Event Taxonomy with Schema Versioning](adrs/adr-003.md) — defines the event envelope the journal encodes

## Deliverables
- `internal/core/run/journal/journal.go` with `Journal` struct, `Open`, `Submit`, `Close` API
- Writer goroutine with batch flush policy and terminal fsync
- Injectable `flushHook` test seam for crash recovery tests
- Metric accessors (`EventsWritten`, `DropsOnSubmit`, `CurrentBufferDepth`)
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration test writing+reading events round-trip through the journal and reader library **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Sequential submits produce monotonic seq numbers 1, 2, 3... with no gaps
  - [ ] Batch flush fires when 32 events accumulate without interval reached
  - [ ] Batch flush fires at 100ms interval even with fewer than 32 events buffered
  - [ ] Terminal event (`run.completed` kind) forces immediate `f.Sync()` regardless of batch state
  - [ ] `Close(ctx)` drains all queued events before closing file
  - [ ] `Close(ctx)` with expired ctx returns error without hanging
  - [ ] `Submit(ctx)` returns ctx error when ctx cancelled before inbox accepts event
  - [ ] Full inbox causes `Submit` to block up to 5s via backpressure before timeout drop
  - [ ] `EventsWritten` counter matches total successful Submit calls
  - [ ] Concurrent Submit from 10 goroutines produces gap-free seq assignment (race detector passes)
  - [ ] `flushHook` test seam fires between buffer write and file sync for crash simulation
  - [ ] After simulated crash (flushHook aborts), re-reading `events.jsonl` via bufio.Scanner yields all pre-crash events and skips any partial final line
- Integration tests:
  - [ ] Round-trip: write 1000 events, reopen file, read all 1000 events with correct seq ordering
  - [ ] Published event on bus carries the same seq that was written to disk
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Zero golangci-lint issues
- Seq assignment is monotonic with no gaps under concurrent Submit load
- Crash recovery test demonstrates trailing partial line tolerance
- Terminal event fsync verified via fsnotify or file mtime check
