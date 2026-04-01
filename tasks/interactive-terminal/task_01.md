## status: pending

<task_context>
  <domain>Runtime, Infrastructure</domain>
  <type>Feature Implementation</type>
  <scope>Full</scope>
  <complexity>medium</complexity>
  <dependencies>none</dependencies>
</task_context>

# Task 1: Terminal Wrapper (PTY + VT Emulator)

## Overview
Create the foundational `Terminal` component that wraps a pseudo-terminal (xpty) and virtual terminal emulator (vt.SafeEmulator) into a single, thread-safe abstraction. This component is the core building block that all other tasks depend on — it manages the lifecycle of spawning a process in a PTY, feeding output through the VT emulator, and exposing a rendered screen buffer for the Bubbletea UI to display.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST add `github.com/charmbracelet/x/xpty` and `github.com/charmbracelet/x/vt` as dependencies via `go get`
- MUST use `vt.NewSafeEmulator()` (thread-safe variant) since PTY reader goroutine and Bubbletea render loop access the emulator concurrently
- MUST forward emulator responses back to the PTY via `io.Copy(pty, emu)` to prevent deadlocks on cursor position queries
- MUST synchronize PTY and emulator resize together
- MUST handle graceful cleanup of PTY and emulator on Close()
- MUST track alive/dead state of the underlying process
</requirements>

## Subtasks
- [ ] 1.1 Add x/vt and x/xpty dependencies to go.mod via `go get`
- [ ] 1.2 Create `internal/core/run/terminal.go` with the `Terminal` struct and full lifecycle (New, Start, Render, WriteInput, Resize, IsAlive, Close)
- [ ] 1.3 Implement PTY→emulator read goroutine and emulator→PTY response forwarding goroutine
- [ ] 1.4 Implement process exit detection (goroutine that waits on cmd.Wait and updates alive state)
- [ ] 1.5 Write comprehensive unit tests for the Terminal component

## Implementation Details
Create a new file `internal/core/run/terminal.go`. The Terminal struct encapsulates:
- `xpty.Pty` for PTY management
- `vt.SafeEmulator` for terminal emulation
- `*exec.Cmd` for the spawned process
- Synchronization via `sync.RWMutex` for alive state

Reference the TechSpec "Core Interfaces" section for the Terminal API surface. Reference ADR-001 for the architectural rationale.

### Relevant Files
- `internal/core/run/terminal.go` — NEW: Terminal wrapper component
- `go.mod` — Add x/vt and x/xpty dependencies

### Dependent Files
- `internal/core/run/execution.go` — Will use Terminal in task_05
- `internal/core/run/ui_model.go` — Will hold Terminal refs in task_05

### Related ADRs
- [ADR-001: PTY + VT Emulator for Terminal Embedding](adrs/adr-001.md) — Defines why xpty + x/vt SafeEmulator was chosen over alternatives

## Deliverables
- `internal/core/run/terminal.go` with complete Terminal implementation
- Updated `go.mod` and `go.sum` with x/vt and x/xpty dependencies
- Unit tests with 80%+ coverage **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Start a simple process (e.g., `echo hello`) in PTY, verify VT emulator captures "hello" in Render() output
  - [ ] Verify WriteInput() delivers bytes to the PTY and the process receives them
  - [ ] Verify Resize() updates both PTY and emulator dimensions
  - [ ] Verify IsAlive() returns true while process runs, false after exit
  - [ ] Verify Close() terminates process and cleans up PTY and emulator
  - [ ] Verify concurrent Render() and Write() do not race (SafeEmulator thread safety)
  - [ ] Verify response forwarding works (emulator responses reach PTY stdin)
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build)
- Terminal can spawn a real process, capture its output, and render it as a string
- No goroutine leaks after Close()
