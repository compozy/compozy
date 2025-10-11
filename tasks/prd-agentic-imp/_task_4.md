## markdown

## status: [DEFERRED]

<task_context>
<domain>engine/llm/tools</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 4.0: Tool Schema, Cooldown & Ergonomics Upgrade

## Overview

Strengthen tool interoperability by adopting JSON Schema argument definitions, enhancing guidance/examples, and adding cooldown logic to mitigate repeated failures.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Extend tool definitions to include JSON Schema argument metadata and exemplar payloads.
- Enforce schema validation before execution and during structured-output planning.
- Implement cooldown tracking for tools with high failure rates, influencing loop budgeting.
- Update telemetry and logging to record schema validation outcomes and cooldown activations.
</requirements>

## Subtasks

- [ ] 4.1 Augment `ToolDefinition` with schema/cooldown fields and migrate existing tool registrations.
- [ ] 4.2 Integrate argument validation prior to execution and feed errors back into loop guidance.
- [ ] 4.3 Implement cooldown manager updating loop state budgets and tool availability.
- [ ] 4.4 Update tool execution tests covering schema validation, cooldown triggering, and telemetry side effects.
- [ ] 4.5 Provide developer docs/examples for authoring schemas and cooldown policies.

## Implementation Details

- PRD “Tool ergonomics could be stronger” calls for schema examples and cooldown penalty systems.
- Ensure schema storage uses `json.RawMessage` to avoid marshaling overhead.
- Cooldown logic should align with progress engine signals introduced in Task 3.0.

### Relevant Files

- `engine/llm/tool_registry.go`
- `engine/llm/tool.go`
- `engine/llm/orchestrator/tool_executor.go`
- `engine/llm/orchestrator/loop.go`

### Dependent Files

- `engine/llm/orchestrator/state_machine.go`
- `engine/llm/orchestrator/request_builder.go`

## Deliverables

- Schema-aware tool registry with cooldown support and validation pipeline.
- Updated loop/tool executor integrating schema errors into guidance feedback.
- Documentation/examples illustrating schema authoring and cooldown configuration.

## Tests

- Tests mapped from PRD test strategy:
  - [ ] Tool cooldown: repeated invalid arg calls lead to penalty and earlier loop exit.
  - [ ] Schema validation failure surfaces actionable guidance without panicking.
  - [ ] Successful tool execution emits telemetry with schema metadata.

## Success Criteria

- Tool misuse surfaced before hitting LLM loop, reducing retries.
- Cooldown state observable in loop context and telemetry.
- All tool-related tests pass alongside existing suites.
- `make fmt && make lint && make test` pass.
