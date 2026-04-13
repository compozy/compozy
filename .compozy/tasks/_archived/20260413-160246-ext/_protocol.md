# Compozy Extension Subprocess Protocol Specification

## Status

Draft

## Date

2026-04-10

## Purpose

This document is the **normative wire-level contract** for Compozy executable extension subprocesses. It complements:

- `_techspec.md` for architecture, package model, Host API inventory, and integration points
- `adrs/adr-001.md` through `adrs/adr-007.md` for rationale behind each decision referenced below

If another document conflicts with this file on **transport framing, lifecycle, handshake fields, error codes, or method-direction semantics**, this file wins.

### Scope

This specification covers **executable extensions** only — extensions that declare a `[subprocess]` section in their manifest and whose runtime logic runs as an OS child process of Compozy. Declarative assets (provider overlays under `[[providers.*]]` and skill packs under `[resources.skills]`) are loaded directly from the manifest by the extension discovery pipeline and never cross this wire. They are described in `_techspec.md` but not in this protocol.

### When the wire contract applies

An executable extension subprocess is spawned and enters the lifecycle described here only when all of the following hold:

1. The invoking Compozy command is one that enables executable extensions:
   - `compozy start` — always enabled
   - `compozy fix-reviews` — always enabled
   - `compozy exec` — enabled only when `--extensions` is supplied
   - Any other command — executable extensions are not spawned even if declarative assets from those extensions participate in the command
2. The extension is **enabled** by the local operator:
   - Bundled extensions are enabled by default
   - User and workspace extensions are disabled by default until the operator runs `compozy ext enable <name>` (workspace enablement is stored outside the repository)
3. The extension manifest declares a `[subprocess]` section. A pure skill-pack or pure provider extension without a subprocess command does not participate in this protocol.

If any of the above is false, no subprocess is spawned and no JSON-RPC frame is exchanged.

---

## 1. Transport

### 1.1 Base transport

- The protocol uses **JSON-RPC 2.0** over the subprocess `stdin`/`stdout` streams.
- Messages are encoded as **UTF-8 JSON**, **one JSON object per line**.
- `stdout` is reserved for protocol frames only.
- Human-readable logs and diagnostics must go to `stderr`.
- Blank lines on `stdout` must be ignored.
- JSON-RPC batch requests are **not supported** in v1.
- Method names beginning with `rpc.` are reserved and must not be used.
- **JSON-RPC notifications** (requests without an `id` field) are **not supported** in v1. All messages must be requests or responses with an `id`. Receivers must ignore notifications silently.
- The transport is **fully multiplexed**. Both peers may have multiple outstanding requests simultaneously. Responses may arrive in any order. Peers must correlate responses by `id`.

### 1.2 Framing rules

- Each line must contain exactly one JSON-RPC request or response object.
- Peers must ignore unknown fields for forward compatibility.
- Per-message encoded size must not exceed **10 MiB**. Messages exceeding this limit must be rejected with `-32603 internal error` carrying `data.reason = "message_too_large"`, and the receiver should close the transport connection.

### 1.3 Request identifiers

- Compozy will use positive integer IDs.
- Extensions may use positive integer IDs or string IDs.
- Fractional numeric IDs must not be used.

### 1.4 Time encoding

- All timestamps are serialized as RFC3339Nano UTC strings, matching Go `time.Time` JSON encoding.

### 1.5 JSON encoding rules

- Struct fields tagged `omitempty` are omitted when zero-valued.
- Fields represented as `json.RawMessage` on the Go side must be serialized as embedded JSON values, not quoted strings.
- Unknown object members must be ignored unless the receiving method explicitly forbids them.

### 1.6 Subprocess environment contract

In addition to whatever the manifest `[subprocess.env]` declares, Compozy injects these environment variables into every spawned extension process:

| Variable | Meaning |
|---|---|
| `COMPOZY_PROTOCOL_VERSION` | Wire version the host will speak (`"1"` in v1) |
| `COMPOZY_RUN_ID` | Current run ID, matches `runtime.run_id` in the initialize request |
| `COMPOZY_PARENT_RUN_ID` | Comma-separated parent chain for recursion detection; empty string for top-level runs |
| `COMPOZY_WORKSPACE_ROOT` | Absolute path to the resolved workspace root |
| `COMPOZY_EXTENSION_NAME` | The extension's own name (for logging) |
| `COMPOZY_EXTENSION_SOURCE` | `bundled`, `user`, or `workspace` — matches `extension.source` in initialize |

These variables exist to make early startup logging possible before the `initialize` handshake completes. They must not be treated as a substitute for the handshake contract.

---

## 2. Roles and Method Directions

Compozy is the **connection initiator** because it launches the subprocess, but after initialization the transport is **bidirectional peer-to-peer JSON-RPC**. Either side may originate requests.

### 2.1 Method families

| Direction | Family | Canonical names |
|---|---|---|
| Compozy → Extension | Base lifecycle methods | `initialize`, `execute_hook`, `on_event`, `health_check`, `shutdown` |
| Extension → Compozy | Host API actions | `host.events.*`, `host.tasks.*`, `host.runs.*`, `host.artifacts.*`, `host.prompts.*`, `host.memory.*` |

Compozy v1 does **not** define a "capability service method" family (the agh pattern where the host calls back into the extension for a declared service like `memory.backend`). If such services are added later they will require a new capability grant and a new method family; v1 extensions never receive Host-API-shaped method names in the `Compozy → Extension` direction.

### 2.2 Naming conventions

- Base lifecycle methods use **snake_case** single tokens: `initialize`, `execute_hook`, `on_event`, `health_check`, `shutdown`.
- Host API methods use the dotted namespace `host.<resource>.<verb>`: `host.tasks.create`, `host.memory.read`, etc.
- Hook event names use **dotted** identifiers: `plan.pre_group`, `prompt.post_build`, `agent.pre_session_create`, etc.

