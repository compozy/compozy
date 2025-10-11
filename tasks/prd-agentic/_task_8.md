---
status: completed
parallelizable: true
blocked_by: ["6.0"]
---

<task_context>
<domain>pkg/config</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies></dependencies>
<unblocks>"9.0","10.0","11.0"</unblocks>
</task_context>

# Task 8.0: Config: runtime.native_tools.agent_orchestrator limits

## Overview

Add configuration knobs for `max_depth`, `max_steps`, `max_parallel`, `timeout_ms`, and planner enable flag under native tools.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Extend config structs, defaults, and environment variable bindings.
- Validate caps and ensure builtin reads them via `config.FromContext(ctx)`.
</requirements>

## Subtasks

- [x] 8.1 Config structs and defaults
- [x] 8.2 Validation and docs

## Sequencing

- Blocked by: 6.0
- Unblocks: 9.0, 10.0, 11.0
- Parallelizable: Yes

## Success Criteria

- Limits applied in executor; verified by tests
