# Technical Specification: Daemon Hardening via Canonical Contracts and Runtime Supervision

## Executive Summary

No `_prd.md` exists for this feature. This TechSpec is derived from the approved design choices in this session, the consolidated daemon improvement analysis in `analysis.md`, and the supporting domain analyses for transport/API, task runtime, observability/storage, testing harness, and reconciliation.

The implementation keeps the current daemon architecture as the baseline: `Host` owns daemon bootstrap and transport lifecycle, `RunManager` owns active runs, `global.db` remains the home-scoped operational catalog, and `run.db` remains the per-run durable store. The primary structural refactor is the introduction of `internal/api/contract` as the canonical daemon contract package consumed by handlers, `apiclient`, SSE helpers, and `pkg/compozy/runs`. The primary hardening track stays inside the existing daemon boundary and closes the operational gaps around signal handling, bounded shutdown, orphan reaping, checkpoint discipline, process termination, ACP liveness, and recovery honesty.

The design deliberately excludes AGH subsystems that do not fit looper's local-first workflow daemon: resource kernel, bridge-instance management, network collaboration, automation runtime, and composed prompt assembly. The main trade-off is a broader migration across transport and read-model code in exchange for one authoritative contract, better runtime resilience, stronger operator visibility, and regression protection through parity tests, a reusable daemon harness, and ACP fault injection.

## System Architecture

### Component Overview

- `internal/api/contract` (new) becomes the single source of truth for JSON DTOs, response envelopes, SSE records, stream cursors, and canonical error codes used by the daemon.
- `internal/api/core` remains the transport-neutral handler and service wiring layer. It keeps service interfaces and shared handler logic, but stops owning JSON-facing type definitions.
- `internal/api/httpapi` and `internal/api/udsapi` continue to share the same route graph and handler core. Listener bootstrap, middleware wiring, and transport-specific shutdown stay transport-specific.
- `internal/api/client` adopts operation-class timeout policies and canonical payloads from `internal/api/contract`.
- `pkg/compozy/runs` converges on the canonical run contract while preserving its public API through an adapter layer where needed.
- `internal/daemon` remains the runtime ownership boundary for host lifecycle, reconciliation, run supervision, transport services, health, and metrics. This layer gains signal-aware shutdown, bounded close semantics, structured logger wiring, richer recovery classification, and observer-style status aggregation.
- `internal/core/run/executor` remains the ACP execution owner. It gains explicit liveness ownership for session-update tracking, stall detection, and integrity signaling.
- `internal/core/subprocess` remains the subprocess wrapper. It gains explicit process-group semantics where supported and platform-specific adapters where not.
- `internal/store/globaldb` and `internal/store/rundb` remain the durable write boundary. Their close paths gain checkpoint discipline, and `run.db` projections become first-class inputs to snapshots, integrity state, and transcript assembly.
- `cmd/compozy` and `internal/cli` remain the operator entry points. Signal wiring stays here for foreground CLI operation, but detached daemon shutdown must no longer depend on CLI-only contexts.
- `internal/logger` (new) owns structured JSON log configuration, file sink creation, mirroring, and rotation policy.
- `internal/testutil/e2e` and `internal/testutil/acpmock` (new) provide the reusable runtime harness, artifact collection, parity infrastructure, and ACP failure fixtures required by the primary scope.

### Data Flow

1. A client starts a task, review, or exec run through UDS or localhost HTTP using canonical contract payloads.
2. Shared handlers validate the request, delegate through existing service interfaces, and return canonical envelopes independent of transport.
3. `TaskService`, `ReviewService`, `ExecService`, and `RunService` remain the transport-facing seams. They delegate run mutation and query behavior to a daemon-internal supervisor facade implemented by `RunManager`; this phase does not replace the existing service split.
4. `RunManager` allocates the run, writes `global.db` and `run.db`, and launches the existing execution pipeline.
5. The journal persists canonical events before publish, while transport streams emit canonical SSE `event`, `heartbeat`, and `overflow` frames.
6. Snapshot and replay consumers read canonical run state through `GET /api/runs/:run_id/snapshot`, `GET /api/runs/:run_id/events`, and `GET /api/runs/:run_id/stream`.
7. On daemon startup and periodic recovery scans, the daemon reconciles interrupted runs, classifies them as crashed or orphaned based on available runtime state, appends synthetic recovery events when possible, and reports the outcome through health, metrics, logs, and snapshot integrity flags.