### 2.3 Direction disambiguates ownership

Method names live in separate namespaces by direction and must never collide. `host.*` methods are always Extension → Compozy. Base lifecycle methods are always Compozy → Extension. SDKs must expose these surfaces separately so authors do not confuse:

- `host.tasks.create(...)` — called by the extension against the host
- `extension.handle("execute_hook", ...)` — registered by the extension to serve host dispatches

---

## 3. Connection Lifecycle

The connection lifecycle has five phases. Each extension subprocess lives for the duration of a single Compozy run and goes through these phases exactly once.

1. **Spawn**: Compozy launches the extension process and connects `stdin`/`stdout`. Environment variables from section 1.6 are injected.
2. **Initialize**: Compozy sends `initialize`; the extension accepts or rejects the session contract.
3. **Ready**: Both peers may exchange operational requests. Compozy begins routing hook dispatches and event subscriptions. Reachable before `plan.Prepare()` so `plan.*` and `prompt.*` hooks can participate in the run.
4. **Draining**: A shutdown has been initiated. No new operational work is accepted. In-flight work may still complete.
5. **Stopped**: The process exits and the transport closes.

### 3.1 Pre-ready rules

- Before `initialize` succeeds, the only valid request is `initialize`.
- Any other request in either direction before readiness must fail with `-32003 not_initialized`.
- Compozy must not route hooks, events, or Host API calls before readiness.
- The extension must not send Host API calls before receiving a successful `initialize` response.

### 3.2 Ready transition

There is **no separate `initialized` notification in v1**. The connection enters **Ready** immediately after:

1. `initialize` returns success
2. Compozy verifies the selected `protocol_version`
3. Compozy verifies that the accepted capability list is a subset of the granted capability list

Compozy emits `EventKindExtensionReady` on the host event bus after transitioning to Ready.

### 3.3 Per-run scope, no restart semantics

Because every run spawns a fresh subprocess and discards it at run end (ADR-002), there is no extension restart or connection resumption in v1. An extension that crashes mid-run is not respawned. The run's remaining hooks skip that extension (for optional hooks) or fail the run (for required hooks), as defined in section 6.8.

A subsequent Compozy run goes through the full Spawn → Initialize → Ready sequence again with no inherited state. Extensions must not assume any in-process state persists across runs. For cross-run state the extension must use `host.memory.*` or its own on-disk storage.

### 3.4 Draining rules

- After `shutdown` has been sent by Compozy or acknowledged by the extension, both peers must stop accepting new operational requests.
- New requests during draining must fail with `-32004 shutdown_in_progress`.
- Responses for already accepted in-flight requests may still be delivered until the process exits or the shutdown deadline elapses.

---

## 4. Initialize Handshake

The `initialize` handshake establishes:

- protocol version compatibility
- the set of capabilities granted by the operator at install/enable time
- runtime identifiers (run, workspace, recursion chain)
- timing policies (hook timeouts, shutdown deadlines)

### 4.1 Initialize request

Compozy must send `initialize` as the first request.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocol_version": "1",
    "supported_protocol_versions": ["1"],
    "compozy_version": "0.1.9",
    "extension": {
      "name": "compozy-ext-userprefs",
      "version": "0.1.0",
      "source": "user"
    },
    "granted_capabilities": [
      "events.read",
      "prompt.mutate",
      "tasks.read",
      "tasks.create",
      "artifacts.read",
      "memory.read"
    ],
    "runtime": {
      "run_id": "run-01K6B5YN9XQH8VZ7R2P3T4F5H6",
      "parent_run_id": "",
      "workspace_root": "/Users/alice/projects/acme",
      "invoking_command": "start",
      "shutdown_timeout_ms": 10000,
      "default_hook_timeout_ms": 5000,
      "health_check_interval_ms": 0
    }
  }
}
```

### 4.2 Initialize request fields

| Field | Type | Required | Meaning |
|---|---|---|---|
| `protocol_version` | string | yes | Compozy's preferred protocol version |
| `supported_protocol_versions` | array&lt;string&gt; | yes | Ordered list of versions Compozy can speak |
| `compozy_version` | string | yes | Compozy semver string, informational |
| `extension.name` | string | yes | Manifest name Compozy loaded |
| `extension.version` | string | yes | Manifest version Compozy loaded |
| `extension.source` | string | yes | Discovery level: `bundled`, `user`, or `workspace`. Indicates where the manifest was found; does not imply differential trust (ADR-005). |
| `granted_capabilities` | array&lt;string&gt; | yes | Capability grants enforced at hook dispatch and Host API boundaries |
| `runtime.run_id` | string | yes | Current run ID; matches `COMPOZY_RUN_ID` |
| `runtime.parent_run_id` | string | yes | Comma-separated parent chain for recursion detection; empty for top-level runs |
| `runtime.workspace_root` | string | yes | Absolute path to the resolved workspace root |
| `runtime.invoking_command` | string | yes | Compozy subcommand that opened the run scope: `start`, `fix-reviews`, or `exec` |
| `runtime.shutdown_timeout_ms` | integer | yes | Graceful shutdown deadline before signal escalation |
| `runtime.default_hook_timeout_ms` | integer | yes | Default timeout when a hook declaration omits one |
| `runtime.health_check_interval_ms` | integer | yes | Periodic probe interval. `0` means no probes (host-driven on demand) |

### 4.3 Initialize response

The extension must answer with the selected version and the accepted session contract.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocol_version": "1",
    "extension_info": {
      "name": "compozy-ext-userprefs",
      "version": "0.1.0",
      "sdk_name": "@compozy/extension-sdk",
      "sdk_version": "0.1.0"
    },
    "accepted_capabilities": [
      "events.read",
      "prompt.mutate",
      "tasks.read",
      "tasks.create",
      "artifacts.read",
      "memory.read"
    ],
    "supported_hook_events": [
      "prompt.pre_build",
      "prompt.post_build",
      "run.post_shutdown"
    ],
    "supports": {
      "health_check": true,
      "on_event": true
    }
  }
}
```

