---
status: pending
title: TUI decoupling via bus-to-uiMsg adapter
type: refactor
complexity: medium
dependencies:
  - task_05
---

# Task 06: TUI decoupling via bus-to-uiMsg adapter

## Overview
Add a subscriber goroutine that attaches to the event bus, translates `events.Event` into the internal `uiMsg` types the Bubble Tea model already understands, and feeds them into the TUI's own channel. Then remove the now-unused `uiCh chan uiMsg` field and related wiring from the executor, leaving the TUI owning its own channel internally. The `tea.Model` logic (`ui_update.go`, `ui_view.go`) stays unchanged.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST add an adapter goroutine in `internal/core/run/ui_model.go` that subscribes to `*events.Bus[events.Event]`
- MUST translate each relevant `events.Event` kind into the corresponding `uiMsg` concrete type (jobStartedMsg, jobFinishedMsg, jobUpdateMsg, usageUpdateMsg, jobRetryMsg, jobFailureMsg, shutdownStatusMsg)
- MUST ignore event kinds the TUI does not render (task.*, review.*, provider.* etc.) without logging noise
- MUST own a TUI-private `events chan uiMsg` channel inside the UI session (not shared with executor)
- MUST preserve existing `tea.Model` (`ui_update.go`, `ui_view.go`) logic and message handling exactly
- MUST delete the `uiCh chan uiMsg` field from `jobExecutionContext` after adapter is verified
- MUST clean up any related wiring in `types.go` and `command_io.go` that referenced the old `uiCh`
- MUST handle unsubscribe on TUI shutdown cleanly via ctx cancel
- MUST respect event bus backpressure: adapter drops messages if the TUI channel is full (same non-blocking pattern)
</requirements>

## Subtasks
- [ ] 6.1 Add bus subscription to `ui_model.go`: subscribe in UI session constructor, return unsubscribe closure
- [ ] 6.2 Implement `translateEvent(events.Event) (uiMsg, bool)` mapping function (bool = whether TUI cares about this kind)
- [ ] 6.3 Launch adapter goroutine that reads from subscription channel, translates, forwards to TUI-owned `events chan uiMsg`
- [ ] 6.4 Ensure adapter goroutine exits on ctx cancel and closes TUI channel cleanly
- [ ] 6.5 Verify TUI still renders correctly end-to-end with bus subscriber in place
- [ ] 6.6 Delete `uiCh` field from `jobExecutionContext`, remove related wiring from `types.go`, `command_io.go`
- [ ] 6.7 Update executor tests and TUI tests for new subscription model

## Implementation Details
See TechSpec "Build Order" steps 8 and 9 for the two-phase TUI decoupling (adapter first, then legacy removal). The TUI's `tea.Model` implementation in `ui_update.go` and `ui_view.go` consumes `uiMsg` concrete types — those remain the TUI's internal protocol. The adapter is the ONLY consumer of `events.Event` in the TUI path. Event types not relevant to TUI (task.*, review.*, provider.*) are filtered by `translateEvent` returning `(nil, false)`.

### Relevant Files
- `internal/core/run/ui_model.go:14-148` — `uiModel`, `uiSession`, `newUIController` wiring
- `internal/core/run/ui_model.go:24` — `events <-chan uiMsg` field that currently receives from executor's `uiCh`
- `internal/core/run/ui_update.go` — `tea.Model.Update` handles `uiMsg` types (unchanged)
- `internal/core/run/ui_view.go` — rendering (unchanged)
- `internal/core/run/types.go:226` — `uiMsg` concrete type definitions (jobStartedMsg, jobFinishedMsg, etc.)
- `internal/core/run/command_io.go:95` — existing wiring that routed uiCh messages
- `internal/core/run/execution.go:53` — `jobExecutionContext.uiCh` field to remove after adapter is in place
- `pkg/compozy/events/bus.go` (task_01) — source of events
- `pkg/compozy/events/kinds/` (task_01) — event payloads that need translation

### Dependent Files
- `internal/core/run/execution_ui_test.go:60` — TUI message sequence tests must exercise bus → adapter → TUI path
- `internal/core/run/ui_update_test.go:96` — update tests still operate on `uiMsg` (unchanged contract)
- `internal/cli/*.go` (task_08) — CLI command constructors pass the shared bus into TUI session

### Related ADRs
- [ADR-002: Custom Event Bus with Bounded Per-Subscriber Backpressure](adrs/adr-002.md) — TUI adapter is one of the bus subscribers with its own drop counter

## Deliverables
- Bus subscription + translator + forwarding goroutine added to UI session in `ui_model.go`
- `translateEvent` function mapping relevant event kinds to TUI `uiMsg` types
- TUI-private `events chan uiMsg` channel (no longer shared with executor)
- `uiCh` field removed from `jobExecutionContext` + related wiring cleaned up
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration test demonstrating full executor → bus → TUI adapter → render pipeline **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] `translateEvent` maps `job.started` event to `jobStartedMsg{Index, Attempt, MaxAttempts}` with correct fields
  - [ ] `translateEvent` maps `session.update` event to `jobUpdateMsg{Index, Snapshot}` preserving session view snapshot
  - [ ] `translateEvent` maps `usage.updated` event to `usageUpdateMsg{Index, Usage}`
  - [ ] `translateEvent` maps `job.retry_scheduled` event to `jobRetryMsg{Index, Attempt, MaxAttempts, Reason}`
  - [ ] `translateEvent` maps `job.failed` event to `jobFailureMsg{Failure}`
  - [ ] `translateEvent` returns `(_, false)` for `task.file_updated`, `review.*`, `provider.*` events (TUI ignores them)
  - [ ] Adapter goroutine exits cleanly on ctx cancel and closes TUI channel exactly once
  - [ ] Adapter drops messages when TUI channel is full (non-blocking forward)
- Integration tests:
  - [ ] End-to-end: executor emits events via journal → bus → adapter → TUI renders expected frames (asserted via recorded TUI state)
  - [ ] TUI session close triggers unsubscribe from bus and goroutine exit without leak (`runtime.NumGoroutine` check)
  - [ ] Executor `jobExecutionContext` compiles and runs without `uiCh` field reference
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Zero golangci-lint issues
- `grep "uiCh" internal/core/run/execution.go` finds zero matches
- TUI continues rendering identically to pre-refactor for equivalent run inputs
- No goroutine leak after TUI session open/close cycle
- `go test -race` passes for run package
