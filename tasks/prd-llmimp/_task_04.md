## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 4.0: Error handling and memory feedback loops

## Overview

Improve remediation hints, surface suggestions for not-found tools, and persist episodic failure context to enable self-corrective retries.

<critical>
- **MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc
- **MUST READ THE SOURCES** before start the task
</critical>

<requirements>
- Map common orchestrate errors to deterministic remediation hints
- When a tool is not found, suggest nearest by name (using registry list)
- Persist failed plan + error fragment to memory for reflexion loops
</requirements>

## Subtasks

- [ ] Extend remediation mapping table in `response_handler.go`
- [ ] Add optional persistence of failure episodes
- [ ] Suggest nearest tool names on registry miss
- [ ] Unit tests for hints, persistence hooks, and suggestions

## Implementation Details

Hook into existing response handler and request builder flows; keep storage behind existing abstractions.

### Sources

- Toolformer: Schick et al., 2023 — `https://arxiv.org/abs/2302.04761`
- Reflexion: Shinn et al., 2023 — `https://arxiv.org/abs/2303.11366`

### Relevant Files

- `engine/llm/orchestrator/response_handler.go`
- `engine/llm/orchestrator/request_builder.go`

### Dependent Files

- `engine/llm/orchestrator/response_handler_test.go`
- `engine/llm/orchestrator/tool_executor_test.go`

## Success Criteria

- Errors yield deterministic remediation; not-found suggests alternatives
- Episodic failure context stored and retrievable for next attempt
