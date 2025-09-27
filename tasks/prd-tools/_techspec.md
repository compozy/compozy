# Compozy Native Tool Migration Tech Spec

## Executive Summary

Transform the default tool suite currently shipped as Bun-executed TypeScript packages into a native Go module set that is always available to agents under the `cp__` prefix. The effort removes the per-call Bun process overhead, centralizes security policy, and standardizes request/response contracts. Local project-defined tools continue to rely on the existing runtime pipeline, but built-ins will execute in-process through a new `engine/tool/builtin` package and integrate with `llm.ToolRegistry` during service bootstrapping. The change deletes the legacy `./tools` workspace, simplifying distribution and removing Node/Bun dependencies for core features.

## System Architecture

### Domain Placement

- **engine/tool/builtin** (new): Houses native implementations of the default tool set plus registration helpers.
- **engine/llm**: Extends service initialization to register builtin tools ahead of runtime-backed ones; keeps `ToolRegistry` abstraction unchanged.
- **engine/runtime**: Remains responsible for user-defined JavaScript tools; no runtime shortcut for cp tools, but retains Bun manager for backwards-compatible custom tools.
- **pkg/config**: Adds optional runtime overrides (e.g., `runtime.native_tools.exec.allowlist`) surfaced via `config.FromContext`.
- **cli/docs**: Update documentation and resource loaders to reflect `cp__*` identifiers and removal of JS workspace tooling.

### Component Overview

- **Builtin Registry**: Provides idempotent registration of cp tools using `context.Context`-aware constructors, wiring logger/config via context retrieval.
- **Tool Implementations**: Individual structs implementing `llm.Tool` (+ `ArgsType`) for `cp__read_file`, `cp__write_file`, `cp__delete_file`, `cp__list_files`, `cp__list_dir`, `cp__grep`, `cp__exec`, and `cp__fetch`.
- **Security Layer**: Shared validation utilities for filesystem path normalization, argument allowlisting, and HTTP safety defaults.
- **Schema Catalog**: Go-defined JSON Schemas for inputs/outputs exposed to orchestrator function calling; ensures consistent contracts.

## Implementation Design

### Core Interfaces

```go
type BuiltinDefinition struct {
    ID          string
    Description string
    InputSchema *schema.Schema
    OutputSchema *schema.Schema
    Handler     func(ctx context.Context, input map[string]any) (core.Output, error)
}

type BuiltinTool struct {
    def BuiltinDefinition
}

func (b *BuiltinTool) Name() string
func (b *BuiltinTool) Description() string
func (b *BuiltinTool) Call(ctx context.Context, input string) (string, error)
func (b *BuiltinTool) ArgsType() any
```

A `RegisterBuiltins(ctx, registry, opts)` helper marshals definitions into `BuiltinTool` instances and inserts them into `llm.ToolRegistry` before agent-specific tools load. Options include feature toggles and exec allowlist extensions sourced from `config.FromContext(ctx)`.

`ArgsType` structs must declare JSON tags (for example, `type readFileArgs struct { Path string \\`json:"path"\\` }`) so that schema generation and LLM function-calling stay aligned.

### Data Models

- **Input/Output Schemas**: Defined using `schema.Schema` to mirror existing contracts but enforce consistent top-level keys (e.g., `entries`, `matches`, `stdout`).
- **Exec Command Config**: Structure specifying allowlisted commands and per-command argument policies; project configuration appends to (never replaces) the default safe set shipped with Compozy.
- **Error Code Catalog**: Enumerated constants (`InvalidArgument`, `PermissionDenied`, `FileNotFound`, `CommandNotAllowed`, `Internal`) surfaced via `core.NewError` for programmatic handling.
- **Fetch Client Config**: Defines `Timeout`, `MaxRedirects`, and safe method set; uses Go's `http.Client` with context deadlines.

### Tool Behaviors

