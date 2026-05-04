# Transport / IPC / API / ACP Bridging — Looper vs AGH Gap Analysis

Scope: UDS server, HTTP server, shared handlers, contract surface, error envelope,
SSE streaming, ACP client (launcher, tool host, permissions, process tree),
bridge SDK, IPC test utilities.

## 1. Quick assessment — looper's current transport shape

Reference files:

- `internal/api/core/routes.go` — 52 lines, ~30 endpoints grouped under
  `/api/daemon`, `/api/workspaces`, `/api/tasks`, `/api/reviews`, `/api/runs`,
  `/api/sync`, `/api/exec`.
- `internal/api/core/handlers.go` — 1159 lines, single monolithic `Handlers`
  struct wired by `HandlerConfig` (`DaemonService`, `WorkspaceService`,
  `TaskService`, `ReviewService`, `RunService`, `SyncService`, `ExecService`).
- `internal/api/core/errors.go` — canonical `TransportError { request_id, code,
  message, details }` envelope with `Problem` wrapper and status/code derivation
  from `errors.As`/`errors.Is`.
- `internal/api/core/middleware.go` — `RequestIDMiddleware` (generates
  `X-Request-Id` using `store.NewID("req")`) and `ErrorMiddleware` for gin error
  fallthrough.
- `internal/api/core/sse.go` — `PrepareSSE`, `WriteSSE`, `StreamCursor` with
  `timestamp|seq` format, `HeartbeatMessage`, `OverflowMessage`.
- `internal/api/udsapi/server.go` — 326 lines. Mounts handlers under `/api`.
  Sets 0o600 on socket, clones handlers via `handlers.Clone()`, manages
  `streamDone` for stream shutdown.
- `internal/api/httpapi/server.go` — 329 lines. Binds loopback only (enforces
  `host == 127.0.0.1`). Uses the same `core.Handlers.Clone()` and registers the
  same routes via `httpapi.RegisterRoutes`.
- `internal/api/client/client.go` — combined UDS-or-HTTP client with 5s default
  timeout, `RemoteError` with request_id formatting, and streaming decoder in
  `runs.go`.
- `internal/daemon/*_transport_service.go` — five focused service implementations
  (`transportSyncService`, `transportTaskService`, `transportReviewService`,
  `transportExecService`, `transportWorkspaceService`, `Service` for daemon
  control). All implement the interfaces in `internal/api/core/interfaces.go`.
- `internal/core/run/internal/acpshared/` — 2 454 lines. Uses the upstream
  `github.com/coder/acp-go-sdk` directly via `internal/core/agent` client,
  no daemon-side ACP process manager. No in-process permission gating,
  no tool host, no launcher abstraction, no process-tree killing, and no
  terminal management.

Effective surface: **fully-working single-binary UDS + HTTP transport pair**,
but tightly scoped to task/review/exec runs that the CLI needs. ACP is driven
client-side per run rather than as a long-lived daemon service.

## 2. Gaps vs AGH reference

### 2.1 Contract package and shared DTOs

**AGH reference**: `internal/api/contract/` — 11 files, ~3 000 lines. Canonical
`SessionPayload`, `ACPCapsPayload`, `AgentEventPayload`, `TokenUsagePayload`,
`ObserveHealthPayload`, `NetworkEnvelopePayload`, `ErrorPayload`,
`DaemonStatusPayload`, `HookCatalogPayload`, `WorkspacePayload`, etc.
`internal/api/contract/responses.go` wraps every payload in an explicit
`FooResponse { Foo FooPayload }` envelope.

**Looper today**: DTOs are defined inline inside
`internal/api/core/interfaces.go` (`DaemonStatus`, `Workspace`, `Run`, etc.).
There is **no dedicated contract package** and no `FooResponse` envelope —
handlers serialize ad-hoc `map[string]any` or anonymous structs (see
`api/client/runs.go:165-172`).

**Why it matters**: When a GUI, TUI, CI tool, or extension imports the public
payload types they currently pull in gin/store dependencies from
`internal/api/core`. Drift between handler-side JSON shape and client-side
decoded shape is not compile-checked because several handlers return inline
anonymous structs (looper `runs.go:165`, `sync.go`, `daemon.go`).

**Action**: Extract `internal/api/contract` analogous to AGH — move every
`json`-tagged type out of `internal/api/core` and add explicit `*Response`
envelopes. Clients then import only `contract` (no gin).

**Priority**: High.

### 2.2 Transport parity integration harness

