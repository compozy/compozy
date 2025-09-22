---
status: pending
parallelizable: true
blocked_by: ["3.0", "4.0", "5.0"]
---

<task_context>
<domain>cli/cmd</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
<unblocks>"7.0","8.0"</unblocks>
</task_context>

# Task 6.0: CLI per‑resource import/export; remove admin

## Overview

Replace `cli/cmd/admin` with per‑resource `import` and `export` commands that call the new endpoints. Preserve `--strategy` for import.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add commands: `compozy workflows import|export`, `compozy agents ...`, etc.
- Implement HTTP calls mirroring `admin` commands’ executor flow
- Remove `cli/cmd/admin` and references
- Suggested file paths:
  - `cli/cmd/workflows/import.go`, `cli/cmd/workflows/export.go`
  - `cli/cmd/agents/import.go`, `cli/cmd/agents/export.go`
  - `cli/cmd/tasks/import.go`, `cli/cmd/tasks/export.go`
  - `cli/cmd/tools/import.go`, `cli/cmd/tools/export.go`
  - `cli/cmd/mcps/import.go`, `cli/cmd/mcps/export.go`
  - `cli/cmd/schemas/import.go`, `cli/cmd/schemas/export.go`
  - `cli/cmd/models/import.go`, `cli/cmd/models/export.go`
  - `cli/cmd/memories/import.go`, `cli/cmd/memories/export.go`
  - `cli/cmd/project/import.go`, `cli/cmd/project/export.go`
</requirements>

## Subtasks

- [ ] 6.1 Implement workflows/agents/tools/tasks commands
- [ ] 6.2 Implement mcps/schemas/models/memories/project commands
- [ ] 6.3 Remove `cli/cmd/admin` and update help/docs

## Success Criteria

- CLI commands function and print results consistently with existing admin commands