## Implementation Design

### Core Interfaces

The existing transport-facing service split remains in place during this effort. A new daemon-internal facade sits beneath it and is implemented by `RunManager`.

```go
type RunSupervisor interface {
    StartTask(context.Context, string, string, contract.TaskRunRequest) (contract.Run, error)
    StartReview(context.Context, string, string, int, contract.ReviewRunRequest) (contract.Run, error)
    StartExec(context.Context, contract.ExecRequest) (contract.Run, error)
    List(context.Context, contract.RunListQuery) ([]contract.Run, error)
    Get(context.Context, string) (contract.Run, error)
    Snapshot(context.Context, string) (contract.RunSnapshot, error)
    Events(context.Context, string, contract.RunEventPageQuery) (contract.RunEventPage, error)
    OpenStream(context.Context, string, contract.StreamCursor) (RunStream, error)
    Cancel(context.Context, string) error
}
```

```go
type RunSnapshot struct {
    Run        Run                    `json:"run"`
    Jobs       []RunJobState          `json:"jobs,omitempty"`
    Transcript []RunTranscriptMessage `json:"transcript,omitempty"`
    Usage      kinds.Usage            `json:"usage,omitempty"`
    Shutdown   *RunShutdownState      `json:"shutdown,omitempty"`
    Incomplete bool                   `json:"incomplete,omitempty"`
    NextCursor *StreamCursor          `json:"-"`
}
```

```go
type RuntimeHarness interface {
    Start(t testing.TB, opts HarnessOptions) *Harness
    HTTPClient() *client.Client
    UDSClient() *client.Client
    CLI() *CLIClient
    Artifacts(runID string) ArtifactManifest
}
```

```go
type AgentLivenessMonitor interface {
    ObserveSessionUpdate(runID string, jobID string, at time.Time)
    MarkStarted(runID string, jobID string, pid int, sessionID string)
    MarkExited(runID string, jobID string, at time.Time, err error)
    DetectStalls(now time.Time) []JobLivenessAlert
}
```

Error handling uses a single canonical transport error envelope across HTTP, UDS, CLI-adapted clients, and `pkg/compozy/runs`. Streaming uses fixed contract records for `event`, `heartbeat`, and `overflow`, with cursor advancement and replay semantics defined in the contract package rather than in transport-specific code.

### Data Models

- `global.db` remains the authoritative home-scoped catalog for workspaces, workflows, run index rows, and sync checkpoints.
- `run.db` remains the authoritative per-run store for:
  - `events`
  - `job_state`
  - `transcript_messages`
  - `hook_runs`
  - `token_usage`
- `job_state` is extended to carry enough runtime ownership data for supervision and recovery:
  - `session_id`
  - `subprocess_pid`
  - `last_update_at`
  - `stall_state`
  - `stall_reason`
- `run.db` gains a singleton integrity record that stores:
  - `incomplete` boolean
  - `reasons_json`
  - `first_detected_at`
  - `updated_at`

The contract layer owns canonical transport models for:

- `DaemonStatusResponse`
- `DaemonHealthResponse`
- `WorkspaceResponse` and workspace mutation envelopes
- `TaskWorkflowResponse`, `TaskItemsResponse`, `ValidationResponse`, `ArchiveResponse`
- `ReviewFetchResponse`, `ReviewSummaryResponse`, `ReviewRoundResponse`, `ReviewIssuesResponse`
- `RunResponse`, `RunListResponse`, `RunSnapshotResponse`, `RunEventPageResponse`
- `StreamCursor`
- `SSEMessage`
- `TransportErrorResponse`

`MetricsPayload` remains transport-neutral because `/api/daemon/metrics` returns rendered Prometheus text rather than a JSON envelope.

#### Error-Code Vocabulary

The canonical contract freezes the current error-code vocabulary and the minting rule:

- Existing codes retained:
  - `invalid_request`
  - `validation_error`
  - `not_found`
  - `conflict`
  - `service_unavailable`
  - `internal_error`
  - `schema_too_new`
  - explicit `Problem` codes already emitted by handlers, including `daemon_not_ready` and `daemon_active_runs`
- New domain-specific codes may be added only by introducing a typed `Problem` site and corresponding contract documentation entry.
- For non-loopback HTTP, internal 5xx responses must support masked messages while preserving stable codes and request IDs.

#### Timeout Policy

The daemon client replaces the current blanket 5-second timeout with operation classes:

| Operation Class | Default Timeout                 | Applies To                                                                                | Caller Override Rule                                                                                |
| --------------- | ------------------------------- | ----------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `probe`         | `2s`                            | `GET /api/daemon/status`, `GET /api/daemon/health`                                        | Smaller caller deadline wins; otherwise default applies                                             |
| `read`          | `15s`                           | list/get endpoints, `snapshot`, `events`, workspace queries, review queries, task queries | Smaller caller deadline wins; otherwise default applies                                             |
| `mutate`        | `30s`                           | `cancel`, `archive`, `validate`, `daemon stop`, workspace mutations                       | Smaller caller deadline wins; otherwise default applies                                             |
| `long_mutate`   | `120s`                          | `sync`, `review fetch`, `task run`, `review run`, `exec start`                            | Smaller caller deadline wins; otherwise default applies                                             |
| `stream`        | no global `http.Client.Timeout` | `GET /api/runs/:run_id/stream`                                                            | Request context controls lifetime; heartbeat gap tolerance is `45s` with a `15s` heartbeat interval |

For streaming, the client must reconnect from the last cursor when no frame arrives for more than `45s`, unless the caller context has already expired or canceled.

#### Snapshot Integrity Semantics

`RunSnapshot.Incomplete` is a sticky per-run flag. Once set for a run, it remains `true` on later snapshots and reattaches.

It becomes `true` when any of the following is observed:

- journal submit drops were recorded while the run was active
- an event sequence gap is detected in persisted `run.db.events`
- a projection row required for the requested snapshot is missing for an already persisted canonical event
- transcript assembly detects an unrecoverable schema or sequence gap

`run.db` stores the specific reason codes so the snapshot surface can expose why the run is incomplete without re-deriving that state from raw logs on every request.

### API Endpoints

All currently registered routes in `internal/api/core/routes.go` remain in scope for the contract refactor unless explicitly deprecated later. This phase does not deprecate any existing route.

#### Daemon

- `GET /api/daemon/status`
  - Returns daemon identity, version, socket/path data, active run count, and workspace count.
  - Response: `DaemonStatusResponse`
  - Status codes: `200`

- `GET /api/daemon/health`
  - Returns readiness, degraded state, uptime, database-size diagnostics, reconcile warnings, and active-run aggregates.
  - Response: `DaemonHealthResponse`
  - Status codes: `200`, `503`

- `GET /api/daemon/metrics`
  - Returns Prometheus text for runtime, reconcile, journal, and ACP-related counters.
  - Response: `text/plain`
  - Status codes: `200`

- `POST /api/daemon/stop`
  - Requests daemon shutdown with optional force semantics.
  - Response: canonical mutation envelope or error envelope
  - Status codes: `202`, `409`

#### Workspaces

- `POST /api/workspaces`
  - Registers a workspace.
  - Response: `WorkspaceResponse`
  - Status codes: `201`, `200`, `422`

- `GET /api/workspaces`
  - Lists registered workspaces.
  - Response: `WorkspaceListResponse`
  - Status codes: `200`

- `GET /api/workspaces/:id`
  - Returns one workspace.
  - Response: `WorkspaceResponse`
  - Status codes: `200`, `404`

- `PATCH /api/workspaces/:id`
  - Updates mutable workspace metadata.
  - Response: `WorkspaceResponse`
  - Status codes: `200`, `404`, `422`

- `DELETE /api/workspaces/:id`
  - Deletes a workspace registration.
  - Response: canonical mutation envelope
  - Status codes: `204`, `404`, `409`

- `POST /api/workspaces/resolve`
  - Resolves a workspace by path.
  - Response: `WorkspaceResponse`
  - Status codes: `200`, `404`

#### Tasks

- `GET /api/tasks`
  - Lists task workflows for a workspace.
  - Response: `TaskWorkflowListResponse`
  - Status codes: `200`

- `GET /api/tasks/:slug`
  - Returns one task workflow summary.
  - Response: `TaskWorkflowResponse`
  - Status codes: `200`, `404`

- `GET /api/tasks/:slug/items`
  - Returns parsed task items.
  - Response: `TaskItemsResponse`
  - Status codes: `200`, `404`

- `POST /api/tasks/:slug/validate`
  - Validates a task workflow.
  - Response: `ValidationResponse`
  - Status codes: `200`, `422`