**AGH reference**: `internal/api/httpapi/transport_parity_integration_test.go`
and `internal/api/udsapi/transport_parity_integration_test.go` plus
`internal/api/httpapi/httpapi_integration_test.go` (2 501 lines) and
`internal/api/udsapi/udsapi_integration_test.go` (2 556 lines). Both mount the
*same* handler set against both transports and verify identical shape.

**Looper today**: `internal/api/httpapi/transport_integration_test.go` exists
(1 289 lines), but there is no counterpart that exercises UDS through the same
test matrix; the UDS server has only unit-level service_test.go. A regression
where UDS returns a different shape from HTTP goes uncaught.

**Action**: Add a single test file that parameterizes by transport and runs the
same request matrix, or a `udsapi/transport_parity_integration_test.go` that
mirrors the HTTP one.

**Priority**: Medium.

### 2.3 Contract-level test suites and `api/spec` package

**AGH reference**: `internal/api/contract/contract_test.go`,
`settings_test.go`, `tasks_test.go`, `bridges_test.go`,
`bridges_integration_test.go`; plus `internal/api/spec/spec.go` +
`spec_test.go` which snapshot-tests every route against a declared spec and
fails the build when a route is added without its spec entry.

**Looper today**: No `api/spec` and no contract-only tests. Routes in
`internal/api/core/routes.go` are only indirectly covered by handler tests.

**Why it matters**: When a handler adds a new route or field, there is no gate
that forces the CLI client (`internal/api/client`) or the transport contract
tests to be updated.

**Action**: Introduce a simple `api/spec` with a golden list of `(method, path,
request_shape, response_shape)` and add a test that walks
`gin.Engine.Routes()` to assert parity.

**Priority**: Medium-High.

### 2.4 API test utilities (stubs / fakes)

**AGH reference**: `internal/api/testutil/apitest.go` — 1 369 lines. Provides
`StubSessionManager`, `StubObserver`, `StubAutomationManager`,
`StubTaskManager`, `StubResourceService`, etc. Every core interface has a
stub with per-method callable fields (`CreateFn`, `ListFn`, …). These stubs
are consumed by every `httpapi_integration_test.go` / `udsapi_integration_test.go`
case.

**Looper today**: No `internal/api/testutil`. Each handler test rebuilds its
own throwaway fake (see the sheer size of
`handlers_service_errors_test.go` = 652 lines and `handlers_test.go` = 315
lines). This discourages adding new tests because of the per-test fake
boilerplate.

**Action**: Extract a `internal/api/testutil` package that exposes
`StubWorkspaceService`, `StubRunService`, `StubTaskService`,
`StubReviewService`, `StubDaemonService`, etc. mirroring the AGH pattern.

**Priority**: Medium.

### 2.5 ACP driver as a daemon-owned service

**AGH reference**: `internal/acp/` — 11 254 lines. `acp.Driver`
(`client.go`) owns a pool of `AgentProcess`es with JSON-RPC 2.0 wire
(via `github.com/coder/acp-go-sdk`), full bidirectional session updates,
permission brokering (`permission.go`, 539 lines), managed terminals
(`handlers.go:83` `terminalManager`), launcher/tool-host abstraction
(`launcher.go` + `tool_host.go`), and per-OS process-tree group-kill
(`process_tree_unix.go`, `process_tree_windows.go`, `procutil`).

**Looper today**: ACP is used strictly from run-time code in
`internal/core/run/internal/acpshared/*` (session_handler.go,
command_io.go, session_exec.go, reusable_agent_lifecycle.go) — a
per-run client that dies with the run, no permission broker, no
tool host, no daemon-side terminal manager, no process tree kill.
`internal/core/subprocess/process.go` implements cooperative close →
SIGTERM → SIGKILL but only on the single managed pid; there is no
`Setpgid`/group-kill path (`Grep` for `Setpgid`/`ProcessTree` returns
only `procutil`-style hits in AGH; in looper only `internal/core/subprocess/process_unix.go`
covers single-pid termination).

**Why it matters**:
1. A daemon-hosted ACP session could be reused across TUI invocations, which
   is the natural evolution for the looper daemon (no per-run subprocess
   start-up cost).
2. Without group-kill, orphan ACP subtrees (MCP children, node workers,
   spawned test runners) may leak under `make verify` and CI.
3. Without a permission broker, there is no way to implement
   approval-on-demand UI for file writes or shell execution, which blocks
   interactive TUIs and any "approve/reject" feature the looper daemon
   might grow.

