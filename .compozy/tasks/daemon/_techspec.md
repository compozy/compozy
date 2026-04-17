# Technical Specification: Home-Scoped Daemon Runtime for Compozy

## Executive Summary

This specification moves Compozy from per-command execution to a single home-scoped daemon that wraps the existing run engine instead of replacing it. No `_prd.md` exists for this feature; this document is derived from the approved daemon architecture review, the accepted ADRs in `adrs/`, and direct exploration of the current Compozy and AGH codebases. The daemon owns runtime lifecycle, workspace registration, transport surfaces, and operational state, while Markdown workflow artifacts remain in the workspace as the human-authoritative source for PRDs, TechSpecs, ADRs, task files, review issues, memory files, protocol files, prompt files, and QA outputs.

The implementation reuses AGH's mature operational patterns where they fit: home path resolution, staged daemon boot, singleton lock and readiness handling, shared Gin-based HTTP and UDS transports, global-plus-per-run SQLite storage, and observer-style status queries. The primary trade-off is deliberate migration work across CLI, persistence, and public run readers in exchange for a stronger local platform: better failover, durable attach and watch behavior, richer extension hooks, and a clean path for a future web client without introducing AGH's automation domain into Compozy.

## System Architecture

### Component Overview

1. `CLI/TUI clients` (`internal/cli`, `internal/core/run/ui`)
   - Replace direct execution entrypoints with daemon-aware commands.
   - Canonical command families:
     - `compozy daemon start|stop|status`
     - `compozy workspaces list|show|register|unregister|resolve`
     - `compozy sync`
     - `compozy archive`
     - `compozy tasks list|show|validate|run`
     - `compozy reviews fetch|list|show|fix`
     - `compozy runs list|show|watch|attach|cancel`
     - `compozy exec`
   - Existing `compozy agents ...` and `compozy ext ...` command families remain available in v1 and stay out of the daemon API until a follow-up spec moves their state onto the daemon control plane.
   - Remove `compozy start`.
   - Preserve TUI-first behavior for interactive `tasks run` and `reviews fix`.

2. `Daemon host` (`internal/daemon`, new)
   - Resolve `~/.compozy` paths from `$HOME`.
   - Implement AGH-style `ensureDaemon()` bootstrap, lock acquisition, stale socket cleanup, startup reconciliation, readiness probes, and graceful shutdown.
   - Do not report `ready` until `global.db` migrations complete, interrupted runs are reconciled, UDS is listening, and localhost HTTP is listening when enabled.
   - Own the daemon lifecycle context for all long-running goroutines.

3. `Config and workspace registry` (minimal home config package plus `internal/core/workspace`)
   - Load daemon-global config from `~/.compozy/config.toml`.
   - Merge workspace-local `.compozy/config.toml` at run time for the selected workspace.
   - Standardize attach defaults under `[runs].default_attach_mode`, where `auto` means `ui` on interactive TTYs and `stream` otherwise.
   - Resolve `auto` on the client side before the start request is sent; the daemon receives only explicit `presentation_mode` values (`ui`, `stream`, or `detach`).
   - Register workspaces lazily on first touch and explicitly through operator commands and REST endpoints.

4. `Artifact sync service` (`internal/core/sync.go`, task/review/memory parsers, new persistence adapters)
   - Parse Markdown artifacts from `.compozy/tasks/<slug>/`.
   - Upsert structured workflow state into `global.db`.
   - Perform pre-run reconciliation automatically for `tasks run` and `reviews fix`.
   - Run `fsnotify` watchers only for the active workflow or review run.
   - Stop generating `_tasks.md` and `_meta.md`.

5. `Run manager` (`internal/core/run`, `internal/core/model.RunScope`, `internal/core/extension/runtime`)
   - Reuse the current planner, executor, journal, event bus, and extension runtime.
   - Add daemon-owned orchestration for run creation, cancellation, snapshot queries, attach, watch, startup reconciliation, retention, and purge.
   - Enforce same-`run_id` exclusion through a transactional `global.db.runs` insert while allowing different run IDs to execute concurrently in the same workspace.
   - Mirror terminal status into both `global.db` and `run.db` before the run is considered complete.

6. `Persistence` (`internal/store/globaldb`, `internal/store/rundb`, new)
   - `global.db` stores workspace registry, workflow snapshots, parsed task and review state, run index rows, and sync checkpoints.
   - `run.db` stores append-only run events, transcript state, hook audit records, token usage, and active-run artifact sync history.
   - Use `modernc.org/sqlite` with WAL and AGH-aligned connection and recovery helpers.
   - Treat `run.db` as a daemon-owned write surface, not as a public read contract.

