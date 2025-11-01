## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/tool</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 1.0: Go-Native Tooling Infrastructure

## Overview

Introduce first-class Go-native tool support across the SDK and engine so handlers implemented in Go can register, resolve, and execute alongside existing runtime-based tools.

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
- Add `Implementation` (or equivalent) field to `engine/tool.Config` with validation and serialization
- Provide SDK ergonomics for registering Go handlers (`tool.WithNativeHandler`, runtime acceptance of `"go"`)
- Build a native handler registry that maps tool IDs to in-process handlers with concurrency safety
- Update LLM service/registry to execute native handlers with schema validation, panic recovery, and context-first logging
- Preserve existing cp__ builtin registration and resource-store persistence semantics
</requirements>

## Subtasks

- [x] 1.1 Extend tool configuration model and SDK constructors for native mode
- [x] 1.2 Implement native handler registry and integrate with LLM tool registry adapters
- [x] 1.3 Add regression coverage (unit + integration) ensuring native tools execute and surface errors

## Implementation Details (**FOR LLM READING THIS: KEEP THIS BRIEFLY AND HIGH-LEVEL, THE IMPLEMENTATION ALREADY EXIST IN THE TECHSPEC**)

Focus on tech spec §5.1 (Go-Native Tool Execution Path) and §6 (Data Model & API Adjustments). Ensure adapters in `engine/llm/service.go` branch on implementation type and wrap handler invocation with input/output schema checks.

### Relevant Files

- `engine/tool/config.go`
- `engine/tool/nativeuser/registry.go` (new)
- `sdk/tool/constructor.go`
- `engine/llm/service.go`
- `engine/llm/tool_registry.go`

### Dependent Files

- `engine/tool/router/*`
- `sdk/compozy/engine_registration.go`

## Deliverables

- Updated tool config schema and generated Go SDK options for native handlers
- Native handler registry package with documentation and tests
- Modified LLM registration pipeline executing Go handlers with telemetry
- Passing unit/integration tests covering native execution path

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [x] Native registry concurrency + registration failures
  - [x] Native adapter input/output validation and panic recovery
  - [x] SDK constructor enforcing native handler requirements

## Success Criteria

- Go-native tools execute via engine without relying on Bun runtime
- All new and affected tests (`go test ./...` scope) pass alongside `make lint`
- Tool resolver returns consistent configs for both native and runtime tools