- `POST /api/tasks/:slug/runs`
  - Starts a daemon-owned task run.
  - Response: `RunResponse`
  - Status codes: `201`, `409`, `422`

- `POST /api/tasks/:slug/archive`
  - Archives a workflow.
  - Response: `ArchiveResponse`
  - Status codes: `200`, `404`, `409`

#### Reviews

- `POST /api/reviews/:slug/fetch`
  - Imports review data for one workflow.
  - Response: `ReviewFetchResponse`
  - Status codes: `200`, `201`, `404`, `422`

- `GET /api/reviews/:slug`
  - Returns the latest review summary.
  - Response: `ReviewSummaryResponse`
  - Status codes: `200`, `404`

- `GET /api/reviews/:slug/rounds/:round`
  - Returns one review round.
  - Response: `ReviewRoundResponse`
  - Status codes: `200`, `404`

- `GET /api/reviews/:slug/rounds/:round/issues`
  - Returns issues for one review round.
  - Response: `ReviewIssuesResponse`
  - Status codes: `200`, `404`

- `POST /api/reviews/:slug/rounds/:round/runs`
  - Starts a daemon-owned review-fix run.
  - Response: `RunResponse`
  - Status codes: `201`, `404`, `409`, `422`

#### Runs

- `GET /api/runs`
  - Lists runs.
  - Response: `RunListResponse`
  - Status codes: `200`

- `GET /api/runs/:run_id`
  - Returns the canonical run summary.
  - Response: `RunResponse`
  - Status codes: `200`, `404`

- `GET /api/runs/:run_id/snapshot`
  - Returns the canonical attach/read model for cold clients.
  - Response: `RunSnapshotResponse`
  - Status codes: `200`, `404`

- `GET /api/runs/:run_id/events`
  - Returns paginated canonical event records with cursor metadata.
  - Response: `RunEventPageResponse`
  - Status codes: `200`, `404`

- `GET /api/runs/:run_id/stream`
  - Returns SSE using canonical `event`, `heartbeat`, and `overflow` frames.
  - Response: SSE stream
  - Status codes: `200`, `404`

- `POST /api/runs/:run_id/cancel`
  - Requests run cancellation.
  - Response: canonical mutation envelope
  - Status codes: `202`, `404`, `409`

#### Sync and Exec

- `POST /api/sync`
  - Reconciles workspace artifacts into daemon state.
  - Response: `SyncResponse`
  - Status codes: `200`, `404`, `422`

- `POST /api/exec`
  - Starts an ad-hoc daemon-owned exec run.
  - Response: `RunResponse`
  - Status codes: `201`, `422`

## Integration Points

- ACP agent subprocesses remain the execution boundary for Claude Code, Codex, Droid, Cursor, and other ACP-compatible agents. This spec does not move ACP into a daemon-resident session service.
- `internal/core/run/executor` owns ACP liveness bookkeeping through `AgentLivenessMonitor`.
- `RunManager` and reconcile flows consume persisted liveness state to classify interrupted runs as crashed, orphaned, or stalled.
- Executable extensions remain per-run subprocess participants through the existing daemon bridge and capability-token model. This spec does not adopt AGH-style long-lived bridge instances or resource-kernel-backed extension orchestration.

## Impact Analysis

