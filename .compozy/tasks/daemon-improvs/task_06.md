---
status: completed
title: ACP Liveness, Subprocess Supervision, and Reconcile Hardening
type: backend
complexity: critical
dependencies:
  - task_02
  - task_03
  - task_05
---

# ACP Liveness, Subprocess Supervision, and Reconcile Hardening

## Overview

This task hardens the daemon's runtime supervision of ACP-backed work without changing the high-level ownership model. It adds persisted liveness metadata, stall detection, stronger subprocess termination semantics, honest reconcile classification, and fault-injection coverage for the ACP failure modes the TechSpec calls out explicitly.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "AgentLivenessMonitor", "Data Models", "Integration Points", and "Development Sequencing" instead of duplicating them here
- FOCUS ON "WHAT" - strengthen supervision and recovery inside the current runtime boundary instead of redesigning executor ownership
- MINIMIZE CODE - persist only the liveness and supervision data needed for reliable reconcile and operator visibility
- TESTS REQUIRED - ACP fault-injection and recovery coverage are mandatory for this task
</critical>

<requirements>
1. MUST add persisted job liveness metadata (`session_id`, subprocess identity, last update timestamp, stall state, stall reason) needed for supervision and recovery decisions.
2. MUST assign ACP liveness ownership to the executor boundary through an `AgentLivenessMonitor`-style seam that records session updates, starts, exits, and stall detection.
3. MUST add Unix process-group aware termination and orphan cleanup behavior, with explicit compile-safe fallback behavior on Windows rather than implied parity.
4. MUST extend reconcile logic to classify interrupted work as crashed, orphaned, or stalled based on available liveness state and to report failures honestly when synthetic crash persistence cannot be completed.
5. SHOULD include ACP mock or fixture infrastructure that can simulate malformed frames, disconnects, and cancellation-sensitive failures through the shared harness.
</requirements>

## Subtasks

- [x] 6.1 Extend executor and run-state persistence with liveness metadata needed for startup and periodic recovery.
- [x] 6.2 Add liveness monitoring hooks for ACP session updates, job starts, exits, and stall detection.
- [x] 6.3 Strengthen subprocess supervision with Unix process-group termination and explicit Windows fallback semantics.
- [x] 6.4 Upgrade reconcile classification and recovery reporting to use the new liveness and subprocess state.
- [x] 6.5 Add ACP fault-injection coverage for disconnect, malformed frame, blocked cancel, and timeout escalation scenarios.

## Implementation Details

Implement the hardening described in the TechSpec sections "AgentLivenessMonitor", "Data Models", "Integration Points", "Known Risks", and "Build Order". This task should keep the executor and daemon ownership model intact while tightening supervision and recovery around ACP-backed subprocesses.

### Relevant Files

- `internal/core/run/executor/execution.go` - ACP execution startup and event flow must start recording liveness ownership details.
- `internal/core/run/executor/shutdown.go` - executor shutdown paths need stall-aware and liveness-aware supervision semantics.
- `internal/core/subprocess/process.go` - cross-platform subprocess shutdown escalation is rooted here.
- `internal/core/subprocess/process_unix.go` - Unix process-group termination and orphan cleanup behavior should live here.
- `internal/core/subprocess/process_windows.go` - Windows fallback behavior must stay explicit and compile-safe in this phase.
- `internal/daemon/reconcile.go` - startup and periodic recovery classification must use the new liveness and subprocess metadata.
- `internal/store/rundb/run_db.go` - persisted job_state and integrity writes must store the new supervision metadata.

### Dependent Files

- `internal/store/rundb/migrations.go` - schema changes are required for persisted liveness and stall metadata.
- `internal/core/run/executor/execution_acp_integration_test.go` - ACP integration coverage must expand to real fault scenarios.
- `internal/core/subprocess/process_unix_test.go` - Unix process-group termination behavior needs direct regression coverage.
- `internal/daemon/reconcile_test.go` - reconcile classification should be verified against crashed, orphaned, and stalled fixtures.
- `internal/testutil/acpmock/driver.go` - ACP mock fixtures should drive the fault scenarios required by this task.

### Related ADRs

- [ADR-002: Incremental Runtime Supervision Hardening Inside the Existing Daemon Boundary](adrs/adr-002.md) - defines runtime supervision hardening as an in-boundary change.
- [ADR-003: Validation-First Daemon Hardening](adrs/adr-003.md) - requires ACP fault-injection and real-daemon validation for these failure modes.

## Deliverables

- Persisted liveness and stall metadata for daemon-managed jobs.
- Executor-owned liveness monitoring hooks for ACP session updates and exits.
- Stronger subprocess termination and orphan handling semantics with explicit Windows fallback.
- Unit tests with 80%+ coverage for liveness state updates, subprocess termination, and reconcile classification helpers **(REQUIRED)**
- Integration tests proving ACP fault-injection, orphan handling, and reconcile behavior under real daemon execution **(REQUIRED)**

## Tests

- Unit tests:
  - [x] Receiving ACP session updates records the latest liveness timestamp and session identity for the owning job state.
  - [x] Detecting a stalled job marks the persisted stall state and reason without corrupting terminal job status.
  - [x] Unix subprocess supervision escalates from cooperative shutdown to process-group termination when the child does not exit in time.
  - [x] Reconcile classification distinguishes crashed, orphaned, and stalled runs based on persisted liveness and subprocess metadata.
- Integration tests:
  - [x] An ACP mid-stream disconnect produces the expected liveness or reconcile classification and does not leave an untracked running child.
  - [x] A malformed ACP frame causes the run to fail with honest persisted recovery details instead of silently hanging.
  - [x] A blocked cancellation path escalates through the configured supervision path and records the expected shutdown outcome.
  - [x] Restarting the daemon after an interrupted ACP-backed run classifies the run correctly and reports synthetic crash-event persistence failures honestly when they occur.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- ACP-backed jobs persist enough liveness data for honest runtime supervision and recovery
- Unix subprocess supervision can terminate the whole owned process group while Windows remains explicit about fallback behavior
- Reconcile results distinguish crashed, orphaned, and stalled runs using persisted runtime evidence instead of guesswork
