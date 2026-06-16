# TechSpec: Worktree-Backed Parallel Multi-Run

## Executive Summary

`compozy tasks run --multiple` will support true parallel execution as an
opt-in mode. The default remains `enqueued`, preserving the existing
one-child-at-a-time behavior. Parallel mode runs child task workflows in
isolated git worktrees, bounds concurrent child starts with a configurable
limit, lets siblings continue after failures, and reports aggregate parent
status after all active children settle.

This TechSpec updates the earlier sequential V1 plan. The implementation keeps
the existing `task_multi` parent run model, dedicated multi-run API routes,
event-derived snapshots, and tabbed attach UI. It changes the daemon
coordinator into one scheduler with explicit `enqueued` and `parallel`
branches, and it treats the multi-run snapshot as the canonical view of parent
and child state across original-workspace and worktree-owned child runs.

## Goals

- Add `--parallel` for `compozy tasks run --multiple`.
- Honor `[tasks.run] run_multiple_mode = "parallel"` instead of downgrading it.
- Add a configurable parallel fanout limit with default `2`.
- Create one detached git worktree per parallel child run.
- Register child runs against their physical worktree workspace roots.
- Preserve all child worktrees for manual review.
- Surface each child status, run id, and worktree path in snapshots, stream
  output, TUI attach, and final handoff.
- Keep enqueued mode backward compatible.

## Non-Goals

- No auto-merge, auto-push, or branch publication.
- No automatic deletion of changed or unchanged child worktrees in V1.
- No conflict prediction or semantic dependency scheduling.
- No broad sandboxing beyond git worktree isolation.
- No new dashboard or replacement for the existing multi-run TUI.

## System Architecture

### Component Overview

| Component | Responsibility |
|-----------|----------------|
| `internal/cli/daemon_commands.go` | Add `--parallel` and `--parallel-limit`, resolve mode and limit precedence, pass both to daemon start. |
| `internal/core/workspace` | Parse, merge, default, and validate `run_multiple_mode` and `run_multiple_parallel_limit`. |
| `internal/api/contract` | Add parallel limit and additive worktree metadata fields to request/snapshot contracts. |
| `internal/api/core`, `internal/api/client` | Preserve dedicated multi-run routes and carry the extended contract. |
| `internal/daemon/task_multi.go` | Refactor current sequential coordinator into a scheduler with `enqueued` and `parallel` branches. |
| New worktree allocator package | Own git worktree path planning, current-branch/HEAD resolution, creation, and metadata. |
| `internal/core/run/ui` | Keep tabbed attach and render per-child worktree metadata. |
| `pkg/compozy/events/kinds` | Add worktree metadata to `TaskRunMultiplePayload` without breaking existing JSON consumers. |

### Scheduler Model

The daemon keeps one parent `task_multi` run. The parent scheduler receives an
ordered set of prepared child items and dispatches one of two branches:

- `enqueued`: preserve current queue semantics and one active child at a time.
- `parallel`: allocate worktrees and start child runs concurrently, bounded by
  the resolved parallel limit.

Both branches should share item status constants, event emission helpers,
snapshot reconstruction, cancellation mapping, and terminal aggregation where
practical. The implementation should avoid two divergent multi-run state
machines.

## CLI And Configuration

### CLI

```bash
compozy tasks run --multiple task_01,task_02 --parallel
compozy tasks run --multiple task_01,task_02 --parallel --parallel-limit 3
```

Rules:

- `--parallel` is valid only with `--multiple`.
- `--parallel-limit <n>` is valid only with `--multiple`.
- `--parallel-limit <n>` is effective only when the resolved mode is
  `parallel`.
- `--parallel` overrides `[tasks.run] run_multiple_mode`.
- `--parallel-limit` overrides `[tasks.run] run_multiple_parallel_limit`.
- CLI should reject zero or negative limits before contacting the daemon.

### Config

```toml
[tasks.run]
run_multiple_mode = "parallel"
run_multiple_parallel_limit = 2
```

Config rules:

- `run_multiple_mode` remains `enqueued` by default.
- Valid modes are `enqueued` and `parallel`.
- `run_multiple_parallel_limit` defaults to `2` when unset.
- A provided `run_multiple_parallel_limit` must be positive.
- The limit does not change enqueued execution.

