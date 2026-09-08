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

> A spec must make its outcome, boundaries, and changed contracts concrete. Include interface signatures when interfaces change, field rationale when data changes, storage-shape decisions when ownership is at stake, and safety invariants when concurrency/leases/permissions are affected.

## Operationalization

Apply the markers relevant to the design, using the current `cy-spec-preflight` contract. Mark an inapplicable marker briefly instead of inventing Go interfaces, side tables, or lease policies. Peer review remains opt-in; a missing applicable contract is repaired before dependent implementation, without imposing a fixed reviewer/model/round count.

## Anti-patterns

- "The function will accept the relevant config and return the result." (no signature)
- "Add a column for ownership tracking." (no rationale, no name, no type)
- "Choose the appropriate storage shape." (no decision)
- "Ensure the lease is held safely." (no invariants)
- "We'll figure out the interface during implementation." (defers the ambiguity)

## Source

- `../analysis/analysis_compozy_tasks.md` §"Markers of 'good enough to execute'" and §"Markers of trouble"
- `docs/_memory/_synthesis.md` skill candidate S-M9 `compozy-techspec-quality-gate`
