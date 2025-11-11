## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>sdk/tool</domain>
<type>testing</type>
<scope>configuration</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 3.0: Validation, Testing, and Documentation Hardening

## Overview

Consolidate end-to-end coverage, ensure validation rules are enforced across APIs, and update documentation/examples to reflect Go-native and inline tooling capabilities.

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
- Add REST-layer validation preventing native implementation payloads without registered handlers
- Expand SDK + runtime documentation to describe new tooling modes and usage patterns
- Update examples to demonstrate hybrid (Go + inline TS) workflow and ensure sample compiles
- Provide regression tests covering API contracts, end-to-end workflow execution, and lint/test automation
</requirements>

## Subtasks

- [ ] 3.1 Harden validation paths (tool router/usecases, SDK guardrails, telemetry)
- [ ] 3.2 Update docs/examples (`sdk/tool/README.md`, `sdk/examples/05_runtime_native_tools.go`, site docs)
- [ ] 3.3 Add end-to-end regression suite exercising hybrid tooling and CI commands

## Implementation Details (**FOR LLM READING THIS: KEEP THIS BRIEFLY AND HIGH-LEVEL, THE IMPLEMENTATION ALREADY EXIST IN THE TECHSPEC**)

Reference tech spec §6 (Data Model & API Adjustments), §7.5 (Validation & Tooling), and §8 (Testing & Validation Strategy). Focus on ensuring user-facing surfaces communicate capabilities and automated tests guard regressions.

### Relevant Files

- `engine/tool/router/*`
- `sdk/tool/README.md`
- `docs/content/docs/core/tools/runtime-environment.mdx`
- `sdk/examples/05_runtime_native_tools.go`
- `Makefile`/CI scripts if adjustments are required

### Dependent Files

- `engine/tool/uc/*`
- `pkg/config/native_tools.go`

## Deliverables

- Validation logic and tests preventing inconsistent tool definitions through REST/SDK
- Updated documentation and examples reflecting new tooling workflows
- Expanded automated test suite (unit/integration/end-to-end) covering hybrid scenarios

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [ ] Router/usecase validation rejecting invalid implementation states
  - [ ] Example workflow execution test combining Go and inline tools
  - [ ] Documentation snippet tests or doctests (if applicable)

## Success Criteria

- Users can follow docs to create Go-native and inline tools without ambiguity
- Regression suite ensures mixed tooling workflows remain functional
- `make lint` and `make test` pass with new coverage included in CI
