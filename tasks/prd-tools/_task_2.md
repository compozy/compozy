## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/runtime</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 2.0: Inline TypeScript Execution Pipeline

## Overview

Materialize `tool.Config.Code` into the runtime by generating inline tool modules, composing an entrypoint, and wiring the Bun manager to consume the generated output automatically.

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
- Build inline code manager that syncs `tool.Config.Code` to `.compozy/runtime/inline/` using atomic writes
- Generate composite entrypoint merging user-provided exports with inline modules
- Subscribe to tool resource changes (store watcher) to trigger regeneration
- Update runtime configuration to point Bun manager at generated entrypoint when inline tools exist
- Preserve security constraints (permissions, path normalization) defined in tech spec §5.2
</requirements>

## Subtasks

- [x] 2.1 Implement inline manager (file emission, entrypoint template, watcher integration)
- [x] 2.2 Update engine bootstrap + runtime wiring to activate inline manager and adjust Bun manager behaviour
- [x] 2.3 Add automated tests validating code generation, regeneration on updates, and Bun execution success

## Implementation Details (**FOR LLM READING THIS: KEEP THIS BRIEFLY AND HIGH-LEVEL, THE IMPLEMENTATION ALREADY EXIST IN THE TECHSPEC**)

Follow tech spec §5.2 (Inline TypeScript Code Integration) and §5.3 (Combined Execution Flow). Ensure generated entrypoint imports user entrypoint when configured and overlays inline exports without mutating existing runtime behaviour.

### Relevant Files

- `engine/tool/inline/manager.go` (new package)
- `engine/runtime/bun_manager.go`
- `engine/runtime/bun/worker.tpl.ts`
- `sdk/compozy/engine.go`

### Dependent Files

- `engine/resources/store.go`
- `engine/tool/router/*`
- `sdk/examples/05_runtime_native_tools.go`

## Deliverables

- Inline manager module with documented API and internal tests
- Updated Bun runtime configuration writing composite entrypoint automatically
- Regenerated or updated examples demonstrating inline execution without manual entrypoint
- Passing unit/integration tests including runtime execution of inline tools

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [x] Inline manager emits deterministic files and handles concurrent updates
  - [x] Resource watcher triggers regeneration on PUT/DELETE
  - [x] Bun runtime executes generated entrypoint successfully

## Success Criteria

- Inline TypeScript tools execute end-to-end using generated entrypoint
- Regeneration occurs automatically when tool configs change
- `make lint` and scoped integration tests covering runtime path pass without flakiness