7. `Transport layer` (`internal/api/core`, `internal/api/httpapi`, `internal/api/udsapi`, new)
   - Expose shared REST resources over UDS for CLI/TUI and localhost HTTP for web and local clients.
   - Use `gin`, shared handler cores, and SSE helpers adapted from AGH.
   - Emit a uniform JSON error envelope with `request_id` and propagate `X-Request-Id` through logs and responses.
   - Bind HTTP only to `127.0.0.1` in v1, use an ephemeral port persisted in `daemon.json`, and set UDS permissions to `0600` under a daemon directory with `0700`.
   - Keep JSON-RPC only at extension subprocess boundaries and ACP at agent runtime boundaries.

8. `Public run readers and observability` (`pkg/compozy/runs`, `internal/observe`, modified/new)
   - Migrate public run inspection from workspace-local JSON files to daemon-backed snapshot and stream queries.
   - Surface daemon health, registered workspaces, active runs, run attach snapshots, run event streams, sync status, and Prometheus-style metrics through API and CLI.

### Data Flow

1. A user runs `compozy tasks run my-feature` from any registered workspace.
2. The CLI resolves the workspace root, resolves `--attach auto` against the current TTY, calls `ensureDaemon()`, and connects over UDS.
3. The daemon resolves or registers the workspace, merges home and workspace config, and runs sync for `my-feature`.
4. Sync parses Markdown artifacts, updates `global.db`, emits workflow sync events, and starts a scoped watcher for the active workflow.
5. The run manager creates a `run_id`, inserts the `runs` row transactionally into `global.db`, allocates `~/.compozy/runs/<run-id>/run.db`, and opens the existing Compozy run scope.
6. The existing executor publishes run events to the daemon event stream, which writes to `run.db`, updates projection tables, and broadcasts to attached clients over SSE and the TUI.
7. `compozy runs show`, `watch`, and `attach` all read the same daemon-owned run state through `GET /runs/:run_id`, `GET /runs/:run_id/snapshot`, and `GET /runs/:run_id/stream`.
8. On daemon restart, startup reconciliation marks interrupted runs as `crashed`, appends a synthetic `run.crashed` event where possible, and only then reopens the transport surfaces for new work.

## Implementation Design

### Core Interfaces

```go
type HomePaths struct {
    HomeDir      string
    ConfigFile   string
    AgentsDir    string
    ExtensionsDir string
    StateDir     string
    DaemonDir    string
    SocketPath   string
    LockPath     string
    InfoPath     string
    DBDir        string
    GlobalDBPath string
    RunsDir      string
    LogsDir      string
    CacheDir     string
}
```

```go
type WorkspaceRegistry interface {
    ResolveOrRegister(ctx context.Context, path string) (Workspace, error)
    Register(ctx context.Context, path, name string) (Workspace, error)
    Unregister(ctx context.Context, id string) error
    Get(ctx context.Context, idOrPath string) (Workspace, error)
    List(ctx context.Context) ([]Workspace, error)
}
```

```go
type RunManager interface {
    StartTaskRun(ctx context.Context, req TaskRunRequest) (RunSnapshot, error)
    StartReviewRun(ctx context.Context, req ReviewRunRequest) (RunSnapshot, error)
    StartExecRun(ctx context.Context, req ExecRunRequest) (RunSnapshot, error)
    Snapshot(ctx context.Context, runID string) (RunSnapshot, error)
    AttachSnapshot(ctx context.Context, runID string) (RunAttachSnapshot, error)
    Watch(ctx context.Context, runID string, cursor string) (<-chan RunEvent, error)
    Cancel(ctx context.Context, runID string, reason string) error
    PurgeTerminalRuns(ctx context.Context, req RunPurgeRequest) (RunPurgeResult, error)
}
```

All daemon service methods take `context.Context` as the first parameter. Errors are wrapped with `%w` and translated to transport status codes in the shared handler layer. Every daemon-owned goroutine is bound to the daemon lifecycle context or to the owning run context; there are no fire-and-forget goroutines.

```go
type TransportError struct {
    Error struct {
        Code    string         `json:"code"`
        Message string         `json:"message"`
        Details map[string]any `json:"details,omitempty"`
    } `json:"error"`
    RequestID string `json:"request_id"`
}
```