**Action**: Decide whether the looper daemon should keep ACP purely run-scoped
or grow a daemon-owned `acp.Driver` analogous to AGH. If the latter:

- Build a `Launcher`/`ToolHost`/`PermissionPolicy` trio (see
  `/Users/pedronauck/dev/compozy/agh/internal/acp/launcher.go`,
  `tool_host.go`, `permission.go`) on top of
  `internal/core/subprocess`.
- Add `internal/procutil` with `ConfigureCommandProcessGroup`,
  `SignalCommandProcessGroup`, `KillCommandProcessGroupAndWait` so that
  Setpgid / job-object kill lands consistently on Unix and Windows.
- Expose UDS endpoints equivalent to AGH's `POST /api/sessions`,
  `POST /api/sessions/:id/prompt`, `POST /api/sessions/:id/approve`,
  `GET /api/sessions/:id/stream`.

**Priority**: Medium (deferred unless the daemon needs to host long-lived ACP
sessions). **High** priority on the process-group kill alone — that is a bug
waiting to bite current single-pid runs.

### 2.6 Bridge / extension SDK and contract

**AGH reference**: `internal/bridgesdk/` (13 files, ~3 000 lines,
runtime + peer + host API + cache + batching + dedup + webhook) and
`internal/bridges/` (16 files, registry + routing + delivery broker +
metrics + lifecycle + target). The SDK is what `agh` extensions import to
implement their own providers.

**Looper today**: `sdk/extension/` and `sdk/extension-sdk-ts/` (present per
CLAUDE.md) but no runtime-level bridges. `internal/core/extension/` exists and
has a review-provider bridge (`internal/core/provider/`), but no
`bridges`/`bridgesdk` concept. Looper's extension model is narrower (review
providers only) than AGH's (any long-lived provider runtime).

**Why it matters**: Today's looper extension model is sufficient for review
providers. If the daemon is to host anything longer-lived (observability,
telemetry, chat bridges, network peers) this package needs to grow.

**Action**: No immediate change. Record as a future-scope item; revisit if
the looper daemon takes on provider lifecycles beyond review.

**Priority**: Low (deferred).

### 2.7 SSE decoding helper

**AGH reference**: `internal/sse/decode.go` (155 lines) — a reusable
`sse.Decode(ctx, body, handler)` API with `ErrStop`, max-line / max-event
byte caps, and proper context cancellation. Used across both ACP and
transport clients.

**Looper today**: Decoding lives inline inside
`internal/api/client/runs.go` (`clientRunStream.read`,
`consumeLine`, `dispatchFrame`). No max-line cap, no context-aware
decoder, not reusable outside runs.

**Why it matters**: If we later add other stream endpoints (e.g., task-run
timeline, log tail, observe events) every one will either copy this code or
write a new parser. A malformed daemon response with one giant data line will
block the scanner.

**Action**: Extract to `internal/sse/decode.go` with `max_line_bytes`,
`ErrStop`, and use it from `runs.go`. Add a benchmark (AGH has
`perf_bench_test.go`).

**Priority**: Medium.

### 2.8 Error envelope enrichment

**AGH reference**: AGH uses `contract.ErrorPayload { error }` at the wire
level plus per-handler typed error codes and a `core.RespondError(c, status,
err, maskInternal)` where `maskInternal=true` is set per handler (httpapi
vs udsapi) so HTTP callers never see internal errors.

**Looper today**: `TransportError { request_id, code, message, details }`
always includes the full error text regardless of status. There is no
`MaskInternalErrors` knob, so a 500 surfaces the underlying Go error to any
HTTP client. Good for developer UX, bad for a future remote HTTP exposure.

**Why it matters**: If the HTTP server is ever exposed beyond 127.0.0.1
(even behind an extension bridge), internal paths and dependency names leak.
UDS is trusted; HTTP is not necessarily.

**Action**: Add a `MaskInternalErrors` flag to `NewHandlers` (AGH's
`BaseHandlerConfig.MaskInternalErrors`) and enable it in `httpapi`
only. UDS keeps full detail; HTTP sanitizes 5xx.

**Priority**: Medium (low today because HTTP already requires loopback; high
once the HTTP server needs to talk to an IDE / browser over non-loopback).

### 2.9 CORS / origin validation on HTTP

**AGH reference**: `internal/api/httpapi/middleware.go` — full CORS
middleware with a loopback-aware origin matcher (`resolveAllowedOrigin`,
`canonicalOriginFromURL`, `isLoopbackHost`, `isWildcardHost`) and a
`loopbackMutationGuard` that returns 403 when a mutating request arrives
non-loopback.