- **Filesystem Tools**: Resolve user-supplied paths against a configured project root (`config.NativeTools.RootDir`), validate the absolute path remains under that root, and hard-deny symlink hops for write/delete operations using `Lstat` checks. Writes create parent directories atomically (`os.MkdirAll`), using POSIX-safe file modes.
- **Filesystem Tools**: Resolve user-supplied paths against configured sandbox roots (`config.NativeTools.RootDir` plus any `AdditionalRoots`), validate each absolute path remains under one of those roots, and hard-deny symlink hops for write/delete operations using `Lstat` checks. Writes create parent directories atomically (`os.MkdirAll`), using POSIX-safe file modes.
- **Grep/List Tools**: Stream results via buffered iteration with `maxResults`, `maxFilesVisited`, and `maxFileBytes` limits, ignoring files that trip the binary heuristic (first 8KiB contains null bytes or >30% non-printable runes).
- **Exec Tool**: Executes via `execabs.CommandContext` (prefers absolute paths) on Unix platforms with context deadlines, per-command argument schema validation (regex/enum constraints, max arg counts), and no shell invocation. Provide a Windows fallback that verifies absolute paths via `LookPath` plus explicit extension checks.
- **Fetch Tool**: Wraps `http.Client` with explicit method allowlist, JSON body serialization for map inputs, and header normalization. Enforces TLS verification but exposes opt-in toggles via config.

### Concurrency & Error Handling

- All handlers accept inherited `ctx`, retrieving structured logger/config via `logger.FromContext` and `config.FromContext` (panic-free guards).
- Outputs marshal through `core.Output` to JSON once per request, preserving compatibility with orchestrator pipeline.
- Introduce shared error helpers to wrap validation faults with `core.NewError` codes and ensure canonical error identifiers are attached for orchestration logic.

### Security Considerations

- Centralize command execution policy in Go and ensure command+arguments are separated; never spawn `bash -c`.
- Use `execabs` to prevent PATH-relative execution and maintain allowlist enforcement recommended for Go-based command execution code, with Windows-specific fallbacks as noted above.
- Enforce path normalization and project root sandboxing for filesystem tools; reject any resolved path outside the configured root and refuse to follow symlinks for destructive operations.
- Apply HTTP client limits (defaults: 5s timeout, 5 redirects, 2MiB response cap) to guard against SSRF and hanging requests; expose overrides via config.
- Re-validate execution context (`ctx.Err()`) within long-running operations such as recursive listings to ensure cancellation propagates promptly.

### API Endpoints

- None. All changes are internal to tool execution; no HTTP contract modifications.

## Integration Points

- **LLM Service**: Modify `llm.NewService` to invoke `builtin.RegisterBuiltins` before registering runtime-backed tools; reserve `cp__*` identifiers so agent-defined tools cannot shadow them unless an explicit `config.NativeTools.AllowShadow` flag is set for migration testing.
- **Runtime Loader**: No changes for custom JS tools; remove expectations that default tools exist in generated `tools.ts` entrypoints.
- **CLI Templates & Docs**: Update scaffolding templates, docs, and examples to reference `cp__*` IDs instead of `@compozy/tool-*` packages; remove npm workspace references.
- **Packaging**: Drop `tools/*` from workspace, prune `package.json` and `bun.lock` entries, and adjust release artifacts accordingly.

## Impact Analysis

| Affected Component        | Type of Impact        | Description & Risk                                                                                      | Required Action                                            |
| ------------------------- | --------------------- | ------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| `engine/llm.Service`      | API/Internal behavior | Adds builtin registration during service bootstrap; low risk if ordered before MCP registration         | Refactor initialization; add unit coverage                 |
| `engine/runtime`          | Dependency cleanup    | No longer compiles default worker bundle containing cp tools; medium risk for tests expecting JS copies | Update tests/mocks, ensure backward-compat pathways remain |
| Docs (`docs/`, templates) | Documentation update  | Must reflect new tool IDs and usage examples                                                            | Rewrite docs, regenerate site                              |
| Frontend/CLI bundling     | Build config          | Remove `tools/*` workspace; adjust turbo/bun pipelines                                                  | Clean workspace config, update scripts                     |
| Security posture          | Command execution     | Stronger enforcement reduces attack surface                                                             | Document policies, provide override knobs                  |

## Testing Approach

### Unit Tests

