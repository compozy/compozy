## status: pending

<task_context>
  <domain>Runtime, Networking</domain>
  <type>Feature Implementation</type>
  <scope>Full</scope>
  <complexity>medium</complexity>
  <dependencies>none</dependencies>
</task_context>

# Task 2: Signal Server (Fiber HTTP)

## Overview
Create a local HTTP server using Fiber that receives job lifecycle signals from Claude Code agents. The primary endpoint `POST /job/done` allows the agent to explicitly signal task completion (via a curl command instructed in the system prompt), replacing the previous exit-code-based detection. This enables the orchestrator to advance to the next job deterministically while keeping completed terminals alive for user review.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST add `github.com/gofiber/fiber/v3` as a dependency via `go get`
- MUST bind to localhost only (no external network exposure)
- MUST support configurable port (default 9877) via --signal-port flag
- MUST deliver events to a buffered Go channel (cap = number of jobs) without blocking the HTTP handler
- MUST validate job IDs against known jobs and return 404 for unknown IDs
- MUST support graceful shutdown via context cancellation
- MUST disable Fiber startup banner (DisableStartupMessage: true)
</requirements>

## Subtasks
- [ ] 2.1 Add Fiber v3 dependency to go.mod via `go get`
- [ ] 2.2 Create `internal/core/run/signal_server.go` with SignalServer struct, SignalEvent type, and endpoint handlers
- [ ] 2.3 Implement POST /job/done endpoint (validates job ID, sends event on channel)
- [ ] 2.4 Implement POST /job/status endpoint (optional progress updates)
- [ ] 2.5 Implement GET /health endpoint (connectivity check)
- [ ] 2.6 Add SignalPort field to RuntimeConfig in model.go and --signal-port flag to CLI
- [ ] 2.7 Write comprehensive unit tests for all endpoints and event delivery

## Implementation Details
Create a new file `internal/core/run/signal_server.go`. The server lifecycle is:
- Created before any jobs launch
- Started in a goroutine
- Events delivered to execution pipeline via channel
- Shutdown gracefully after all jobs complete or on SIGINT/SIGTERM

Reference the TechSpec "Signal Server Endpoints" section for endpoint specs. Reference ADR-002 for the architectural rationale.

### Relevant Files
- `internal/core/run/signal_server.go` — NEW: Fiber HTTP server
- `internal/core/model/model.go` — Add SignalPort field to RuntimeConfig
- `internal/cli/root.go` — Add --signal-port flag

### Dependent Files
- `internal/core/run/execution.go` — Will start/stop server in task_05
- `internal/core/run/ui_update.go` — Will receive events in task_05

### Related ADRs
- [ADR-002: Fiber HTTP Server for Job Signaling](adrs/adr-002.md) — Defines why HTTP over file watchers, Unix sockets, or MQTT

## Deliverables
- `internal/core/run/signal_server.go` with complete SignalServer implementation
- Updated `model.go` with SignalPort field
- Updated `root.go` with --signal-port flag
- Updated `go.mod` and `go.sum` with Fiber dependency
- Unit tests with 80%+ coverage **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] POST /job/done with valid job ID returns 200 and delivers SignalEvent on channel
  - [ ] POST /job/done with unknown job ID returns 404
  - [ ] POST /job/done with malformed JSON returns 400
  - [ ] POST /job/status delivers status event on channel
  - [ ] GET /health returns 200 OK
  - [ ] Event channel does not block handler when buffer is full (non-blocking send)
  - [ ] Graceful shutdown completes without error
  - [ ] Server binds to specified port and rejects if port is in use
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build)
- Server starts, accepts requests, delivers events, and shuts down cleanly
- `curl -X POST localhost:9877/job/done -d '{"id":"test"}'` works from a separate terminal