### 4.4 Initialize response rules

- `protocol_version` must be one of the versions Compozy offered in `supported_protocol_versions`.
- `accepted_capabilities` must be a subset of `granted_capabilities` from the request. The extension may accept fewer capabilities than granted; it must not accept any it was not offered.
- `supported_hook_events` must list only event names from the Compozy hook taxonomy (section 6.5). Any unknown name must be rejected by Compozy and results in a session termination.
- `supports.health_check = true` indicates the extension implements the `health_check` method. If `false`, Compozy must not send probes.
- `supports.on_event = true` indicates the extension implements the `on_event` method for bus events. If the extension accepted the `events.read` capability but reports `supports.on_event = false`, Compozy rejects the session with `-32602 invalid params` because the contract is internally inconsistent.

### 4.5 Capability negotiation semantics

Capabilities flow through two stages:

1. **Static declaration** in the manifest's `[security].capabilities` list. The operator sees this list at install/enable time (`compozy ext install` prints it and requires confirmation).
2. **Runtime grant** in `initialize.granted_capabilities`. Compozy always sends exactly the capabilities the operator confirmed. There is no source-tier redaction step (ADR-005).

If the extension cannot operate with the granted capabilities — for example, the manifest has drifted from the operator-confirmed set due to a corrupted install — it must reject initialization with `-32001 capability_denied`:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32001,
    "message": "Capability denied",
    "data": {
      "missing": ["memory.write"],
      "granted": ["events.read", "prompt.mutate", "memory.read"]
    }
  }
}
```

### 4.6 Generic initialization failure

If the extension cannot initialize for application-level reasons (missing config, unreachable dependency), it must return `-32603 internal error` with structured `data`:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32603,
    "message": "Internal error",
    "data": {
      "reason": "config_missing",
      "detail": "expected COMPOZY_USERPREFS_PATH in environment"
    }
  }
}
```

### 4.7 Version mismatch

