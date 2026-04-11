# TechSpec: Compozy Extensibility System

## Executive Summary

Compozy gains an extensibility system that lets third parties observe lifecycle events, mutate workflow pipeline stages, register providers (IDE adapters, review sources, model adapters), and ship installable skill packs, all without forking or recompiling the binary. Executable extension logic runs as OS subprocesses and communicates with the host over JSON-RPC 2.0 on stdio, reusing the same transport primitives already used for ACP agent runtimes. Declarative assets such as provider registrations and skill-pack resources are discovered from extension manifests without requiring the extension subprocess to be running. SDKs ship for Go and TypeScript with a capability-declarative manifest (TOML-first, JSON fallback) and starter templates.

The primary technical trade-off is **per-call IPC cost (~100-500 us) accepted in exchange for language portability, crash isolation, and protocol consistency with the broader agent ecosystem of JSON-RPC-based tooling**. A sandboxed in-process tier (WebAssembly) is deliberately deferred until a measured latency bottleneck or a public-marketplace threat model appears. Executable extensions are run-scoped for `start` and `fix-reviews`, and opt-in for `exec` via `--extensions`; declarative assets are resolved during command bootstrap regardless of whether a subprocess will be spawned.

## System Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────────┐
│ Compozy host process (one per run)                              │
│                                                                 │
│   internal/core/kernel  ─────┐                                  │
│   internal/core/plan         │                                  │
│   internal/core/prompt       │  writes to                       │
│   internal/core/agent        ├──►  pkg/compozy/events.Bus       │
│   internal/core/run          │    (existing pub/sub)            │
│   internal/core/run/journal ─┘                                  │
│                                                                 │
│   internal/core/extension  ◄── new, consumes event bus &        │
│      ├── Manager              dispatches to subprocess          │
│      ├── Registry              extensions                       │
│      ├── Discovery                                              │
│      ├── HostAPI   ─────────┐                                   │
│      ├── HookDispatcher     │                                   │
│      └── Capability         │                                   │
│                             │ JSON-RPC 2.0 over stdio           │
│   internal/core/subprocess  │ (extracted from internal/core/    │
│      ├── Process            │  agent, shared with ACP)          │
│      ├── Transport          │                                   │
│      ├── Handshake          │                                   │
│      └── Signals            ▼                                   │
└─────────────────────────────┼───────────────────────────────────┘
                              │
            ┌─────────────────┴──────────────────┐
            │                                    │
    ┌───────▼────────┐                  ┌────────▼───────┐
    │ Extension A    │                  │ Extension B    │
    │ (Go subprocess)│                  │ (TS subprocess)│
    │                │                  │                │
    │ sdk/extension  │                  │ @compozy/      │
    │ (Go SDK)       │                  │ extension-sdk  │
    └────────────────┘                  └────────────────┘
