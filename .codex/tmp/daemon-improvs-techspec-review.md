Review the following Technical Specification draft critically.

Context:

- Repository: Compozy / looper
- Scope type: design review only
- The draft is not yet approved or saved as the final `_techspec.md`
- Review priorities:
  1. contradictions against the current codebase
  2. missing technical constraints or migration risks
  3. weak or ambiguous API/contract decisions
  4. sequencing flaws
  5. testing or observability gaps
- Focus on high-signal issues. Do not rewrite the whole spec.

Return strict JSON with this shape:
{
"summary": "short string",
"blocking_issues": [
{
"title": "string",
"severity": "high|medium|low",
"reason": "string",
"recommended_change": "string"
}
],
"non_blocking_improvements": [
{
"title": "string",
"reason": "string",
"recommended_change": "string"
}
],
"approval_recommendation": "approve|approve_with_edits|major_rewrite"
}

Technical Specification Draft:

## Technical Specification: Daemon Hardening via Canonical Contracts and Runtime Supervision

## Executive Summary

No `_prd.md` exists for this feature. This TechSpec is derived from the approved design choices in this session, the consolidated daemon improvement analysis in `analysis.md`, and the domain reports for transport/API, task runtime, observability/storage, testing harness, and reconciliation.

The implementation keeps the current daemon architecture as the baseline: `Host` owns daemon bootstrap and transport lifecycle, `RunManager` owns active runs, `global.db` remains the home-scoped operational catalog, and `run.db` remains the per-run durable store. The primary structural refactor is the introduction of `internal/api/contract` as the canonical daemon contract consumed by handlers, `apiclient`, SSE helpers, and `pkg/compozy/runs`. The primary hardening track stays inside the existing daemon boundary and closes the operational gaps around signal handling, bounded shutdown, orphan reaping, process-group termination, checkpoint discipline, and recovery honesty.

This design deliberately excludes AGH’s resource kernel, network collaboration, automation runtime, bridge-instance management, and other platform-scale subsystems that do not fit looper’s local-first workflow daemon. The main trade-off is a broader migration across transport and read-model code in exchange for one authoritative contract, better runtime resilience, stronger operator visibility, and regression protection through parity tests, a reusable daemon harness, and ACP fault injection.

## System Architecture

### Component Overview

- `internal/api/contract` (new) becomes the single source of truth for daemon JSON payloads, response envelopes, error payloads, stream cursors, heartbeat frames, and overflow records.
- `internal/api/core` keeps transport-neutral handler and service wiring, but no longer owns JSON-facing DTO definitions.
- `internal/api/httpapi` and `internal/api/udsapi` continue to share the same route graph and handler core; only listener bootstrap and transport-specific middleware remain transport-specific.
- `internal/api/client` and `pkg/compozy/runs` converge on the canonical contract package, with adapters only where public API stability requires them.
- `internal/daemon` remains the runtime ownership boundary for host lifecycle, reconciliation, run supervision, transport services, health, and metrics. This layer gains signal-aware shutdown, bounded close semantics, orphan cleanup, richer recovery signals, and observer-style status aggregation.
- `internal/store/globaldb` and `internal/store/rundb` remain the durable write boundary. Their close paths gain checkpoint discipline, and their projections become first-class inputs to snapshots and transcript assembly.
- `internal/testutil/e2e` and `internal/testutil/acpmock` (new) provide the reusable runtime harness, artifact collection, parity infrastructure, and ACP failure fixtures required by the primary scope.

### Data Flow

- A client starts a task, review, or exec run through UDS or localhost HTTP using the canonical contract payloads.
- Shared handlers validate the request, delegate to `RunManager`, and return canonical envelopes independent of transport.
- `RunManager` allocates the run, writes `global.db` and `run.db`, and launches the existing execution pipeline.
- The journal persists canonical events before publish, while transport streams emit contract-defined SSE frames.
- Snapshot and replay consumers read canonical run state through `GET /api/runs/:run_id/snapshot`, `GET /api/runs/:run_id/events`, and `GET /api/runs/:run_id/stream`.
- On daemon startup or periodic recovery scans, the daemon reconciles interrupted runs, appends synthetic recovery events when possible, and reports the outcome through health, metrics, and structured logs.

