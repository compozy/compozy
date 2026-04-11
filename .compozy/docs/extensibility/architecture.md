# Architecture Overview

Compozy extension execution is intentionally simple: every executable extension is a subprocess that speaks JSON-RPC 2.0 over stdin/stdout.

## Lifecycle

Each run follows the same high-level sequence:

1. Compozy discovers extensions from bundled, user, and workspace scopes.
2. Enablement is resolved locally on the operator machine.
3. The extension manager spawns enabled executable extensions for the run scope.
4. Compozy sends `initialize` as the first request.
5. The extension answers with protocol version `1`, accepted capabilities, supported hooks, and lifecycle support flags.
6. Compozy dispatches hooks and events during the run.
7. Compozy sends `shutdown` before tearing the run scope down.

There is no restart or resume semantics in v1. Every Compozy run gets a fresh subprocess.

## Discovery and precedence

Discovery order is:

- bundled
- user
- workspace

Effective precedence is the reverse:

- workspace overrides user
- user overrides bundled

Enablement is separate from discovery. Bundled extensions are active by default. User and workspace extensions stay disabled until the local operator enables them.

## Wire model

The transport is line-delimited UTF-8 JSON. Each line is one JSON-RPC message. There are no batch requests in v1.

The two traffic directions are:

- Compozy -> extension: `initialize`, `execute_hook`, `on_event`, `health_check`, `shutdown`
- extension -> Compozy: `host.events.*`, `host.tasks.*`, `host.runs.*`, `host.artifacts.*`, `host.prompts.render`, `host.memory.*`

Responses can arrive out of order. Correlation is by JSON-RPC `id`.

## Capability model

Capabilities are declared in the manifest and granted during initialize. The runtime enforces them in two places:

- when the extension registers or serves hooks
- when the extension calls Host API methods

Two capabilities are advisory only:

- `network.egress`
- `subprocess.spawn`

They communicate intent and improve auditability, but the operating system does not enforce them.

## Hook execution model

Mutable hooks are synchronous and chain in priority order. The output of one extension becomes the input to the next one.

Observe-only hooks are dispatched concurrently on a best-effort basis. They may receive the same payload shape as a mutable hook family, but any returned patch is ignored.

## Event delivery model

Extensions that accept `events.read` and report `supports.on_event = true` may receive bus events through `on_event`.

By default the subscription is unfiltered. An extension can narrow it with `host.events.subscribe({ kinds })`.

## Host API model

The Host API exists so extensions do not need to shell out to `compozy` or write internal files directly.

Notable guarantees:

- `host.tasks.create` uses the real task writer and emits task-file events
- `host.memory.write` uses the workflow-memory writer and preserves Markdown document semantics
- `host.runs.start` applies recursion protection through the parent-run chain
- `host.artifacts.*` is scoped to the workspace root and `.compozy/`

## Observability

Every hook dispatch and Host API call is recorded in:

```text
.compozy/runs/<run-id>/extensions.jsonl
```

The event bus also emits extension lifecycle events such as `extension.loaded`, `extension.ready`, and `extension.failed`.

## Design references

- ADR-001: subprocess-only execution model
- ADR-003: JSON-RPC 2.0 over stdio
- ADR-005: capability-based security without trust tiers
- ADR-006: minimal Host API surface
- ADR-007: three-level discovery with local enablement