### Precedence

Parallel mode:

1. `--parallel`
2. `[tasks.run] run_multiple_mode`
3. default `enqueued`

Parallel limit:

1. `--parallel-limit <n>`
2. `[tasks.run] run_multiple_parallel_limit`
3. default `2`

## API And Data Model

### Request Contract

Extend the existing multi-run request additively:

```go
type TaskRunMultipleRequest struct {
	Workspace        string          `json:"workspace"`
	Slugs            []string        `json:"slugs"`
	Mode             string          `json:"mode,omitempty"`
	ParallelLimit    int             `json:"parallel_limit,omitempty"`
	PresentationMode string          `json:"presentation_mode,omitempty"`
	RuntimeOverrides json.RawMessage `json:"runtime_overrides,omitempty"`
}
```

### Snapshot Item Contract

Extend each snapshot item additively:

```go
type TaskRunMultipleItem struct {
	Slug           string `json:"slug"`
	Status         string `json:"status"`
	RunID          string `json:"run_id,omitempty"`
	ErrorText      string `json:"error_text,omitempty"`
	WorktreePath   string `json:"worktree_path,omitempty"`
	BaseBranch     string `json:"base_branch,omitempty"`
	BaseCommit     string `json:"base_commit,omitempty"`
	WorktreeStatus string `json:"worktree_status,omitempty"`
}
```

`TaskRunMultiplePayload` should receive matching optional JSON fields so parent
events can reconstruct snapshots after detach or daemon restart.

### Routes

Keep the existing dedicated routes:

- `POST /api/task-runs/multiple`
- `GET /api/task-runs/multiple/:run_id/snapshot`

Child run snapshots and streams continue to use the existing child run routes.

## Worktree Lifecycle

### Allocation

Parallel mode resolves the parent workspace's current branch and `HEAD` once
when the parent run starts. The parent workspace must be on a named branch.
Starting parallel mode from a detached parent checkout returns a clear
validation error.

For each child item:

1. Build deterministic metadata: slug, index, total, parent run id, base
   branch, base commit, and target worktree path.
2. Emit parent worktree metadata before child launch.
3. Run `git worktree add --detach <path> <base_commit>`.
4. Resolve/register the worktree workspace.
5. Remap the child runtime config to the worktree root and child task dir.
6. Start the normal child `task` run with `ParentRunID` set to the parent.

Detached worktrees avoid branch-name collisions. The metadata still records the
source branch and commit so the user can create branches or merge manually.

### Path Strategy

Use a short home-scoped path outside the tracked repository tree, for example:

```text
~/.compozy/state/worktrees/<workspace-hash>/<parent-short>/<nn-slug>
```

Path requirements:

- deterministic for a parent run and child index
- short enough to avoid daemon/socket path length problems
- safe slug normalization for filesystem names
- parent-run scoped so repeated batches do not collide
- not under `.compozy/tasks` or any tracked source directory

### Preservation

V1 preserves every child worktree regardless of child status. Compozy should not
delete, merge, push, or clean child worktrees automatically. The final handoff
must include enough path and status information for manual inspection.

## Child Runtime And Workspace Ownership

Parallel children execute as ordinary `task` runs, but their physical workspace
is the worktree root.

For each parallel child:

- `RuntimeConfig.WorkspaceRoot = <worktree_path>`
- `RuntimeConfig.TasksDir = <worktree_path>/.compozy/tasks/<slug>`
- `RuntimeConfig.ParentRunID = <parent_run_id>`
- `startRun` receives the workspace row resolved from `<worktree_path>`
- `workflowRoot` points at the worktree task directory

The parent remains registered under the original workspace. Parent and children
are grouped by `parent_run_id` and by the multi-run snapshot, not by a shared
workspace id.

This avoids split runtime state where the database says a child belongs to the
original checkout but task sync, watchers, and extension hooks execute in a
different tree.

## Scheduler Behavior

### Enqueued Branch

Enqueued mode should preserve current behavior:

- preflight all slugs before parent creation
- queue all items
- start one child at a time
- wait for terminal status before starting the next child
- preserve existing attach and stream behavior

Any intentional behavior change for enqueued mode must be covered by tests.

