---
status: pending
title: Events package (types + taxonomy + bus)
type: refactor
complexity: medium
dependencies: []
---

# Task 01: Events package (types + taxonomy + bus)

## Overview
Create the public `pkg/compozy/events/` package that defines the typed event envelope, event kinds, payload structs by domain, and a generic bounded-buffer pub/sub bus with per-subscriber backpressure. This package is the first public API surface of Compozy and the foundation every subsequent task depends on.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST create package at `pkg/compozy/events/` (public, semver-stable post-Phase B)
- MUST expose `Event` envelope with `SchemaVersion`, `RunID`, `Seq`, `Timestamp`, `Kind`, `Payload` fields
- MUST define `EventKind` typed string enum with 27 const values across 9 domains (run/job/session/tool_call/usage/task/review/provider/shutdown)
- MUST place per-domain payload structs under `pkg/compozy/events/kinds/` (one Go file per domain)
- MUST implement `Bus[T any]` with per-subscriber subscription struct carrying its own `dropped atomic.Uint64` counter
- MUST use snapshot-and-publish pattern (acquire RLock, copy subscribers, release, then send)
- MUST expose `Subscribe() (SubID, <-chan T, func())`, `Publish(ctx, evt)`, `Close(ctx)`, `DroppedFor(id)`, `SubscriberCount()`
- MUST be `ctx`-aware on Publish and Close so shutdown drains cleanly
- MUST rate-limit drop warnings to at most 1/sec per subscriber via CAS on `lastWarnedAt`
- MUST use only stdlib dependencies (no third-party libs introduced here)
</requirements>

## Subtasks
- [ ] 1.1 Define `Event` envelope type and `SchemaVersion = "1.0"` constant
- [ ] 1.2 Define `EventKind` string enum with all 27 const values per ADR-003 taxonomy (prompt domain removed, tool_call.completed removed — see ADR-003 "Removed Event Kinds")
- [ ] 1.3 Create `kinds/` subpackage with 9 Go files (one per domain, no prompt.go); define typed payload structs
- [ ] 1.4 Implement `Bus[T]` with snapshot publish, per-subscriber drop counter, rate-limited warn
- [ ] 1.5 Implement `Subscribe`/`Publish`/`Close`/`DroppedFor`/`SubscriberCount` public methods
- [ ] 1.6 Write unit tests covering fanout, backpressure, unsubscribe-during-publish, Close idempotency, goroutine leak

## Implementation Details
Create the package at `pkg/compozy/events/`. The `Event` envelope, bus contract, and subscription struct layout are defined in the TechSpec "Core Interfaces" section and ADR-002. The event taxonomy (27 kinds across 9 domains) with payload struct contracts is defined in ADR-003. The prompt domain was removed (no emission site) and `tool_call.completed` was removed (redundant with `tool_call.updated{state=completed}`) — see ADR-003 "Removed Event Kinds". No executor or journal integration in this task — those come in task_03 and task_05.

### Relevant Files
- `internal/core/model/content.go:9-25` — existing `ContentBlock` types referenced by `session.update` payload
- `internal/core/model/content.go:153` — existing `model.SessionUpdate` wrapped in `session.update` payload
- `internal/core/model/content.go:167-191` — existing `model.Usage` type referenced by `usage.*` payloads
- `go.mod` — no new dependencies needed for this task

### Dependent Files
- `internal/core/run/journal/` (task_03) — will import `pkg/compozy/events`
- `internal/core/kernel/` (task_04) — `KernelDeps` carries `*events.Bus[events.Event]`
- `internal/core/run/execution.go` (task_05) — will emit events via journal
- `pkg/compozy/runs/` (task_07) — will consume `events.Event` in its public API

### Related ADRs
- [ADR-002: Custom Event Bus with Bounded Per-Subscriber Backpressure](adrs/adr-002.md) — defines bus contract and snapshot pattern
- [ADR-003: Event Taxonomy with Schema Versioning](adrs/adr-003.md) — defines 27 event kind catalog across 9 domains

## Deliverables
- `pkg/compozy/events/event.go` with `Event`, `EventKind`, `SchemaVersion`, `SubID` types
- `pkg/compozy/events/bus.go` implementing `Bus[T]` (~150 LOC)
- `pkg/compozy/events/kinds/` subpackage with 9 files (run.go, job.go, session.go, tool_call.go, usage.go, task.go, review.go, provider.go, shutdown.go)
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration test asserting bus + 3 concurrent subscribers deliver in order **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] `Event` struct marshals to JSON with snake_case field names and round-trips unchanged
  - [ ] Each payload struct in `kinds/` round-trips through `json.Marshal`+`json.Unmarshal`
  - [ ] `Bus.Subscribe` returns unique monotonic `SubID` values across 1000 subscriptions
  - [ ] `Bus.Publish` delivers the same event to all active subscribers in fanout order
  - [ ] Slow subscriber (channel never read) causes `dropped` counter to increment while fast subscribers continue receiving
  - [ ] `Bus.DroppedFor(id)` returns accurate counter for each subscriber independently
  - [ ] Unsubscribe-during-publish: 100 subscribers with random unsubscribe calls and concurrent publish produces no panic and no goroutine leak
  - [ ] `Bus.Close(ctx)` closes all subscriber channels exactly once and is idempotent on second call
  - [ ] `Bus.Close(ctx)` with expired ctx returns error without hanging
  - [ ] Drop warning log fires at most once per second per subscriber via CAS on `lastWarnedAt`
  - [ ] `Publish` with already-cancelled ctx returns immediately without sending to any subscriber
- Integration tests:
  - [ ] 3 concurrent subscribers with different processing speeds all receive events in monotonic seq order; slow subscriber drops quantified while fast ones stay current
  - [ ] After 10000 Subscribe+unsub cycles, `runtime.NumGoroutine` delta is ≤ 2 (no leaks)
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `pkg/compozy/events/` and `pkg/compozy/events/kinds/` compile cleanly with `make verify`
- Public API matches TechSpec "Core Interfaces" section and ADR-002 contract
- Zero golangci-lint issues
- No goroutine leaks after 10000 Subscribe+unsub cycles
