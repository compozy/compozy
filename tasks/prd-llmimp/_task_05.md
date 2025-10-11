## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/llm</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 5.0: Tool registry discovery and caching improvements

## Overview

Introduce TTL-based caching and allow/deny lists to reduce “tool not found” errors and improve discovery performance.

<critical>
- **MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc
- **MUST READ THE SOURCES** before start the task
</critical>

<requirements>
- TTL cache for `Find` with background refresh
- Allow/deny list support in config
- Adapter passthrough to orchestrator registry kept intact
</requirements>

## Subtasks

- [ ] Implement cache layer with TTL and refresh strategy
- [ ] Add allow/deny list filters and config wiring
- [ ] Unit tests for hit/miss, expiry, and filtering

## Implementation Details

Preserve `ToolRegistry` interface; implement behavior in concrete registry. Ensure thread-safety.

### Sources

- AutoGen proxies and dynamic planners — `https://arxiv.org/abs/2308.08155`
- LangChain tool schemas and discovery — `https://github.com/langchain-ai/langchain`

### Relevant Files

- `engine/llm/tool_registry.go`
- `engine/llm/service.go`

### Dependent Files

- `engine/llm/tool_registry_test.go`

## Success Criteria

- Cache and filtering verified by tests; orchestrator continues to resolve tools correctly
