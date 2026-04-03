---
status: completed
domain: Execution
type: Feature Implementation
scope: Full
complexity: critical
dependencies:
  - task_01
---

# Task 2: Execution & Logging Pipeline Migration

## Overview

Replace the process-based execution and Claude-specific logging pipelines with ACP session lifecycle management. This is the highest-impact task: it rewires how Compozy spawns agents, monitors liveness, processes output, tracks token usage, and handles retries — all through ACP's typed `SessionUpdate` stream instead of raw stdout parsing and heuristic activity monitoring.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST replace `createIDECommand()` (command_io.go:60) with ACP `Client` creation using the registry from task_01
- MUST replace `setupCommandExecution()` (execution.go:703) to use ACP `Client.CreateSession()` instead of `exec.Cmd` + I/O wiring
- MUST replace `executeCommandAndResolve()` (execution.go:756) to use `Session.Done()` + `ctx.Done()` instead of `cmd.Wait()` + activity watchdog
- MUST remove `startActivityWatchdog()` (execution.go:797) — replaced by ACP session lifecycle (ADR-005)
- MUST replace `jsonFormatter` (logging.go:259) with an `UpdateHandler` that processes `SessionUpdate` notifications from the ACP session
- MUST remove `activityMonitor` (logging.go:184) — replaced by ACP session status
- MUST replace `uiLogTap` (logging.go:205) to route typed `ContentBlock` to the UI channel instead of raw text lines
- MUST update retry logic in `jobRunner.run()` (execution.go:297) to use ACP session error codes instead of process exit codes
- MUST update `tokenUsageUpdateMsg` to carry `model.Usage` instead of `TokenUsage`
- MUST update `jobLogUpdateMsg` to carry `[]model.ContentBlock` instead of signaling index-only refresh
- MUST keep a process-level `context.WithTimeout()` as ultimate backstop per ADR-005
- MUST remove Claude-specific `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` env var injection (command_io.go:130)
- MUST preserve the existing job lifecycle phases (queued → scheduled → running → succeeded/failed/retrying/canceled)
- MUST preserve post-success hooks (`afterTaskJobSuccess`, `afterReviewJobSuccess`) unchanged
- MUST log ACP session lifecycle events via `slog` with agent ID, session ID, and duration fields
</requirements>

## Subtasks

- [x] 2.1 Replace command creation and I/O setup with ACP client/session creation — `createIDECommand()`, `setupCommandExecution()`, `configureCommandEnvironment()` replaced by `agent.NewClient()` + `client.CreateSession()`
- [x] 2.2 Replace `executeCommandAndResolve()` and `startActivityWatchdog()` with ACP session lifecycle — listen on `Session.Done()` + `ctx.Done()`, keep context timeout as backstop
- [x] 2.3 Build `UpdateHandler` implementation that replaces `jsonFormatter`, `activityMonitor`, and `uiLogTap` — processes `SessionUpdate` notifications, routes content blocks to log files and UI channel, extracts usage
- [x] 2.4 Update retry logic to use ACP session error codes — `Session.Err()` provides structured errors instead of `exec.ExitError` parsing
- [x] 2.5 Update message types: `jobLogUpdateMsg` → `jobUpdateMsg` carrying `[]model.ContentBlock`, `tokenUsageUpdateMsg` → `usageUpdateMsg` carrying `model.Usage`
- [x] 2.6 Add structured `slog` logging for ACP session lifecycle events (created, update, completed, error)
- [x] 2.7 Write unit and integration tests using the mock ACP server from task_01

## Implementation Details

The TechSpec §Execution Pipeline and §Logging Pipeline describe the target architecture. The TechSpec §Data Models defines the message type changes. ADR-005 specifies the session lifecycle timeout strategy.

### Relevant Files