### Parallel Branch

Parallel mode should:

- preflight all slugs before parent creation
- resolve base branch and base commit once
- queue all items before child starts
- use a context-aware semaphore with the resolved limit
- start child runs in goroutines owned by the parent coordinator
- wait for every launched child goroutine before final parent status
- continue scheduling siblings when one child fails or cannot start
- emit item failure when allocation or start fails

Every goroutine must have explicit ownership through the parent context and a
`sync.WaitGroup` or equivalent. There should be no fire-and-forget child
workers.

## Terminal And Cancellation Semantics

### Fail-Late Aggregation

A child failure does not cancel siblings in parallel mode. The parent reaches a
terminal status only after all active children settle and queued children are
accounted for.

Parent status rules:

- `completed` when every child completes
- `failed` when any child fails, crashes, or cannot start
- `canceled` when parent cancellation is requested

The final parent error should summarize failed child slugs and include the
first actionable error message without hiding other failed items.

### Parent Cancellation

Canceling the parent in parallel mode must:

- cancel all running children owned by the parent
- stop scheduling queued children
- mark not-started items canceled
- preserve already allocated worktrees
- wait for cancellation propagation before parent terminal status

Cancellation should be idempotent so repeated stop requests do not emit
conflicting item states.

## Events And Snapshot Reconstruction

Keep the current `task.multi.*` event kinds. Extend payloads additively with
worktree metadata.

Required event properties:

- parent `started` event records mode, slugs, total, and resolved parallel
  limit when parallel
- item `queued` events preserve ordered item state
- worktree metadata is emitted before child launch
- child started/completed/failed/canceled events include the child worktree path
  when available
- queue completed/canceled events include aggregate status and summary error

`RunMultipleSnapshot` remains event-derived. Snapshot reconstruction must
handle old events that do not contain worktree fields.

## UI And Output

### TUI

The multi-run TUI keeps ordered tabs and isolated child stream state. It should
render worktree metadata without introducing a new dashboard:

- tab/status list includes child status and worktree preservation indicator
- selected child detail includes run id and worktree path
- missing worktree metadata renders as empty/unknown for backward
  compatibility

### Stream And Final Summary

Non-TUI output should include a final handoff table or equivalent structured
lines with:

- slug
- final status
- child run id, when started
- worktree path, when allocated

The parent command exits non-zero if the aggregate parent status is failed,
canceled, or crashed.

## Error Handling

Return typed/problem errors for:

- `--parallel` without `--multiple`
- `--parallel-limit` without `--multiple`
- zero or negative parallel limit
- configured invalid `run_multiple_mode`
- configured invalid `run_multiple_parallel_limit`
- daemon receiving unsupported mode or invalid limit
- parent workspace not in a git repository
- parent workspace on detached `HEAD`
- git worktree allocation failure
- missing child task directory inside an allocated worktree

Errors should include enough context to identify the slug, index, parent run,
and worktree path when relevant.

## Testing Approach

### Unit Tests

- Config parsing accepts positive `run_multiple_parallel_limit`.
- Config defaults unset limit to `2`.
- Config rejects zero and negative limits.
- Config merge gives workspace config precedence over global config.
- CLI resolves `--parallel` over config/default mode.
- CLI resolves `--parallel-limit` over config/default limit.
- CLI rejects `--parallel` and `--parallel-limit` without `--multiple`.
- API contract encodes/decodes `parallel_limit` and worktree item metadata.
- Payload compatibility tests accept old events and new worktree fields.

### Daemon Tests

- Daemon accepts `parallel` instead of returning `unsupported_run_multiple_mode`.
- Parallel scheduling never starts more than the configured limit.
- Child failure does not prevent sibling scheduling.
- Parent finishes failed after at least one child fails.
- Parent cancellation cancels all running children and marks queued children
  canceled.
- Worktree metadata is emitted before child start.
- Snapshot reconstruction preserves worktree path, base branch, base commit,
  and preservation status.
- Parallel child run rows use worktree workspace ids.
- Enqueued behavior remains one-child-at-a-time.

### Worktree Tests

- Allocator resolves current branch and commit once.
- Detached parent checkout returns a validation error.
- Allocator runs `git worktree add --detach <path> <base_commit>`.
- Path planner generates short, deterministic, sanitized paths.
- Existing path collision returns a clear error.

