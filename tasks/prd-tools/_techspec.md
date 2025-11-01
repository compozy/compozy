# Compozy Tooling Refactor: Architecture, Issues, and Upgrade Plan

## 1. Purpose and Scope
- Enable **Go-native tools** to coexist with the current TypeScript/Bun runtime so SDK users can author tools directly in Go.
- Integrate the `tool.Config.Code` field into the execution pipeline so inline TypeScript code is runnable without manual entrypoint stitching.
- Stay aligned with project rules (context-first logger/config, function size limits, no backwards-compat constraints) and Go 1.25+ features.

Deliverable: A clear blueprint covering current architecture, gaps, design decisions, validation strategy, and phased plan for implementation.

## 2. Current Architecture Snapshot

```
Go SDK (tool.New) ──► engine/tool.Config ──► resources.Store ──► appstate
                                                     │
                                                     ▼
                                               LLM Service
                                                     │
                                                     ▼
                          ┌────────── Tool Registry (engine/llm/tool_registry.go:538) ─────────┐
                          │                                                                    │
                native.Definitions() (Go built-ins)                      runtimeAdapter.ExecuteTool()
                          │                                                                    │
                  builtin.Handler (cp__ tools)                    runtime.Runtime (Bun) executes entrypoint
```

Key components and references:
- **SDK constructor** trims and validates runtime/code but only accepts `"bun"` (`sdk/tool/constructor.go:15-118`).
- **Tool configs** store metadata plus `Runtime`, `Code`, schemas (`engine/tool/config.go:35-358`).
- **Registration** persists tool configs via `compozy.Engine` into the resource store (`sdk/compozy/engine_registration.go:111-157`).
- **Execution pipeline**:
  - `engine/llm/service.go:118-189` registers cp__ built-ins then wraps `tool.Config` as `localToolAdapter`.
  - `localToolAdapter` delegates to `runtimeAdapter.ExecuteTool`, which invokes `runtime.Runtime.ExecuteTool` (`engine/llm/tool_registry.go:538-576`).
  - Bun worker (`engine/runtime/bun/worker.tpl.ts`) loads exports from a project entrypoint configured through `runtime.Config.EntrypointPath` (`engine/runtime/bun_manager.go:700-747`).
- **Built-in Go tooling** (cp__ prefixed) is managed through `native.Definitions` + `builtin.RegisterBuiltins` (`engine/tool/native/catalog.go`, `engine/tool/builtin/definition.go`).

### Observations
- Go-only tooling is restricted to hard-coded cp__ built-ins; user-defined Go handlers are absent.
- Inline TypeScript strings validated at SDK level never reach the runtime; no pipeline writes or evaluates `Code`.
- Runtime assumes an existing entrypoint file, typically crafted manually or via template scaffolding.

## 3. Problem Statement
1. **Gap:** No path for programmatic Go tools in SDK (`Runtime=go` unsupported, handlers cannot be registered).
2. **Gap:** `tool.Config.Code` is validated (`sdk/tool/constructor.go:103-111`) but ignored downstream; examples such as `sdk/examples/05_runtime_native_tools.go` rely on this placeholder.
3. **Operational friction:** Maintaining manual entrypoint files for inline code conflicts with SDK ergonomics.
4. **Schema + validation:** Input/output schemas exist but are enforced only inside runtime worker; Go handlers don’t benefit from the same pipeline today.

## 4. Requirements

### Functional
1. Allow SDK consumers to register Go-native tools with:
   - Context-first handler signature (`context.Context`, input map, config map).
   - Optional schemas and metadata reused by LLM function calling.
2. Inline TypeScript (`tool.Config.Code`) must execute without manual entrypoint authoring.
3. Agents must mix Go-native and TypeScript tools transparently.
4. Tool definitions still persist to the resource store, but non-serialisable handler state must be reattached programmatically per process.

### Non-Functional
1. Preserve context propagation rules (`logger.FromContext`, `config.FromContext`).
2. Keep functions under 50 lines, use Go 1.25+ idioms (e.g., `sync.WaitGroup.Go` where relevant).
3. No backwards-compatibility support required; we can break legacy patterns.
4. Maintain deterministic tool IDs and avoid cp__ collisions.
5. Resilient file generation (atomic writes into `.compozy`), safe for concurrent updates.

## 5. Proposed Architecture

### 5.1 Go-Native Tool Execution Path

#### 5.1.1 New Concepts
- **Tool Mode:** Extend `tool.Config` with an enum-like field (e.g., `Implementation` or `Kind`) defaulting to `runtime`. `native` indicates Go handler.
- **Native Tool Registry:** New package, e.g., `engine/tool/nativeuser`, storing `Definition{ID, Description, InputSchema, OutputSchema, Handler}` registered at runtime. Backed by `sync.Map` + `Register/Lookup`.
- **Handler Signature:** `func(ctx context.Context, input map[string]any, cfg map[string]any) (map[string]any, error)`. Wrapper will convert to `core.Output`.

