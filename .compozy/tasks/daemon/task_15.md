---
status: completed
title: Reviews and Exec Flow Migration
type: refactor
complexity: high
dependencies:
  - task_05
  - task_10
  - task_11
---

# Reviews and Exec Flow Migration

## Overview
This task migrates review and exec workflows onto the daemon control plane. It keeps the current authored review artifacts and exec ergonomics, but moves run lifecycle, persistence, and resume behavior to daemon-owned execution so reviews and ad hoc exec flows behave like first-class daemon runs.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Reviews", "Sync and exec", and "Data Flow" instead of duplicating them here
- FOCUS ON "WHAT" — keep review and exec behavior recognizable while moving lifecycle ownership to the daemon
- MINIMIZE CODE — reuse the daemon run manager and existing review/exec parsers instead of creating one-off execution paths
- TESTS REQUIRED — unit and integration coverage are mandatory for review sync, exec persistence, and output compatibility
</critical>

<requirements>
1. MUST run review flows through daemon-backed sync and run lifecycle before any review-fix execution starts.
2. MUST migrate `exec` to a daemon-backed run that binds to the current workspace, persists `mode=exec`, and supports later inspection or resume.
3. MUST preserve current exec input/output behavior, including prompt file, stdin, JSON, and raw-JSON surfaces.
4. MUST keep review round and issue Markdown artifacts materialized in the workspace while operational state is mirrored into daemon storage.
5. SHOULD preserve review-provider and extension hook behavior across daemon-backed review and exec runs.
</requirements>

## Subtasks
- [x] 15.1 Route review fetch, list, show, and fix flows through daemon-backed sync and run-manager behavior.
- [x] 15.2 Route ad hoc exec runs through the daemon with persisted mode, workspace binding, and later inspectability.
- [x] 15.3 Preserve review artifact materialization and provider bridge behavior under the new control plane.
- [x] 15.4 Keep exec input/output formats and prompt sources compatible after the migration.
- [x] 15.5 Add tests covering review workflows, exec persistence, and output compatibility.

## Implementation Details
Implement the migration described in the TechSpec "Reviews", "Sync and exec", and "Data Flow" sections. This task should make review and exec flows first-class daemon runs while preserving the authored Markdown review model and the current exec ergonomics users already depend on.

### AGH Reference Files
- `~/dev/compozy/agh/internal/session/manager.go` — reference for making review and ad hoc execution first-class daemon-managed sessions.
- `~/dev/compozy/agh/internal/store/sessiondb/session_db.go` — reference for per-run persistence tied to non-workflow execution modes.
- `~/dev/compozy/agh/internal/daemon/daemon.go` — reference for lifecycle ownership across different run modes.

### Relevant Files
- `internal/core/reviews/parser.go` — review issue parsing that must continue feeding authored review artifacts.
- `internal/core/reviews/store.go` — review round discovery and persistence seams affected by daemon-backed review flows.
- `internal/core/run/exec/exec.go` — current ad hoc exec engine that must move under daemon-managed lifecycle.
- `internal/core/run/exec_facade.go` — exec facade entrypoint that should route through the daemon-backed path.
- `internal/core/run/preflight.go` — preflight behavior that still needs to apply before daemon-backed review and exec runs.
- `internal/core/extension/review_provider_bridge.go` — provider bridge behavior that must survive the daemon migration.

### Dependent Files
- `internal/cli/commands.go` — review and exec command surfaces must call the daemon-backed flows.
- `internal/cli/run.go` — run-oriented command dispatch must stop owning local review and exec execution.
- `internal/cli/root.go` — top-level command wiring and help must reflect the daemon-backed flows.
- `internal/core/extension/runtime.go` — extension behavior must stay compatible with review and exec runs after the migration.

### Related ADRs
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — review markdown remains human-authored while daemon state mirrors it operationally.
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — review and exec flows must consume the shared API contract.
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — review-fix and exec behavior must stay ergonomic for interactive users.

## Deliverables
- Daemon-backed review and exec execution flows.
- Preserved review artifact materialization with daemon-owned operational state.
- Preserved exec output behavior with persisted daemon run state.
- Unit tests with 80%+ coverage for review and exec migration seams **(REQUIRED)**
- Integration tests covering daemon-backed review-fix and exec flows **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Review flows trigger sync and use daemon-backed lifecycle state before starting a review-fix run.
  - [x] Exec requests preserve prompt-file, stdin, JSON, and raw-JSON behavior after moving under daemon lifecycle ownership.
  - [x] Conflicting exec input sources are resolved deterministically according to the final CLI contract.
  - [x] Review issue and round materialization remains compatible with the authored Markdown contract.
  - [x] Review runs continue to batch and target the expected issue set when daemon-backed sync updates round state before execution.
- Integration tests:
  - [x] `compozy reviews fix` runs through the daemon, persists lifecycle state, and leaves review markdown artifacts aligned with operational status.
  - [x] A review round edited manually between sync and run start is re-read correctly before the daemon launches the review-fix run.
  - [x] `compozy exec` binds to the current workspace, persists `mode=exec`, and can be inspected after the initial invocation exits.
  - [x] `compozy exec` auto-registers an unseen workspace and still skips workflow sync as defined in the TechSpec.
  - [x] Review-provider and extension hooks continue to work during daemon-backed review and exec runs.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Review and exec flows are first-class daemon-managed runs
- Authored review artifacts remain intact while operational state moves to daemon storage
- Exec ergonomics and output formats remain stable after the migration
