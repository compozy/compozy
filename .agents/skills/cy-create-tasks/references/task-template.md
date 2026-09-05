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
- Read the task's cited contracts and relevant repository instructions; use `_spec.md` File References to expand context when needed. Full-loop phase artifacts apply only inside an explicitly requested `cy-loop-tasks` run.
- REFERENCE `_spec.md` Part II for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- Verify every invariant assigned in ## Tests through its owning suite or existing gate; add a test only for a coverage gap.
</critical>

<requirements>
- [Requirement 1 — specific technical requirement using MUST/SHOULD language]
- [Requirement 2 — e.g., "MUST authenticate users via JWT tokens"]
- [Requirement 3]
</requirements>

## Visual Contract

[Include this section only when the task names a visual reference.
Derive rows from that reference — one row per touched artboard
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

- [ADR-NNN: Title](adrs/adr-NNN.md) — Relevance to this task

## Deliverables

- [Concrete output 1]
- [Concrete output 2]
- Every assigned invariant verified in its owning suite or existing gate
- The `## Shippable Outcome` verification tier executed with its evidence recorded **(REQUIRED)**
- [Named visual references only: every Visual Contract row has a durable passing evidence bundle **(REQUIRED)**]

## Tests

Cases assigned from `_tests.md` — read each assigned definition before editing. Include only the owning levels below; reuse existing coverage by path and invariant, without duplicating it across layers.

- [ ] UT-NNN, UT-NNN, UT-NNN — [component/behavior these cover]
- [ ] IT-NNN — [flow these cover]
- [ ] E2E-NNN — [journey this covers]

[When the workflow has no `_tests.md`, list concrete cases inline instead — exact input, condition, and expected result per case.]

## Success Criteria

- Every assigned invariant verified; required new tests implemented and passing
- The Shippable Outcome is reachable through its real entry path and its verification tier passed
- [Measurable outcome 1]
- [Measurable outcome 2]
- [Named visual references only: every Visual Contract row is `PASS` with zero unresolved blocking divergence]
```

## Guidelines

- Write one subtask per coherent unit of work — no fixed count.
- Sizing, independence, and test-assignment rules live in SKILL.md; adapt the guidance above to the task without adding unrelated stages.
