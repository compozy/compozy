---
status: pending
title: Reader library over .compozy/runs/
type: backend
complexity: high
dependencies:
  - task_01
---

# Task 07: Reader library over .compozy/runs/

## Overview
Create the public `pkg/compozy/runs/` package with typed operations for enumerating runs, opening individual runs, replaying historical events, live-tailing `events.jsonl`, and watching the workspace for new runs. This becomes the first public consumption API of Compozy, consumed by the Phase B daemon, future SDKs, debugging tools, and CI integrations.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST create public package `pkg/compozy/runs/`
- MUST add direct dependencies `github.com/nxadm/tail` and `github.com/fsnotify/fsnotify` via `go get`
- MUST expose `RunSummary` struct with `RunID`, `Status`, `Mode`, `IDE`, `Model`, `WorkspaceRoot`, `StartedAt`, `EndedAt *time.Time`, `ArtifactsDir`
- MUST expose `ListOptions` for filtering by Status, Mode, Since, Until, Limit
- MUST expose `List(workspaceRoot, opts) ([]RunSummary, error)` scanning `.compozy/runs/` via `os.ReadDir` and parsing each `run.json`
- MUST expose `Open(workspaceRoot, runID) (*Run, error)` returning a handle carrying the loaded summary and artifact paths
- MUST expose `(*Run).Summary() RunSummary`
- MUST expose `(*Run).Replay(fromSeq) iter.Seq2[events.Event, error]` yielding events from fromSeq to EOF via `bufio.Scanner`
- MUST expose `(*Run).Tail(ctx, fromSeq) (<-chan events.Event, <-chan error)` first replaying from fromSeq then live-tailing via `nxadm/tail`
- MUST expose `RunEvent` with `Kind RunEventKind`, `RunID`, `Summary *RunSummary` and three const kinds: `RunEventCreated`, `RunEventStatusChanged`, `RunEventRemoved`
- MUST expose `WatchWorkspace(ctx, workspaceRoot) (<-chan RunEvent, <-chan error)` using `fsnotify` on the runs directory
- MUST tolerate a truncated or partial final line in `events.jsonl` during Replay (skip with error indication)
- MUST close returned channels when ctx is cancelled or a terminal condition is reached
- MUST be semver-stable at v0.x until Phase B daemon validates the API
</requirements>

## Subtasks
- [ ] 7.1 Add `nxadm/tail` and `fsnotify` dependencies via `go get`; run `go mod tidy`
- [ ] 7.2 Create `pkg/compozy/runs/summary.go` with `RunSummary`, `ListOptions` types
- [ ] 7.3 Implement `List` with directory scan + `run.json` parsing + filter application
- [ ] 7.4 Implement `Open` + `Run` handle + `Summary()` accessor
- [ ] 7.5 Implement `Replay` using `bufio.Scanner` over `events.jsonl`, yielding `iter.Seq2`, tolerating partial last line
- [ ] 7.6 Implement `Tail` using `nxadm/tail` with replay-from-cursor then live-follow handoff
- [ ] 7.7 Implement `RunEvent`, `RunEventKind` constants, `WatchWorkspace` using `fsnotify.Watcher`
- [ ] 7.8 Write unit and integration tests for all five public operations

## Implementation Details
See TechSpec "Core Interfaces" — `RunSummary`, `Run`, and on-disk layout — and ADR-005 for the full decision on the public API contract, dependencies (`nxadm/tail`, `fsnotify`), and versioning posture. The package depends on `pkg/compozy/events/` from task_01 for the `events.Event` type exposed in `Replay` and `Tail` return types. Directory layout under `.compozy/runs/<run-id>/` is already established by exec-command task_02 (run.json, events.jsonl, jobs/, etc.).

### Relevant Files
- `internal/core/model/model.go:155` — existing `RunArtifacts` struct defining on-disk paths to parallel
- `internal/core/run/exec_flow.go` — existing code that writes run.json and events.jsonl (contract to read)
- `pkg/compozy/events/event.go` (task_01) — `events.Event` type used in Replay/Tail return types
- `go.mod` — needs nxadm/tail, fsnotify additions

### Dependent Files
- Phase B daemon (future) — will call List/Open/Tail/WatchWorkspace to expose runs over RPC
- Phase D SDKs (future) — will re-export these types
- Debugging tools, CI integrations — downstream consumers after Phase A ships

### Related ADRs
- [ADR-005: Reader Library over `.compozy/runs/` using nxadm/tail and fsnotify](adrs/adr-005.md) — defines API surface, library choices, versioning, Tail/WatchWorkspace contract
- [ADR-003: Event Taxonomy with Schema Versioning](adrs/adr-003.md) — schema_version field the library must respect

## Deliverables
- `pkg/compozy/runs/` package with List, Open, Replay, Tail, WatchWorkspace and all required types
- Two new dependencies added to go.mod: nxadm/tail, fsnotify
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests including WatchWorkspace subdirectory detection and schema version compatibility **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] `List` returns all runs when no filters applied, sorted by StartedAt descending
  - [ ] `List` filters by `Status=["failed"]` excluding completed runs
  - [ ] `List` filters by `Mode=["exec"]` excluding batch runs
  - [ ] `List` applies `Since` and `Until` time bounds correctly
  - [ ] `List` applies `Limit` capping result count
  - [ ] `List` skips subdirectories missing `run.json` with warning log instead of erroring
  - [ ] `Open` loads run.json and returns `*Run` with populated `Summary`
  - [ ] `Open` returns descriptive error for missing run ID
  - [ ] `Replay` yields all events from `fromSeq=0` matching the sequence written
  - [ ] `Replay` starting from `fromSeq=N` skips events with seq < N
  - [ ] `Replay` tolerates truncated last line: yields all complete events, emits error for truncated line, iteration terminates
  - [ ] `Tail` replays historical events from fromSeq before delivering live events
  - [ ] `Tail` cancels cleanly when ctx is cancelled; both channels close
  - [ ] `WatchWorkspace` emits `RunEvent{Kind: RunEventCreated}` when a new run directory appears
  - [ ] `WatchWorkspace` emits `RunEvent{Kind: RunEventStatusChanged}` when run.json status field is rewritten
  - [ ] `WatchWorkspace` emits `RunEvent{Kind: RunEventRemoved}` when a run directory is removed
  - [ ] `WatchWorkspace` cancels cleanly on ctx cancel
- Integration tests:
  - [ ] Round-trip: journal writes 100 events to events.jsonl; Replay yields identical sequence with matching seq numbers
  - [ ] Live-tail: start Tail on existing file, append 50 new events; all 50 delivered via channel in order
  - [ ] Schema version compat: events with `schema_version="1.0"` + unknown additive field parse successfully; events with `schema_version="99.0"` yield typed error
  - [ ] WatchWorkspace integration: external process creates `.compozy/runs/new-run/run.json`; WatchWorkspace delivers RunEventCreated within 1s
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Zero golangci-lint issues
- `nxadm/tail` + `fsnotify` added to go.mod with minimal transitive impact
- Public API signatures match ADR-005 "frozen contract" section exactly
- Package compiles as public (no `internal/` violations)
- WatchWorkspace detects new-run-created within 1s on Linux/macOS