#### 5.1.2 SDK Surface (example)
```go
handler := func(ctx context.Context, input map[string]any, cfg map[string]any) (map[string]any, error) {
    log := logger.FromContext(ctx)
    log.Info("native tool invoked", "tool", "weather-brief")
    // … business logic …
    return map[string]any{"summary": "sunny"}, nil
}

cfg, err := tool.New(
    ctx,
    "weather-brief",
    tool.WithName("Weather Brief"),
    tool.WithDescription("Summarises weather from in-memory data"),
    tool.WithNativeHandler(handler, tool.WithInputSchema(schema), tool.WithOutputSchema(schema)),
)
```
- `WithNativeHandler` registers the handler inside the native registry and mutates the config to `Runtime="go"` (or `Implementation=native`).
- The handler is not serialised; registry must be repopulated each process start.

#### 5.1.3 Engine Integration
- **Registration:** `compozy.Engine.registerTool` continues to persist config for ID tracking, but when `Implementation=native`, the resource store record omits runtime-only handler state.
- **Execution:** Update `registerRuntimeTools` (`engine/llm/service.go:170-189`) to branch:
  - If `cfg.Runtime` empty or `"bun"` ⇒ existing `localToolAdapter`.
  - If `cfg.Runtime`/`Implementation` == `"go"` ⇒ register `nativeToolAdapter`.
- **nativeToolAdapter (new):**
  - Retrieves registered handler from `nativeuser.Lookup`.
  - Runs validation using `cfg.ValidateInput` and `cfg.ValidateOutput`.
  - Wraps handler panics to `core.NewError`.
  - Provides metrics/logging parity with JS path.
- **Telemetry:** Extend `logNativeTools` to list user native tool IDs alongside cp__ built-ins.

#### 5.1.4 Tool Resolution
- `ToolResolver` already clones tool configs; no change needed. Ensure deep copies do not lose Implementation flag.
- For REST-created tools, reject `Implementation=native` payloads with validation errors to avoid handler-less configs.

#### 5.1.5 Concurrency & Safety
- Registry operations protected with `sync.RWMutex` or `sync.Once`.
- Optional `context.WithCancel` pipeline inside handler to manage timeouts (surfaced via config).

### 5.2 Inline TypeScript Code Integration

#### 5.2.1 Design Options Reviewed
| Option | Summary | Pros | Cons |
| --- | --- | --- | --- |
| A | Generate files under `.compozy/runtime/inline/` and build a composite entrypoint | Leverages existing Bun worker (no runtime protocol change), supports multiple tools | Requires file IO + watcher infrastructure |
| B | Send `Code` with each ExecuteTool request, evaluate dynamically in worker | No disk writes, immediate updates | Bun worker changes, runtime must compile TS per request, potential perf hit |
| C | Pre-bundle into shared library via CLI | Aligns with distribution flows | Requires new build step, complicates hot reload |

**Chosen:** Option A (file generation + composite entrypoint) for predictable runtime and reuse of existing Bun executor.

#### 5.2.2 Inline Tool Manager
- New component `engine/tool/inline` started during engine boot.
- Responsibilities:
  1. **Sync Files:** Iterate tool configs, for each with `Code` write `./.compozy/runtime/inline/<tool-id>.ts` (stable naming via slug/hash). Use atomic rename to avoid partial files.
  2. **Generate Composite Entrypoint:** Template merges user entrypoint (if configured) and inline exports:
     ```ts
     import * as userExports from "{{userEntrypointRel}}"; // optional
     import tool_weather from "./inline/weather-brief.ts";
     export default {
       ...(userExports.default ?? userExports),
       "weather-brief": tool_weather,
     };
     ```
  3. **Watch Store:** Subscribe to `ResourceStore.Watch(project, ResourceTool)`; re-sync on PUT/DELETE.
- Update runtime wiring: if inline manager detects at least one inline tool, set `runtime.Config.EntrypointPath` to generated file. Preserve user-specified entrypoint (import + merge).
- Ensure `.compozy` directory uses permissions from `runtime.Config.WorkerFilePerm`.

#### 5.2.3 Validation / Error Handling
- Invalid TypeScript should surface as compilation failure during Bun execution; propagate errors via existing `ToolExecutionError`.
- Add lint check in manager to ensure `Code` is non-empty (already enforced at SDK) and optionally run `bun build --check` during sync (guarded by config flag).

### 5.3 Combined Execution Flow (after changes)

```
Agent call ─► ToolResolver ─► Tool Registry
                  │
                  ├─ Native Tool Adapter ─► nativeuser.Handler (Go)
                  └─ Runtime Tool Adapter ─► Bun Runtime
                                              │
                                              ├─ Generated inline entrypoint exports inline code
                                              └─ User entrypoint exports legacy modules
```

## 6. Data Model & API Adjustments
| Area | Change |
| --- | --- |
| `engine/tool.Config` | Add `Implementation string` (json/yaml `implementation,omitempty`), default `runtime`. Keep `Runtime` for backwards compatibility but deprecate once implementation is present. |
| SDK | Add options: `WithNativeHandler`, `WithImplementation`, `WithEntrypointAlias` (optional for custom names). Runtime validation accepts `"bun"` or `"go"`. |
| REST API | Reject `implementation=native` to avoid handler-less configs; allow `code` updates. Document new field. |
| Runtime Config | Inline manager sets `EntrypointPath` to generated file when needed; keep user path in metadata. |

