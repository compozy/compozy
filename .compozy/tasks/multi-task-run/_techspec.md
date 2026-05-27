# TechSpec: Multi-Task Run

## Executive Summary

`compozy tasks run-multiple` will add a dedicated daemon-backed workflow for running multiple task slugs from one command without changing `compozy tasks run`. V1 is strictly sequential: it preflights every requested slug before starting, creates one daemon-owned parent multi-run, then starts normal child task runs one at a time.

The main trade-off is implementation scope. A CLI-only queue would be smaller, but it cannot preserve the existing `Close TUI` behavior for queued tasks. The daemon-owned coordinator makes the whole queue survive TUI detach, at the cost of new API, run manager, event, and tabbed TUI work.

## System Architecture

### Component Overview

- CLI command: `internal/cli/daemon_commands.go` adds `tasks run-multiple [slugs]`, parses comma-separated slugs, applies task-run defaults, preflights all slugs, resolves attach mode, and starts the daemon parent run.
- Config: `internal/core/workspace` adds `[tasks.run] run_multiple_mode`, defaulting to `enqueued`; `parallel` validates but falls back to enqueued with a V2/worktree message.
- Daemon API: a new multi-run start endpoint accepts the ordered slug list and returns the parent run.
- Run manager: `internal/daemon` adds a `task_multi` parent mode, modeled after review-watch parent/child orchestration.
- TUI: `internal/core/run/ui` adds a multi-run remote attach surface that renders tabs above the existing single-run task UI.

## Implementation Design

### Core Interfaces

```go
type TaskRunMultipleRequest struct {
	Workspace        string          `json:"workspace"`
	Slugs            []string        `json:"slugs"`
	Mode             string          `json:"mode,omitempty"`
	PresentationMode string          `json:"presentation_mode,omitempty"`
	RuntimeOverrides json.RawMessage `json:"runtime_overrides,omitempty"`
}

type TaskRunMultipleItem struct {
	Slug      string `json:"slug"`
	Status    string `json:"status"`
	RunID     string `json:"run_id,omitempty"`
	ErrorText string `json:"error_text,omitempty"`
}
```

### Data Models

- Add `TaskRunConfig.RunMultipleMode *string` with TOML key `run_multiple_mode`.
- Add daemon run mode constant `task_multi` for the parent run.
- Child runs remain normal `task` runs and set `ParentRunID` to the parent.
- Parent multi-run state is append-only in parent run events: queued item, child started, child completed, child failed, queue canceled, queue completed.
- Duplicate slugs after trimming are invalid because running the same workflow twice in one queue is ambiguous.

### API Endpoints

- `POST /api/task-runs/multiple`: start a daemon-owned parent multi-run.
- Request: `TaskRunMultipleRequest`.
- Response: existing `RunResponse` containing the parent run.
- `GET /api/task-runs/multiple/:run_id/snapshot`: return parent run plus ordered `TaskRunMultipleItem` state for attach/re-attach.
- Existing child run endpoints remain unchanged: snapshots and streams still use `/api/runs/:run_id/...`.

## Integration Points

No external services or third-party integrations are required. The feature integrates only with Compozy's existing daemon transport, global run index, per-run databases, task workflow metadata, and Bubble Tea TUI runtime.

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|----------------------|-----------------|
| `internal/cli/daemon_commands.go` | new | Adds command while preserving `tasks run` behavior | Add parser, command state, start handling, tests |
| `internal/core/workspace` | modified | Config decoding rejects unknown fields today | Add field, merge, validation, config tests |
| `internal/api/contract`, `internal/api/core`, `internal/api/client` | modified | New daemon transport surface | Add request/response, routes, client method, contract tests |
| `internal/daemon` | modified | Parent/child queue ownership | Add coordinator, parent mode, cancellation, event emission |
| `internal/core/run/ui` | modified | Current UI assumes one run and job indexes are local | Add multi-run wrapper with isolated child state per tab |
| docs/README/config examples | modified | New command and config key | Document syntax, fallback, quit behavior |