### Data Models

#### Stable Home Layout

The daemon owns the following stable filesystem contract:

```text
~/.compozy/
  config.toml
  agents/
  extensions/
  state/
  daemon/
    daemon.sock
    daemon.lock
    daemon.json
  db/
    global.db
  runs/
    <run-id>/
      run.db
  logs/
    daemon.log
  cache/
```

Temporary prompt files or process scratch files may still exist during execution, but they are not part of the persisted contract.

Operational permissions are part of the contract:

- `~/.compozy/daemon/` is created with `0700`.
- `daemon.sock` is created with `0600`, matching AGH's UDS posture.
- `daemon.json` persists daemon PID, version, socket path, chosen HTTP port, boot timestamp, and readiness state.
- Localhost HTTP binds explicitly to `127.0.0.1` in v1 and uses an ephemeral port recorded in `daemon.json`.
- `daemon.log` rotates at 50 MiB with 5 retained files.

#### Identity Rules

- `workspace_id`: opaque daemon-owned identifier for one registered workspace.
- `workflow slug`: human-facing identifier mapped to `.compozy/tasks/<slug>` and unique among active, non-archived workflows inside one workspace.
- `run_id`: operational execution identifier generated by the daemon by default. Explicit `run_id` is allowed only for advanced flows such as replay, debugging, import, or manual attach.
- `run_id` is globally unique in `global.db.runs` and conflicts return `409` rather than racing in process memory.

#### `global.db`

| Table | Purpose | Key fields |
| --- | --- | --- |
| `workspaces` | Registered workspace catalog | `id`, `root_dir` (unique), `name`, `created_at`, `updated_at` |
| `workflows` | One row per active or archived workflow slug | `id`, `workspace_id`, `slug`, `archived_at`, `last_synced_at`, `created_at`, `updated_at` |
| `artifact_snapshots` | Parsed Markdown snapshot index for web and CLI queries | `workflow_id`, `artifact_kind`, `relative_path`, `checksum`, `frontmatter_json`, `body_text`, `body_storage_kind`, `source_mtime`, `synced_at` |
| `task_items` | Structured task file state | `workflow_id`, `task_number`, `task_id`, `title`, `status`, `kind`, `depends_on_json`, `source_path`, `updated_at` |
| `review_rounds` | Review round metadata | `workflow_id`, `round_number`, `provider`, `pr_ref`, `resolved_count`, `unresolved_count`, `updated_at` |
| `review_issues` | Structured review issue state | `round_id`, `issue_number`, `severity`, `status`, `source_path`, `updated_at` |
| `runs` | Run index and lifecycle state | `run_id`, `workspace_id`, `workflow_id`, `mode`, `status`, `presentation_mode`, `started_at`, `ended_at`, `error_text`, `request_id` |
| `sync_checkpoints` | Sync cursor and watcher state | `workflow_id`, `scope`, `checksum`, `last_scan_at`, `last_success_at`, `last_error_text` |

`global.db` must also carry a `schema_migrations` table, plus unique constraints on normalized workspace roots, `(workspace_id, slug)` for active workflows, and `run_id` for run creation. `artifact_snapshots.body_text` is stored only when the checksum changes, is capped at 256 KiB per row, and spills to an overflow reference when a rendered body exceeds that cap. `sync` writes to these tables but does not regenerate `_tasks.md` or any `_meta.md`.

#### `run.db`

| Table | Purpose | Key fields |
| --- | --- | --- |
| `events` | Canonical append-only run event stream | `sequence`, `event_kind`, `payload_json`, `timestamp`, `job_id`, `step_key` |
| `job_state` | Latest per-job snapshot for quick reads | `job_id`, `task_id`, `status`, `agent_name`, `summary_json`, `updated_at` |
| `transcript_messages` | Structured transcript view for attach and replay | `sequence`, `stream`, `role`, `content`, `metadata_json`, `timestamp` |
| `hook_runs` | Extension and runtime hook audit log | `id`, `hook_name`, `source`, `outcome`, `duration_ns`, `payload_json`, `recorded_at` |
| `token_usage` | Optional token and cost accounting | `turn_id`, `input_tokens`, `output_tokens`, `total_tokens`, `cost_amount`, `timestamp` |
| `artifact_sync_log` | Active-run file watcher history | `sequence`, `relative_path`, `change_kind`, `checksum`, `synced_at` |

