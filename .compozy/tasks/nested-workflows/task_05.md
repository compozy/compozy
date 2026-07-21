---
status: completed
title: "Planning and Skill Workflow Integration"
type: docs
complexity: high
---

# Task 5: Planning and Skill Workflow Integration

## Overview

Extend bundled planning and review skills so a maintainer can explicitly choose ordinary workflow generation or editable Task Group generation, then use a clean final review to record task group lifecycle completion safely. The skills remain guidance and lifecycle integration only: they must not introduce automatic chaining, Git automation, or copied specification corpora.

<critical>
- ALWAYS READ the PRD, the TechSpec, and their catalogs (`_user_stories.md`, `_tests.md`) before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — implement every test case assigned in ## Tests
</critical>

<requirements>
- MUST offer Task Groups only after readable canonical PRD and TechSpec validation, and MUST retain the ordinary flow when the user declines.
- MUST keep task group proposals editable in session memory and write no marker, task group directory, task, or temporary artifact before confirmation.
- MUST atomically generate `_task_groups.md` and task-group-local task suites with qualified task ownership and exactly-once initiative-wide test assignment.
- MUST make review guidance resolve shared root specifications plus selected task group artifacts and call the hidden completion bridge only after clean review, resolved history, and final verification.
</requirements>

## Subtasks

- [x] 5.1 Document the ordinary-versus-Work-Task Group planning decision and coherent recommendation criteria.
- [x] 5.2 Define editable, cancellable proposal and confirmation behavior for new and existing plans.
- [x] 5.3 Define atomic task-group-plan and task-group-local task-suite generation requirements.
- [x] 5.4 Define initiative-wide qualified ownership and exactly-once test-assignment audit requirements.
- [x] 5.5 Update review-round guidance for root specifications, selected task group scope, and sibling-change warnings.
- [x] 5.6 Define the clean-review completion bridge, idempotency, and separate completion/sync status reporting.
- [x] 5.7 Preserve ordinary workflow, current-branch, and explicit user-invoked lifecycle behavior in both skills.

## Implementation Details

Task 1 supplies the valid manifest contract and Task 3 supplies the runtime completion bridge. Update the installable bundled skills and their references so an execution agent has explicit artifact paths, validation rules, and stop conditions; do not encode these rules only in prose outside the skills.

### Relevant Files

- `skills/cy-create-tasks/SKILL.md` — planning workflow, approval boundary, task generation, and validation instructions.
- `skills/cy-create-tasks/references/task-template.md` — task-file requirements to preserve for each task group suite.
- `skills/cy-create-tasks/references/task-context-schema.md` — task-graph and task metadata compatibility contract.
- `skills/cy-review-round/SKILL.md` — review lifecycle and final-review integration instructions.
- `skills/cy-review-round/references/review-criteria.md` — selected outcome, owned scope, and sibling-change criteria.
- `skills/embed.go` — verifies modified skill and reference assets remain bundled.

### Dependent Files

- `internal/core/taskgroups` — validates generated plan and task group manifests.
- `internal/core/tasks/manifest.go` — accepts each task group’s logical workflow identity.
- `compozy internal task-groups complete` — hidden bridge delivered by Task 3.
- `internal/setup/skills_selected.go` — installs embedded skill assets for users.

### Related ADRs

- [ADR-002: Optional Task Groups](adrs/adr-002.md) — opt-in planning and explicit user-controlled lifecycle.
- [ADR-003: Hidden Child Workflows](adrs/adr-003.md) — generated storage structure and clean-review completion bridge.

## Deliverables

- Updated task-creation and review-round skills with task-group-aware approval, generation, scope, and completion procedures.
- Skill-level verification scenarios for cancellation, stale plans, ownership, and clean-review completion integration.
- Every test case assigned in `## Tests` implemented and passing **(REQUIRED)**.

## Tests

Cases assigned from `_tests.md`, the test contract — read each ID's full definition there before writing tests.

- [x] UT-001, UT-002, UT-004 — delivery-shape recommendation, large proposal rendering, and zero-node ordinary recommendation.
- [x] IT-001, IT-002, IT-003, IT-004, IT-005, IT-006, IT-007, IT-008, IT-009, IT-010, IT-039 — planning preconditions, edit/cancel/stale-write behavior, task group generation, ownership audit, and planning freshness.
- [ ] E2E-001, E2E-002, E2E-003 — ordinary/task-group choice, plan editing, and accessible large-proposal journeys.

## Success Criteria

- Every assigned test case implemented and passing.
- A cancelled proposal leaves no new executable task group artifact; a confirmed proposal is valid, atomic, and test-accountable.
- Review guidance completes only the selected task group through the hidden verified bridge and never automates Git or lifecycle chaining.