## 7. Implementation Phases

### Phase 1 – Discovery Hardening (already in progress)
- Finalise code references, confirm resource store behaviour (completed by this document).

### Phase 2 – Foundations
1. Add `Implementation` field and update serializers (`engine/tool/config.go`, schema updates).
2. Implement native registry (`engine/tool/nativeuser/registry.go`) with unit tests.
3. Extend SDK options and ensure unit coverage (`sdk/tool/constructor_test.go`).

### Phase 3 – Execution Adapters
1. Implement `nativeToolAdapter` and update `registerRuntimeTools`.
2. Update telemetry/logging and ensure cp__ built-ins unaffected (`engine/llm/service.go`, `engine/tool/builtin/registry.go`).
3. Add panic recovery + schema validation in adapter.

### Phase 4 – Inline Code Manager
1. Introduce `inline.Manager` with sync + watcher logic (new package).
2. Update engine startup to instantiate manager once runtime + store available (probably in `sdk/compozy/engine.go` after store initialisation).
3. Modify runtime config builder to consume generated entrypoint.
4. Add integration tests verifying Bun worker loads generated file (`engine/runtime/bun_manager_test.go`).

### Phase 5 – Validation & Tooling
1. Unit tests for native registry, inline manager, runtime adapters.
2. Integration test using SDK example: Go native tool + inline TS tool in same workflow.
3. Update docs (SDK README, examples) to showcase Go handler usage and inline code support.

## 8. Testing & Validation Strategy
- **Unit**: native registry concurrency, inline manager file output, adapter behaviour with schema validation, error propagation.
- **Integration**: 
  - Run workflow executing Go tool and JS tool sequentially.
  - Ensure `tool.Config.Code` is honoured by verifying generated file contents.
  - Watcher test: Update tool via store stub, expect regenerated entrypoint.
- **End-to-end**: Expand `sdk/examples/05_runtime_native_tools.go` to include Go handler scenario and assert output via `gotestsum`.
- **Performance**: Benchmark inline manager sync on large tool sets; ensure Bun worker initialisation unaffected.
- **Lint/Test gates**: `make lint`, `make test` mandatory before completion per project rules.

## 9. Risks & Mitigations
| Risk | Mitigation |
| --- | --- |
| Handler registry not populated before tool usage | Require `WithNativeHandler` to register during construction; add runtime check raising descriptive error if handler missing. |
| File system race during regeneration | Use atomic temp-file writes + `os.Rename`; guard sync with mutex. |
| User-provided entrypoint conflicts with generated exports | Merge order ensures inline exports override duplicates; document precedence. |
| REST clients attempt to create native tools | Validate in router usecases (e.g., `tooluc.Upsert`) and reject gracefully. |
| Context misuse inside handlers | Provide helper utilities enforcing `logger.FromContext`, `config.FromContext` usage and document expectation. |

## 10. Open Questions
1. Should we allow YAML-defined inline code (from CLI) or keep SDK-only? Decision impacts security posture.
2. Do we need hot reload for inline code (e.g., `compozy dev` watching SDK files)? If so, integrate manager with existing dev loop.
3. Should Bun worker cache compiled inline modules across executions for performance? Evaluate after baseline.

## 11. Relevant Files & References
- `sdk/tool/constructor.go` – runtime validation and code trimming logic.
- `sdk/examples/05_runtime_native_tools.go` – demonstrates placeholder inline code usage.
- `sdk/compozy/engine_registration.go` – tool persistence flow.
- `engine/tool/config.go` – current config schema and validation (no enforcement of runtime/code).
- `engine/llm/service.go` (`registerNativeBuiltins`, `registerRuntimeTools`) – tool registration pipeline.
- `engine/llm/tool_registry.go` (`localToolAdapter`) – runtime adapter logic.
. `engine/runtime/bun_manager.go` & `engine/runtime/bun/worker.tpl.ts` – Bun execution path, entrypoint expectations.
- `engine/tool/native/catalog.go` & `engine/tool/builtin/definition.go` – cp__ built-in infrastructure to emulate.
- `pkg/config/native_tools.go` – telemetry + config gating for native/built-in tooling.
- `engine/resources/store.go` – resource persistence contract (clarifies why handlers must be re-registered in-process).

## 12. Sources Consulted
- `sdk/tool/constructor.go`
- `sdk/tool/options_generated.go`
- `sdk/examples/05_runtime_native_tools.go`
- `sdk/compozy/engine_registration.go`
- `engine/tool/config.go`
- `engine/tool/router/dto.go`
- `engine/tool/native/catalog.go`
- `engine/tool/builtin/definition.go`
- `engine/llm/service.go`
- `engine/llm/tool_registry.go`
- `engine/runtime/bun_manager.go`
- `engine/runtime/bun/worker.tpl.ts`
- `pkg/config/native_tools.go`
- `engine/resources/store.go`

The above references are all within the current repository (`/Users/pedronauck/Dev/compozy/compozy`).