`run.db` must also carry a `schema_migrations` table. It should follow AGH's per-session writer-loop pattern so that event writes remain serialized and replay ordering stays deterministic. Direct `run.db` reads are an internal implementation detail; public readers go through daemon snapshots, pagination, and streaming APIs instead of opening SQLite files directly.

#### Run Lifecycle and Recovery

- The daemon performs startup reconciliation before it reports ready:
  - scan `global.db.runs` for rows in `starting` or `running`
  - mark them `crashed` with `ended_at=now`
  - append a synthetic `run.crashed` event to the matching `run.db` when the DB is still present and openable
  - record a best-effort error summary in `global.db.runs.error_text` when the per-run DB cannot be updated
- `StartTaskRun`, `StartReviewRun`, and `StartExecRun` create the run directory and `run.db`, then insert the `global.db.runs` row inside one transaction before any external process starts.
- Terminal states (`completed`, `failed`, `cancelled`, `crashed`) are mirrored into both databases before transports report the run as finished.
- Retention is explicit:
  - `[runs].keep_terminal_days = 14`
  - `[runs].keep_max = 200`
  - `compozy runs purge` deletes retained run directories and compacted rows in oldest-first order
- `daemon stop` returns `409` while active runs exist unless `force=true` is supplied. Forced shutdown cancels active runs, waits up to 30 seconds for writer loops and child processes to drain, sends `SIGTERM` to extension subprocesses, and flushes state before exit.

#### Sync and Archive Semantics

- `compozy sync` and `POST /api/sync` parse Markdown and update `global.db`. They no longer write `_tasks.md` or `_meta.md`.
- `tasks run` and `reviews fix` always run sync before the run starts.
- During an active run, a scoped watcher reparses only changed workflow artifacts and emits `artifact.synced` events.
- Watchers cover the active workflow's task, review, protocol, prompt, ADR, QA, and `memory/` files. Writes outside the active workflow root are logged but not reparsed.
- Watchers debounce bursty writes with a default 500 ms delay and store the effective checkpoint in `sync_checkpoints`.
- The first sync on a legacy workflow removes or renames generated `_tasks.md` and `_meta.md` artifacts, records a single migration warning, and never recreates them.
- `compozy archive` still moves the workflow directory to `.compozy/tasks/_archived/<timestamp-ms>-<shortid>-<slug>`, but archive eligibility is computed from synced DB state rather than filesystem `_meta.md` files and returns `409` if an active run still exists for that workflow.

### API Endpoints

UDS and localhost HTTP expose the same route set. UDS is the default CLI transport; localhost HTTP is for web and local clients. All routes live under `/api`.

#### Daemon

| Method | Path | Purpose | Success | Notes |
| --- | --- | --- | --- | --- |
| `GET` | `/daemon/status` | Return daemon pid, version, start time, socket path, HTTP port, active run count, and workspace count | `200` | Primary health view |
| `GET` | `/daemon/health` | Return readiness and degraded-state details | `200` or `503` | JSON health view for CLI and web |
| `GET` | `/daemon/metrics` | Return Prometheus text metrics | `200` | Local-only operational scrape surface |
| `POST` | `/daemon/stop` | Request graceful shutdown | `202` or `409` | Used by `compozy daemon stop`; `force=true` bypasses active-run rejection |

`daemon start` is handled by client-side bootstrap and does not require a running API server beforehand.

#### Workspaces

| Method | Path | Purpose | Success | Notes |
| --- | --- | --- | --- | --- |
| `POST` | `/workspaces` | Register a workspace explicitly | `201` or `200` | Idempotent on normalized path |
| `GET` | `/workspaces` | List registered workspaces | `200` | Supports future filtering |
| `GET` | `/workspaces/:id` | Fetch one workspace by id or normalized path key | `200` | Used by `workspaces show` |
| `PATCH` | `/workspaces/:id` | Update workspace display metadata | `200` | Limited to operator-managed fields |
| `DELETE` | `/workspaces/:id` | Unregister a workspace | `204` | Reject if active runs exist |
| `POST` | `/workspaces/resolve` | Resolve or lazily register a workspace path | `200` | Used by most other commands |

#### Task workflows

