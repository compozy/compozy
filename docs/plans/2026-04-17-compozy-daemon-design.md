# Compozy Daemon Design

Date: 2026-04-17
Status: Approved in brainstorming
Scope: daemon architecture, storage model, CLI shape, transport model, and operator UX

## Context

Today Compozy runs primarily as per-command execution. That keeps the system simple, but it also creates structural limits:

- failover and restart handling are harder than they should be
- adding a second interface such as a web client is awkward
- extensions cannot interact with a long-lived control plane
- state is spread across filesystem artifacts without a central operational source of truth

The goal of this redesign is to move Compozy to a single-binary daemon model, similar in operational posture to AGH, while preserving Compozy's current workflow model and keeping Markdown artifacts as first-class human-owned documents.

## Goals

- Turn Compozy into a single global daemon per user/machine.
- Keep the product local-first and single-binary.
- Preserve Compozy's current workflow concepts and artifact flow.
- Introduce a robust operational source of truth in SQLite.
- Keep TUI as a first-class client for interactive terminal users.
- Make room for a web client and richer extension integrations.
- Reuse AGH patterns aggressively where they reduce design risk.

## Non-goals

- Introduce AGH-style top-level automations in v1.
- Replace Markdown workflow artifacts with database-only records.
- Keep `_tasks.md` or any `_meta.md` file in the daemonized model.
- Turn extensions into permanently resident background services in v1.
- Expose remote TCP access by default.
- Preserve compatibility with workspace-local `.compozy/runs/` runtime files.
- Push JSON-RPC across every daemon boundary.
- Preserve `compozy start` for compatibility.

## Approaches Considered

### 1. Thin daemon wrapper over current filesystem runtime

This approach would keep most of the current `.compozy/runs/` filesystem model intact and add a small daemon shell around it.

Pros:

- lowest implementation risk in the short term
- reuses current artifact readers almost directly
- minimal migration pressure

Cons:

- keeps too much operational truth in ad hoc files
- limits failover, queryability, and multi-client coordination
- does not fully solve the "platform" problem the redesign is targeting

### 2. Full AGH transplant

This approach would copy AGH's daemon, registry, and storage posture much more directly.

Pros:

- strongest convergence with an internal system we already understand
- mature patterns for singleton boot, UDS/HTTP, and runtime state
- lower conceptual ambiguity

Cons:

- too much of AGH's domain would leak into Compozy
- pushes concepts Compozy does not want in v1
- risks replacing Compozy's existing run engine instead of wrapping it

### 3. Recommended: Compozy local platform with AGH operational patterns

This is the approved direction. Compozy keeps its own workflow model and current run engine, but gains a global daemon, central operational DBs, and structured transport surfaces modeled after AGH.

Pros:

- solves the real platform needs without importing AGH wholesale
- keeps Markdown workflow ownership intact
- preserves current TUI ergonomics while enabling web and richer extension behaviors
- gives a clean path to better failover, attach, replay, and observability

Cons:

- requires a more deliberate migration than a thin wrapper
- demands a well-defined sync boundary between Markdown and DB
- needs explicit daemon lifecycle and workspace registry design

## Approved Design

### 1. Product shape

Compozy becomes a local platform built around one daemon process per user/machine. The daemon is the operational control plane. Workspaces remain the home of human-owned workflow artifacts. The daemon owns runtime state, orchestration state, event history, and multi-client access.

This is not a web-first replacement of the terminal UX. It is a daemon-first architecture with TUI, CLI, and web as clients.

### 2. Architectural blocks

The daemon is organized into five logical areas:

1. `daemon/host`
   - boot sequence
   - singleton lock
   - readiness and health
   - transport startup
   - global config loading from `~/.compozy`

2. `workflow service`
   - workspace registration
   - workspace resolution
   - workflow discovery
   - Markdown parsing
   - Markdown to DB sync
   - sync checkpoints and active-run watch scope

3. `run service`
   - run creation
   - attach, watch, cancel, resume
   - execution orchestration
   - integration with the current Compozy run engine

4. `observe/event store`
   - global operational DB
   - per-run event DB
   - live subscriptions
   - replay
   - snapshot/status views

### 3. Runtime reuse from current Compozy

The daemon does not replace Compozy's current execution kernel. It wraps and reuses it.

Primary reuse targets:

- `RunScope`
- the current planner and executor flow
- journal/event bus concepts
- extension runtime hooks
- `pkg/compozy/runs` reader/watch semantics where still useful

Primary AGH patterns to copy:

