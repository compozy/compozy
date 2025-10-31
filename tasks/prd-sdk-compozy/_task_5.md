
## markdown

## status: completed

<task_context>
<domain>sdk2/compozy</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>medium</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 5.0: Implement resource loading and validation layer

## Overview

Build YAML loading helpers, programmatic registration glue, and dependency graph validation to ensure resources are consistent regardless of origin.

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
- Implement YAML loaders (`loader.go`) using shared generics to decode files and directories per §4.3 and §6.3.
- Connect loaders to generated registration methods ensuring deduplication and error wrapping with file paths (§2.2.4, §6.3).
- Build validation engine computing dependency graph, detecting circular/missing references, and returning `ValidationReport` (§3.4, §2.2.5).
- Provide APIs to load individual files, directories, and validate entire project per interface in §3.1.
- Ensure loaders respect configuration from context and log using `logger.FromContext` for errors.
</requirements>

## Subtasks

- [x] 5.1 Implement generic YAML decode helper and directory walker handling `.yaml`/`.yml` extensions (§6.3).
- [x] 5.2 Wire `Load*` and `Register*` methods to resource store interactions with duplicate detection (§2.2.4).
- [x] 5.3 Implement validation routines building dependency graph and producing `ValidationReport` (§3.4, §2.2.5).
- [x] 5.4 Add unit tests for loaders and validation covering success and failure cases.
- [x] 5.5 Add hybrid YAML integration test covering mixed loading strategy.

## Implementation Details (**FOR LLM READING THIS: KEEP THIS BRIEFLY AND HIGH-LEVEL, THE IMPLEMENTATION ALREADY EXIST IN THE TECHSPEC**)

Reference §4.3 for dynamic loading flows, §2.2.4–§2.2.5 for resource manager responsibilities, and `ValidationReport` structure in §3.4.

### Relevant Files

- `sdk2/compozy/loader.go`
- `sdk2/compozy/engine_loading.go`
- `sdk2/compozy/engine_registration.go`
- `sdk2/compozy/validation.go`
- `sdk2/compozy/validation_test.go`

### Dependent Files

- `engine/resources`
- `sdk2/resource` packages for schema definitions
- `pkg/template`

## Deliverables

- Complete loading and registration layer with reusable helpers and informative errors.
- Validation engine producing actionable reports for CLI or programmatic consumption.
- Unit and integration tests demonstrating loader and validator correctness.

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [x] `sdk2/compozy/resources/graph_test.go`
  - [x] `sdk2/compozy/config/yaml_loader_test.go`
  - [x] `sdk2/compozy/integration/hybrid_yaml_integration_test.go`

## Success Criteria

- Loaders import YAML directories with deduplication and fail fast on schema errors.
- Validation detects cycles/missing references and returns structured report matching spec.
- Integration test exercises hybrid loading flow and passes with race detector.