```

**Components:**

| Component | Responsibility | Boundary |
|---|---|---|
| `internal/core/subprocess` | Spawn, JSON-RPC framing, initialize handshake, signal handling, graceful shutdown. Protocol-agnostic. | New package extracted from `internal/core/agent`. Consumed by both agent and extension. |
| `internal/core/extension` | Extension manager, discovery, manifest parsing, operator-local enablement resolution, capability enforcement, hook dispatch, Host API server, provider overlay assembly, bundled extension loading via `go:embed`. | New package. Run-scoped manager is created during command bootstrap before planning begins. |
| `internal/core/kernel` | Owns command bootstrap for extension-aware commands. Allocates run scope early, assembles provider overlays, and decides whether subprocess extensions are active for the current invocation. | Modified. Becomes the earliest orchestration seam for extension-aware commands. |
| `internal/core/run/executor` | Consumes an already-started extension manager during the per-run lifecycle and emits run/job/review/artifact hooks while jobs execute. | Modified. No longer responsible for creating the manager after planning. |
| `pkg/compozy/events` | Existing event bus. Extensions subscribe through the extension manager which proxies events to subprocess subscribers. | Unmodified. Extensions read through the extension manager, not directly. |
| `sdk/extension` | Go SDK for extension authors. Imports extension types and JSON-RPC client. | New public Go package. |
| `@compozy/extension-sdk` | TypeScript/JavaScript SDK for extension authors. Published to npm. | New repository/package. |
| `internal/cli/extension` | CLI subcommands for listing, installing, inspecting, uninstalling extensions. | New CLI surface (not extension-added commands — builtin management). |
| `internal/setup` + `internal/cli/skills_preflight.go` | Materialize extension-provided skill packs into agent skill directories and verify drift alongside bundled Compozy skills. | Modified. Extension skill packs are installed like agent assets, not merged only in host memory. |

### Data Flow

**Run startup:**

1. Command bootstrap decides whether executable extensions are active for this invocation.
2. `compozy start` and `compozy fix-reviews` always enable executable extensions. `compozy exec` enables them only when `--extensions` is supplied. Commands that only need provider overlays or skill-pack metadata do not spawn extension subprocesses.
3. Bootstrap calls `Discovery.Discover()` to enumerate bundled + user + workspace extensions, then resolves operator-local enablement. Bundled extensions default enabled. User and workspace extensions default disabled until explicitly enabled on the local machine.
4. Bootstrap loads declarative assets from enabled manifests:
   - provider registrations are merged into per-command overlay registries
   - declared skill packs are handed to setup/preflight code so the selected ACP agent can see them as installed skills
5. For invocations with executable extensions enabled, bootstrap allocates run artifacts, opens the run journal, creates the event bus, and constructs `extension.Manager` with `(ctx, journal, eventBus, workspaceRoot, runID)`.
6. For each enabled extension with a `[subprocess]` section, manager spawns the binary via `subprocess.Launch()` and performs the initialize handshake.
7. Manager registers declared hooks into a per-hook chain, sorted by priority, before planning begins.
8. `plan.Prepare(ctx, cfg, scope)` runs with the extension manager already available, so `plan.*` and `prompt.*` hooks can participate while jobs and prompt artifacts are being assembled.
9. Manager subscribes to the event bus and begins forwarding events to extensions that declared `events.read` capability.
10. Executor runs jobs. Hook dispatches happen synchronously at each mutation point; event subscribers receive events asynchronously on a best-effort basis.

**Provider resolution (non-run and pre-run commands):**

1. Command bootstrap discovers enabled extensions and loads their `[[providers.*]]` declarations without spawning extension subprocesses.
2. The provider and ACP runtime registries are overlaid with extension-declared entries for the lifetime of the command.
3. `compozy fetch-reviews`, `compozy start`, `compozy fix-reviews`, and `compozy exec` can resolve those provider entries exactly like built-in ones.

**Hook dispatch (mutable):**

1. A pipeline phase reaches a mutable hook point (e.g., `prompt.Build` finishes rendering).
2. Phase calls `manager.DispatchMutable(ctx, "prompt.post_build", currentValue)`.
3. Manager looks up the priority-sorted chain for that hook name.
4. For each extension in the chain:
   - Send `execute_hook` JSON-RPC call with the current value as `payload`.
   - Receive patch in the response.
   - Apply patch to `currentValue`.
   - Log the capability exercised to `.compozy/runs/<run-id>/extensions.jsonl`.
5. Return mutated `currentValue` to the phase.

**Host API call (extension → host):**

1. Extension SDK code calls `host.tasks.create(spec)`.
2. SDK sends JSON-RPC call `host.tasks.create` with `spec` params over stdio.
3. Host receives the request in the extension subprocess reader loop.
4. Host resolves the method, checks the extension's declared capabilities include `tasks.create`.
5. If granted, host invokes the typed kernel/service path for that resource instead of shelling out to a CLI command.
6. Host emits any resulting events (`EventKindTaskFileUpdated`).
7. Host logs the call to `extensions.jsonl`.
8. Host returns the result as a JSON-RPC response.

**Run shutdown:**

1. Executor cancels the run context.
2. Manager sends `shutdown` RPC to every extension with `deadline_ms` from manifest.
3. Extensions stop accepting new requests, drain in-flight work, respond.
4. Manager waits for deadline.
5. If any subprocess still alive after deadline, manager sends SIGTERM; after grace period, SIGKILL.
6. Manager closes the event bus subscription and returns.

## Implementation Design

### Core Interfaces

**Extension manager public surface** (`internal/core/extension/manager.go`):

```go
type Manager struct {
    runID       string
    logger      *slog.Logger
    journal     *journal.Journal
    eventBus    *events.Bus[events.Event]
    registry    *Registry
    dispatcher  *HookDispatcher
    hostAPI     *HostAPI
    subprocs    map[string]*subprocess.Process
    mu          sync.RWMutex
}

type Config struct {
    WorkspaceRoot string
    RunID         string
    ParentRunID   string
    EventBus      *events.Bus[events.Event]
    Journal       *journal.Journal
}

func NewManager(ctx context.Context, cfg Config) (*Manager, error)
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) DispatchMutable(ctx context.Context, hook HookName, input any) (any, error)
func (m *Manager) DispatchObserver(ctx context.Context, hook HookName, payload any)
func (m *Manager) Shutdown(ctx context.Context) error
```

**Run-scope bootstrap** (`internal/core/extension/runtime.go` or equivalent early bootstrap seam):

```go
type RunScope struct {
    Artifacts          model.RunArtifacts
    Journal            *journal.Journal
    EventBus           *events.Bus[events.Event]
    ExtensionsEnabled  bool
    Manager            *Manager
}

type OpenRunScopeOptions struct {
    EnableExecutableExtensions bool
}

func OpenRunScope(
    ctx context.Context,
    cfg *model.RuntimeConfig,
    opts OpenRunScopeOptions,
) (*RunScope, error)

func (s *RunScope) Close(ctx context.Context) error
```

`OpenRunScope` exists to solve the current lifecycle mismatch in Compozy's codebase: run artifacts, journal, bus, and the extension manager must exist before `plan.Prepare()` if `plan.*` and `prompt.*` hooks are part of v1.

**Hook declaration** (`internal/core/extension/hooks.go`):

```go
type HookName string

