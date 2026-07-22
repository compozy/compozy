# Task File Template

Use this structure for every individual task file. The file must start with YAML frontmatter containing the parseable metadata.

```markdown
---
status: pending
title: [Task title]
type: [one of frontend, backend, docs, test, infra, refactor, chore, bugfix, or a project-specific [tasks].types override]
complexity: [low, medium, high, critical]
---

# Task N: [Title]

## Overview
[2-3 sentences: what slice of the system this task delivers and why it matters in the context of the project.]

<critical>
- ALWAYS READ the PRD, the TechSpec, and their catalogs (`_user_stories.md`, `_tests.md`) before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — implement every test case assigned in ## Tests
</critical>

<requirements>
- [Requirement 1 — specific technical requirement using MUST/SHOULD language]
- [Requirement 2 — e.g., "MUST authenticate users via JWT tokens"]
- [Requirement 3]
</requirements>

## Subtasks
- [ ] N.1 [Subtask description — WHAT to accomplish]
- [ ] N.2 [Subtask description]
- [ ] N.3 [Subtask description]

## Implementation Details
[File paths to create or modify and integration points.
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
- Every test case assigned in `## Tests` implemented and passing **(REQUIRED)**

## Tests

Cases assigned from `_tests.md`, the test contract — read each ID's full definition there before writing tests.

- [ ] UT-NNN, UT-NNN, UT-NNN — [component/behavior these cover]
- [ ] IT-NNN — [flow these cover]
- [ ] E2E-NNN — [journey this covers]

[When the workflow has no `_tests.md`, list concrete cases inline instead — exact input, condition, and expected result per case.]

## Success Criteria
- Every assigned test case implemented and passing
- [Measurable outcome 1]
- [Measurable outcome 2]

[For a task that touches persistence, replace generic outcomes with distinct
criteria in the applicable categories below. Every persistence constraint MUST
name its observable property, target operation or query, and runtime measurement
mechanism. Omit both categories when the task does not touch persistence.]

### Persistence Correctness and Consistency
- Property: [correctness or consistency invariant]; Target: [operation, query, or transaction]; Measurement: [runtime assertion and expected observation]

### Persistence Performance
- Property: [quantified performance threshold]; Target: [operation or query under a named dataset/load]; Measurement: [runtime assertion and bound]

[Supported runtime assertions include exact or bounded executed SQL statement
counts, proof that reads share one snapshot transaction, proof that row and total
queries use the same predicates, generated query-plan checks, and
statement-observer evidence. Classify statement counts by intent: correctness/consistency
when they prove atomic behavior, performance when they impose a resource bound.
Reject a persistence criterion that lacks an observable assertion or whose test
only inspects SQL text, query-builder configuration, or implementation constants
without executing the database behavior.]
```

## Guidelines

- Write one subtask per coherent unit of work — WHAT to accomplish, not HOW; robust tasks typically carry 5-12.
- Sizing, independence, and test-assignment rules live in SKILL.md; the `<critical>` block above ships verbatim in every generated task file.

## Task Group task-suite additions

When task belongs to opted-in Task Group, preserve this shape in task-group-local
suite:

- Store a new suite under its manifest-declared readable directory,
  `.compozy/tasks/<initiative>/_task_groups/NNN-<brief>/`. Preserve
  `_task_groups/TG-NNN/` when editing a legacy plan.
- Set `_tasks.md` frontmatter `workflow` to logical public reference
  `<initiative>/TG-NNN`; do not derive identity from `filepath.Base()`.
- Keep local filenames (`task_01.md`, `task_02.md`) but qualify ownership audit
  keys as `<task-group-id>/<task-id>`. Repeated local task numbers across task groups
  are valid; one qualified key may occur in only one manifest.
- Assign every `_tests.md` case exactly once across all task group suites. A case
  assigned to consumer task group covers integration with producer outcome; it does
  not copy or re-own producer task.
- Keep initiative PRD, TechSpec, stories, tests, and ADRs at root and reference
  them from task context. Do not copy specification corpora into task group.
- Task Group task suite is not executable until plan, manifest, ownership,
  dependency, and exactly-once test-assignment audits pass.
