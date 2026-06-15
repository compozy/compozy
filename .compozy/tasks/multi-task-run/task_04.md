---
status: completed
title: Add Multi-Run Event and Snapshot Worktree Metadata
type: backend
complexity: high
dependencies:
  - task_02
---

# Task 4: Add Multi-Run Event and Snapshot Worktree Metadata

## Overview

This task makes parent multi-run events durable enough to reconstruct worktree-aware snapshots. It extends `task.multi.*` payloads and snapshot rebuilding while remaining compatible with older parent events that lack worktree metadata.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- `TaskRunMultiplePayload` MUST include optional `parallel_limit`, `worktree_path`, `base_branch`, `base_commit`, and `worktree_status` fields.
- Snapshot reconstruction MUST copy worktree metadata from parent events into `TaskRunMultipleItem`.
- Snapshot reconstruction MUST handle old events with missing metadata.
- Worktree metadata MUST be representable before child launch.
- Existing event kind names MUST remain unchanged.
- Payload compatibility tests MUST cover both old and new JSON shapes.
</requirements>

## Subtasks

- [x] 4.1 Add worktree and parallel limit fields to the multi-run event payload.
- [x] 4.2 Update snapshot reconstruction to copy optional metadata fields.
- [x] 4.3 Add helper behavior for metadata-only item updates before child start.
- [x] 4.4 Preserve old event compatibility.
- [x] 4.5 Add payload compatibility and daemon snapshot tests.

## Implementation Details

Use the TechSpec "Events And Snapshot Reconstruction" section. This task adds persistence and snapshot behavior only; worktree allocation and scheduler production of the metadata happen later.

### Relevant Files

- `pkg/compozy/events/kinds/task.go` — defines `TaskRunMultiplePayload`.
- `pkg/compozy/events/kinds/payload_compat_test.go` — protects public event JSON compatibility.
- `internal/daemon/task_multi.go` — contains the snapshot builder and event decode helpers.
- `internal/daemon/task_multi_test.go` — parent event and snapshot reconstruction tests.
- `pkg/compozy/events/event.go` — declares current multi-run event kinds, which should remain stable.

### Dependent Files

- `internal/core/run/ui/multi_remote.go` — later task renders metadata from snapshots/events.
- `internal/cli/run_observe.go` — later task streams metadata from parent events.
- `docs/events.md` — later task documents the additive payload fields.

### Related ADRs

- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md) — Requires durable child worktree metadata.
- [ADR-007: Use One Task-Multi Scheduler with Worktree-Owned Child Runs](adrs/adr-007.md) — Chooses parent events as the metadata persistence surface.

## Deliverables

- Additive multi-run event payload fields.
- Snapshot reconstruction for optional worktree metadata.
- Backward compatibility for existing parent event streams.
- Unit tests with 80%+ coverage for payload and snapshot metadata behavior **(REQUIRED)**.
- Integration tests for reconstructing worktree-aware snapshots from parent events **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] New `TaskRunMultiplePayload` JSON includes worktree metadata when set.
  - [x] Old `TaskRunMultiplePayload` JSON without metadata still decodes.
  - [x] Snapshot builder applies `worktree_path` before child run id exists.
  - [x] Snapshot builder preserves child run id and error text alongside metadata.
  - [x] Snapshot builder keeps requested item order after metadata events.
- Integration tests:
  - [x] Parent event replay reconstructs path, base branch, base commit, and worktree status for each child.
  - [x] Old enqueued parent event replay still reconstructs slug/status/run id correctly.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Parent events can durably reconstruct worktree-aware multi-run snapshots.
