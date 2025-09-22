---
status: pending
parallelizable: false
blocked_by: []
---

<task_context>
<domain>engine/resources</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>"2.0","3.0","4.0"</unblocks>
</task_context>

# Task 1.0: Importer/Exporter symmetry + type‑specific functions

## Overview

Add per‑type entry points for import/export and ensure symmetry across all resource types (including tasks, memories, project). This enables collection‑level `/import` and `/export` handlers to act only on their resource.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add `ExportTypeToDir(ctx, project, store, root, typ)` in `engine/resources/exporter/exporter.go`
- Extend `DirForType` for `ResourceTask`, `ResourceMemory`, `ResourceProject`
- Update the existing `ExportToDir` to delegate to `ExportTypeToDir` in a loop
- Add `ImportTypeFromDir(ctx, project, store, root, strategy, updatedBy, typ)` in `engine/resources/importer/importer.go`
- Add `tasks` mapping in importer dir resolution and `taskUpsert` using `taskuc.NewUpsert`
- Ensure context/logger usage follows standards (inherit `ctx`, use `logger.FromContext`)
</requirements>

## Subtasks

- [ ] 1.1 Add `ExportTypeToDir` and extend `DirForType`
- [ ] 1.2 Update `ExportToDir` to call the new per‑type function
- [ ] 1.3 Add importer dir mapping for `tasks/` → `ResourceTask`
- [ ] 1.4 Implement `taskUpsert` and register in `standardUpsertHandlers`
- [ ] 1.5 Add `ImportTypeFromDir` mirroring `applyForType`
- [ ] 1.6 Unit tests: exporter/importer new APIs (deterministic counts, error cases)

## Sequencing

- Blocked by: None
- Unblocks: 2.0 (guard), 3.0/4.0 (routers)
- Parallelizable: Limited; do this first to unblock handlers

## Implementation Details

### Relevant Files

- `engine/resources/exporter/exporter.go`
- `engine/resources/importer/importer.go`
- `engine/task/uc/*` (for upsert)

### Dependent Files

- `engine/infra/server/reg_admin_export.go` (to be removed later)
- `engine/infra/server/reg_admin_import.go` (to be removed later)

## Success Criteria

- `ExportTypeToDir` and `ImportTypeFromDir` compile and are covered by tests
- Importer/exporter support `tasks`, `memories`, and `project` symmetrically
- `make lint` and `make test` pass
