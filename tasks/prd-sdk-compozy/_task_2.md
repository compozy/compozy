
## markdown

## status: completed

<task_context>
<domain>sdk/internal/sdkcodegen</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 2.0: Implement code generation pipeline for SDK resources

## Overview

Extend the sdk code generator so `go generate` produces options, execution, loading, and registration helpers aligned with the new resource specs.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technicals docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Define `ResourceSpec` entries for all supported resources matching §6.1.
- Update generators to emit functional options, execution, loading, and registration methods per §6.3.
- Wire `go:generate` directives in `sdk/compozy/generate.go` and ensure outputs land in expected files from §5.1.
- Keep generated files deterministic and formatted; document regeneration steps in README.
- Validate generated code compiles against constructor config set by Task 1.
</requirements>

## Subtasks

- [x] 2.1 Extend `internal/sdkcodegen/spec.go` with full resource list and metadata (§6.1).
- [x] 2.2 Implement/refresh generators for options, execution, loading, and registration files using `github.com/dave/jennifer/jen` (§6.3).
- [x] 2.3 Add/verify `go:generate` directives and run generator to produce `options_generated.go`, `engine_execution.go`, `engine_loading.go`, `engine_registration.go` (§6.2).
- [x] 2.4 Create/update README documenting generator usage and pipeline.
- [x] 2.5 Write unit tests covering generator output hashes and regression cases.

## Implementation Details (**FOR LLM READING THIS: KEEP THIS BRIEFLY AND HIGH-LEVEL, THE IMPLEMENTATION ALREADY EXIST IN THE TECHSPEC**)

Use §6 as the authoritative reference for generator responsibilities; ensure emitted functions match API signatures from §3 and option semantics from §4.1.

### Relevant Files

- `sdk/compozy/generate.go`
- `sdk/compozy/options_generated.go`
- `sdk/compozy/engine_execution.go`
- `sdk/compozy/engine_loading.go`
- `sdk/compozy/engine_registration.go`
- `sdk/internal/sdkcodegen/spec.go`
- `sdk/internal/sdkcodegen/*_generator.go`

### Dependent Files

- `sdk/compozy/options.go`
- `sdk/compozy/constructor.go`
- `sdk/client`

## Deliverables

- Updated generator specs and code producing all required helper files.
- Generated Go files checked in and formatted with `gofmt`.
- README or inline docs describing generator invocation and regeneration steps.

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [x] `sdk/compozy/codegen/generator_test.go`

## Success Criteria

- `go generate ./sdk/compozy` completes without diffs when rerun.
- Generated files compile and integrate cleanly with constructor and engine code.
- Generator tests assert expected output structure and fail on drift.