- `internal/core/run/execution.go` — `executeJobWithTimeout()` (line 681), `setupCommandExecution()` (line 703), `executeCommandAndResolve()` (line 756), `startActivityWatchdog()` (line 797), `jobRunner.run()` (line 297), `nextTimeout()` (line 392)
- `internal/core/run/command_io.go` — `createIDECommand()` (line 60), `setupCommandIO()` (line 74), `configureCommandEnvironment()` (line 122), `buildUITaps()` (line 34), `buildCLITaps()` (line 71)
- `internal/core/run/logging.go` — `jsonFormatter` (line 259), `activityMonitor` (line 184), `uiLogTap` (line 205), `buildUITaps()`, `buildCLITaps()`
- `internal/core/run/types.go` — `TokenUsage` (line 145), `ClaudeMessage` (line 167), `jobLogUpdateMsg` (line 132), `tokenUsageUpdateMsg` (line 136), `job` struct (line 232)
- `internal/core/run/execution_test.go` — Existing test patterns (stub providers, temp dirs)

### Dependent Files

- `internal/core/agent/client.go` — Consumed: `Client` interface (from task_01)
- `internal/core/agent/session.go` — Consumed: `Session` interface (from task_01)
- `internal/core/agent/registry.go` — Consumed: `AgentSpec` registry (from task_01)
- `internal/core/model/content.go` — Consumed: `SessionUpdate`, `ContentBlock`, `Usage` types (from task_01)
- `internal/core/run/ui_update.go` — Will need handler updates for new message types (task_03)
- `internal/core/run/ui_model.go` — Will need model changes for new message types (task_03)

### Related ADRs

- [ADR-001: Full Replacement of Legacy Agent Drivers with ACP](../adrs/adr-001.md) — No legacy fallback; all agents use ACP exclusively
- [ADR-003: Subprocess-over-Stdio for ACP Transport](../adrs/adr-003.md) — Process lifecycle tied to context cancellation
- [ADR-005: ACP Session Lifecycle for Timeout and Retry Management](../adrs/adr-005.md) — Session.Done() + ctx.Done() replaces activity watchdog; process-level timeout as backstop

## Deliverables

- Rewritten `execution.go` using ACP `Client`/`Session` for job execution
- Rewritten `command_io.go` replacing command creation with ACP client setup
- Rewritten `logging.go` with `UpdateHandler` replacing `jsonFormatter`, `activityMonitor`, `uiLogTap`
- Updated `types.go` with new message types (`jobUpdateMsg`, `usageUpdateMsg`) and removed `TokenUsage`/`ClaudeMessage`
- Structured `slog` logging for ACP session lifecycle events
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests using mock ACP server for full execution pipeline **(REQUIRED)**

## Tests

- Unit tests:
  - [x] UpdateHandler processes SessionUpdate with text blocks → routes to log file and UI channel
  - [x] UpdateHandler processes SessionUpdate with mixed block types (text, diff, tool_use) → all blocks routed correctly
  - [x] UpdateHandler extracts Usage from SessionUpdate → sends usageUpdateMsg
  - [x] UpdateHandler handles SessionUpdate with completion status → signals session done
  - [x] UpdateHandler handles SessionUpdate with error status → propagates error
  - [x] Retry logic: ACP session error → retries with backoff
  - [x] Retry logic: ACP session success → no retry, triggers post-success hook
  - [x] Retry logic: context cancellation → marks job canceled, no retry
  - [x] Timeout: context deadline exceeded → kills process as backstop
  - [x] Message type: jobUpdateMsg carries correct ContentBlocks
  - [x] Message type: usageUpdateMsg carries correct model.Usage fields
- Integration tests:
  - [x] Full pipeline: mock ACP server → client → session → UpdateHandler → UI channel receives typed blocks
  - [x] Retry pipeline: mock ACP server returns error on first attempt, success on second → job succeeds after retry
  - [x] Multi-job execution: two jobs run concurrently via semaphore, both complete via ACP
  - [x] Post-success hooks: afterTaskJobSuccess and afterReviewJobSuccess still fire correctly after ACP session completion
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- `make verify` passes with zero lint issues
- No references to `exec.Cmd` for agent execution (only in `agent.Client` internal implementation)
- No references to `activityMonitor`, `jsonFormatter`, `ClaudeMessage`, or `TokenUsage` in `run/` package
- Job lifecycle phases preserved: queued → running → succeeded/failed/retrying/canceled
- Post-success hooks unchanged and functional
- Structured slog output for session lifecycle events
