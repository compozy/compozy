---
status: completed
parallelizable: true
blocked_by: ["6.0"]
---

<task_context>
<domain>engine/tool/native</domain>
<type>integration</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies></dependencies>
<unblocks>"8.0","11.0"</unblocks>
</task_context>

# Task 7.0: Register builtin in native catalog and service wiring

## Overview

Expose the new builtin through `engine/tool/native/catalog.go` and ensure `llm/service` registers it.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Add definition to catalog and confirm it appears in registry.
- Verify conflicts with `cp__` reserved prefix rules.
</requirements>

## Subtasks

- [x] 7.1 Catalog registration
- [x] 7.2 Service wiring verification

## Sequencing

- Blocked by: 6.0
- Unblocks: 8.0, 11.0
- Parallelizable: Yes

## Success Criteria

- Tool present in registry list; basic call passes
- Verified `cp__agent_orchestrate` via `native.Definitions(env)` in handler tests and ensured `llm.Service` registers the builtin using the injected tool environment.
