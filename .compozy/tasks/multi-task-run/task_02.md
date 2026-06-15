---
status: completed
title: Extend Multi-Run API and Client Contracts
type: backend
complexity: high
dependencies:
  - task_01
---

# Task 2: Extend Multi-Run API and Client Contracts

## Overview

This task extends the daemon API contract so callers can pass the resolved parallel limit and receive worktree metadata in snapshots. It keeps the existing multi-run routes stable while updating Go and generated TypeScript contract surfaces.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- `TaskRunMultipleRequest` MUST carry the resolved parallel limit.
- `TaskRunMultipleItem` MUST expose optional worktree path, base branch, base commit, and worktree status fields.
- Existing `POST /api/task-runs/multiple` and snapshot routes MUST remain unchanged.
- API client start and snapshot methods MUST preserve the new fields.
- OpenAPI and generated TypeScript types MUST include the new fields.
- Existing clients that omit the new fields MUST remain compatible.
</requirements>

## Subtasks

- [x] 2.1 Add parallel limit and worktree metadata fields to contract/core types.
- [x] 2.2 Update daemon client request and snapshot mapping.
- [x] 2.3 Update API handler/service forwarding for the new request field.
- [x] 2.4 Regenerate OpenAPI and TypeScript schema artifacts.
- [x] 2.5 Add request, response, client, and OpenAPI contract tests.

## Implementation Details

Use the TechSpec "API And Data Model" section for field names and compatibility expectations. This task should only update transport and schema surfaces; event persistence and scheduler behavior are handled by later tasks.

### Relevant Files

- `internal/api/contract/types.go` — defines request and snapshot item contract types.
- `internal/api/core` — carries API-level request and snapshot structures.
- `internal/api/client/client.go` — maps start requests into HTTP payloads.
- `internal/api/client/runs.go` — decodes multi-run snapshots.
- `internal/api/core/handlers.go` — forwards request data into daemon services.
- `openapi/compozy-daemon.json` — daemon OpenAPI schema.
- `web/src/generated/compozy-openapi.d.ts` — generated TypeScript schema.

### Dependent Files

- `internal/cli/daemon_commands.go` — later task passes the resolved CLI limit through this request.
- `pkg/compozy/events/kinds/task.go` — later task adds matching event payload metadata.
- `internal/core/run/ui/multi_remote.go` — later task renders snapshot item metadata.

### Related ADRs

- [ADR-007: Use One Task-Multi Scheduler with Worktree-Owned Child Runs](adrs/adr-007.md) — Requires snapshot items to expose worktree-owned child metadata.
- [ADR-008: Make the Parallel Multi-Run Limit Configurable](adrs/adr-008.md) — Requires daemon requests to carry the resolved limit.

## Deliverables

- Additive API/core/client fields for `parallel_limit` and worktree metadata.
- Regenerated OpenAPI and TypeScript schema output.
- Backward-compatible contract tests for old and new payload shapes.
- Unit tests with 80%+ coverage for changed mapping code **(REQUIRED)**.
- Integration tests for daemon client request and snapshot round trips **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] Encoding `TaskRunMultipleRequest` includes `parallel_limit` when provided.
  - [x] Decoding a snapshot item with worktree metadata preserves all fields.
  - [x] Decoding a snapshot item without worktree metadata still succeeds.
  - [x] API contract conversion preserves ordered child items.
- Integration tests:
  - [x] `StartTaskRunMultiple` posts slugs, mode, presentation mode, runtime overrides, and parallel limit.
  - [x] `GetTaskRunMultipleSnapshot` decodes worktree metadata from a daemon response.
  - [x] OpenAPI contract tests assert the new fields are present.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- The daemon API can carry the resolved parallel limit and return worktree metadata without route changes.