| Method | Path | Purpose | Success | Notes |
| --- | --- | --- | --- | --- |
| `GET` | `/tasks` | List task workflows for a workspace | `200` | Returns synced workflow summaries |
| `GET` | `/tasks/:slug` | Return one workflow summary | `200` | Includes sync and archive state |
| `GET` | `/tasks/:slug/items` | List parsed `task_XX.md` rows | `200` | Backed by `task_items` |
| `POST` | `/tasks/:slug/validate` | Validate task files without starting a run | `200` or `422` | No Markdown mutation |
| `POST` | `/tasks/:slug/runs` | Start a task run | `201` | Request includes attach mode and runtime overrides |
| `POST` | `/tasks/:slug/archive` | Archive a completed workflow | `200` or `409` | Reject if tasks or reviews remain unresolved |

#### Reviews

| Method | Path | Purpose | Success | Notes |
| --- | --- | --- | --- | --- |
| `POST` | `/reviews/:slug/fetch` | Import provider feedback into a review round | `201` or `200` | Accepts provider, PR ref, and optional round |
| `GET` | `/reviews/:slug` | Return latest review summary for a workflow | `200` | Includes latest round pointer |
| `GET` | `/reviews/:slug/rounds/:round` | Return one review round summary | `200` | Backed by `review_rounds` |
| `GET` | `/reviews/:slug/rounds/:round/issues` | List issue rows for a round | `200` | Backed by `review_issues` |
| `POST` | `/reviews/:slug/rounds/:round/runs` | Start a review-fix run | `201` | Request includes attach mode and batching |

#### Runs

| Method | Path | Purpose | Success | Notes |
| --- | --- | --- | --- | --- |
| `GET` | `/runs` | List runs across workspaces or one workspace | `200` | Filters by workspace, status, and mode |
| `GET` | `/runs/:run_id` | Return the latest run snapshot | `200` | Used by `runs show` |
| `GET` | `/runs/:run_id/snapshot` | Return dense attach state for TUI and web clients | `200` | Includes job tree, step state, latest transcript window, and next cursor |
| `GET` | `/runs/:run_id/events` | Page through persisted events | `200` | Cursor-based pagination |
| `GET` | `/runs/:run_id/stream` | SSE stream for live observation | `200` | Used by `runs watch` and web clients |
| `POST` | `/runs/:run_id/cancel` | Cancel an active run | `202` | Idempotent on completed runs |

`runs attach` is a client behavior built on `GET /runs/:run_id/snapshot` plus `GET /runs/:run_id/stream`; v1 does not require a dedicated attach endpoint.

#### Sync and exec

| Method | Path | Purpose | Success | Notes |
| --- | --- | --- | --- | --- |
| `POST` | `/sync` | Reconcile one workflow or an entire workspace into `global.db` | `200` | Used by top-level `compozy sync` |
| `POST` | `/exec` | Start an ad hoc daemon-backed exec run | `201` | Binds to `cwd`, auto-registers the workspace, skips workflow sync, and persists `mode=exec` |

#### Transport Contract

- All JSON responses include `X-Request-Id`, and all non-2xx responses serialize `TransportError`.
- Conflict cases use `409`:
  - duplicate `run_id`
  - archive while a workflow run is active
  - daemon stop without `force=true` while runs are active
  - workspace unregister while runs are active
- Validation failures use `422`.
- `schema_too_new` returns `409` with remediation details instead of silently opening newer SQLite files.

#### SSE Contract

- `GET /runs/:run_id/stream` uses standard SSE frames with `id:`, `event:`, and `data:`.
- `id` follows AGH's stable cursor shape: `RFC3339Nano|sequence`.
- `Last-Event-ID` resumes from the next event after the supplied cursor.
- The server emits heartbeat frames every 15 seconds while the stream is idle.
- Slow consumers receive an `event: overflow` frame and must reconnect from the last acknowledged cursor.
- `GET /runs/:run_id/snapshot` returns the dense state needed to render the initial TUI or web view without replaying the entire stream from sequence zero.

### Integration Points

1. `Workspace filesystem`
   - Purpose: read Markdown workflow artifacts and archive completed workflows.
   - Auth: none; local filesystem access only.
   - Error handling: parse and validation failures return command and API errors without crashing the daemon. Active watchers debounce changes and reparse only affected files.

2. `SQLite`
   - Purpose: durable operational state.
   - Auth: local file permissions only.
   - Error handling: reuse AGH-style WAL configuration, busy timeout handling, integrity validation, schema migration bookkeeping, and best-effort recovery helpers. Fatal DB open failures keep the daemon from reporting ready, and newer unsupported schemas fail with `schema_too_new`.