**Looper today**: `internal/api/httpapi/server.go` enforces `host == "127.0.0.1"`
at bind time and uses only `core.RequestIDMiddleware`, `core.ErrorMiddleware`,
gin recovery. **No CORS, no origin check, no loopback mutation guard.** This
is safe for the current local-only posture but blocks any future browser-hosted
UI from attaching.

**Action**: Port the CORS middleware and `loopbackMutationGuard` when
browser UIs are targeted. Today record as a deferred item.

**Priority**: Low until a browser client exists.

### 2.10 Request tracing beyond `X-Request-Id`

**AGH reference**: `contract.NetworkSendRequest` carries `trace_id`,
`causation_id`, `interaction_id`, `reply_to`, `expires_at`. Full tracing
envelope is used for network delivery, automation, and bridge routes.

**Looper today**: `X-Request-Id` only. `Run` includes `request_id` but not a
trace/causation chain. SSE heartbeats carry `run_id + cursor + ts` and nothing
else.

**Why it matters**: When a CLI invocation triggers a task run, which spawns
an ACP session, which triggers N tool calls, correlating those across
`events.Event`, transcript, journal, and HTTP access logs is hard. AGH already
made that investment.

**Action**: Propagate `X-Request-Id` → `context` → every runtime event's
`payload.request_id` (already partly done) and add a `trace_id` propagation
that survives goroutine hops. This aligns with CLAUDE.md's observability
discipline.

**Priority**: Medium.

### 2.11 Heartbeat / overflow SSE semantics

**AGH reference**: `internal/api/core/sse.go` has `EmitObserveEvents` that
handles cursor tracking per event type (`ObserveCursor{Timestamp, Sequence,
ID}`). Observe streams use composite cursors to survive replay across sharded
event sources.

**Looper today**: Single `StreamCursor{Timestamp, Sequence}` with `heartbeat`
and `overflow` events, which is sufficient for the single run-event journal
but has no fallback for event streams with non-monotonic sequence.

**Why it matters**: If looper ever shares a stream between multiple sub-runs
(reusable agents, parallel jobs), a pure `uint64 Sequence` cursor is
ambiguous.

**Action**: No change needed today. Recorded for future consideration.

**Priority**: Low (deferred).

### 2.12 Gin engine construction and recovery

**AGH reference**: `udsapi.ensureEngine` uses `gin.Recovery()`; `httpapi`
adds request logging + CORS + loopback guard + error middleware.

**Looper today**: Both transports use `gin.CustomRecovery` that routes panics
into the canonical transport error envelope (better than AGH in this respect)
and share `RequestIDMiddleware` + `ErrorMiddleware`. Looper is ahead here.

**Gap**: Looper does not have a request-logging middleware (AGH's
`requestLoggingMiddleware` in `httpapi/middleware.go`). Without it, there are
no request-level `slog.Info` lines showing path, status, latency, client.

**Action**: Add `RequestLoggingMiddleware` in `internal/api/core/middleware.go`
(shared by UDS + HTTP), gated by a flag so low-chatter tests don't flood
stderr.

**Priority**: Medium.

### 2.13 Daemon-facing service interfaces are large and coarse

**AGH reference**: AGH splits services by domain (SessionManager,
TaskService, NetworkService, Observer, ResourceService, AutomationManager,
BridgeService, BundleService, SettingsService, etc.) each with its own
package. Each handler function depends only on its service.

**Looper today**: `internal/api/core/interfaces.go` defines seven interfaces
(Daemon, Workspace, Task, Review, Run, Sync, Exec) — reasonable. But the
`Handlers` struct holds **all** of them and takes a single monolithic
`HandlerConfig`. Tests therefore have to fake every field even if they
exercise one route.

**Action**: Either split handlers by domain (`daemon_handlers.go`,
`workspace_handlers.go`, etc. already exist in practice but share a struct)
or allow `HandlerConfig` to treat unset services as a 503 ServiceUnavailable
response automatically. Looper currently returns 503 for the review/exec
services via the transport_service wrappers, which is close.

**Priority**: Low (code-organization hygiene).

### 2.14 Contract versioning / back-compat strategy

**AGH reference**: No explicit `/v1` prefix, but `contract` package is the
single import surface. `api/spec` snapshots every route so changes are
reviewed.

**Looper today**: Routes are un-versioned (`/api/...`), there is no spec
snapshot, and no contract package. Back-compat is best-effort per hand-edit.

