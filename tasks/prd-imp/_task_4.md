---
status: pending
parallelizable: true
blocked_by: ["1.0", "2.0"]
---

<task_context>
<domain>engine/{mcp,schema,model,memoryconfig,project}/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>"5.0","8.0"</unblocks>
</task_context>

# Task 4.0: Resource routers — mcps, schemas, models, memories, project

## Overview

Add POST `/export` and `/import` (scope permitting) for remaining resources. Project import/export acts on the single project document.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Apply the same handler pattern as in Task 3.0 (no admin guard)
- Ensure exporter now supports memories and project; if not, complete Task 1.0 first
- Swagger annotations added
</requirements>

## Subtasks

- [ ] 4.1 MCPs import/export handlers + register
- [ ] 4.2 Schemas import/export handlers + register
- [ ] 4.3 Models import/export handlers + register
- [ ] 4.4 Memories import/export handlers + register
- [ ] 4.5 Project import/export handlers + register

## Success Criteria

- Endpoints reachable and return expected payloads
- Swagger updated accordingly