| Component                                             | Impact Type | Description and Risk                                                                                        | Required Action                                                                                                     |
| ----------------------------------------------------- | ----------- | ----------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `internal/api/contract`                               | new         | Canonical transport boundary; medium migration risk because many consumers converge here                    | Add DTOs, envelopes, SSE records, cursor types, and canonical error-code vocabulary                                 |
| `internal/api/core`                                   | modified    | Inline DTO ownership and anonymous handler shapes must be removed; medium risk                              | Rewire handlers and services to contract types while preserving the current service split                           |
| `internal/api/httpapi` / `internal/api/udsapi`        | modified    | Transport shutdown and parity behavior must stay aligned; medium risk                                       | Adopt canonical contracts and shared parity fixtures                                                                |
| `internal/api/client`                                 | modified    | Timeout and decode semantics change; medium risk                                                            | Adopt contract package and operation-class timeout policy                                                           |
| `pkg/compozy/runs`                                    | modified    | Public run readers converge on daemon contract; high compatibility risk if shape drift is mishandled        | Preserve the current snapshot shape via adapters or mark any breaking change explicitly                             |
| `internal/daemon`                                     | modified    | Shutdown, recovery, health, metrics, and logger wiring become richer; high operational value                | Add signal handling, timeout orchestration, observer logic, richer reconcile classification, and structured logging |
| `internal/core/run/executor`                          | modified    | ACP liveness ownership and transcript/integrity behavior become explicit; medium risk                       | Add liveness monitor wiring, stall detection, and incomplete-state propagation                                      |
| `internal/core/subprocess`                            | modified    | Process termination semantics become explicit; medium risk                                                  | Add process-group support where available and defined fallback behavior elsewhere                                   |
| `internal/store/globaldb` / `internal/store/rundb`    | modified    | Close paths, liveness metadata, integrity state, and read-side assembly change; low-to-medium risk          | Add checkpoint discipline and projection-backed snapshot/replay state                                               |
| `cmd/compozy` / `internal/cli`                        | modified    | Foreground signal wiring and daemon start behavior must align with detached shutdown semantics; medium risk | Keep CLI signal ownership for foreground runs, but stop relying on it for detached daemon lifecycle                 |
| `internal/logger`                                     | new         | New shared logging package; low runtime risk, high operator value                                           | Add JSON file sink, rotation, mirroring, and startup policy                                                         |
| `internal/testutil/e2e` / `internal/testutil/acpmock` | new         | New integration infrastructure; low product risk, medium maintenance cost                                   | Build harness, artifact collector, ACP fault driver, and parity helpers                                             |

## Testing Approach

### Unit Tests

- Freeze JSON and SSE semantics in `internal/api/contract` with explicit serialization tests.
- Add handler and client tests that assert envelope symmetry, canonical error-code output, and operation-class timeout behavior.
- Add runtime supervision tests for signal handling, bounded shutdown, checkpoint-on-close, and reconcile classification that can be covered without a real daemon subprocess.
- Add transcript assembly and integrity-state tests that verify deterministic replay from `run.db` projections and sticky `Incomplete` semantics.

### Integration Tests

- Build a reusable runtime harness that boots a real `compozy` daemon with isolated `$HOME`, workspace roots, transport clients, and artifact capture.
- Add HTTP/UDS parity suites for daemon, workspaces, tasks, reviews, runs, sync, and exec surfaces using the same assertions.
- Add ACP fault-injection tests for mid-stream disconnects, malformed frames, cancellation blocking, and timeout escalation.
- Add recovery tests for stale daemon artifacts, detached `SIGTERM` shutdown, orphan cleanup after crash, process-group termination, and snapshot correctness after restart.
- Add CLI parity tests for the following primary-scope commands:
  - `compozy daemon status`
  - `compozy tasks run <slug>`
  - `compozy reviews fix <slug>`
  - `compozy runs list`
  - `compozy runs show <run-id>`
  - `compozy runs watch <run-id>` or equivalent stream surface

### Test Lanes

- Real-daemon harness tests use `//go:build integration`.
- ACP failure scenarios that depend on Unix signal/process behavior use `//go:build integration && !windows` unless and until Windows parity is implemented.
- `make verify` remains unit/race oriented.
- Add `make test-integration` for the harness suite and wire it as a dedicated CI lane.

## Development Sequencing

### Build Order

1. Freeze the complete route inventory, canonical error-code vocabulary, snapshot shape compatibility rules, and operation-class timeout policy in `internal/api/contract`; no dependencies.
2. Add the integration-lane scaffolding, artifact collector, and minimum viable runtime harness shell; depends on step 1.
3. Extract `internal/api/contract` into `internal/api/core`, HTTP/UDS servers, `internal/api/client`, and `pkg/compozy/runs` while preserving the existing snapshot shape; depends on steps 1 and 2.
4. Build ACP mock-driver fixtures and transport/CLI parity suites so contract migration and runtime changes can be verified through a real daemon; depends on steps 2 and 3.
5. Land unit-coverable hardening first: bounded shutdown orchestration, checkpoint-on-close, canonical logger wiring, client timeout policy, and foreground/detached signal ownership rules; depends on steps 1, 3, and 4.
6. Land integration-dependent hardening next: orphan reaping, process-group termination, ACP stall detection, and richer reconcile classification; depends on steps 4 and 5.
7. Add observability upgrades including richer health, typed metrics schema, run-integrity reporting, and canonical transcript assembly; depends on steps 3, 5, and 6.
8. Remove obsolete inline DTO logic, close adapter gaps, and document the new daemon contract and verification lanes; depends on steps 6 and 7.