const (
    HookPlanPreDiscover       HookName = "plan.pre_discover"
    HookPlanPostDiscover      HookName = "plan.post_discover"
    HookPlanPreGroup          HookName = "plan.pre_group"
    HookPlanPostGroup         HookName = "plan.post_group"
    HookPlanPrePrepareJobs    HookName = "plan.pre_prepare_jobs"
    HookPlanPostPrepareJobs   HookName = "plan.post_prepare_jobs"
    HookPromptPreBuild        HookName = "prompt.pre_build"
    HookPromptPostBuild       HookName = "prompt.post_build"
    HookPromptPreSystem       HookName = "prompt.pre_system"
    HookAgentPreSessionCreate HookName = "agent.pre_session_create"
    HookAgentPostSessionCreate HookName = "agent.post_session_create"
    HookAgentPreSessionResume HookName = "agent.pre_session_resume"
    HookAgentOnSessionUpdate  HookName = "agent.on_session_update"
    HookAgentPostSessionEnd   HookName = "agent.post_session_end"
    HookJobPreExecute         HookName = "job.pre_execute"
    HookJobPostExecute        HookName = "job.post_execute"
    HookJobPreRetry           HookName = "job.pre_retry"
    HookRunPreStart           HookName = "run.pre_start"
    HookRunPostStart          HookName = "run.post_start"
    HookRunPreShutdown        HookName = "run.pre_shutdown"
    HookRunPostShutdown       HookName = "run.post_shutdown"
    HookReviewPreFetch        HookName = "review.pre_fetch"
    HookReviewPostFetch       HookName = "review.post_fetch"
    HookReviewPreBatch        HookName = "review.pre_batch"
    HookReviewPostFix         HookName = "review.post_fix"
    HookReviewPreResolve      HookName = "review.pre_resolve"
    HookArtifactPreWrite      HookName = "artifact.pre_write"
    HookArtifactPostWrite     HookName = "artifact.post_write"
)

type HookDeclaration struct {
    Event    HookName
    Priority int           // 0-1000, default 500
    Required bool
    Timeout  time.Duration
}
```

**Manifest schema** (`internal/core/extension/manifest.go`):

```go
type Manifest struct {
    Extension  ExtensionInfo       `toml:"extension"  json:"extension"`
    Subprocess *SubprocessConfig   `toml:"subprocess" json:"subprocess,omitempty"`
    Security   SecurityConfig      `toml:"security"   json:"security"`
    Hooks      []HookDeclaration   `toml:"hooks"      json:"hooks,omitempty"`
    Resources  ResourcesConfig     `toml:"resources"  json:"resources,omitempty"`
    Providers  ProvidersConfig     `toml:"providers"  json:"providers,omitempty"`
}

type ExtensionInfo struct {
    Name             string `toml:"name"`
    Version          string `toml:"version"`
    Description      string `toml:"description"`
    MinCompozyVersion string `toml:"min_compozy_version"`
}

type SubprocessConfig struct {
    Command           string            `toml:"command"`
    Args              []string          `toml:"args"`
    Env               map[string]string `toml:"env"`
    ShutdownTimeout   time.Duration     `toml:"shutdown_timeout"`
    HealthCheckPeriod time.Duration     `toml:"health_check_period"`
}

type SecurityConfig struct {
    Capabilities []string `toml:"capabilities"`
}

type ResourcesConfig struct {
    Skills []string `toml:"skills"` // glob patterns relative to extension root
}

type ProvidersConfig struct {
    IDE    []ProviderEntry `toml:"ide"`
    Review []ProviderEntry `toml:"review"`
    Model  []ProviderEntry `toml:"model"`
}

type ProviderEntry struct {
    Name     string            `toml:"name"`
    Command  string            `toml:"command"`
    Metadata map[string]string `toml:"metadata"`
}
```

**Host API handler** (`internal/core/extension/host_api.go`):

```go
type HostAPI struct {
    registry     *Registry
    workspaceRoot string
    runID        string
    parentChain  []string
    auditLog     *AuditLogger
    kernel       KernelOps  // interface abstracting needed kernel operations
}

type KernelOps interface {
    CreateTask(ctx context.Context, req TaskCreateRequest) (*Task, error)
    ListTasks(ctx context.Context, workflow string) ([]Task, error)
    GetTask(ctx context.Context, workflow string, number int) (*Task, error)
    StartRun(ctx context.Context, cfg RunStartRequest) (*RunHandle, error)
    ReadArtifact(ctx context.Context, path string) ([]byte, error)
    WriteArtifact(ctx context.Context, path string, content []byte) error
    RenderPrompt(ctx context.Context, name string, params any) (string, error)
    ReadMemory(ctx context.Context, req MemoryReadRequest) (*MemoryDocument, error)
    WriteMemory(ctx context.Context, req MemoryWriteRequest) (*MemoryDocument, error)
}

