## status: pending

<task_context>
  <domain>UI, Execution</domain>
  <type>Refactor</type>
  <scope>Full</scope>
  <complexity>high</complexity>
  <dependencies>task_01, task_02, task_03, task_04</dependencies>
</task_context>

# Task 5: UI Refactor + Execution Pipeline + Cleanup

## Overview
Wire all components from tasks 1-4 together into the full interactive terminal experience. Refactor the Bubbletea UI to render VT emulator screens instead of log lines, add navigate/terminal interaction modes with key forwarding, and handle job-done signals from the Fiber server. Refactor the execution pipeline to use Terminal wrappers instead of pipe-based exec.Command, integrate readiness detection and composer simulation for prompt delivery, and manage Signal Server lifecycle. Clean up removed code (command_io.go, jsonFormatter, uiLogTap).

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- UI MUST support two interaction modes: modeNavigate (arrow keys navigate sidebar) and modeTerminal (keys forwarded to active PTY)
- Enter in modeNavigate MUST switch to modeTerminal for the selected job
- Esc in modeTerminal MUST switch back to modeNavigate
- Main pane MUST render the selected job's VT emulator screen via Terminal.Render()
- Completed job terminals MUST stay alive for user to revisit (PTYs killed only on compozy exit)
- Execution pipeline MUST start Signal Server before launching any jobs
- Execution pipeline MUST create a Terminal per job, use readiness detection, then send prompt via composer simulation
- Job completion MUST be driven by jobDoneSignalMsg from Signal Server (not exit code)
- Activity timeout MUST remain as safety net for agents that fail without sending done signal
- command_io.go MUST be removed (stream-json IO taps no longer needed)
- jsonFormatter and uiLogTap in logging.go MUST be removed; activityMonitor MUST be kept
- CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 env var MUST be set on PTY process
</requirements>

## Subtasks
- [ ] 5.1 Create `internal/core/run/ui_messages.go` with new message types (terminalOutputMsg, terminalReadyMsg, jobDoneSignalMsg, composerSendMsg)
- [ ] 5.2 Refactor `ui_model.go`: add terminals slice, interaction mode enum, signal channel to uiModel; update setupUI() to accept signal channel
- [ ] 5.3 Refactor `ui_update.go`: add modeNavigate/modeTerminal key handling, terminal output message handler, job-done signal handler, Enter/Esc mode switching
- [ ] 5.4 Refactor `ui_view.go`: render VT emulator screen in main pane via Terminal.Render(), update sidebar to show interaction mode indicator
- [ ] 5.5 Refactor `execution.go`: integrate Signal Server lifecycle (start before jobs, shutdown after), create Terminal per job, wire readiness detection + composer simulation for prompt delivery, handle jobDoneSignalMsg
- [ ] 5.6 Remove `command_io.go` entirely
- [ ] 5.7 Simplify `logging.go`: remove jsonFormatter, uiLogTap, lineFilterWriter; keep activityMonitor and lineRing
- [ ] 5.8 Update existing tests in execution_test.go and logging_test.go for new architecture
- [ ] 5.9 Write integration tests for the full flow (Terminal + Signal Server + UI)

## Implementation Details
This is the integration task that connects all prior components. The execution pipeline changes from:
`exec.Command → pipe stdout/stderr → jsonFormatter → uiLogTap → UI`
to:
`Terminal.Start(cmd) → PTY read goroutine → VT emulator → terminalOutputMsg → UI renders emu.Render()`

The job completion path changes from:
`cmd.Run() returns → exit code → jobFinishedMsg`
to:
`agent curls /job/done → Signal Server → jobDoneSignalMsg → UI marks done, switches to next job`

Reference the TechSpec "UI Model Changes", "Data Flow", and "Execution Layer" sections.

### Relevant Files
- `internal/core/run/ui_messages.go` — NEW: PTY-specific message types
- `internal/core/run/ui_model.go` — MODIFY: add terminals, interaction mode, signal channel
- `internal/core/run/ui_update.go` — MODIFY: key forwarding, terminal output, signal handling
- `internal/core/run/ui_view.go` — MODIFY: render VT emulator screen
- `internal/core/run/execution.go` — MODIFY: PTY-based execution, Signal Server lifecycle
- `internal/core/run/command_io.go` — DELETE
- `internal/core/run/logging.go` — SIMPLIFY: remove JSON-specific code

### Dependent Files
- `internal/core/run/terminal.go` — Uses Terminal from task_01
- `internal/core/run/signal_server.go` — Uses SignalServer from task_02
- `internal/core/run/keytranslate.go` — Uses translateKey() from task_03
- `internal/core/run/composer.go` — Uses sendComposerInput() from task_03
- `internal/core/run/readiness.go` — Uses waitForReady() from task_03
- `internal/core/prompt/system.go` — Uses BuildSystemPrompt() from task_04
- `internal/core/agent/ide.go` — Uses updated Command() from task_04
- `internal/core/run/types.go` — Existing message types and job phases
- `internal/core/run/ui_layout.go` — Layout calculations (unchanged)
- `internal/core/run/execution_test.go` — Existing tests need updating
- `internal/core/run/logging_test.go` — Existing tests need updating

### Related ADRs
- [ADR-001: PTY + VT Emulator for Terminal Embedding](adrs/adr-001.md) — Terminal rendering approach
- [ADR-002: Fiber HTTP Server for Job Signaling](adrs/adr-002.md) — Job completion signaling
- [ADR-003: Composer Simulation for Initial Prompt](adrs/adr-003.md) — Prompt delivery mechanism
- [ADR-004: Mode-Specific System Prompts](adrs/adr-004.md) — System prompt injection

## Deliverables
- New `ui_messages.go` with terminal-specific message types
- Refactored `ui_model.go`, `ui_update.go`, `ui_view.go` with terminal rendering and interaction modes
- Refactored `execution.go` with PTY-based execution and Signal Server integration
- Deleted `command_io.go`
- Simplified `logging.go`
- Updated existing tests
- Integration tests for full flow **(REQUIRED)**
- Unit tests with 80%+ coverage **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] modeNavigate: up/down changes selected job, Enter switches to modeTerminal
  - [ ] modeTerminal: Esc switches back to modeNavigate
  - [ ] modeTerminal: key messages are translated and forwarded to correct Terminal
  - [ ] terminalOutputMsg updates the correct job's terminal state and triggers re-render
  - [ ] jobDoneSignalMsg marks job as completed and auto-selects next pending job
  - [ ] Completed job terminals remain accessible (Render() still works)
  - [ ] Signal Server starts before first job and shuts down on cleanup
  - [ ] Activity timeout still triggers for jobs that never send done signal
- Integration tests:
  - [ ] Full lifecycle: start job → terminal renders output → agent sends done signal → next job starts
  - [ ] User interaction: select job → enter terminal mode → send keystrokes → exit back to navigate
  - [ ] Graceful shutdown: SIGTERM → all terminals closed, Signal Server stopped
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build)
- User can see Claude Code TUI rendering live in the Bubbletea viewport
- User can switch between navigate and terminal modes
- User can interact with running Claude Code instances via keyboard
- Job completion via agent curl triggers automatic view switch to next job
- Completed job terminals remain browsable
- No goroutine leaks on shutdown
- command_io.go is fully removed
- logging.go has no JSON-specific code remaining