3. `Extension subprocesses`
   - Purpose: preserve current executable extension model.
   - Auth: same run-scoped permissions and config that exist today, plus a per-run Host API capability token injected through the environment when daemon callbacks are required.
   - Error handling: extensions continue to speak stdio JSON-RPC to the run process. JSON-RPC failures are recorded in `run.db` hook tables and surfaced through the run event stream; they must not crash unrelated runs.

4. `Agent runtimes and IDE subprocesses`
   - Purpose: continue using ACP and existing downstream coding tools.
   - Auth: unchanged from current Compozy runtime flags and permission modes.
   - Error handling: child process exits and runtime hook failures are captured as run events and reflected in run status rows.

5. `Local clients`
   - Purpose: UDS for CLI and TUI, localhost HTTP for web and other local tools.
   - Auth: no auth in v1. The daemon must bind HTTP only to `127.0.0.1`, keep TCP disabled by default, create UDS with `0600`, and persist the selected HTTP port in `daemon.json`.
   - Error handling: UDS and HTTP share handler logic, request IDs, JSON error envelopes, and status mapping so transport choice does not change behavior.

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|---------------------|-----------------|
| `internal/cli` | modified | High risk. Direct command execution paths must become daemon clients without regressing current UX. | Replace `start`/`fix-reviews`/`sync`/`archive`/`exec` flows with daemon requests and add `daemon`, `workspaces`, `tasks`, `reviews`, and `runs` command families. |
| `internal/cli/workspace_config.go` | modified | Medium risk. Config loading is currently CLI-owned and not reusable by a daemon. | Extract shared config resolution so daemon host and clients can both merge home and workspace config. |
| `internal/core/run`, `internal/core/model.RunScope` | modified | High risk. This is the execution seam that must remain behaviorally stable while gaining daemon ownership. | Wrap the existing planner and executor in a daemon run manager; keep current execution semantics. |
| `internal/core/sync.go`, task/review/memory stores | modified | High risk. Current sync and archive semantics depend on `_meta.md`. | Convert sync into DB reconciliation and move archive eligibility checks to `global.db` state. |
| `internal/core/extension` | modified | Medium risk. Extension lifecycle remains per-run but now depends on daemon-owned run context. | Keep stdio JSON-RPC boundaries, add per-run Host API capability tokens, and adapt runtime ownership to daemon-managed runs. |
| `pkg/compozy/runs` | modified | High risk. Public readers assume workspace-local run layout. | Rework readers to use daemon-backed snapshots, pagination, and streaming instead of reading workspace-local files or opening SQLite directly. |
| `internal/core/kernel` and adapters | modified | Medium risk. Existing dispatcher adapters bridge old CLI shapes into core execution. | Add daemon-backed command handlers while preserving typed command boundaries. |
| `internal/daemon` | new | High risk. New host runtime and singleton lifecycle are foundational. | Implement home path resolution, lock and info handling, readiness probes, boot ordering, and shutdown. |
| `internal/api/core`, `internal/api/httpapi`, `internal/api/udsapi` | new | Medium risk. New transport layer adds route, request-ID, health, metrics, and SSE contracts. | Port AGH transport patterns, shared handlers, route parity, request-ID middleware, and health surfaces. |
| `internal/store/globaldb`, `internal/store/rundb`, `internal/observe` | new | Medium risk. New durable state model underpins all daemon features. | Implement schemas, migrations, writer loops, and observer queries aligned with AGH patterns. |

## Testing Approach

### Unit Tests

- Home path resolution and daemon bootstrap state transitions:
  - resolved paths always root at `$HOME`
  - stale lock, stale socket, and stale daemon info cleanup
  - `daemon start` idempotence
- Workspace registry rules:
  - path normalization
  - lazy register and explicit register parity
  - unregister rejection with active runs
  - duplicate path races collapse to one row through a single DB transaction
- Sync logic:
  - Markdown parsing into `artifact_snapshots`, `task_items`, `review_rounds`, and `review_issues`
  - no `_tasks.md` or `_meta.md` writes
  - one-time legacy `_tasks.md` and `_meta.md` cleanup
  - archive eligibility computed from DB rows
- Run manager rules:
  - same `run_id` rejection
  - concurrent different `run_id`s in the same workspace
  - attach mode resolution from config and flags on the client side
  - startup reconciliation of interrupted runs
  - retention and purge ordering
- Handler tests:
  - UDS and HTTP status mapping parity
  - request-id propagation and error envelope shape
  - SSE cursor behavior
  - validation and conflict responses (`409`, `422`)