func (h *HostAPI) Handle(ctx context.Context, extName, method string, params json.RawMessage) (any, error)
```

`CreateTask` is intentionally modeled as a core task-writing service, not as a hidden dependency on a nonexistent `create-tasks` CLI command. It owns task numbering, frontmatter emission, metadata refresh, and event emission directly.

### Data Models

**Runtime extension registry entry:**

| Field | Type | Purpose |
|---|---|---|
| `Name` | `string` | Unique ID from manifest |
| `Version` | `string` | Semver |
| `Source` | `enum{bundled,user,workspace}` | Discovery level |
| `ManifestPath` | `string` | Absolute path to the manifest file |
| `Manifest` | `*Manifest` | Parsed manifest |
| `Capabilities` | `[]string` | Granted capabilities after install confirmation |
| `Process` | `*subprocess.Process` | Nil if no subprocess (pure skill pack) |
| `NegotiatedProtocol` | `string` | `"1"` from handshake |
| `Hooks` | `map[HookName]HookDeclaration` | Declared hooks by event |
| `State` | `enum{loaded,initializing,ready,degraded,stopped}` | Lifecycle state |

**JSON-RPC initialize request** (host → extension):

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocol_version": "1",
    "compozy_version": "0.1.9",
    "extension": {
      "name": "compozy-ext-userprefs",
      "version": "0.1.0",
      "source": "user"
    },
    "granted_capabilities": ["events.read", "prompt.mutate"],
    "runtime": {
      "run_id": "run-01K...",
      "parent_run_id": "",
      "workspace_root": "/Users/x/project",
      "shutdown_timeout_ms": 10000,
      "default_hook_timeout_ms": 5000
    }
  }
}
```

**JSON-RPC execute_hook request** (host → extension):

```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "method": "execute_hook",
  "params": {
    "invocation_id": "hook-01K...",
    "hook": "prompt.post_build",
    "mutable": true,
    "payload": {
      "prompt_text": "...rendered prompt...",
      "batch_params": { /* BatchParams snapshot */ }
    }
  }
}
```

Hook response (patch shape is hook-specific):

```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "result": {
    "patch": {
      "prompt_text": "...rendered prompt with appended user preferences..."
    }
  }
}
```

**Audit log entry** (`extensions.jsonl`):

```json
{
  "ts": "2026-04-10T14:15:22.123Z",
  "extension": "compozy-ext-userprefs",
  "direction": "host→ext|ext→host",
  "method": "execute_hook|host.tasks.create|...",
  "capability": "prompt.mutate",
  "latency_ms": 12,
  "result": "ok|error",
  "error": "..."
}
```

### API Endpoints

Extensions add no HTTP surface. All communication is over stdio JSON-RPC between host and subprocess.

**Host → extension (methods the extension must implement):**

| Method | When called | Response |
|---|---|---|
| `initialize` | Once at startup, before any other call | Accepted capabilities, implemented methods |
| `execute_hook` | Per mutable hook invocation in the pipeline | Patch (hook-specific shape) |
| `on_event` | Per event when extension subscribed via `events.read` | Acknowledgement (no body) |
| `health_check` | Periodically if `health_check_period` set | `{healthy: bool, message?: string}` |
| `shutdown` | Once at run end | Acknowledgement |

**Extension → host (Host API methods):**

| Method | Capability | Params | Returns |
|---|---|---|---|
| `host.events.subscribe` | `events.read` | `{filter: string[]}` | `{subscription_id}` |
| `host.events.publish` | `events.publish` | `{kind, payload}` | `{seq}` |
| `host.tasks.list` | `tasks.read` | `{workflow}` | `Task[]` |
| `host.tasks.get` | `tasks.read` | `{workflow, number}` | `Task` |
| `host.tasks.create` | `tasks.create` | `TaskCreateRequest` | `Task` |
| `host.runs.start` | `runs.start` | `RunStartRequest` | `{run_id}` |
| `host.artifacts.read` | `artifacts.read` | `{path}` | `{content}` |
| `host.artifacts.write` | `artifacts.write` | `{path, content}` | `{bytes_written}` |
| `host.prompts.render` | (none) | `{template, params}` | `{rendered}` |
| `host.memory.read` | `memory.read` | `{workflow, task_file?}` | `{path, content, needs_compaction}` |
| `host.memory.write` | `memory.write` | `{workflow, task_file?, content}` | `{path, bytes_written}` |

**CLI management surface** (`internal/cli/extension/`):

| Command | Purpose |
|---|---|
| `compozy ext list` | List discovered extensions, showing source, version, status, capabilities |
| `compozy ext inspect <name>` | Show full manifest, declaring file, active hook declarations |
| `compozy ext install <path>` | Copy an extension directory to `~/.compozy/extensions/<name>/`, prompt for capability confirmation, and record local operator state |
| `compozy ext enable <name>` / `disable <name>` | Toggle operator-local enabled state for user or workspace extensions. Workspace enablement is stored outside the repo so cloning a repo does not auto-enable its extensions |
| `compozy ext uninstall <name>` | Remove an extension from the user directory (never touches bundled or workspace) |
| `compozy ext doctor` | Validate manifests, check version compatibility, warn about priority ties, warn about unused declared capabilities, and report skill-pack/provider overlay drift |

