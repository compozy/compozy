# Task File Template

Use this structure for every individual task file. The file must start with YAML frontmatter containing the parseable metadata.

```markdown
---
status: pending
title: [Task title]
type: [one of frontend, backend, docs, test, infra, refactor, chore, bugfix, or a project-specific [tasks].types override]
complexity: [low, medium, high, critical]
dependencies:
  - task_01
  - task_02
---

# Task N: [Title]

## Overview
[2-3 sentences: what the task accomplishes and why it matters in the context of the project.]

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- [Requirement 1 — specific technical requirement]
- [Requirement 2 — e.g., "MUST authenticate users via JWT tokens"]
- [Requirement 3]
</requirements>

## Subtasks
- [ ] N.1 [Subtask description — WHAT to accomplish]
- [ ] N.2 [Subtask description]
- [ ] N.3 [Subtask description]

## Implementation Details
[File paths to create or modify, integration points, and dependencies.
Reference the TechSpec implementation section for code patterns and interface designs.]

### Relevant Files
- `path/to/file` — [brief reason this file is relevant]

### Dependent Files
- `path/to/dependency` — [brief reason this file is affected]

### Related ADRs
- [ADR-NNN: Title](../adrs/adr-NNN.md) — Relevance to this task

## Deliverables
- [Concrete output 1]
- [Concrete output 2]
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for [feature] **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] [Test case 1 — e.g., "Happy path: valid input returns expected output"]
  - [ ] [Test case 2 — e.g., "Error path: invalid input returns descriptive error"]
  - [ ] [Edge cases and boundary conditions]
- Integration tests:
  - [ ] [Test case — e.g., "End-to-end flow from request to response"]
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- [Measurable outcome 1]
- [Measurable outcome 2]
```

## Guidelines

- Every task must be independently implementable when its dependencies are met.
- Every task MUST include a Tests section and test items in Deliverables.
- Never create separate tasks dedicated solely to testing.
- Subtasks describe WHAT needs to happen, not HOW to implement it.
- Minimize code in tasks. Show code only to illustrate current structure or problem areas.
- Implementation details should reference the TechSpec for patterns rather than duplicating them.
