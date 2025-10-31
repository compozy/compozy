
## markdown

## status: completed

<task_context>
<domain>sdk/compozy</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 3.0: Build engine core lifecycle and client integration

## Overview

Implement the `Engine` struct, lifecycle management, client wiring, and introspection methods that power workflow/task/agent execution.

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
- Implement `Engine` struct fields covering resource slices, stores, client, mode, and HTTP server references per §2.2 and §3.1.
- Develop lifecycle methods (`Start`, `Stop`, `Wait`) handling embedded server bootstrapping hooks and deferring mode-specific wiring to Task 4.
- Integrate `sdk/client` for all execution methods, reusing generated helpers and mapping request/response types (§7.2).
- Expose introspection methods (`Server`, `Router`, `Config`, `ResourceStore`, `Mode`, `IsStarted`) matching interface in §3.1.
- Ensure context usage pulls config/logger via `config.FromContext` and `logger.FromContext` per project rules.
</requirements>

## Subtasks

- [x] 3.1 Define `Engine` struct and supporting private helpers in `engine.go` (§2.2.1).
- [x] 3.2 Implement lifecycle methods with proper context propagation and error handling (§7.2).
- [x] 3.3 Wire execution helpers to `sdk/client`, covering sync/async/streaming flows via generated code (§4.2, §7.2).
- [x] 3.4 Implement resource store selection logic stub aligning with mode defaults (standalone memory vs distributed redis) (§7.1); full wiring completed in Task 4.
- [x] 3.5 Add unit tests for lifecycle and execution method conversions.

## Implementation Details (**FOR LLM READING THIS: KEEP THIS BRIEFLY AND HIGH-LEVEL, THE IMPLEMENTATION ALREADY EXIST IN THE TECHSPEC**)

Follow §2.2 for component responsibilities and §7.2 for client integration. Lifecycle should prepare base URL and instantiate client as demonstrated in sample code around §7.2.

### Relevant Files

- `sdk/compozy/engine.go`
- `sdk/compozy/lifecycle.go`
- `sdk/compozy/engine_execution.go`
- `sdk/compozy/constants.go`
- `sdk/compozy/errors.go`

### Dependent Files

- `sdk/client`
- `engine/resources`
- `pkg/config`
- `pkg/logger`

## Deliverables

- Engine core implementation with lifecycle, execution delegation, and introspection APIs.
- Resource store selection hooks ready for mode-specific overrides.
- Unit tests verifying lifecycle transitions and client request/response translation.

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [ ] `sdk/compozy/engine_test.go`
  - [ ] `sdk/compozy/lifecycle_test.go`
  - [ ] `sdk/compozy/execution/client_test.go`

## Success Criteria

- Engine starts and stops cleanly in unit tests using in-memory dependencies.
- Execution helpers produce correct client calls for workflow/task/agent operations.
- Tests pass with race detector and integration with generated code verified.
