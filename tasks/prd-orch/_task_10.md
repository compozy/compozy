---
status: pending
parallelizable: false
blocked_by: ["2.0", "8.0", "9.0"]
---

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>implementation</type>
<scope>cleanup</scope>
<complexity>medium</complexity>
<dependencies>config, tests, docs</dependencies>
<unblocks></unblocks>
</task_context>

# Task 10.0: Remove feature flag/legacy path and update tests/docs

## Overview

Delete any feature flag gating and the legacy loop code path, keeping behavior intact under the FSM. Update tests to remove flag toggles and finalize documentation (state diagram, developer guide).

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- No feature flags; FSM is the only path
- Remove obsolete code, config, and references
- Ensure all orchestrator tests are green without flags
- Update docs with a clear state diagram and notes
</requirements>

## Subtasks

- [ ] 10.1 Remove feature flag and configuration toggles
- [ ] 10.2 Delete legacy loop code paths
- [ ] 10.3 Update tests to drop flag usage; ensure green
- [ ] 10.4 Add `docs/orchestrator-fsm.md` with Mermaid diagram

## Sequencing

- Blocked by: 2.0, 8.0, 9.0
- Unblocks: —
- Parallelizable: No

## Implementation Details

Coordinate with PRD/Tech Spec: the approach is greenfield replacement (no backwards compatibility, no feature flags).

### Relevant Files

- `engine/llm/orchestrator/loop.go`
- `pkg/config` (remove flags)
- `docs/orchestrator-fsm.md`

### Dependent Files

- `engine/llm/orchestrator/*_test.go`

## Success Criteria

- No feature flag code remains; tests and lints pass