Tests should prefer real temporary SQLite databases and temp directories over mocks. Mocking is acceptable only at IDE, extension subprocess, and external provider boundaries.

### Integration Tests

- `ensureDaemon()` bootstraps a daemon automatically, reuses an already healthy daemon, and recovers from stale singleton artifacts.
- UDS and localhost HTTP serve the same route behavior for daemon status, health, metrics, workspaces, task runs, review runs, and run streams.
- `tasks run` auto-syncs before execution and starts a scoped watcher that updates DB state when Markdown changes during the run.
- `reviews fix` auto-syncs before execution and keeps live review issue state aligned with manual file edits.
- `sync` updates `global.db` but does not recreate `_meta.md`.
- `archive` moves `.compozy/tasks/<slug>` into `.compozy/tasks/_archived/<timestamp-ms>-<shortid>-<slug>` only when DB state says the workflow is complete.
- `pkg/compozy/runs` uses daemon-backed snapshot and stream APIs and surfaces event playback correctly without reading SQLite directly.
- Different run IDs in the same workspace can run concurrently; the same `run_id` returns a conflict.
- `compozy runs watch` can reconnect to a persisted run stream after client disconnect.
- `compozy runs attach` restores from `GET /runs/:run_id/snapshot` and then continues from the returned cursor.
- A daemon crash mid-run is reconciled on restart and leaves the run in `crashed` with a synthetic recovery event.
- `compozy exec` binds to the current workspace, auto-registers it, skips workflow sync, and persists `mode=exec`.

## Development Sequencing

### Build Order

1. Home path resolution and daemon bootstrap - no dependencies.
2. Global DB schema, migrations, and workspace registry - depends on step 1.
3. Per-run DB schema, writer loop, startup reconciliation, and run index integration - depends on steps 1 and 2.
4. Sync rewrite, legacy metadata cleanup, and archive gating against DB state - depends on steps 2 and 3.
5. Shared transport layer (`api/core`, UDS, HTTP, health, metrics, request IDs, SSE) - depends on steps 1, 2, 3, and 4.
6. Daemon run manager around the existing Compozy executor, including retention and purge - depends on steps 3 and 5.
7. CLI rewrite and `ensureDaemon()` client path, including client-side attach mode resolution - depends on steps 4, 5, and 6.
8. TUI attach and watch behavior plus `pkg/compozy/runs` daemon-client migration - depends on steps 5, 6, and 7.
9. Review fetch and fix flows, exec flow, workspace commands, and shutdown semantics - depends on steps 5, 6, and 7.
10. End-to-end migration cleanup, docs, and regression coverage - depends on steps 4 through 9.

### Technical Dependencies

- Add `github.com/gin-gonic/gin` for the daemon transport layer, matching AGH.
- Add `modernc.org/sqlite` and AGH-aligned SQLite helpers for WAL, busy timeout, and recovery.
- Reuse existing `github.com/fsnotify/fsnotify`; do not introduce a global always-on watch service.
- No web frontend implementation is required to complete this TechSpec. The daemon API and transport surfaces are the scope boundary.

## Monitoring and Observability

- Key metrics and counters:
  - `daemon_active_runs`
  - `daemon_registered_workspaces`
  - `daemon_boot_failures_total`
  - `daemon_shutdown_conflicts_total`
  - `workflow_sync_duration_ms`
  - `workflow_sync_failures_total`
  - `run_event_stream_lag_ms`
  - `runs_starting_stuck_total`
  - `runs_reconciled_crashed_total`
- Structured log fields:
  - `request_id`
  - `workspace_id`
  - `workspace_root`
  - `workflow_slug`
  - `run_id`
  - `mode`
  - `presentation_mode`
  - `transport`
  - `artifact_path`
  - `checksum`
  - `sync_reason`
  - `event_sequence`
- Health surfaces:
  - `GET /daemon/health` returns JSON readiness and degraded-state details for CLI and local web use
  - `GET /daemon/metrics` returns Prometheus text format
- Readiness and thresholds:
  - daemon reports unhealthy if DB open, migration, or startup reconciliation fails
  - daemon reports unhealthy until required transports are listening
  - daemon reports degraded if active-run watcher lag exceeds `[health].watcher_lag_degraded_after`
  - daemon reports degraded if a run remains in `starting` without events beyond `[health].starting_stuck_after`
  - daemon reports degraded if repeated boot failures exceed `[health].boot_failures_degraded_threshold` inside `[health].boot_failure_window`

## Technical Considerations

