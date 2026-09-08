# Task File Template

Use the applicable sections below. YAML frontmatter follows `task-context-schema.md`;
body headings are a writing aid, not extra acceptance gates. Keep each outcome,
constraint, and evidence obligation in one place; omit sections that would repeat it.

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
- Verify in this task: [owning suite/probe and the behavior it proves; a live entry-path check when required by the outcome]
- Integration verification: [named QA task + remaining journeys/visual rows, or `none`; this does not defer an explicit task acceptance requirement]

## Requirements

[Task-specific constraints; link the accepted contract for shared requirements.
Do not copy generic coding, reading, or testing reminders into every task.]

## Visual Contract

[Include this section only when the task names a visual reference.
Derive rows from that reference — one row per touched artboard
section, state, and viewport; do not use an “all states” catch-all row.
Name the evidence owner: this task for standalone delivery or explicit
task-owned proof; the integration QA task for final loop delivery. Inspect a
representative implementation state early to catch structural errors.]

| ID    | Reference artifact + state           | Implementation target + state | Viewport | Fidelity  | Authorized differences + authority |
| ----- | ------------------------------------ | ----------------------------- | -------- | --------- | ---------------------------------- |
| VC-01 | `path/to/reference.html` — populated | `/route` — populated fixture  | 1440×900 | normative | None                               |

Evidence owner: [this task or the named QA task; list exceptions by row ID].

Evidence for each row at its owning boundary: `.compozy/tasks/<workflow>/evidence/visual/<task-id>/<contract-id>/{reference.png,implementation.png,side-by-side.png,diff.png,comparison.json,review.md}` (or `<QA_OUTPUT_PATH>/qa/visual-contract/<task-id>/...` for isolated QA). Reuse valid bundles; capture only missing or invalidated rows.

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

## Tests

Cases assigned from `_tests.md` — read each assigned definition before editing. Include only the owning levels below; reuse existing coverage by path and invariant, without duplicating it across layers.

- [ ] UT-NNN, UT-NNN, UT-NNN — [component/behavior these cover]
- [ ] IT-NNN — [flow these cover]
- [ ] E2E-NNN — [journey this covers]

[When the workflow has no `_tests.md`, list concrete cases inline instead — exact input, condition, and expected result per case.]

## Success Criteria

[Observable acceptance not already stated in Shippable Outcome or Requirements.
The task completes after its assigned checks and evidence pass; integration-owned
checks remain explicit for the final delivery owner.]
```

## Guidelines

- Write one subtask per coherent unit of work — no fixed count.
- Sizing, independence, and test-assignment rules live in SKILL.md; adapt the guidance above to the task without adding unrelated stages.