### Technical Dependencies

- No external control plane or AGH resource kernel is introduced.
- Signal-aware daemon shutdown, timeout orchestration, checkpoint-on-close, and logger wiring are cross-platform requirements.
- Orphan reaping and process-group termination ship in this phase for Unix. Windows keeps compile-safe behavior in this phase; Windows-specific process-group parity is deferred explicitly rather than implied.
- Existing `global.db` and `run.db` migration machinery remains the only schema migration mechanism.

## Monitoring and Observability

Health must expose readiness, degraded reasons, uptime, active-run counts, reconcile warnings, and database-size diagnostics.

Metrics become part of the contract with explicit schema:

| Metric                              | Type    | Labels                | Unit                              | Description                                       |
| ----------------------------------- | ------- | --------------------- | --------------------------------- | ------------------------------------------------- | ------------------------------------------------- | --------------------------------- |
| `daemon_active_runs`                | gauge   | none                  | count                             | Current live runs owned by the daemon             |
| `daemon_registered_workspaces`      | gauge   | none                  | count                             | Current registered workspaces                     |
| `daemon_shutdown_conflicts_total`   | counter | none                  | count                             | Stop requests rejected due to active runs         |
| `daemon_reconcile_runs_total`       | counter | `crash_event=appended | missing`, `classification=crashed | orphaned`                                         | count                                             | Runs processed by reconcile logic |
| `daemon_journal_submit_drops_total` | counter | `kind=terminal        | non_terminal`                     | count                                             | Event submissions dropped by journal backpressure |
| `daemon_run_terminal_total`         | counter | `mode`, `status`      | count                             | Terminal run outcomes                             |
| `daemon_acp_stall_total`            | counter | `mode`                | count                             | Jobs classified as stalled by liveness monitoring |
| `daemon_uptime_seconds`             | gauge   | none                  | seconds                           | Uptime since daemon start                         |

Structured daemon logging must write JSON to `~/.compozy/logs/daemon.log` with:

- rotation at `50 MiB`
- `5` retained files
- stderr mirroring when the daemon is started in foreground mode
- startup failure if the configured file sink cannot be opened for a detached daemon

`GET /api/runs/:run_id/snapshot` becomes the canonical attach/read surface for cold clients. Transcript assembly must reconstruct a stable post-hoc view from persisted data rather than relying only on in-memory UI state.

## Technical Considerations

### Key Decisions

- Keep `Host`, `RunManager`, and the `global.db` / `run.db` topology as the runtime ownership model and harden it instead of redesigning the daemon core.
- Use `internal/api/contract` as the authoritative boundary for handlers, `apiclient`, SSE, and `pkg/compozy/runs`.
- Keep `TaskService`, `ReviewService`, `ExecService`, and `RunService` as the transport-facing seams in this phase; the new supervisor facade is daemon-internal.
- Treat validation infrastructure as part of delivery, not follow-up work.
- Reuse AGH selectively for contract discipline, runtime harness patterns, ACP fault tooling, checkpoint discipline, and observer-style health, but do not port bridges, automation, network collaboration, or the resource kernel.

### Known Risks

- Contract migration can leave temporary dual-shape code if cleanup is not sequenced tightly.
- `pkg/compozy/runs` compatibility will break if the current snapshot shape is changed implicitly rather than through explicit adapters.
- Harness and ACP driver code can become flaky if cleanup and timeouts are not deterministic.
- Snapshot payloads can grow too large unless bounded explicitly and flagged as incomplete through durable integrity state rather than ad-hoc read-time guesses.
- Unix-only hardening paths can create false confidence if Windows behavior is treated as implicitly covered.

## Architecture Decision Records

- [ADR-001: Canonical Daemon Transport Contract](adrs/adr-001.md) — Makes `internal/api/contract` the single source of truth for daemon HTTP, UDS, SSE, and run-reader payloads.
- [ADR-002: Incremental Runtime Supervision Hardening Inside the Existing Daemon Boundary](adrs/adr-002.md) — Keeps the current daemon ownership model and closes the operational gaps inside it.
- [ADR-003: Validation-First Daemon Hardening](adrs/adr-003.md) — Requires runtime harnesses, transport parity, and ACP fault injection in the primary scope.
- [ADR-004: Observability as a First-Class Daemon Contract](adrs/adr-004.md) — Treats health, metrics, snapshots, and transcript assembly as part of the same contract effort.