### UI And Output Tests

- TUI renders worktree path/status from initial snapshot.
- TUI applies later worktree metadata events to existing tabs.
- Stream mode prints worktree metadata for child events.
- Final summary includes slug, status, run id, and worktree path.

### Verification

Every implementation slice must finish with:

```bash
make verify
```

In this local environment, if `GOROOT` points at a stale local Go install, run:

```bash
env -u GOROOT make verify
```

## Development Sequencing

1. Add config field, effective default helper, merge, validation, and tests.
2. Add `--parallel` and `--parallel-limit` CLI parsing and precedence tests.
3. Extend API contracts, OpenAPI, generated types, and client tests.
4. Extend `TaskRunMultiplePayload` and snapshot item metadata.
5. Add the worktree allocator/path planner and git command boundary.
6. Refactor `task_multi` into one scheduler with enqueued and parallel
   branches.
7. Register/remap parallel child runs under worktree workspace roots.
8. Implement fail-late aggregation and parent cancellation fanout.
9. Update TUI, stream output, and final summary rendering.
10. Update README/config docs and end-to-end coverage.

## Monitoring And Observability

Log structured fields:

- `parent_run_id`
- `child_run_id`
- `workflow_slug`
- `index`
- `total`
- `run_multiple_mode`
- `parallel_limit`
- `worktree_path`
- `base_branch`
- `base_commit`

Metrics should continue counting `task_multi` active and terminal runs. If
child fanout needs observability, add counters for worktree allocation failures
and parallel child start failures rather than inferring them from logs.

## Technical Considerations

### Key Decisions

- Use one `task_multi` scheduler with `enqueued` and `parallel` branches.
  Rationale: keeps events, snapshots, cancellation, and terminal aggregation in
  one model.
- Register children under worktree workspaces.
  Rationale: aligns database workspace identity with runtime paths, task sync,
  watchers, and extension hooks.
- Persist worktree metadata in parent events.
  Rationale: matches the current event-derived snapshot architecture and avoids
  a new DB table in V1.
- Use detached worktrees from current branch `HEAD`.
  Rationale: avoids branch collisions while preserving enough metadata for
  manual review.
- Make the parallel limit configurable with default `2`.
  Rationale: conservative default with explicit user control for local
  capacity.

### Known Risks

- Normal original-workspace run lists may not show worktree-owned child rows.
  Mitigate by making the multi-run snapshot, TUI, stream, and final summary the
  canonical child view.
- High parallel limits can overload local resources or provider quotas.
  Mitigate with conservative default, validation, and documentation.
- Worktrees can be orphaned after interruption.
  Mitigate by emitting metadata before child launch and preserving paths for
  manual cleanup.
- Independent tasks can still conflict semantically.
  Mitigate by documenting that V1 does not merge or predict conflicts.

## Architecture Decision Records

- [ADR-001: Use Multi-Task Execution as Explicit Task-Run Orchestration](adrs/adr-001.md) - Treat multi-task execution as orchestration over independent task workflows.
- [ADR-002: Introduce a Dedicated Multi-Run Command for V1](adrs/adr-002.md) - Earlier command-shape decision, superseded by the current `tasks run --multiple` surface where implementation already moved there.
- [ADR-003: Fix V1 Command Name and Config Behavior](adrs/adr-003.md) - Earlier config and fallback decision, superseded where it conflicts with true parallel mode.
- [ADR-004: Use a Daemon-Owned Sequential Multi-Run Coordinator](adrs/adr-004.md) - Baseline daemon-owned parent model preserved by this TechSpec.
- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md) - Accept true parallel mode with git worktree isolation.
- [ADR-006: Use a Speed-First Parallel MVP for the PRD Scope](adrs/adr-006.md) - Defines the product scope for fast independent batches.
- [ADR-007: Use One Task-Multi Scheduler with Worktree-Owned Child Runs](adrs/adr-007.md) - Defines the scheduler, workspace ownership, and event metadata architecture.
- [ADR-008: Make the Parallel Multi-Run Limit Configurable](adrs/adr-008.md) - Defines the configurable fanout limit and precedence.