## Testing Approach

### Unit Tests

- Slug parser: trims whitespace, rejects empty entries, rejects duplicates, preserves order.
- Config: default mode is `enqueued`; `enqueued` and `parallel` validate; unknown values fail.
- CLI: `tasks run-multiple a,b` starts one parent request and does not call `StartTaskRun` directly.
- CLI fallback: configured `parallel` prints the V2/worktree fallback message and sends `enqueued`.
- Daemon coordinator: preflights all slugs before parent creation; starts exactly one child at a time; stops on first child failure.
- Cancellation: parent cancel cancels active child and marks queued items canceled.

### Integration Tests

- In-process daemon command test for `run-multiple` with two workflows.
- Parent/child global DB linkage test asserting child `ParentRunID`.
- Remote multi-run snapshot reconstruction from parent events plus child runs.
- TUI model tests for tab rendering, tab navigation, and quit dialog action mapping.

## Development Sequencing

### Build Order

1. Add slug parsing and `run_multiple_mode` config support - no dependencies.
2. Add CLI `tasks run-multiple` shell and tests - depends on step 1.
3. Add API contract/client/handler types for parent multi-run start and snapshot - depends on step 2.
4. Add daemon parent coordinator and `task_multi` run mode - depends on step 3.
5. Add parent event payloads and snapshot reconstruction - depends on step 4.
6. Add multi-run TUI wrapper with tabs and existing quit dialog behavior - depends on step 5.
7. Wire CLI attach/stream/detach handling for parent multi-runs - depends on step 6.
8. Add docs and end-to-end/integration coverage - depends on steps 1-7.

### Technical Dependencies

No external services or new third-party dependencies are required. V2 parallel execution depends on git worktree isolation and is explicitly out of scope.

## Monitoring and Observability

- Emit parent run events for queue started, item queued, child started, child completed, child failed, queue canceled, and queue completed.
- Log structured fields: `parent_run_id`, `child_run_id`, `workflow_slug`, `index`, `total`, `run_multiple_mode`.
- Count `task_multi` active and terminal runs in daemon mode metrics, alongside existing run modes.

## Technical Considerations

### Key Decisions

- Decision: use a dedicated `tasks run-multiple` command.
  Rationale: avoids regressions in `tasks run`.
  Trade-off: shared flags/config must be wired twice.
- Decision: use daemon-owned parent queue.
  Rationale: required for the existing `Close TUI` behavior to apply to queued work.
  Trade-off: larger daemon/API/TUI change set.
- Decision: V1 accepts `parallel` but runs enqueued.
  Rationale: preserves forward config compatibility while avoiding same-worktree agent collisions.
  Trade-off: users must see a clear fallback message.

### Known Risks

- TUI state collision: isolate each child run UI state and translator by tab/run ID.
- Partial queue starts: preflight all slugs before parent creation and duplicate-check before API start.
- Cancel ambiguity: map the existing dialog exactly: `Close TUI` detaches, `Stop Run` cancels active and queued work, `Cancel` returns to the UI.
- API route conflicts: use `/api/task-runs/multiple` instead of adding a static route under `/api/tasks`.

## Architecture Decision Records

- [ADR-001: Use Multi-Task Execution as Explicit Task-Run Orchestration](adrs/adr-001.md) - Treat multi-task execution as orchestration over independent task workflows.
- [ADR-002: Introduce a Dedicated Multi-Run Command for V1](adrs/adr-002.md) - Add `tasks run-multiple` instead of widening `tasks run`.
- [ADR-003: Fix V1 Command Name and Config Behavior](adrs/adr-003.md) - Set command/config names, parallel fallback, and tab expectation.
- [ADR-004: Use a Daemon-Owned Sequential Multi-Run Coordinator](adrs/adr-004.md) - Own the queue in the daemon so the current TUI detach/stop behavior applies to the whole queue.