- `internal/config/home.go`: `ResolveHomePaths`, `EnsureHomeLayout`, and the home-scoped path constants
- `internal/daemon/boot.go` and `internal/daemon/daemon.go`: staged boot, cleanup ordering, lock/info/probe handling, and idempotent daemon startup
- `internal/api/core/interfaces.go`: shared daemon-facing service interfaces across transports
- `internal/api/core/handlers.go` and `internal/api/core/sse.go`: shared handler core plus SSE helpers
- `internal/api/httpapi/{server.go,routes.go}` and `internal/api/udsapi/{server.go,routes.go}`: `gin`-based transport parity over localhost HTTP and UDS
- `internal/store/globaldb/global_db.go` and `internal/store/sessiondb/session_db.go`: global DB plus per-session DB split, including the dedicated per-session writer loop
- `internal/observe/observer.go`: observer layering over global registry state and per-session stores
- `internal/session/manager.go`: lifecycle manager shape for create/list/status/events/transcript/stop/resume flows

### 3.1 AGH reuse map

The implementation should not re-invent daemon host, transport, and runtime persistence from scratch. These AGH files are the direct reference map:

| AGH reference                                                        | Pattern to reuse                                       | Compozy target                                        |
| -------------------------------------------------------------------- | ------------------------------------------------------ | ----------------------------------------------------- |
| `internal/config/home.go`                                            | Home path resolution and layout creation               | `~/.compozy` path resolver and bootstrap layout       |
| `internal/daemon/boot.go`                                            | Locking, stale daemon cleanup, readiness boot flow     | `ensureDaemon()` and daemon auto-start                |
| `internal/daemon/daemon.go`                                          | Boot/shutdown composition root                         | Compozy daemon host lifecycle                         |
| `internal/api/core/interfaces.go`                                    | Transport-facing service interfaces                    | Compozy daemon service contracts                      |
| `internal/api/core/handlers.go`                                      | Shared transport-neutral handlers                      | Shared REST handler layer for UDS and localhost HTTP  |
| `internal/api/core/sse.go`                                           | SSE framing and cursoring helpers                      | Run watch/observe streaming                           |
| `internal/api/httpapi/server.go`                                     | Localhost HTTP server with options                     | Web/client transport                                  |
| `internal/api/udsapi/server.go`                                      | UDS server with the same handler core                  | CLI/TUI transport                                     |
| `internal/api/httpapi/routes.go` and `internal/api/udsapi/routes.go` | Route parity and resource-oriented endpoint style      | Compozy daemon endpoint layout                        |
| `internal/store/globaldb/global_db.go`                               | Global registry schema and workspace catalog           | `global.db` workspace/run registry                    |
| `internal/store/sessiondb/session_db.go`                             | Per-session event DB and dedicated writer loop         | `run.db` event/transcript store                       |
| `internal/observe/observer.go`                                       | Global observability over runtime plus persisted state | Daemon status/observe surfaces                        |
| `internal/session/manager.go`                                        | Long-lived runtime manager patterns                    | Run/session manager around Compozy's execution engine |

Automation-specific AGH surfaces are intentionally out of scope for Compozy v1.

### 4. Storage model

#### 4.1 Source of truth split

Compozy uses a dual ownership model:

- Markdown remains the source of truth for human-authored workflow content.
- SQLite becomes the source of truth for operational state.

#### 4.2 What stays in Markdown

These remain real workspace artifacts:

- `_prd.md`
- `_techspec.md`
- ADRs
- `task_XX.md`
- `reviews-NNN/issue_XXX.md`
- `memory/MEMORY.md`
- `memory/task_XX.md`
- `_protocol.md`
- `_prompt.md`
- `qa/` outputs

#### 4.3 What becomes DB-backed

These should be treated as operational DB-backed state:

- run state and event history
- attach/watch snapshots
- runtime diagnostics
- workspace registry data
- sync checkpoints
- daemon-owned lifecycle state

The daemonized model does not keep `_tasks.md` or any workflow/review `_meta.md` files.

### 5. Database topology

The approved topology mirrors AGH at a high level:

- one global DB for catalog and operational state
- one DB per run for detailed events, transcript, and replayable execution data

Recommended initial split:

- `~/.compozy/db/global.db`
  - workspace registry
  - workflow summaries
  - run index and lifecycle state
  - sync checkpoints
  - daemon-owned metadata

- `~/.compozy/runs/<run-id>/run.db`
  - event stream
  - transcript
  - checkpoints
  - per-job/per-step operational state
  - attach/watch replay data

### 6. Home layout

All operational state lives under `~/.compozy`, while Markdown workflow artifacts stay in each workspace.

Recommended layout:

```text
~/.compozy/
  config.toml

  agents/
    <agent-name>/

  extensions/
    <extension-name>/
      extension.json
      .compozy-state.json

  state/
    workspace-extensions.json
    update-state.json

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

### 7. Daemon lifecycle

The daemon is a global singleton per user/machine, rooted at `$HOME`, like AGH.

Important rules:

- the daemon always boots with `$HOME` as its base
- the daemon does not inherit repository `cwd` as its base
- workspace roots are explicit inputs resolved by the client
- run execution still happens with the workspace root as the effective execution directory

#### 7.1 Auto-start

The daemon supports auto-start through an `ensureDaemon()` bootstrap sequence modeled directly after the AGH boot flow in `internal/daemon/boot.go`:

1. probe current daemon endpoint
2. if healthy, reuse it
3. if unhealthy, try to acquire bootstrap lock
4. after lock acquisition, probe again
5. if still absent, boot daemon
6. other clients wait for `ready` instead of failing with "already started"

#### 7.2 Singleton safety

The singleton model uses three artifacts together:

- lock file
- info file
- health/readiness probe

This must handle:

- duplicate start attempts
- stale socket files
- stale lock files
- dead daemon PID in info file
- daemon already running

### 8. Transport and contracts

Compozy does not move to JSON-RPC everywhere.

Approved contract split:

- in-process services: typed Go interfaces
- CLI to daemon: UDS
- local web/UI to daemon: HTTP on localhost
- streaming: SSE
- extension subprocess boundaries: JSON-RPC
- agent runtime boundaries: ACP

The shared transport shape should follow AGH's split:

- transport-neutral service interfaces in `internal/api/core/interfaces.go`
- transport-neutral request/response logic in `internal/api/core/handlers.go`
- SSE framing in `internal/api/core/sse.go`
- near-parity route registration in `internal/api/httpapi/routes.go` and `internal/api/udsapi/routes.go`

Default network stance for v1:

- UDS enabled
- localhost HTTP enabled
- TCP disabled by default
- TCP can be added later behind explicit configuration

### 9. Sync and watch model

Sync is hybrid and intentional, not a permanent global watcher.

Rules:

- `compozy sync` remains explicit
- entering `tasks run` or `reviews fix` performs an automatic reconciliation first
- during an active run, the daemon may start scoped watchers for that run/workflow
- broad always-on whole-workspace watching is avoided

This keeps the system responsive without paying permanent cost across every workspace on the machine.

### 10. Extension model

Extensions remain run-scoped subprocesses in v1.

That means:

- the daemon is long-lived
- extensions are still started for runs or explicit workflows
- extensions can talk to the daemon API/control plane
- extensions do not become permanent resident services yet

This keeps Compozy extensible without importing AGH's automation model.

### 11. CLI taxonomy

Approved CLI direction:

- `compozy sync`
- `compozy archive`
- `compozy tasks list|show|validate|run`
- `compozy reviews fetch|list|show|fix`
- `compozy runs list|show|watch|attach|cancel`
- `compozy daemon start|stop|status`
- `compozy exec`
- `compozy agents list|inspect`
- `compozy ext ...`
- `compozy setup`
- explicit workspace registry commands/endpoints are required, with exact command names deferred to the TechSpec

Explicitly removed:

- `compozy start`

### 12. Run interaction UX

The daemonized model must preserve current terminal ergonomics.

Approved behavior:

- `compozy tasks run <feature>` attaches to TUI by default in interactive TTYs
- headless/non-interactive execution falls back to `stream` or `detach`
- `--detach` starts the run and returns immediately
- `--stream` attaches in textual streaming mode
- `--ui` forces TUI attach
- `compozy runs attach <run-id>` reattaches to the interactive TUI
- `compozy runs watch <run-id>` follows the live textual stream

Foreground/background wording is reserved for daemon lifecycle, not run observation.

### 13. Meaning of run inspection commands

- `runs show <run-id>`: static snapshot of the run
- `runs watch <run-id>`: live stream of run progress/events
- `runs attach <run-id>`: richer interactive attach, primarily the TUI

### 14. Failover and recovery posture

This redesign improves failure handling in several ways:

- daemon bootstrap is idempotent
- operational state is centralized in SQLite
- runs become replayable through per-run DBs
- attach/watch can reconnect to persisted state
- stale daemon artifacts can be detected and cleaned up
- multiple clients can coordinate through one control plane instead of racing through ad hoc files

### 15. Deferred detail for the TechSpec

The following are intentionally deferred to the technical specification:

- exact schema for `global.db`
- exact schema for `run.db`
- API route and payload definitions
- SSE event contract
- operator messaging and cleanup behavior for the hard cutover from existing `.compozy/runs/`
- exact config namespace for run attachment defaults
- sync semantics and conflict policy for DB versus Markdown updates
- cleanup and retention policies for historical runs

## Recommendation

Proceed with the "Compozy local platform with AGH operational patterns" approach.

It best matches the product intent:

- richer local platform
- extensibility-first posture
- support for future web surfaces
- preservation of Compozy's artifact-centric workflow model
- no forced import of AGH automations

## Transition

This design is ready to be converted into a technical specification focused on:

- database schema
- daemon service boundaries
- API contracts
- sync semantics and watcher flows
- migration and rollout plan