**Action**: Once `internal/api/contract` exists, gate any field rename or
removal on an `api/spec` snapshot test diff and tag a version in
`internal/version` so the client can send `X-Api-Version` and the server can
refuse older / newer clients cleanly.

**Priority**: Medium.

### 2.15 Client timeouts and cancellation

**AGH reference**: Clients are generated per-service with per-call deadlines.

**Looper today**: `internal/api/client/client.go` sets a single
`defaultRequestTimeout = 5 * time.Second` for all non-streaming calls and
`withRequestTimeout` short-circuits if the caller already has a deadline. For
long operations like `POST /api/sync` or `POST /api/tasks/:slug/runs` that
can take seconds to minutes, this is a problem — the call times out before
the daemon finishes, yet the run may still start.

**Why it matters**: Sync against a big workspace or a review fetch from a
remote provider will exceed 5s. This manifests as user-visible timeouts with
no clean recovery.

**Action**: Make the timeout method-specific. Use the default for
health/status/list/get and a longer timeout (or none, with caller context)
for mutating endpoints. AGH lets each call pass its own timeout.

**Priority**: High (this is a user-visible correctness bug in the current
daemon client; see `client.go:20` `defaultRequestTimeout = 5 * time.Second`
applied uniformly via `doJSON`).

### 2.16 Streaming error frames

**AGH reference**: AGH emits structured SSE `error` frames with canonical
`ErrorPayload` when a stream aborts mid-flight.

**Looper today**: `client/runs.go:dispatchStreamError` can parse a
`TransportError`-shaped `event: error` frame, and `core/sse.go` exposes
`HeartbeatMessage`/`OverflowMessage`. However, I did not find a server-side
emitter for `event: error` in `internal/api/core/handlers.go`. The client is
prepared to receive them but the server never sends them.

**Action**: Add an `ErrorMessage(runID, code, message)` helper and call it
from `handlers.StreamRun` when the journal returns a terminal error
mid-stream, so the CLI can render a clean error instead of a silent EOF.

**Priority**: Medium.

## 3. Explicitly skipped (out of scope or not applicable)

- **Automation jobs/triggers/runs** (`/api/automation/...`) — looper does not
  have an automation manager and the current scope does not warrant one.
- **Network peers / channels / envelopes** (`/api/network/...`,
  `NetworkEnvelopePayload`, `bridges/delivery_broker`) — looper has no
  daemon-to-daemon networking layer and no plan to.
- **Bundle catalog and activations** (`/api/bundles/...`) — AGH-specific
  extension distribution layer.
- **Settings surface** (`/api/settings/general|memory|skills|automation|
  network|observability|providers|mcp-servers|environments|hooks|actions`) —
  AGH exposes runtime-mutable settings via UDS; looper currently uses
  TOML-on-disk config and does not need a settings API.
- **Skills HTTP surface** (`/api/skills/*`) — looper ships embedded skills and
  does not currently need a runtime API to manage them.
- **Hook catalog / runs / events** (`/api/hooks/...`) — looper has a hook
  system inside `internal/core/extension` but no transport surface for it and
  none is needed yet.
- **Memory store endpoints** (`/api/memory/...`) — AGH hosts durable memory;
  looper uses `.compozy/tasks/.../memory/*.md` via the `cy-workflow-memory`
  skill.
- **Dream trigger** — AGH-specific background consolidation.
- **Resources / agents as desired-state CRUD** (`/api/resources`,
  `/api/agents`) — looper's agents are embedded in `agents/` and the `agents`
  catalog package, not runtime-mutable.

## 4. Recommended priority order (consolidated)

Top of backlog:

1. **2.15 Client timeouts** — correctness bug. Low effort.
2. **2.5 (process-group kill only)** — leaking subprocess trees on crash. Low
   effort.
3. **2.1 Contract package extraction** — enables later improvements and is a
   pure refactor.
4. **2.7 SSE decode helper** — small but high-ROI reuse win.
5. **2.8 Error masking for HTTP 5xx** — hardening for future non-loopback
   exposure.
6. **2.12 Request logging middleware** — observability hygiene.
7. **2.2 Transport parity integration tests** — prevents drift.
8. **2.3 / 2.4 api/spec + testutil** — forces discipline when routes change.
9. **2.10 Trace propagation** — when building multi-run TUI flows.
10. **2.16 Streaming error frames** — when the UI surfaces mid-run errors.

Deferred:

- 2.5 (full ACP daemon driver), 2.6 (bridge SDK), 2.9 (CORS), 2.11 (cursor
  extension), 2.13 (handler split), 2.14 (API versioning).
