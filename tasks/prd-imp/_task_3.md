---
status: pending
parallelizable: true
blocked_by: ["1.0", "2.0"]
---

<task_context>
<domain>engine/{workflow,agent,tool,task}/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>"5.0","8.0"</unblocks>
</task_context>

# Task 3.0: Resource routers — workflows, agents, tools, tasks

## Overview

Add POST `/export` and `/import` to the top‑level routes for workflows, agents, tools, and tasks. Handlers call the new per‑type importer/exporter functions.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Handlers must:
  - Get store/project via router helpers; resolve project root dir from state CWD
  - Export: call `exporter.ExportTypeToDir(..., ResourceX)`
  - Import: parse `strategy` query (default `seed_only`), call `importer.ImportTypeFromDir(..., ResourceX)`
  - Return scoped counts and message strings identical to admin endpoints
- Add Swagger annotations for both endpoints
- No admin-only gating; follow global auth settings
</requirements>

## Subtasks

- [ ] 3.1 Workflows import/export handlers + register
- [ ] 3.2 Agents import/export handlers + register
- [ ] 3.3 Tools import/export handlers + register
- [ ] 3.4 Tasks import/export handlers + register

## Success Criteria

- Endpoints reachable and return expected payloads
- Swagger for these resources includes the new endpoints