Unsupported protocol versions must use standard JSON-RPC `-32602 invalid params`:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params",
    "data": {
      "reason": "unsupported_protocol_version",
      "requested": "2",
      "supported_protocol_versions": ["1"]
    }
  }
}
```

---

## 5. Operational Requests

### 5.1 Base methods (Compozy → Extension)

| Method | Required | Purpose |
|---|---|---|
| `execute_hook` | yes, if any hook is declared | Dispatch one hook invocation with a typed payload |
| `on_event` | yes, if `events.read` accepted | Deliver one lifecycle event from the bus to the extension |
| `health_check` | yes, if `supports.health_check = true` | Probe liveness/readiness |
| `shutdown` | yes | Begin graceful drain and exit |

### 5.2 Host API methods (Extension → Compozy)

The canonical Host API inventory for v1:

| Method | Required capability | Purpose |
|---|---|---|
| `host.events.subscribe` | `events.read` | Register a server-side filter for lifecycle events the extension wants to receive |
| `host.events.publish` | `events.publish` | Publish a custom event onto the bus (emits `EventKindExtensionEvent`) |
| `host.tasks.list` | `tasks.read` | Enumerate task files in a workflow directory |
| `host.tasks.get` | `tasks.read` | Read a single task file with parsed frontmatter and body |
| `host.tasks.create` | `tasks.create` | Create a new task file through the typed task service (owns numbering, frontmatter, metadata refresh, and event emission). Not a shell-out to any CLI subcommand. |
| `host.runs.start` | `runs.start` | Programmatically start a new Compozy run, propagating `COMPOZY_PARENT_RUN_ID` (see 5.4) |
| `host.artifacts.read` | `artifacts.read` | Read a file under the allowed artifact roots |
| `host.artifacts.write` | `artifacts.write` | Write a file under the allowed artifact roots; emits `EventKindArtifactUpdated` |
| `host.prompts.render` | none | Render a built-in prompt template by name with params. Helper only; no side effects. |
| `host.memory.read` | `memory.read` | Read a workflow memory document (see 5.5) |
| `host.memory.write` | `memory.write` | Write a workflow memory document (see 5.5) |

Normative rules that apply to every Host API call:

- **Authorization**: Compozy checks the requested method against `accepted_capabilities` from the initialize response. Unauthorized calls return `-32001 capability_denied` (section 9.2).
- **Timeout**: Host API calls use Compozy's default request timeout unless the method documents its own deadline. The default timeout is 30 s in v1.
- **Rate limiting**: Per-extension rate limits may return `-32002 rate_limited` with `data.retry_after_ms` whenever estimable.
- **Startup gating**: Calls before the extension has processed the initialize response return `-32003 not_initialized`.
- **Shutdown gating**: Calls after the extension has acknowledged `shutdown` return `-32004 shutdown_in_progress`.
- **Audit**: Every call (success or failure) is written to `.compozy/runs/<run-id>/extensions.jsonl` with the extension name, method, capability, latency, and result code.
- **Path scoping**: `host.artifacts.*` reject any path that is not under `.compozy/` or the workspace root resolved at initialize time. No path traversal, no absolute paths outside those roots.

### 5.3 `host.tasks.create` contract

Compozy's task creation is served by the typed task service in `internal/core/extension` that wraps the same writers used by the in-process `cy-create-tasks` flow. It is **not** a shell-out to any CLI command.

Request:

```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "method": "host.tasks.create",
  "params": {
    "workflow": "feat-extensibility",
    "title": "Post-run verification",
    "body": "Run make verify and attach the output.",
    "frontmatter": {
      "status": "pending",
      "type": "chore"
    }
  }
}
```

Response:

```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "result": {
    "workflow": "feat-extensibility",
    "number": 7,
    "path": ".compozy/tasks/feat-extensibility/task_07.md",
    "status": "pending"
  }
}
```

The host assigns the next available task number within the workflow, writes the file through the task writer, refreshes any task-index metadata, and emits `EventKindTaskFileUpdated` before returning. The extension must not assume a specific number in advance.

### 5.4 `host.runs.start` and recursion protection

`host.runs.start` lets an extension programmatically launch a new Compozy run (for example, to trigger `compozy exec` after a successful `compozy fix-reviews`). To prevent infinite recursion Compozy enforces a bounded parent chain.

Rules:

- Compozy maintains a parent chain stored in `COMPOZY_PARENT_RUN_ID`. For top-level runs it is the empty string. For child runs launched through `host.runs.start` it is the comma-separated list of ancestor run IDs ending with the direct parent.
- When an extension calls `host.runs.start`, Compozy appends the current `run_id` to the chain and exposes the new chain to the child run via env and the child's initialize `runtime.parent_run_id`.
- If the current chain length is already 3 or greater, Compozy rejects the call with error code `-32001 capability_denied` and `data.reason = "recursion_depth_exceeded"`.
- Extensions cannot disable this guard.

Request:

```json
{
  "jsonrpc": "2.0",
  "id": 20,
  "method": "host.runs.start",
  "params": {
    "command": "exec",
    "args": ["--task", "post-run-check"],
    "inherit_env": true
  }
}
```

Response:

```json
{
  "jsonrpc": "2.0",
  "id": 20,
  "result": {
    "run_id": "run-01K6B5ZD3P2QH7M8T4X5V7W9K1",
    "parent_run_id": "run-01K6B5YN9XQH8VZ7R2P3T4F5H6"
  }
}
```

The call returns as soon as the child run has been accepted and its `run_id` allocated. It does not block for completion. Extensions that need completion signals must subscribe to `run.post_shutdown` or the `EventKindRunCompleted` bus event.

### 5.5 `host.memory.read` and `host.memory.write` contract

Compozy's workflow memory is Markdown-backed (see `cy-workflow-memory` skill). Memory is modeled as a **document**, not as an opaque key/value store. Each document corresponds to a single `.md` file under `.compozy/tasks/<workflow>/memory/`.

Identifiers:

- `workflow` — required. Workflow slug used by the task directory.
- `task_file` — optional. When present, identifies a per-task memory file (for example, `task_03.md`). When omitted, the request targets the workflow's aggregate `MEMORY.md`.

`host.memory.read` request:

```json
{
  "jsonrpc": "2.0",
  "id": 30,
  "method": "host.memory.read",
  "params": {
    "workflow": "feat-extensibility",
    "task_file": "task_03.md"
  }
}
```

`host.memory.read` response:

```json
{
  "jsonrpc": "2.0",
  "id": 30,
  "result": {
    "path": ".compozy/tasks/feat-extensibility/memory/task_03.md",
    "content": "## Context\\n...markdown body...\\n",
    "exists": true,
    "needs_compaction": false
  }
}
```

Fields:

| Field | Type | Required | Meaning |
|---|---|---|---|
| `path` | string | yes | Absolute workspace-relative path the host resolved |
| `content` | string | yes | Full Markdown document contents; empty string if `exists = false` |
| `exists` | boolean | yes | Whether the document file exists on disk |
| `needs_compaction` | boolean | yes | Whether the document exceeds the compaction threshold and should be summarized by the caller |

`host.memory.write` request:

```json
{
  "jsonrpc": "2.0",
  "id": 31,
  "method": "host.memory.write",
  "params": {
    "workflow": "feat-extensibility",
    "task_file": "task_03.md",
    "content": "## Context\\n...updated markdown body...\\n",
    "mode": "replace"
  }
}
```

`mode` accepts `replace` (default) or `append`. In `append` mode the host atomically appends the provided content to the existing document with a single newline separator. Writes always go through the same writer used by the `cy-workflow-memory` flow, which emits `EventKindTaskMemoryUpdated`.

Response:

```json
{
  "jsonrpc": "2.0",
  "id": 31,
  "result": {
    "path": ".compozy/tasks/feat-extensibility/memory/task_03.md",
    "bytes_written": 412
  }
}
```

Extensions must not write memory documents through `host.artifacts.write`. The memory writer enforces structural invariants that the generic artifact writer does not.

---

## 6. Hook Dispatch: `execute_hook`

`execute_hook` is the canonical Compozy → extension hook invocation method. It is used for both mutable hooks (where the extension returns a patch that mutates pipeline state) and observe-only hooks (where the extension is informed but cannot change the pipeline).

### 6.1 Request shape

```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "method": "execute_hook",
  "params": {
    "invocation_id": "hook-01K6B5YN9XQH8VZ7R2P3T4F5H6",
    "hook": {
      "name": "append-user-preferences",
      "event": "prompt.post_build",
      "mutable": true,
      "required": false,
      "priority": 500,
      "timeout_ms": 5000
    },
    "payload": {
      "run_id": "run-01K6B5YN9XQH8VZ7R2P3T4F5H6",
      "job_id": "job-01K6B5ZA2MXN3P7F9H6K2T1V4B",
      "prompt_text": "...rendered prompt body...",
      "batch_params": { "name": "feat-x", "mode": "prd", "round": 1 }
    }
  }
}
```

### 6.2 Request fields

| Field | Type | Required | Meaning |
|---|---|---|---|
| `invocation_id` | string | yes | Opaque identifier for one hook invocation. Extensions must not parse its internal structure. |
| `hook.name` | string | yes | The extension's own declaration name for this hook (from the manifest). |
| `hook.event` | string | yes | Canonical hook event name from section 6.5. |
| `hook.mutable` | boolean | yes | `true` when the extension's patch will be applied to the live pipeline. `false` when the event is observe-only (any returned patch is discarded). |
| `hook.required` | boolean | yes | Whether an extension failure aborts the run (required) or is skipped with a warning (optional). |
| `hook.priority` | integer | yes | The effective priority of this extension in the chain for this event (range `[0, 1000]`, default 500). Informational — the host already ordered the chain. |
| `hook.timeout_ms` | integer | yes | Effective timeout Compozy will enforce for this invocation. |
| `payload` | object | yes | Event-specific payload object. Shape defined per event in section 6.5. |

### 6.3 Response shape

```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "result": {
    "patch": {
      "prompt_text": "...rendered prompt body...\\n\\nUser preferences: prefers terse output."
    }
  }
}
```

### 6.4 Response rules

- A successful response must return an object.
- `{}` means **no-op**. Compozy must accept this form.
- `{"patch": {}}` also means no-op.
- `patch` must match the event's patch schema (section 6.5). Unknown fields are ignored.
- For observe-only events (`mutable = false`), any returned `patch` is accepted by the host for forward compatibility but is not applied to pipeline state.
- If the extension has nothing to change, it should prefer `{}` over `{"patch": {}}`.

### 6.5 Event matrix

Events are grouped by pipeline phase. Each row specifies the canonical event name, the payload fields the host sends, the patch fields the host accepts, whether the hook runs on the synchronous critical path, and whether the event is mutable or observe-only.

#### Plan phase

| Event | Payload | Patch | Sync | Mutable |
|---|---|---|---|---|
| `plan.pre_discover` | `{run_id, workflow, mode}` | `{extra_sources?: string[]}` | yes | yes |
| `plan.post_discover` | `{run_id, workflow, entries: IssueEntry[]}` | `{entries?: IssueEntry[]}` | yes | yes |
| `plan.pre_group` | `{run_id, entries: IssueEntry[]}` | `{entries?: IssueEntry[]}` | yes | yes |
| `plan.post_group` | `{run_id, groups: map<string, IssueEntry[]>}` | `{groups?: map<string, IssueEntry[]>}` | yes | yes |
| `plan.pre_prepare_jobs` | `{run_id, groups: map<string, IssueEntry[]>}` | `{groups?: map<string, IssueEntry[]>}` | yes | yes |
| `plan.post_prepare_jobs` | `{run_id, jobs: Job[]}` | `{jobs?: Job[]}` | yes | yes |

#### Prompt phase

| Event | Payload | Patch | Sync | Mutable |
|---|---|---|---|---|
| `prompt.pre_build` | `{run_id, job_id, batch_params: BatchParams}` | `{batch_params?: BatchParams}` | yes | yes |
| `prompt.post_build` | `{run_id, job_id, prompt_text, batch_params: BatchParams}` | `{prompt_text?: string}` | yes | yes |
| `prompt.pre_system` | `{run_id, job_id, system_addendum, batch_params: BatchParams}` | `{system_addendum?: string}` | yes | yes |

#### Agent phase

| Event | Payload | Patch | Sync | Mutable |
|---|---|---|---|---|
| `agent.pre_session_create` | `{run_id, job_id, session_request: SessionRequest}` | `{session_request?: SessionRequest}` | yes | yes |
| `agent.post_session_create` | `{run_id, job_id, session_id, identity: SessionIdentity}` | none | yes | observe-only |
| `agent.pre_session_resume` | `{run_id, job_id, resume_request: ResumeSessionRequest}` | `{resume_request?: ResumeSessionRequest}` | yes | yes |
| `agent.on_session_update` | `{run_id, job_id, session_id, update: SessionUpdate}` | none | yes | observe-only |
| `agent.post_session_end` | `{run_id, job_id, session_id, outcome: SessionOutcome}` | none | yes | observe-only |

#### Job / Run phase

| Event | Payload | Patch | Sync | Mutable |
|---|---|---|---|---|
| `job.pre_execute` | `{run_id, job: Job}` | `{job?: Job}` | yes | yes |
| `job.post_execute` | `{run_id, job: Job, result: JobResult}` | none | yes | observe-only |
| `job.pre_retry` | `{run_id, job: Job, attempt, last_error}` | `{proceed: boolean, delay_ms?: integer}` | yes | yes |
| `run.pre_start` | `{run_id, config: RuntimeConfig, artifacts: RunArtifacts}` | `{config?: RuntimeConfig}` | yes | yes |
| `run.post_start` | `{run_id, config: RuntimeConfig}` | none | yes | observe-only |
| `run.pre_shutdown` | `{run_id, reason}` | none | yes | observe-only |
| `run.post_shutdown` | `{run_id, reason, summary: RunSummary}` | none | yes | observe-only |

#### Review phase (active only under `compozy fix-reviews`)

| Event | Payload | Patch | Sync | Mutable |
|---|---|---|---|---|
| `review.pre_fetch` | `{run_id, pr, provider, fetch_config}` | `{fetch_config?: FetchConfig}` | yes | yes |
| `review.post_fetch` | `{run_id, pr, issues: ReviewIssue[]}` | `{issues?: ReviewIssue[]}` | yes | yes |
| `review.pre_batch` | `{run_id, pr, groups: map<string, ReviewIssue[]>}` | `{groups?: map<string, ReviewIssue[]>}` | yes | yes |
| `review.post_fix` | `{run_id, pr, issue: ReviewIssue, outcome: FixOutcome}` | none | yes | observe-only |
| `review.pre_resolve` | `{run_id, pr, issue: ReviewIssue, outcome: FixOutcome}` | `{resolve: boolean, message?: string}` | yes | yes |

#### Artifact phase

| Event | Payload | Patch | Sync | Mutable |
|---|---|---|---|---|
| `artifact.pre_write` | `{run_id, path, content_preview}` | `{path?: string, content?: string, cancel?: boolean}` | yes | yes |
| `artifact.post_write` | `{run_id, path, bytes_written}` | none | yes | observe-only |

**Mutable** events apply the returned patch to the live pipeline value. **Observe-only** events accept a patch for forward compatibility but do not mutate state.

### 6.6 Mutation pipeline semantics

Multiple extensions may declare the same mutable hook. For each mutable event Compozy builds a **priority-ordered chain** and feeds the output of one extension as the input to the next.

Ordering rules (ADR-004):

- Priority is an integer in `[0, 1000]`, default `500`. Lower priority runs earlier.
- Ties are broken alphabetically by extension name.
- The chain for a given event is frozen during `Ready` transition and does not change for the duration of the run.
- Each extension in the chain receives the already-mutated value from the previous extension as the `payload` of its `execute_hook` request. No extension sees the original unmutated value except the first extension in the chain.
- An extension that returns `{}` or `{"patch": {}}` passes the current value through unchanged.
- Patch semantics are **field-replacement**: any field present in the patch replaces that field in the current value. Arrays are replaced wholesale, not merged. Extensions that need additive semantics must compose the full array themselves from the payload.

Observe-only events (`mutable = false`) are not ordered by priority. Compozy dispatches them concurrently to all subscribed extensions on a best-effort basis and does not wait for the responses to proceed with the pipeline.

### 6.7 Chain failure semantics

- If a **required mutable** hook in the chain fails with a JSON-RPC error, a timeout, or a transport error, Compozy aborts the chain and fails the pipeline phase. The run fails with an error that includes the failing extension name, hook event, and original error.
- If an **optional mutable** hook fails, Compozy logs the failure to `extensions.jsonl`, drops that extension's patch, and continues the chain with the value it had before the failing extension.
- If the chain completes with all extensions succeeding or safely skipped, the final value is returned to the pipeline phase as the mutation result.
- Patch rejection (structurally valid JSON that fails event-specific validation) is treated the same as a hook failure.

### 6.8 Deny-style patches

Some hooks allow the extension to veto an operation (e.g., `artifact.pre_write.cancel = true`, `job.pre_retry.proceed = false`). Deny-style fields are only valid on mutable events and are only honored when:

- the invocation is on the critical path (`sync = yes` in the matrix)
- the event schema defines the deny field

Invalid deny attempts are treated as patch rejection.

---

## 7. Event Delivery: `on_event`

Extensions that accepted `events.read` in the initialize response and reported `supports.on_event = true` receive lifecycle events from the Compozy bus via the `on_event` method.

### 7.1 Subscription

By default Compozy forwards **all** bus events to subscribed extensions. An extension may narrow its subscription by calling `host.events.subscribe` with a filter:

```json
{
  "jsonrpc": "2.0",
  "id": 60,
  "method": "host.events.subscribe",
  "params": {
    "kinds": ["run.completed", "job.failed", "task_file.updated"]
  }
}
```

Response:

```json
{
  "jsonrpc": "2.0",
  "id": 60,
  "result": {
    "subscription_id": "sub-01K6B..."
  }
}
```

Until `host.events.subscribe` is called, the extension is considered subscribed to the unfiltered bus. Calling `host.events.subscribe` replaces any previous filter.

### 7.2 `on_event` request

```json
{
  "jsonrpc": "2.0",
  "id": 70,
  "method": "on_event",
  "params": {
    "event": {
      "schema_version": "1.0",
      "run_id": "run-01K6B5YN9XQH8VZ7R2P3T4F5H6",
      "seq": 42,
      "timestamp": "2026-04-10T14:15:22.123456Z",
      "kind": "job.completed",
      "payload": {
        "job_id": "job-01K...",
        "duration_ms": 42123,
        "status": "completed"
      }
    }
  }
}
```

### 7.3 `on_event` response

```json
{
  "jsonrpc": "2.0",
  "id": 70,
  "result": {}
}
```

The response must be an acknowledgement with an empty result object. `on_event` is a one-way delivery: the extension cannot mutate the event or block the bus.

### 7.4 Best-effort delivery

`on_event` delivery is **best-effort** and follows Compozy's bounded event bus semantics:

- Each subscribed extension has a bounded per-extension delivery queue.
- When the queue is full (the extension is slower than the bus producer), new events are dropped for that extension only. The bus producer does not block.
- Dropped events are counted; the host emits `EventKindExtensionEvent` with `payload.kind = "delivery_dropped"` and logs a warning.
- Extensions that need lossless streams must persist state on receipt (e.g., via `host.artifacts.write`) and tolerate gaps.
- Mutable hook chains (`execute_hook` with `mutable = true`) are **not** subject to best-effort delivery. They run on the critical path and their ordering is deterministic.

### 7.5 Failure semantics

- If `on_event` returns a JSON-RPC error, Compozy logs the failure and continues. One failure does not unsubscribe the extension.
- If `on_event` times out repeatedly (more than 5 consecutive timeouts within a single run), Compozy marks the extension's subscription degraded, logs a warning, and stops forwarding further events to that extension for the rest of the run.
- Transport-level failures (process exit, stdin closed) terminate the subscription.

---

## 8. Health Protocol: `health_check`

`health_check` is an optional liveness/readiness probe for extensions that implement long-lived internal state (connections, caches) and want the host to observe their health.

### 8.1 Negotiation

- An extension advertises support via `supports.health_check = true` in the initialize response.
- If the manifest sets `health_check_period` Compozy honors it; otherwise no periodic probes are sent.
- `runtime.health_check_interval_ms = 0` in the initialize request means "no periodic probes for this run".

### 8.2 Request

```json
{
  "jsonrpc": "2.0",
  "id": 90,
  "method": "health_check",
  "params": {}
}
```

### 8.3 Response

```json
{
  "jsonrpc": "2.0",
  "id": 90,
  "result": {
    "healthy": true,
    "message": "",
    "details": {
      "active_requests": 0,
      "queue_depth": 0
    }
  }
}
```

### 8.4 Response fields

| Field | Type | Required | Meaning |
|---|---|---|---|
| `healthy` | boolean | yes | Whether the extension considers itself ready to serve requests |
| `message` | string | no | Human-readable summary for diagnostics |
| `details` | object | no | Optional structured metrics |

### 8.5 Probe policy

- Default probe interval is the manifest's `health_check_period`, or disabled if omitted.
- Default per-probe timeout is **5 s**.
- A transport timeout, disconnect, or JSON-RPC error counts as a failed probe.
- `healthy: false` counts as a failed probe.

### 8.6 Unhealthy threshold

Compozy marks the extension **unhealthy** when either condition occurs:

1. One response explicitly returns `healthy: false`.
2. Two consecutive probes fail by timeout, disconnect, or JSON-RPC error.

When an extension becomes unhealthy, Compozy must:

1. Stop routing new hook dispatches and events to it for the remainder of the run.
2. Log the failure and emit `EventKindExtensionFailed`.
3. Begin graceful shutdown for that extension; other extensions and the run continue.

Because extensions are per-run (ADR-002), unhealthy extensions are **not** respawned within the same run.

---

## 9. Graceful Shutdown: `shutdown`

`shutdown` is Compozy's cooperative drain request. It is always sent in addition to OS signals to give the extension a chance to flush state.

### 9.1 Request

```json
{
  "jsonrpc": "2.0",
  "id": 99,
  "method": "shutdown",
  "params": {
    "reason": "run_completed",
    "deadline_ms": 10000
  }
}
```

Valid `reason` values: `run_completed`, `run_cancelled`, `run_failed`, `manager_error`, `health_failed`.

### 9.2 Response

```json
{
  "jsonrpc": "2.0",
  "id": 99,
  "result": {
    "acknowledged": true
  }
}
```

### 9.3 Shutdown rules

- The extension must answer `shutdown` promptly, even if it has unfinished background work.
- After answering, it must stop accepting new operational requests.
- It may complete in-flight requests until `deadline_ms` elapses.
- After the `shutdown` response is received, Compozy closes the extension's `stdin` to signal that no more requests will arrive.
- The extension should then close its protocol streams and exit with status `0`.

### 9.4 Signal escalation

If the process does not exit after the cooperative shutdown deadline:

1. Compozy ensures the extension's `stdin` is closed.
2. Compozy sends `SIGTERM` to the managed process group on Unix (via the same `Setpgid`-based mechanism already used for ACP agents), or the platform-equivalent termination on Windows.
3. Compozy waits a short post-signal grace period (implementation-defined, bounded).
4. If the process is still alive, Compozy sends `SIGKILL` on Unix or the platform-equivalent forced termination on Windows.

### 9.5 Default timing

- Default graceful shutdown deadline is the manifest's `subprocess.shutdown_timeout`, or **10 s** if omitted.
- The post-`SIGTERM` grace period is short and bounded; extensions must not rely on it to complete meaningful work.

---

## 10. Error Model

The protocol uses JSON-RPC 2.0 error objects.

### 10.1 Standard JSON-RPC errors

| Code | Message | Use |
|---|---|---|
| `-32700` | `Parse error` | Invalid JSON on the wire |
| `-32600` | `Invalid request` | Invalid JSON-RPC envelope |
| `-32601` | `Method not found` | The receiving peer does not implement the method |
| `-32602` | `Invalid params` | Invalid method parameters, including unsupported protocol version during `initialize` |
| `-32603` | `Internal error` | Unhandled receiver-side failure |

### 10.2 Compozy-defined server errors

| Code | Message | Use |
|---|---|---|
| `-32001` | `Capability denied` | Method, hook event, or Host API action not authorized for this session |
| `-32002` | `Rate limited` | Local backpressure or explicit per-extension rate limit |
| `-32003` | `Not initialized` | Request arrived before successful `initialize` |
| `-32004` | `Shutdown in progress` | Receiver is draining and will not accept new work |

### 10.3 `Method not found` versus `Capability denied`

Use `-32601 method not found` when:

- The receiver does not recognize the method string at all.
- The method is optional and was never implemented on that peer.

Use `-32001 capability denied` when:

- The method exists but the caller was not granted that capability.
- The hook event family exists but was not negotiated for this session.
- A Host API call fails a recursion guard (`data.reason = "recursion_depth_exceeded"`).
- A Host API path is outside the allowed roots (`data.reason = "path_out_of_scope"`).

### 10.4 Error data

Errors should include structured `data` when helpful.

#### Capability denied

```json
{
  "code": -32001,
  "message": "Capability denied",
  "data": {
    "method": "host.tasks.create",
    "required": ["tasks.create"],
    "granted": ["tasks.read"]
  }
}
```

#### Recursion depth exceeded

```json
{
  "code": -32001,
  "message": "Capability denied",
  "data": {
    "method": "host.runs.start",
    "reason": "recursion_depth_exceeded",
    "depth": 3,
    "limit": 3,
    "parent_chain": ["run-01K...A", "run-01K...B", "run-01K...C"]
  }
}
```

#### Path out of scope

```json
{
  "code": -32001,
  "message": "Capability denied",
  "data": {
    "method": "host.artifacts.write",
    "reason": "path_out_of_scope",
    "path": "/etc/passwd",
    "allowed_roots": [".compozy/", "/Users/alice/projects/acme"]
  }
}
```

#### Rate limited

```json
{
  "code": -32002,
  "message": "Rate limited",
  "data": {
    "scope": "host_api.tasks.create",
    "retry_after_ms": 1000,
    "limit": 10,
    "burst": 20
  }
}
```

#### Not initialized

```json
{
  "code": -32003,
  "message": "Not initialized",
  "data": {
    "allowed_methods": ["initialize"]
  }
}
```

#### Shutdown in progress

```json
{
  "code": -32004,
  "message": "Shutdown in progress",
  "data": {
    "deadline_ms": 10000
  }
}
```

### 10.5 Transport failures versus JSON-RPC errors

The following are **transport failures**, not JSON-RPC error responses:

- Peer disconnects before a response arrives.
- Probe or request timeouts.
- OS-level process termination.
- Message exceeds the 10 MiB framing limit.

Callers must treat these as failed requests and apply the lifecycle/recovery rules from this specification.

---

## 11. Rate Limiting and Backpressure

Compozy may protect Host API surfaces with per-extension rate limits. The specific limits are implementation-defined but the wire contract is fixed.

### 11.1 Receiver behavior

- When a peer is willing to reject and retry later, it should return `-32002 rate_limited`.
- `data.retry_after_ms` should be present whenever the receiver can estimate a retry delay.

### 11.2 Caller behavior

- Callers should not immediately retry a `rate_limited` request.
- SDKs should expose `retry_after_ms` to extension authors.

### 11.3 Observer backpressure

Observer delivery (`on_event`) is best-effort (section 7.4). Queue saturation before wire send results in a **local drop**:

- A local drop does not generate an `on_event` request.
- A local drop increments the drop counter for the extension.
- A local drop emits a warning log and may emit `EventKindExtensionEvent` with `payload.kind = "delivery_dropped"`.

Mutable hook chains and Host API calls are **not** subject to best-effort drop; they are either delivered or fail explicitly.

---

## 12. Protocol Versioning

### 12.1 Version token

- v1 uses the exact string `"1"`.
- Protocol versions are exact-match string tokens, not numeric comparisons.

### 12.2 Negotiation

- Compozy sends its preferred version in `protocol_version`.
- Compozy also sends all supported versions in `supported_protocol_versions`.
- The extension must either:
  - return a supported `protocol_version` in the response
  - or reject initialization with `-32602 invalid params` and include `supported_protocol_versions` in the error data

### 12.3 Forward compatibility

Within the same protocol version:

- Receivers must ignore unknown fields.
- Optional fields may be added.
- New optional methods may be added if they are negotiated explicitly during initialization via `supports` flags.

A new protocol version is required when:

- A required field is removed or renamed.
- A method's semantics change incompatibly.
- An existing success or error contract changes incompatibly.

### 12.4 `compozy_version` versus `protocol_version`

`compozy_version` and `protocol_version` are separate:

- `compozy_version` identifies the Compozy build.
- `protocol_version` identifies the subprocess wire contract.

Extensions must not infer protocol compatibility from `compozy_version` alone.

---

## 13. Conformance Rules

An extension is v1-conformant only if it satisfies all of the following:

- Speaks JSON-RPC 2.0 over line-delimited UTF-8 JSON on `stdin`/`stdout`.
- Emits protocol frames only on `stdout`.
- Implements `initialize` and `shutdown`.
- Implements `execute_hook` if any hook declaration appears in the manifest.
- Implements `on_event` if it accepts the `events.read` capability and advertises `supports.on_event = true`.
- Implements `health_check` if it advertises `supports.health_check = true`.
- Honors the negotiated `accepted_capabilities` list and does not call Host API methods whose required capability was not accepted.
- Returns standard JSON-RPC errors for envelope/params failures.
- Returns Compozy custom errors (`-32001` through `-32004`) for capability, rate-limit, and lifecycle gating failures.
- Exits cooperatively after `shutdown`, or tolerates signal escalation.
- Does not emit notifications (section 1.1).
- Does not send Host API requests before receiving a successful `initialize` response.

Compozy is v1-conformant only if it satisfies all of the following:

- Spawns executable extension subprocesses only under the conditions in section "When the wire contract applies".
- Sends `initialize` as the first request.
- Never routes `execute_hook`, `on_event`, or `health_check` before the Ready transition.
- Orders mutable hook chains by priority with alphabetical tiebreak (ADR-004).
- Feeds the output of each mutable hook as the input to the next.
- Enforces `accepted_capabilities` at every Host API entry point.
- Enforces the recursion guard on `host.runs.start` at depth 3.
- Scopes `host.artifacts.read/write` paths to the allowed roots resolved at initialize time.
- Delivers `on_event` best-effort with per-extension bounded queues.
- Performs cooperative shutdown before signal escalation.
- Writes every hook dispatch and Host API call to `.compozy/runs/<run-id>/extensions.jsonl`.

---

## 14. Normative References

- `_techspec.md` — Component architecture, data flow, Core Interfaces, build order.
- `adrs/adr-001.md` — Subprocess-only extension model rationale.
- `adrs/adr-002.md` — Per-run extension lifetime rationale.
- `adrs/adr-003.md` — Shared `internal/core/subprocess` package and JSON-RPC/stdio choice.
- `adrs/adr-004.md` — Priority-ordered mutation pipeline rationale.
- `adrs/adr-005.md` — Capability-based security without trust tiers.
- `adrs/adr-006.md` — Host API surface rationale.
- `adrs/adr-007.md` — Three-level discovery with TOML-first manifest.
- JSON-RPC 2.0 specification — base wire protocol.
- Language Server Protocol base protocol — line-framing precedent.