## Implementation Design

### Core Interfaces

```go
type RunSupervisor interface {
	StartTaskRun(context.Context, contract.TaskRunRequest) (contract.Run, error)
	StartReviewRun(context.Context, contract.ReviewRunRequest) (contract.Run, error)
	StartExecRun(context.Context, contract.ExecRequest) (contract.Run, error)
	Snapshot(context.Context, string) (contract.RunSnapshot, error)
	OpenStream(context.Context, string, contract.StreamCursor) (RunStream, error)
	Cancel(context.Context, string) error
}
```

```go
type RunSnapshot struct {
	Run        Run                 `json:"run"`
	Jobs       []RunJobSummary     `json:"jobs"`
	Transcript []TranscriptMessage `json:"transcript,omitempty"`
	TokenUsage []TokenUsage        `json:"token_usage,omitempty"`
	Incomplete bool                `json:"incomplete,omitempty"`
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

Error handling uses a single canonical transport error envelope across HTTP, UDS, CLI-adapted clients, and `pkg/compozy/runs`. Streaming uses fixed contract records for `event`, `heartbeat`, and `overflow`, with cursor advancement defined in the contract package rather than in transport-specific code.

### Data Models

- `global.db` remains the authoritative home-scoped catalog for workspaces, workflows, run index rows, and sync checkpoints.
- `run.db` remains the authoritative per-run store for `events`, `job_state`, `transcript_messages`, `hook_runs`, and `token_usage`.
- The contract layer introduces canonical transport models for:
  - `Run`
  - `RunSnapshot`
  - `RunEventPage`
  - `StreamCursor`
  - `TransportErrorResponse`
  - `DaemonHealthResponse`
  - `DaemonStatusResponse`
- Snapshot assembly reads persisted `run.db` projections and must mark the payload as incomplete when journal-drop or projection-loss conditions are detected.
- Client timeout policy replaces the current blanket 5-second timeout with operation classes:
  - short bounded timeout for status and health
  - long bounded timeout for start, sync, and fetch
  - context-driven streaming without a global `http.Client` timeout on long-lived streams

### API Endpoints

- `GET /api/daemon/status`
  - Returns daemon identity, version, socket/path data, active run count, and workspace count.
  - Response: canonical status envelope.
  - Status codes: `200`.

- `GET /api/daemon/health`
  - Returns `ready`, `degraded`, uptime, database-size diagnostics, reconcile warnings, and active-run aggregates.
  - Response: canonical health envelope.
  - Status codes: `200`, `503`.

- `GET /api/daemon/metrics`
  - Returns Prometheus text for runtime, reconcile, journal, and extension-related counters.
  - Response: `text/plain`.
  - Status codes: `200`.

- `POST /api/tasks/:slug/runs`
  - Starts a daemon-owned task run using canonical request/response payloads.
  - Request: `TaskRunRequest`.
  - Response: canonical run envelope.
  - Status codes: `201`, `409`, `422`.

- `POST /api/reviews/:slug/rounds/:round/runs`
  - Starts a daemon-owned review-fix run.
  - Request: `ReviewRunRequest`.
  - Response: canonical run envelope.
  - Status codes: `201`, `404`, `409`, `422`.

- `POST /api/exec`
  - Starts an ad-hoc daemon-owned exec run.
  - Request: `ExecRequest`.
  - Response: canonical run envelope.
  - Status codes: `201`, `422`.

- `GET /api/runs/:run_id`
  - Returns the canonical run summary.
  - Response: canonical run envelope.
  - Status codes: `200`, `404`.

- `GET /api/runs/:run_id/snapshot`
  - Returns the canonical attach/read model for cold clients.
  - Response: canonical snapshot envelope.
  - Status codes: `200`, `404`.

- `GET /api/runs/:run_id/events`
  - Returns paginated canonical event records with cursor metadata.
  - Response: canonical event-page envelope.
  - Status codes: `200`, `404`.

- `GET /api/runs/:run_id/stream`
  - Returns SSE using canonical `event`, `heartbeat`, and `overflow` frames.
  - Response: SSE stream.
  - Status codes: `200`, `404`.

- `POST /api/runs/:run_id/cancel`
  - Requests run cancellation through the existing run manager boundary.
  - Response: canonical mutation result.
  - Status codes: `202`, `404`, `409`.

## Integration Points

- ACP agent subprocesses remain the execution boundary for Claude Code, Codex, Droid, Cursor, and other ACP-compatible agents. This spec does not move ACP into a daemon-resident session service, but it requires stronger shutdown, liveness, and failure-testing behavior at this boundary.
- Executable extensions remain per-run subprocess participants through the existing daemon bridge and capability-token model. This spec does not adopt AGH-style long-lived bridge instances or resource-kernel-backed extension orchestration.

## Impact Analysis

| Component                                             | Impact Type | Description and Risk                                                                         | Required Action                                                                    |
| ----------------------------------------------------- | ----------- | -------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `internal/api/contract`                               | new         | Canonical transport boundary; medium migration risk because many consumers converge here     | Add DTOs, envelopes, SSE records, cursor types, and error payloads                 |
| `internal/api/core`                                   | modified    | Inline DTO ownership and anonymous handler shapes must be removed; medium risk               | Rewire handlers and services to contract types                                     |
| `internal/api/client`                                 | modified    | Timeout and decode semantics change; medium risk                                             | Adopt contract package and operation-specific timeout policy                       |
| `pkg/compozy/runs`                                    | modified    | Public run readers converge on daemon contract; medium risk                                  | Add adapters only where public stability requires them                             |
| `internal/daemon`                                     | modified    | Shutdown, recovery, health, metrics, and logger wiring become richer; high operational value | Add signal handling, timeout orchestration, observer logic, and structured logging |
| `internal/core/subprocess`                            | modified    | Process-group and liveness behavior become explicit; medium risk                             | Add group-aware termination and health probing                                     |
| `internal/store/globaldb` / `internal/store/rundb`    | modified    | Close paths and read-side assembly change; low-to-medium risk                                | Add checkpoint discipline and projection-backed snapshot/replay                    |
| `internal/testutil/e2e` / `internal/testutil/acpmock` | new         | New integration infrastructure; low product risk, medium maintenance cost                    | Build harness, artifact collector, and ACP fault driver                            |

## Testing Approach

### Unit Tests

- Freeze JSON and SSE semantics in `internal/api/contract` with explicit serialization tests.
- Add handler and client tests that assert envelope symmetry and operation-specific timeout behavior.
- Add runtime supervision tests for signal handling, bounded shutdown, orphan reaping, process-group termination, checkpoint-on-close, and recovery classification.
- Add transcript assembly tests that verify deterministic replay from `run.db` projections and explicit incomplete-data signaling.

### Integration Tests

- Build a reusable runtime harness that boots a real `compozy` daemon with isolated `$HOME`, workspace roots, transport clients, and artifact capture.
- Add HTTP/UDS parity suites for daemon, tasks, reviews, runs, and exec surfaces using the same assertions.
- Add ACP fault-injection tests for mid-stream disconnects, malformed frames, cancellation blocking, and timeout escalation.
- Add recovery tests for stale daemon artifacts, detached `SIGTERM` shutdown, orphan cleanup after crash, and snapshot correctness after restart.
- Add CLI parity tests for selected daemon-facing surfaces once the runtime harness exists.

## Development Sequencing

### Build Order

1. Extract `internal/api/contract` and freeze envelopes, SSE cursor rules, and error payloads; no dependencies.
2. Migrate `internal/api/core`, HTTP/UDS servers, `internal/api/client`, and `pkg/compozy/runs` to the canonical contract; depends on step 1.
3. Implement runtime supervision hardening in `internal/daemon`, `internal/core/subprocess`, `globaldb`, and `rundb`; depends on step 2.
4. Add observability upgrades including richer health, metrics, structured logger wiring, run snapshots, and transcript assembly; depends on steps 2 and 3.
5. Build `internal/testutil/e2e` with artifact capture and real-daemon boot helpers; depends on steps 2 and 3.
6. Build `internal/testutil/acpmock`, ACP fault fixtures, and transport parity suites; depends on steps 2 and 5.
7. Remove obsolete inline DTO logic, tighten regression gates, and document the new daemon contract; depends on steps 3, 4, and 6.

### Technical Dependencies

- No external control plane or AGH resource kernel is introduced.
- Unix signal and process-group support require platform-aware adapters; Windows support should fit behind the same interface even if the implementation differs.
- Existing `global.db` and `run.db` migration machinery remains the only schema migration mechanism.
- Runtime harness adoption requires lane separation so unit verification remains fast and deterministic.

## Monitoring and Observability

- Health must expose readiness, degraded reasons, uptime, active-run counts by mode, reconcile warnings, and database-size diagnostics.
- Metrics must expose at least:
  - `daemon_active_runs`
  - `daemon_registered_workspaces`
  - `daemon_shutdown_conflicts_total`
  - `runs_reconciled_total`
  - `run_reconcile_crash_event_failures_total`
  - `journal_drops_on_submit_total`
  - daemon uptime metrics
- Structured daemon logging must write to `~/.compozy/logs/daemon.log` with scoped fields such as `component`, `run_id`, `workspace_id`, and `request_id`.
- `GET /api/runs/:run_id/snapshot` becomes the canonical attach/read surface for cold clients.
- Transcript assembly must reconstruct a stable post-hoc view from persisted data rather than relying only on in-memory UI state.

## Technical Considerations

### Key Decisions

- Keep `Host`, `RunManager`, and the `global.db` / `run.db` topology as the runtime ownership model and harden it instead of redesigning the daemon core.
- Use `internal/api/contract` as the authoritative boundary for handlers, `apiclient`, SSE, and `pkg/compozy/runs`.
- Treat validation infrastructure as part of delivery, not follow-up work.
- Reuse AGH selectively for contract discipline, runtime harness patterns, ACP fault tooling, checkpoint discipline, and observer-style health, but do not port bridges, automation, network collaboration, or the resource kernel.

### Known Risks

- Contract migration can leave temporary dual-shape code if cleanup is not sequenced tightly.
- Process-group handling differs across operating systems and must be verified behind an interface.
- Harness and ACP driver code can become flaky if cleanup and timeouts are not deterministic.
- Snapshot and transcript payloads can grow too large unless bounded explicitly and flagged as incomplete when data loss occurs.

## Architecture Decision Records

- [ADR-001: Canonical Daemon Transport Contract](adrs/adr-001.md) — Makes `internal/api/contract` the single source of truth for daemon HTTP, UDS, SSE, and run-reader payloads.
- [ADR-002: Incremental Runtime Supervision Hardening Inside the Existing Daemon Boundary](adrs/adr-002.md) — Keeps the current daemon ownership model and closes the operational gaps inside it.
- [ADR-003: Validation-First Daemon Hardening](adrs/adr-003.md) — Requires runtime harnesses, transport parity, and ACP fault injection in the primary scope.
- [ADR-004: Observability as a First-Class Daemon Contract](adrs/adr-004.md) — Treats health, metrics, snapshots, and transcript assembly as part of the same contract effort.
