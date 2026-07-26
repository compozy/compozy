# L-012 — TechSpec without Go interface signatures triggers heavy review rework

**Class:** Spec authoring
**Date discovered:** 2026-04-25 / 2026-04-26 (autonomy techspec vs. release-adjustments comparison)
**Evidence sources:** `../analysis/analysis_compozy_tasks.md` §PRD/TechSpec Quality Patterns

## Context

Two TechSpecs from the same period delivered radically different review trajectories:

- **Autonomy techspec** (706 lines): MVP boundary at top, listed Architectural Boundaries, Go interface signatures pasted as code blocks (`ClaimCriteria`, `ClaimedRun`, `TaskClaimer`, `SpawnOpts`, `PermissionNarrower`), data-model fields with rationale, side-table-vs-JSON decisions explicit, lease invariants enumerated as a numbered list. Eighteen tasks executed cleanly with **one** review round.
- **Release-adjustments / qa-review** (no `_techspec.md`, just review-only directories): unresolved review queues persisted across multiple PRs. Tasks 07-09 of autonomy that touched contract-laden interfaces had **exactly one** round of fixes because the techspec gave the implementer no contract ambiguity.

The differentiator was not length — it was concreteness. Specs that paste signatures, list fields with rationale, and enumerate invariants leave nothing to interpretation. Specs that describe the same mechanics in prose force the implementer to invent shapes that reviewers then reject.

## Root cause

Prose-only descriptions produce N implementations, where N is the number of agents that read the spec. Reviewers then converge each implementation toward the implicit intent through review rounds — that is the rework. Code blocks (interface signatures, struct fields, SQL DDL, enum values) are unambiguous; reviewers either approve or reject specific tokens, and the spec author resolves the ambiguity once instead of N times.

## Rule

> A TechSpec is not ready for review until it carries the **six quality markers**:
>
> 1. MVP boundary statement at top.
> 2. Architectural Boundaries section.
> 3. Concrete Go interface signatures pasted as code blocks (not prose).
> 4. Data-model field rationale (purpose + shape per new column / frontmatter field / config key).
> 5. Side-table-vs-JSON decision stated for every new domain entity.
> 6. Lease / safety invariants as a numbered list.
>
> Specs without these markers are pre-rejected — they will need multiple review rounds.

## Operationalization

`cy-spec-peer-review` invokes Opus with a six-marker checklist embedded in the prompt. `cy-spec-preflight` blocks `cy-create-techspec` from completing until the six markers are present.

When a spec is missing a marker, fix the spec — do not start tasks against the gap.

## Anti-patterns

- "The function will accept the relevant config and return the result." (no signature)
- "Add a column for ownership tracking." (no rationale, no name, no type)
- "Choose the appropriate storage shape." (no decision)
- "Ensure the lease is held safely." (no invariants)
- "We'll figure out the interface during implementation." (defers the ambiguity)

## Source

- `../analysis/analysis_compozy_tasks.md` §"Markers of 'good enough to execute'" and §"Markers of trouble"
- `docs/_memory/_synthesis.md` skill candidate S-M9 `agh-techspec-quality-gate`
