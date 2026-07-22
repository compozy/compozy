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
[Integration points and classified file paths to inspect, modify, create, or generate.
Reference the TechSpec implementation section for code patterns and interface designs.
Use exactly one path classification from the contract below for every row.]

### Relevant Files

| Classification | Path | Evidence / relevance |
| --- | --- | --- |
| `[existing|proposed|generated|possible]` | `path/to/file` | [Why the path is relevant and the repository or TechSpec evidence for it] |

### Dependent Files

| Classification | Path | Evidence / effect |
| --- | --- | --- |
| `[existing|proposed|generated|possible]` | `path/to/dependency` | [Why repository analysis shows this file is affected] |

### Related ADRs
- [ADR-NNN: Title](../adrs/adr-NNN.md) — Relevance to this task

## Deliverables
- [Concrete output 1]
- [Concrete output 2]
- Every test case assigned in `## Tests` implemented and passing **(REQUIRED)**

## Tests

Cases assigned from `_tests.md`, the test contract — read each ID's full definition there before writing tests.

- Unit
  - [ ] `UT-NNN` — [component/behavior this covers]
- Integration
  - [ ] `IT-NNN` — [flow this covers]
- End-to-end
  - [ ] `E2E-NNN` — [journey this covers]

[Repeat one nested checkbox per assigned ID. When the workflow has no `_tests.md`,
list one concrete case per nested checkbox instead — exact input, condition, and
expected result per case.]

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
- Sizing and independence rules live in SKILL.md; this file owns the task shape
  and test-record rendering contract. The `<critical>` block above ships verbatim
  in every generated task file.

### Test assignment contract

- Model every catalog-backed assignment as one structured record with `id`,
  `owner`, `status`, and `behavior`. The `id` is the exact globally unique ID from
  `_tests.md`; the `status` is that record's own `[ ]` or `[x]` checkbox state.
- The containing task's qualified ID is the record's sole owner: use `task_NN` in
  an ordinary suite and `<task-group-id>/<task-id>` in a Task Group suite. Reject
  multiple owners for one ID.
- Render test-level groups as parents with exactly one nested checklist entry per
  record. Group headings are plain bullets, never checkboxes. Derive group
  completion from its children; any incomplete child keeps the group incomplete.
  Never place multiple test IDs in one checklist entry.
- Preserve each unchanged ID's existing checkbox status during regeneration by
  merging on the stable ID. New IDs start unchecked; never copy a group-level or
  sibling status onto a child record.
- Validate the entire workflow before publishing: reject duplicate IDs in
  `_tests.md`, duplicate assignments, orphaned assignments, missing assignments,
  or multiple owners. Validation and progress output MUST name every missing or
  duplicate ID directly, include each conflicting owner, and list incomplete IDs
  instead of reporting only group counts.

### Path classification contract

- Every path MUST use exactly one classification:
  - `existing` — a repository file confirmed at its current path that the task must inspect or modify.
  - `proposed` — a new source or configuration file suggested by repository analysis or required by the approved TechSpec.
  - `generated` — an artifact produced by a named build, code-generation, or execution step; identify that producer in the evidence column.
  - `possible` — an unconfirmed architectural location that may help discovery but is not an implementation commitment.
- Before generating the task, validate each `existing` path against the repository. Treat a missing or renamed `existing` path as a blocking error: resolve the current path or remove the claim before publishing the task.
- Treat `proposed` and `possible` paths as advisory unless the approved TechSpec mandates the exact location. Express subtasks, deliverables, and success criteria as outcomes, not advisory filenames.
- Trace imports, calls, configuration, generated outputs, and test ownership to derive `Dependent Files` from repository analysis rather than guesses. Record that evidence in the table.

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
