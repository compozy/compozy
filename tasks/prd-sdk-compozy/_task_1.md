
## markdown

## status: completed

<task_context>
<domain>sdk/compozy</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 1.0: Establish SDK foundation and constructor

## Overview

Create the base `sdk/compozy` package structure, shared types, and functional options so downstream engine work has consistent primitives.

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
- Implement request/response structs and mode enums exactly as defined in §3.3–3.4 of the tech spec.
- Provide a `New(ctx, ...Option)` constructor with option application pipeline per §3.2 and §4.1.
- Ensure config struct stores resource builders for workflows, agents, tools, etc., matching generator expectations in §6.1.
- Wire baseline validation for required inputs (non-nil ctx, at least one resource) while deferring full mode checks to later tasks.
- Cover constructor and option behaviors with unit tests using table-driven style.
</requirements>

## Subtasks

- [x] 1.1 Scaffold `sdk/compozy` package files (`types.go`, `options.go`, `constructor.go`, `doc.go`) per layout in §5.1.
- [x] 1.2 Define execution request/response structs, mode constants, and core config struct with context storage (§3.3–§3.4).
- [x] 1.3 Implement functional options for registering resources and basic engine configuration, ensuring slices initialized safely (§4.1, §6.3).
- [x] 1.4 Add constructor logic with input validation and baseline tests covering happy/error paths.

## Implementation Details (**FOR LLM READING THIS: KEEP THIS BRIEFLY AND HIGH-LEVEL, THE IMPLEMENTATION ALREADY EXIST IN THE TECHSPEC**)

Follow §3 for API signatures, §4.1 for option usage, and §5.1 for file placement. Config struct should retain resource slices aligning with generator builders described in §6.1.

### Relevant Files

- `sdk/compozy/types.go`
- `sdk/compozy/options.go`
- `sdk/compozy/constructor.go`
- `sdk/compozy/doc.go`

### Dependent Files

- `sdk/workflow`
- `sdk/agent`
- `sdk/tool`

## Deliverables

- New package scaffold with constructor, options, shared types, and doc comments ready for GoDoc.
- Unit tests validating option application and constructor error handling.
- Initial GoDoc coverage describing package purpose.

## Tests

- Unit tests mapped from `_tests.md` for this feature:
- [x] `sdk/compozy/app_test.go`
- [x] `sdk/compozy/options_test.go`

## Success Criteria

- `go test ./sdk/compozy -run "(App|Options)"` passes with race detector.
- Functional options register resources without nil panics and constructor enforces required parameters.
- Lint and formatting checks succeed for new files.