## Integration Points

**Existing Compozy components touched:**

| Component | Interaction |
|---|---|
| `pkg/compozy/events.Bus` | Manager subscribes with a single subscription; fans out to per-extension queues that handle backpressure. |
| `internal/core/run/journal` | Extensions write events via `host.events.publish`, which adds them to the journal before fanning out. Audit log `extensions.jsonl` sits next to `events.jsonl`. |
| `internal/core/plan` | Mutable hooks called at `pre/post_discover`, `pre/post_group`, `pre/post_prepare_jobs`. |
| `internal/core/prompt` | Mutable hooks at `pre_build`, `post_build`, `pre_system`. `Build()` receives the mutated `BatchParams` / returns to the mutated final string. |
| `internal/core/agent` | Mutable hooks at `pre_session_create`, `pre_session_resume`; observe at `post_session_create`, `post_session_end`. |
| `internal/core/kernel` | Run-start bootstrap allocates run scope before planning, applies provider overlays for command execution, and exposes typed task/memory services to the Host API. |
| `internal/core/provider` + `internal/core/agent` | Gain overlay registries assembled from extension manifests so declarative providers participate in command resolution without recompiling Compozy. |
| `internal/setup` + `internal/cli/skills_preflight.go` | Extension-provided skill packs are installed and verified using the same agent-facing installation model as bundled skills. They are not only merged into host memory at runtime. |

**External system boundaries:**

- **Filesystem**: Extensions read manifests and resource files from disk; write through `host.artifacts.write` only (for audit). Extensions can still write anywhere via raw `os.WriteFile` (no OS sandbox), but doing so loses the audit trail.
- **Network**: Extensions may make outbound calls (`network.egress` capability is advisory - the OS cannot block it).
- **Child processes**: Extensions can spawn children (`subprocess.spawn` capability is advisory).
- **Trust/enablement**: Workspace and user extensions are discoverable but not executable by default. The local operator must enable them explicitly before their subprocess hooks or provider overlays become active.

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
|---|---|---|---|
| `internal/core/agent` | Modified | Refactor: subprocess plumbing extracted. Medium risk — touches production ACP code path. | Execute refactor with existing ACP tests as safety net; use a commit per step. |
| `internal/core/kernel` | Modified | Run-start bootstrap moves earlier so extensions can participate before planning, and provider overlays become command-visible. Medium risk — touches the main orchestration seam. | Extract run-scope bootstrap first; keep typed command boundaries intact. |
| `internal/core/subprocess` | New | Shared package used by agent and extension. Low risk once extracted. | New package; move files, verify agent tests pass, then add extension consumer. |
| `internal/core/extension` | New | All extension manager, registry, dispatch, host API, discovery logic. Largest new surface. | New package with comprehensive unit tests and integration tests using mock extensions. |
| `internal/core/plan` | Modified | Insert mutable hook calls at phase boundaries. Low risk — additive. | Add hook dispatch; default no-op when manager not present. |
| `internal/core/prompt` | Modified | Insert mutable hook calls. Low risk. | Add hook dispatch points; keep existing pure builder as fallback. |
| `internal/core/run/executor` | Modified | Consumes a pre-started manager and emits in-run hook points. Medium risk. | Keep shutdown ownership here, but move bootstrap earlier than planning. |
| `internal/cli/extension` | New | CLI surface for managing extensions. Low risk. | New subcommands added to root Cobra command. |
| `internal/cli/commands.go` | Modified | `exec` gains an explicit `--extensions` flag so hook/process activation is opt-in for ad hoc prompts. Low risk. | Keep default exec behavior unchanged unless flag is present. |
| `internal/setup` + `internal/cli/skills_preflight.go` | Modified | Extension skill packs must be materialized into agent-facing directories and verified for drift. Medium risk because the start/fix-reviews preflight is strict today. | Reuse existing setup/install/verify abstractions instead of inventing a second runtime-only skill path. |
| `internal/core/provider` + `internal/core/agent` | Modified | Need overlay registries for extension-declared providers. Medium risk — registry lookup becomes multi-source. | Keep built-in registries as the base; apply manifest overlays per command. |
| `pkg/compozy/events` | Unchanged | Read-only consumer. No API change. | None. |
| `pkg/compozy/runs` | Unchanged | Not affected. | None. |
| `sdk/extension` | New | Go SDK package (public API). Versioned with Compozy. | New public package; publish alongside Compozy releases. |
| `@compozy/extension-sdk` | New | Independent npm package. Versioned independently but tied to protocol version. | New repo or new subdir; publish to npm. |
| `internal/version` | Modified | Export `ProtocolVersion = "1"` constant used by initialize handshake. | Minor addition. |
| `make verify` (fmt/lint/test/build) | Unchanged | Must keep passing at 100%. | Required at every step. |

