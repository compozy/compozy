
## markdown

## status: completed

<task_context>
<domain>sdk2/compozy</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 4.0: Deliver standalone and distributed mode orchestration

## Overview

Implement mode-specific orchestration for standalone (embedded Temporal/Redis) and distributed (external services) execution, including validation and health management.

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
- Implement standalone mode bootstrapping (`standalone.go`) starting embedded Temporal and Redis per §2.2.3 and §7.1, respecting configuration defaults and timeouts.
- Implement distributed mode configuration (`distributed.go`) to connect to external services, validating required URLs/secrets and surfacing actionable errors.
- Integrate mode selection logic with engine lifecycle (Task 3) to choose resource stores and clients based on mode (§2.2.3, §7.1).
- Add health checks and cleanup paths ensuring `Stop` gracefully tears down embedded services.
- Provide validations and unit tests ensuring default mode is standalone with override support (§15.3 decision).
</requirements>

## Subtasks

- [x] 4.1 Implement standalone mode struct, configuration, and boot sequence with embedded Temporal/Redis wiring (§2.2.3, §7.1).
- [x] 4.2 Implement distributed mode setup using external service clients and validation logic (§2.2.3).
- [x] 4.3 Connect mode selection to engine lifecycle hooks, including resource store provisioning and client base URL resolution.
- [x] 4.4 Add metrics/logging for mode startup and shutdown using context-based logger/config.
- [x] 4.5 Create unit and integration tests covering both modes.

## Implementation Details (**FOR LLM READING THIS: KEEP THIS BRIEFLY AND HIGH-LEVEL, THE IMPLEMENTATION ALREADY EXIST IN THE TECHSPEC**)

Reference §2.2.3 for mode responsibilities and §7.1 for dependencies. Default mode is standalone (§15.3) with memory store unless Redis configured.

### Relevant Files

- `sdk2/compozy/mode.go`
- `sdk2/compozy/standalone.go`
- `sdk2/compozy/distributed.go`
- `sdk2/compozy/standalone_test.go`
- `sdk2/compozy/distributed_test.go`

### Dependent Files

- `engine/infra/server`
- `engine/infra/cache`
- `engine/worker/embedded`
- `pkg/config`
- `pkg/logger`

## Deliverables

- Fully implemented mode abstractions with validation, startup, and teardown logic.
- Integration with engine lifecycle selecting correct stores and base URLs.
- Tests demonstrating standalone and distributed mode behavior, including failure cases.

## Tests

- Unit tests mapped from `_tests.md` for this feature:
- [x] `sdk2/compozy/mode_test.go`
- [x] `sdk2/compozy/integration/standalone_integration_test.go`
- [x] `sdk2/compozy/integration/distributed_integration_test.go`

## Success Criteria

- Standalone mode boots embedded Temporal/Redis within configured timeouts and cleans up on stop.
- Distributed mode validates required endpoints and connects to external services without leaking goroutines.
- Integration tests pass locally using Docker-based Redis fixture and race detector.