- Implement tool-specific tests under `engine/tool/builtin/...` covering validation, success paths, and error codes.
- Mock filesystem and HTTP interfaces using in-memory FS or temp dirs; ensure concurrency-safe operations.
- Add registry tests verifying cp tools appear in `ToolRegistry.ListAll` and respect override precedence.

### Integration Tests

- Extend existing MCP proxy or workflow integration suites to call cp tools end-to-end without Bun runtime.
- Add regression tests ensuring runtime execution of user-defined JS tools still functions when cp tools are absent from `tools.ts`.

### Performance Tests

- Benchmark exec/list operations comparing new in-process tools vs legacy Bun path to validate latency gains.

## Development Sequencing

1. **Scaffold builtin package**: Create `engine/tool/builtin` with definitions and registration helper; add placeholder unit tests.
2. **Implement filesystem + grep tools**: Port logic, create shared validation utilities, ensure schemas align.
3. **Implement exec + fetch**: Add security validations, context-aware networking, configuration integration.
4. **Integrate with LLM service**: Register builtins during startup, update tests.
5. **Deprecate JS workspace**: Introduce feature flag to disable Bun tools by default, keep JS workspace available for rollback, and update docs/examples to champion `cp__` usage.
6. **Regression & benchmarks**: Run full suite, compare performance, update docs.
7. **Final cleanup**: After feature-flagged rollout stabilizes, remove `tools/*`, delete Bun workspace wiring, and retire the temporary flag.

## Technical Dependencies

- Requires Go 1.22+ (Unix builds rely on `golang.org/x/sys/execabs`; Windows variant uses standard library `exec.Cmd` with additional path validation).
- `golang.org/x/sys/execabs` dependency for secure command execution on Unix.
- Coordination with documentation pipeline to refresh references post-removal.

## Monitoring & Observability

- Emit debug logs on tool registration and execution start/finish with execution IDs.
- Add counters (`cp_tool_success_total`, `cp_tool_error_total`) via existing telemetry hooks.
- For exec/fetch tools, log sanitized command/method identifiers, exit codes, and truncated stderr (first 1KiB) alongside latency metrics.

## Technical Considerations

### Key Decisions

- **Go-native execution** for default tools to eliminate Bun dependency.
- **execabs + allowlist** ensures commands resolve to absolute paths and align with industry guidance.
- **Schema-first contracts** to keep LLM function calling reliable and consistent across tools.

### Known Risks

- Potential breaking changes for workflows referencing old tool IDs; mitigated by documentation and migration guide.
- Exec allowlist might block legitimate project-specific commands; provide configuration-based extension.
  - Config updates append entries to the standard list rather than replacing defaults to avoid privilege regression.
- Removing JS packages reduces flexibility for hotfixes via npm; offset by Go release cadence.

### Special Requirements

- Preserve backwards compatibility for custom tools; do not alter runtime configuration semantics beyond cp defaults.
- Ensure context propagation rules (no `context.Background()` in runtime paths) remain intact.

### Standards Compliance

- Adheres to `.cursor/rules/architecture.mdc` by isolating infrastructure concerns in `engine/tool` domain (Use Cases in `engine/tool/uc`, integrations in `engine/llm`).
- Conforms to Go coding standards (context-first, logger/config from context) and test standards (table-driven, `t.Run("Should...")`).
- Implements command security recommendations highlighted by industry tooling.

## Rollout & Verification

- Feature flag optional: allow disabling cp builtins for staged rollout.
- Require `make lint` and `make test` completion before merge.
- Capture before/after latency metrics for representative tool invocations.

### Validation Checklist

1. Run `make lint` and `make test` locally; gate pull requests on both targets.
2. Execute new unit tests under `engine/tool/builtin/...` directly via `go test ./engine/tool/builtin/...`.
3. Run integration workflow covering cp tools plus a custom JS tool to verify mixed execution paths (`go test ./test/integration/... -run ToolExecution`).
4. Benchmark representative filesystem and exec operations comparing Go-native vs Bun-backed path; document results in PR notes.
5. Smoke-test CLI scaffolding commands to confirm generated projects reference `cp__` identifiers.
6. Validate documentation site build (`make docs-build`) after removing `tools/*` workspace assets.