## Testing Approach

### Unit Tests

**`internal/core/subprocess`** — extracted plumbing:

- Table-driven tests for JSON-RPC envelope encoding/decoding (valid, malformed, large).
- Process launch/wait/kill on both Unix and Windows (platform build tags).
- Signal escalation: SIGTERM → wait → SIGKILL with configurable deadlines.
- Handshake flow: accept/reject invalid protocol version, missing required fields.
- Use `t.TempDir()` for any on-disk tests and `t.Parallel()` for independent subtests.

**`internal/core/extension/manifest`** — manifest parser:

- Parse valid TOML and JSON variants.
- Reject invalid capability names, invalid priorities, unknown hook events.
- Enforce `min_compozy_version` semantics.
- Verify precedence rule (workspace > user > bundled).
- Verify operator-local enablement resolution (bundled enabled by default, user/workspace disabled by default until enabled).
- Verify declarative capabilities such as `providers.register` and `skills.ship` are validated at discovery/install time, not only at runtime RPC enforcement.

**`internal/core/extension/dispatcher`** — hook dispatch:

- Priority ordering across 1, 2, 5, 10 extensions on the same hook.
- Tiebreak on equal priority (alphabetical).
- Required vs optional hook failure semantics.
- Chain abort on error with structured error wrapping.
- Observer dispatch runs concurrently and does not block mutation chain.

**`internal/core/extension/host_api`** — capability enforcement:

- Unauthorized method returns `-32001 capability_denied` with structured data listing missing grants.
- Audit log entry written for every call (success and failure).
- Path scoping: `host.artifacts.read/write` reject traversal and paths outside `.compozy/` or workspace root.
- Recursion guard in `host.runs.start` at depth 3.
- Memory document operations reflect the current Markdown-backed memory model (`MEMORY.md` and per-task memory files), not a synthetic key/value store.

**`internal/core/provider` + `internal/core/agent` overlay registries**:

- Built-in registries remain the base layer.
- Enabled extension manifests contribute overlay entries without mutating the built-in catalog globally.
- `fetch-reviews` can resolve extension review providers without spawning the extension subprocess.

**`internal/setup` + `internal/cli/skills_preflight.go`**:

- Extension skill packs install into agent-visible directories and are re-verified like bundled skills.
- Drift detection covers bundled skills plus enabled extension skill packs for the selected agent.
- Workspace extension skill packs do not become active until the operator enables that workspace extension locally.

**Mocking boundary**: The hook dispatcher and host API accept interfaces (`KernelOps`, `Transport`). Unit tests provide mocks at those boundaries, never at production types. Follow `testing-anti-patterns` skill — no test-only methods on production types, no mocked internal calls within a single package.

### Integration Tests

**End-to-end run with a mock extension:**

- Build a tiny Go-based mock extension that the integration test spawns. The mock responds to initialize, logs every hook it receives, and echoes payloads back with a marker so the test can verify mutation ordering.
- Integration test runs `compozy start --name test-fixture --ide codex` with the mock installed in a workspace `.compozy/extensions/` directory.
- Assert: run completes, extensions.jsonl captures all expected entries, prompts visible in `runs/<run-id>/jobs/*/prompt.txt` contain the mock's injected marker.

**Capability denial flow:**

- Install an extension that declares it needs `events.read` only but calls `host.tasks.create` at runtime.
- Assert: call fails with `capability_denied`, run continues (since the call was optional), audit log records the denial.

**Recursion guard:**

- Install an extension that calls `host.runs.start` on every run.
- Assert: nesting stops at depth 3 with `ErrRecursionDepthExceeded`.

**Manifest precedence:**

- Install the same extension name in bundled (fixture), user, and workspace levels.
- Assert: `compozy ext list` shows workspace wins; `compozy ext inspect` explains the override.

**Subprocess shutdown:**

- Install a mock extension that ignores shutdown.
- Assert: host escalates SIGTERM → SIGKILL within the deadline; run completes with warning.

**`exec --extensions` opt-in:**

- Run `compozy exec` without the flag and assert no extension subprocesses are started.
- Run `compozy exec --extensions` with the same installed extension and assert prompt/session hooks, audit log entries, and shutdown flow all execute.

**Skill-pack delivery path:**

- Install an extension that declares `[resources.skills]`.
- Assert: the relevant agent preflight can verify/install those skills into the target agent directory and the run sees them exactly like bundled skills.

**Provider overlay path:**

- Install an extension that declares a review provider and an ACP runtime entry.
- Assert: `compozy fetch-reviews --provider <ext-provider>` and extension-aware run commands can resolve those entries without recompiling Compozy.

**Platform coverage**: Integration tests run on Linux and macOS via CI; Windows gets a subset (subprocess launch and handshake) since Compozy primarily targets Unix developer workstations.

## Development Sequencing

### Build Order