### Key Decisions

- Decision: one home-scoped daemon per user and machine.
  - Rationale: matches AGH's operational posture and enables multi-workspace coordination, reattach, and web access.
  - Trade-offs: requires singleton lifecycle handling and an explicit workspace registry.
  - Alternatives rejected: per-command helpers and one-daemon-per-workspace.

- Decision: keep Markdown workflow artifacts in the workspace and move operational state to SQLite.
  - Rationale: preserves Compozy's existing artifact-centric workflow while providing a durable control plane.
  - Trade-offs: sync semantics must be explicit and tested carefully.
  - Alternatives rejected: DB-only workflows and continued dependence on workspace-local `.compozy/runs/`.

- Decision: use `global.db` plus per-run `run.db`.
  - Rationale: mirrors AGH's proven split between global registry state and session-local event state.
  - Trade-offs: introduces two migration and query surfaces instead of one.
  - Alternatives rejected: single monolithic DB and JSONL-only event files.

- Decision: `pkg/compozy/runs` becomes daemon-backed instead of reading SQLite directly.
  - Rationale: keeps one read contract for CLI, TUI, and future web clients while letting the daemon own SQLite concurrency and recovery semantics.
  - Trade-offs: public readers now depend on daemon availability or client bootstrap instead of raw filesystem access.
  - Alternatives rejected: direct read-only SQLite access and dual filesystem/daemon codepaths.

- Decision: REST over shared Gin handlers for UDS and localhost HTTP.
  - Rationale: gives CLI, TUI, and future web clients one transport contract while reusing AGH route and SSE patterns.
  - Trade-offs: adds a transport layer that does not exist today.
  - Alternatives rejected: command-style HTTP endpoints and JSON-RPC for all surfaces.

- Decision: `sync` becomes DB reconciliation and active-run watch, not metadata file generation.
  - Rationale: `_tasks.md` and `_meta.md` are removed from the daemonized model.
  - Trade-offs: current sync and archive code must be rewritten around DB state.
  - Alternatives rejected: preserving generated metadata files as a compatibility layer.

- Decision: preserve TUI-first UX with `tasks run` and `reviews fix`.
  - Rationale: interactive users should not experience the daemon as a regression.
  - Trade-offs: attach mode defaults and detached behavior need careful documentation and testing.
  - Alternatives rejected: stream-first default output and explicit attach-only workflows.

- Decision: no auth for localhost HTTP in v1.
  - Rationale: local-only binding is enough for the first release and keeps the transport model simple.
  - Trade-offs: binding rules, socket permissions, and request tracing must stay strict.
  - Alternatives rejected: introducing an auth scheme before a non-local deployment model exists.

### Known Risks

- Risk: `pkg/compozy/runs` migration breaks callers that assume workspace-local JSON files.
  - Likelihood: high.
  - Mitigation: add dedicated compatibility tests and keep the public API stable even as the backing store changes.

- Risk: sync and watcher logic drift from Markdown reality or miss updates.
  - Likelihood: medium.
  - Mitigation: use checksums plus mtimes, emit explicit sync events, and cover manual edit scenarios in integration tests.

- Risk: archive behavior changes unexpectedly when `_meta.md` is removed.
  - Likelihood: medium.
  - Mitigation: move archive eligibility entirely to synced DB state and add regression tests covering completed and incomplete workflows.

- Risk: daemon bootstrap deadlocks or leaves stale singleton artifacts after crashes.
  - Likelihood: medium.
  - Mitigation: follow AGH's staged boot, cleanup ordering, and readiness probe model closely.

- Risk: attach mode semantics confuse users who expect current `start` behavior.
  - Likelihood: medium.
  - Mitigation: keep `tasks run` TUI-first in interactive terminals, add explicit `--detach`, `--stream`, and `--ui` flags, and support `runs attach`.

- Risk: per-run storage and snapshot tables grow faster than expected on large workflows.
  - Likelihood: medium.
  - Mitigation: cap inline snapshot bodies, make retention explicit, add `runs purge`, and rotate daemon logs by size.

## Architecture Decision Records

- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — Define one `$HOME`-rooted daemon per user and machine with AGH-style singleton boot semantics.
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — Keep Markdown workflow artifacts in the workspace while moving operational truth to `global.db` and `run.db`.
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — Standardize on shared REST resources over UDS and localhost HTTP with SSE.
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — Keep interactive TUI defaults while adding daemon auto-start, attach modes, and explicit workspace registry operations.
