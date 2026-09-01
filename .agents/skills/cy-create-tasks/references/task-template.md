# Task File Template

Use this structure for every individual task file. The file must start with YAML frontmatter containing the parseable metadata.

```markdown
---
status: pending
title: [Task title]
type: [a standard work-type slug or an approved project-specific lowercase hyphenated slug]
complexity: [low, medium, high, critical]
---

# Task N: [Title]

## Overview

[2-3 sentences: what a user, agent, or operator can newly do when this task merges, and why it matters in the context of the project.]

## Shippable Outcome

- Outcome: [the observable behavior after merge, reached through its real entry path]
- Verify: [`gate` — the slice's tests/lints prove it | `probe` — exact CLI/HTTP/UDS command | `smoke` — surface entry path + touched Visual Contract sections]

<critical>
- ALWAYS READ `_spec.md` and its catalogs (`_user_stories.md`, `_dx.md`, `_uiux.md` when present, `_tests.md`) before starting
- REFERENCE `_spec.md` Part II for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — implement every test case assigned in ## Tests
</critical>

<requirements>
- [Requirement 1 — specific technical requirement using MUST/SHOULD language]
- [Requirement 2 — e.g., "MUST authenticate users via JWT tokens"]
- [Requirement 3]
</requirements>

## Visual Contract

[Include this section when the task's slice touches a surface mapped in
`_uiux.md`. Derive rows from that inventory — one row per touched artboard
section, state, and viewport; do not use an “all states” catch-all row.]

| ID    | Reference artifact + state           | Implementation target + state | Viewport | Fidelity  | Authorized differences + authority |
| ----- | ------------------------------------ | ----------------------------- | -------- | --------- | ---------------------------------- |
| VC-01 | `path/to/reference.html` — populated | `/route` — populated fixture  | 1440×900 | normative | None                               |

Evidence for each row: `.compozy/tasks/<workflow>/evidence/visual/<task-id>/<contract-id>/{reference.png,implementation.png,side-by-side.png,diff.png,comparison.json,review.md}` (or `<QA_OUTPUT_PATH>/qa/visual-contract/<task-id>/...` for isolated QA).

## Subtasks

- [ ] N.1 [Subtask description — WHAT to accomplish]
- [ ] N.2 [Subtask description]
- [ ] N.3 [Subtask description]

## Implementation Details

[File paths to create or modify and integration points.
Reference the `_spec.md` Part II Implementation Design for code patterns and interface designs.]

### Relevant Files

- `path/to/file` — [brief reason this file is relevant]

### Dependent Files

- `path/to/dependency` — [brief reason this file is affected]

### Competitor References

[Only when `_spec.md` File References cites `.resources/` sources — copy this task's subset: same paths, never paraphrased.]

- `.resources/<repo>/<path>:100-150` — [what to mirror or reject here, and why]

### Related ADRs

- [ADR-NNN: Title](../adrs/adr-NNN.md) — Relevance to this task

## Deliverables

- [Concrete output 1]
- [Concrete output 2]
- Every test case assigned in `## Tests` implemented and passing **(REQUIRED)**
- The `## Shippable Outcome` verification tier executed with its evidence recorded **(REQUIRED)**
- [UI-bearing slices only: every Visual Contract row has a durable passing evidence bundle **(REQUIRED)**]

## Tests

Cases assigned from `_tests.md`, the test contract — read each ID's full definition there before writing tests.

- [ ] UT-NNN, UT-NNN, UT-NNN — [component/behavior these cover]
- [ ] IT-NNN — [flow these cover]
- [ ] E2E-NNN — [journey this covers]

[When the workflow has no `_tests.md`, list concrete cases inline instead — exact input, condition, and expected result per case.]

## Success Criteria

- Every assigned test case implemented and passing
- The Shippable Outcome is reachable through its real entry path and its verification tier passed
- [Measurable outcome 1]
- [Measurable outcome 2]
- [UI-bearing slices only: every Visual Contract row is `PASS` with zero unresolved blocking divergence]
```

## Guidelines

- Write one subtask per coherent unit of work — WHAT to accomplish, not HOW; robust tasks typically carry 5-12.
- Sizing, independence, and test-assignment rules live in SKILL.md; the `<critical>` block above ships verbatim in every generated task file.