1. **Extract `internal/core/subprocess` package from `internal/core/agent`** — no dependencies. Move files verbatim (process.go, transport.go, handshake.go, signals.go, process_unix.go, process_windows.go). Update agent imports. Run `make verify`.

2. **Add protocol version constant to `internal/version`** — depends on step 1. Export `ExtensionProtocolVersion = "1"`.

3. **Scaffold `internal/core/extension` package with manifest parser and enablement model** — depends on step 1 and step 2. Implement TOML and JSON loaders, local enablement resolution, and validation. Unit tests only, no runtime wiring.

4. **Implement discovery plus provider/skill asset extraction** — depends on step 3. Three-level scan (bundled via `go:embed`, user, workspace), precedence resolution, provider overlay assembly, and extension skill-pack inventory.

5. **Implement capability enforcement and audit log** — depends on step 3. `Capability` type, check function, JSONL writer for `extensions.jsonl`. Unit tests.

6. **Implement hook dispatcher (mutation pipeline)** — depends on steps 3, 5. Priority-ordered chain, tiebreak, required/optional semantics, error wrapping.

7. **Implement Host API handler skeleton** — depends on steps 3, 5. JSON-RPC method router, capability checks at entry, stub implementations that return not-implemented. Unit tests for routing and auth.

8. **Wire Host API to typed task, artifact, prompt, memory, and run services** — depends on step 7. Implement each method (`host.tasks.*`, `host.runs.start`, `host.artifacts.*`, `host.prompts.render`, `host.memory.*`) against typed kernel/service paths rather than shelling out. Integration tests for each.

9. **Implement early run-scope bootstrap** — depends on steps 4, 5, 6. Allocate run artifacts/journal/bus and create the extension manager before planning so `plan.*` and `prompt.*` hooks can run in v1.

10. **Implement extension manager lifecycle (spawn, init, shutdown)** — depends on steps 1, 8, 9. Spawn subprocesses, perform handshake, register hooks and event subscriptions, coordinate shutdown.

11. **Integrate bootstrap into run-start and exec entry points** — depends on step 10. `start` and `fix-reviews` enable executable extensions by default; `exec` adds an explicit `--extensions` flag and only builds the manager when the flag is present.

12. **Insert hook dispatches in pipeline phases** — depends on step 11. Add `manager.DispatchMutable` calls at `plan.*`, `prompt.*`, `agent.*`, `job.*`, `run.*`, `review.*`, `artifact.*` boundaries. Each insertion is a separate commit.

13. **Add `internal/cli/extension` management commands and local enablement state** — depends on step 4 and step 11. `list`, `inspect`, `install`, `enable`, `disable`, `uninstall`, `doctor`.

14. **Integrate extension skill packs into setup/preflight** — depends on step 4 and step 13. Reuse `internal/setup` and `skills_preflight` patterns so enabled extension skill packs become agent-visible assets.

15. **Integrate provider overlays into command resolution** — depends on step 4 and step 13. Built-in registries stay the base; command-scoped overlays layer extension entries for ACP runtimes, review providers, and models.

16. **Implement Go SDK `sdk/extension`** — depends on step 10 (protocol frozen). Extension struct, handler registration, HostAPI client, mock transport for unit tests.

17. **Implement TypeScript SDK `@compozy/extension-sdk`** — depends on step 16 (shape reference). Mirrors Go SDK for TS authors. Includes scaffolding CLI (`npx @compozy/create-extension`).

18. **Ship starter templates** — depends on steps 16 and 17. `lifecycle-observer`, `prompt-decorator`, `review-provider`, `skill-pack` templates for each SDK.

19. **Documentation and examples** — depends on everything. Author guide, protocol reference, capability reference, migration guide, and the explicit trust/enablement model for workspace extensions.

### Technical Dependencies

- No external service dependencies. All work is local-filesystem and in-process.
- Requires Go 1.24+ (current Compozy minimum). No new compiler or toolchain requirements.
- TypeScript SDK requires Node 18+ for stdin byte handling and modern typings.
- `make verify` must pass at every step. Every commit runs fmt + lint + test + build with zero tolerance for warnings.

## Monitoring and Observability

**Structured logging** via existing `log/slog`:

- `level=info component=extension.manager action=start run_id=... extension=... source=workspace`
- `level=info component=extension.manager action=initialize extension=... protocol_version=1 latency_ms=12`
- `level=warn component=extension.dispatcher action=hook_timeout hook=prompt.post_build extension=... deadline_ms=5000`
- `level=error component=extension.host_api action=capability_denied extension=... method=host.tasks.create missing=[tasks.create]`

**Audit log**:

- `.compozy/runs/<run-id>/extensions.jsonl` — every hook dispatch and host API call, JSON Lines, append-only, written alongside the existing `events.jsonl` in the same run directory.
- Observer event delivery is best-effort. When an extension subscriber falls behind, the host follows the current event-bus semantics and records dropped-delivery warnings/metrics instead of blocking the run.

**Events emitted to the bus** (new event kinds under `pkg/compozy/events/kinds`):

