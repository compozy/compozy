---
status: completed
title: Host API services for tasks, runs, memory, artifacts, prompts, and events
type: backend
complexity: high
dependencies:
  - task_05
---

# Task 06: Host API services for tasks, runs, memory, artifacts, prompts, and events

## Overview
Implement the eleven Host API methods defined in `_protocol.md` section 5.2 as typed service handlers registered with the `HostAPIRouter` from task 05. Each method wraps an existing kernel/service path (task writer, run starter, memory writer, artifact writer, prompt renderer, event bus) so extension-initiated writes flow through the same code as the CLI and emit the same events. The router handles capability enforcement and audit logging; this task focuses on the kernel-side business logic.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md`, `_protocol.md`, and ADR-006.
</critical>

<requirements>
- MUST implement a `KernelOps` interface in `internal/core/extension` listing every kernel operation Host API needs, and provide a default implementation backed by existing Compozy internals (not CLI shellouts).
- MUST implement `host.tasks.list`, `host.tasks.get`, and `host.tasks.create` methods. `host.tasks.create` must own task numbering, frontmatter emission, metadata refresh, and emit `EventKindTaskFileUpdated`.
- MUST implement `host.runs.start` with the recursion guard at depth 3 per ADR-006 using `COMPOZY_PARENT_RUN_ID` propagation.
- MUST implement `host.memory.read` and `host.memory.write` against the Markdown-backed memory model described in `_protocol.md` section 5.5, including `needs_compaction` detection and `mode: replace|append` semantics.
- MUST implement `host.artifacts.read` and `host.artifacts.write` with path scoping that rejects any path outside `.compozy/` or the resolved workspace root; denial returns `-32001 capability_denied` with `data.reason = "path_out_of_scope"`.
- MUST implement `host.prompts.render` as a side-effect-free helper wrapping `internal/core/prompt.Build` and `BuildSystemPromptAddendum`.
- MUST implement `host.events.subscribe` (filter-by-kind) and `host.events.publish` (emit `EventKindExtensionEvent`).
- MUST group implementations into three files at most to stay within the mega-task limit: writes (tasks.create, runs.start, memory.write, artifacts.write), reads (tasks.list, tasks.get, memory.read, artifacts.read), helpers (prompts.render, events.subscribe, events.publish).
- MUST NOT spawn subprocesses, open transports, or construct the extension manager — those belong to tasks 07 and 08.
</requirements>

## Subtasks
- [x] 06.1 Define the `KernelOps` interface covering all eleven Host API methods with typed request/result structs.
- [x] 06.2 Implement `host_writes.go` containing `tasks.create`, `runs.start`, `memory.write`, and `artifacts.write` handlers, wired to existing task writer, kernel run start, memory writer, and artifact writer.
- [x] 06.3 Implement `host_reads.go` containing `tasks.list`, `tasks.get`, `memory.read`, and `artifacts.read` handlers with path scoping and `needs_compaction` detection.
- [x] 06.4 Implement `host_helpers.go` containing `prompts.render`, `events.subscribe`, and `events.publish` handlers.
- [x] 06.5 Register all handlers with `HostAPIRouter.RegisterService` under namespaces `host.tasks`, `host.runs`, `host.memory`, `host.artifacts`, `host.prompts`, `host.events`.
- [x] 06.6 Implement the recursion guard for `host.runs.start` using a parent chain with depth bound 3.
- [x] 06.7 Write tests covering each method's happy path, capability denial, path scoping rejection, and the recursion guard.

## Implementation Details
See TechSpec "Implementation Design → Core Interfaces" for the `KernelOps` interface shape, `_protocol.md` section 5 for method signatures and response shapes, `_protocol.md` section 5.5 for the memory document model, and ADR-006 for rationale.

Place files under:
- `internal/core/extension/host_writes.go` — write handlers (tasks.create, runs.start, memory.write, artifacts.write)
- `internal/core/extension/host_reads.go` — read handlers (tasks.list, tasks.get, memory.read, artifacts.read)
- `internal/core/extension/host_helpers.go` — prompts.render, events.subscribe, events.publish
- `internal/core/extension/host_writes_test.go`
- `internal/core/extension/host_reads_test.go`
- `internal/core/extension/host_helpers_test.go`

Key integration points that must be reached without shelling out:
- `host.tasks.create` writes task files through the existing task writer used by the in-process PRD→tasks pipeline (search for task writer / frontmatter emission in `internal/core/prompt/common.go` and related code).
- `host.runs.start` invokes the kernel dispatcher directly with a new command, passing the parent chain in environment variables.
- `host.memory.read/write` uses the memory document writer from the `cy-workflow-memory` flow (same writer the skill uses).
- `host.artifacts.read/write` uses a scoped filesystem wrapper that validates every path against the resolved workspace root from initialize.
- `host.events.publish` pushes an `EventKindExtensionEvent` into the journal so it is persisted before fan-out.

### Relevant Files
- `internal/core/prompt/common.go` — Existing `Build` and `BuildSystemPromptAddendum` functions.
- `internal/core/kernel/handlers.go` — Existing kernel operations that `host.runs.start` will invoke.
- `internal/core/plan/prepare.go` — Existing task file resolution that `host.tasks.*` depends on.
- `internal/core/run/journal/journal.go` — Existing journal writer used by `host.events.publish`.
- `pkg/compozy/events/event.go` — Existing event kinds and payload format.
- `_protocol.md` sections 5.3, 5.4, 5.5 — Contract specifics for `host.tasks.create`, `host.runs.start`, `host.memory.*`.
- `adrs/adr-006.md` — Host API surface rationale.

### Dependent Files
- Task 08 (manager lifecycle) constructs `KernelOps` with a real Compozy kernel binding and passes it to the router.
- Task 14 (Go SDK) mirrors these method signatures on the SDK client side.
- Task 15 (TypeScript SDK) mirrors the same shapes in TypeScript.

### Related ADRs
- [ADR-006: Host API Surface for Extension Callbacks](adrs/adr-006.md) — Governs the full method inventory.
- [ADR-005: Capability-Based Security Without Trust Tiers](adrs/adr-005.md) — Every method is capability-gated.

## Deliverables
- Three handler files implementing all eleven Host API methods.
- `KernelOps` interface with a concrete implementation wired to existing kernel paths.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for recursion guard, path scoping, and typed kernel paths **(REQUIRED)**

## Tests
- Unit tests:
  - [x] `host.tasks.create` returns a task with the next sequential number within the workflow directory.
  - [x] `host.tasks.create` emits `EventKindTaskFileUpdated` with the new task path.
  - [x] `host.tasks.list` returns tasks in filename-sorted order.
  - [x] `host.tasks.get` returns the parsed frontmatter and body for an existing task.
  - [x] `host.runs.start` returns a new run id and increments the parent chain length.
  - [x] `host.runs.start` returns `-32001 capability_denied` with `data.reason = "recursion_depth_exceeded"` when the chain is already at length 3.
  - [x] `host.memory.read` returns `{exists: false, content: ""}` when the memory file is absent.
  - [x] `host.memory.read` returns `needs_compaction: true` when the document exceeds the compaction threshold.
  - [x] `host.memory.write` in `append` mode atomically appends with a newline separator.
  - [x] `host.memory.write` in `replace` mode overwrites the document and emits `EventKindTaskMemoryUpdated`.
  - [x] `host.artifacts.write` rejects an absolute path outside the workspace root with `data.reason = "path_out_of_scope"`.
  - [x] `host.artifacts.write` rejects a path containing `..` traversal.
  - [x] `host.artifacts.read` returns bytes for a file under `.compozy/`.
  - [x] `host.prompts.render` returns the rendered prompt for a valid template name and params.
  - [x] `host.events.publish` emits `EventKindExtensionEvent` on the bus.
  - [x] `host.events.subscribe` returns a subscription id and accepts a filter list.
- Integration tests:
  - [x] End-to-end write path: extension calls `host.tasks.create`, then `host.tasks.get` returns the created task with matching content.
  - [x] End-to-end recursion guard: three nested `host.runs.start` calls succeed, the fourth is rejected.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- All eleven Host API methods are reachable through `HostAPIRouter` and produce audit entries
- No Host API method shells out to `compozy` subcommands
