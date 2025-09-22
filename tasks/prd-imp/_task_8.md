---
status: pending
parallelizable: true
blocked_by: ["1.0", "2.0", "3.0", "4.0", "5.0", "6.0", "7.0"]
---

<task_context>
<domain>tests</domain>
<type>testing</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>[]</unblocks>
</task_context>

# Task 8.0: Tests — importer/exporter + router + CLI

## Overview

Add comprehensive tests per standards to cover new importer/exporter functions, router endpoints (payloads and auth behavior following global settings), and CLI wiring.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Use `t.Run("Should ...")` pattern and `testify`
- Unit tests for importer/exporter per‑type functions
- Router tests for at least workflows and tools (payload shape, project root resolution)
- CLI tests (command wiring, URL composition, strategy flag)
- Keep integration tests, if any, under `test/integration/`
</requirements>

## Subtasks

- [ ] 8.1 Unit: exporter/importer per‑type
- [ ] 8.2 Router: workflows/tools import/export
- [ ] 8.3 CLI: per‑resource commands

## Success Criteria

- `make lint` and `make test` pass with coverage acceptable for business logic packages