- `EventKindExtensionLoaded` — extension registered in the manager.
- `EventKindExtensionReady` — handshake completed successfully.
- `EventKindExtensionFailed` — handshake or runtime failure; payload has extension name, phase, error.
- `EventKindExtensionEvent` — custom event published by an extension via `host.events.publish`.
- `EventKindTaskFileUpdated`, `EventKindArtifactUpdated`, `EventKindTaskMemoryUpdated` — existing kinds reused when extension-initiated writes go through the kernel.

**Metrics** (log-derived for v1; no Prometheus export):

- Extension spawn latency (per extension, per run)
- Hook dispatch latency histogram (per hook name, per extension)
- Capability denial counter
- Recursion guard trips

**Alerting thresholds**: none in v1. This is a local developer tool, not a production service. Failures surface in the CLI and in the Bubble Tea UI as warnings on the run summary.

## Technical Considerations

### Key Decisions

- **Subprocess-only tier (ADR-001)**: one IPC model, per-run lifetime; chosen over multi-tier because Compozy is a per-run CLI and has zero current users demanding sandbox or in-process performance.
- **Per-run lifetime with exec opt-in (ADR-002)**: executable extension state scopes naturally to a single run's artifacts; `exec` remains opt-in so ad hoc prompts keep their current fast path unless the operator asks for extensions.
- **Shared subprocess package (ADR-003)**: avoids duplicating ACP plumbing; one code path for process management across all Compozy subprocess use cases.
- **Priority-ordered mutation pipeline (ADR-004)**: deterministic execution across multi-extension compositions; matches ecosystem precedent (webpack, vite, babel).
- **Declarative capabilities without trust tiers (ADR-005)**: appropriate for local-first tool; clear audit trail at install/enable and per-call; capability list visible to users before an extension becomes active.
- **Minimal Host API (ADR-006)**: covers the three validated user stories (prompt augmentation, run-triggered commands, follow-up task creation) without exposing the kernel dispatcher, and aligns memory/task operations to Compozy's real file-backed model.
- **Three-level discovery with TOML-first manifest (ADR-007)**: matches existing Compozy conventions; supports bundled first-party, global user, and per-project workspace extensions with operator-local enablement.

### Known Risks

- **Refactor of `internal/core/agent` subprocess logic**: touches a working path. Mitigation: move files verbatim first, update imports, verify existing ACP tests pass, then introduce abstractions only when both callers prove they need them.
- **Per-run startup cost for expensive extensions**: extensions that load large models/indexes on startup make every run slower. Mitigation: document lifecycle contract, recommend lazy init, preserve a future persistent-host escape valve.
- **Skill-pack drift across agents**: extension-provided skills can drift out of sync with the installed agent directories. Mitigation: reuse existing setup/verify flows and surface extension skill drift in `compozy ext doctor` and command preflight.
- **Capability granularity too coarse**: `artifacts.write` covers all paths under `.compozy/`. Mitigation: acceptable for v1; add path scopes in v1.1 if users report issues.
- **Extension that exceeds declared capabilities silently**: capabilities like `subprocess.spawn` and `network.egress` are advisory, not enforced. Mitigation: document clearly; audit log exposes what extensions touched so users can verify intent.
- **Recursion via `host.runs.start`**: mitigated by parent-chain env var with depth bound of 3. Authors can still create cycles across extensions, but the depth cap turns infinite loops into bounded errors.
- **Priority ties across extensions**: mitigated by alphabetical tiebreak and `compozy ext doctor` warnings.
- **Best-effort observer delivery**: slow event subscribers can drop notifications under the current bounded bus implementation. Mitigation: document that only mutable hook chains are synchronous and deterministic; observer streams are diagnostic/observational, not exactly-once.

## Architecture Decision Records

ADRs documenting key decisions made during technical clarification:

- [ADR-001: Subprocess-Only Extension Model](adrs/adr-001.md) — No WebAssembly or Go-native tiers in v1; single subprocess IPC model reusing existing ACP plumbing.
- [ADR-002: Per-Run Extension Lifetime](adrs/adr-002.md) — Extensions spawn at run start and shut down at run end; no daemon mode in v1.
- [ADR-003: JSON-RPC 2.0 over stdio with Shared `internal/core/subprocess` Package](adrs/adr-003.md) — Extract and share subprocess plumbing across ACP and extensions.
- [ADR-004: Priority-Ordered Mutation Pipeline for Hooks](adrs/adr-004.md) — Deterministic chain with numeric priority, matching webpack/vite conventions.
- [ADR-005: Capability-Based Security Without Trust Tiers](adrs/adr-005.md) — Declarative capabilities enforced at Host API and dispatch, no marketplace-oriented tier system.
- [ADR-006: Host API Surface for Extension Callbacks](adrs/adr-006.md) — Minimal callback surface covering tasks, runs, artifacts, events, prompts, memory.
- [ADR-007: Three-Level Discovery with TOML-First Manifest](adrs/adr-007.md) — Bundled + user + workspace discovery with TOML primary and JSON fallback.
