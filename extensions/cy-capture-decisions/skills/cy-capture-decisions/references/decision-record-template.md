# Decision Record Template

Use this exact structure for every promoted decision body. The file is parsed by the `decisionlog`
format validator (task_02) and read on demand by future features. One decision per file.

## Filename Pattern

```
.compozy/decisions/AD-NNN.md
```

- `NNN` is a three-digit zero-padded, sequential id (`001`, `002`, ...).
- Assigned to a NEW/SUPERSEDE record as the next sequential id; SKILL.md step 7 defines the exact derivation (from the maximum existing `AD-NNN` suffix, so a numbering gap never reuses an id).
- The bodies live at the **workspace root**, never under `.compozy/tasks/<slug>/`.

## Format

```
---
id: AD-001
title: Event-sourcing for orders
status: proven
tags: [orders, async]
source_slug: feat-orders
source_adr: adrs/adr-002.md
promoted_at: 2026-07-11
supersedes: []
superseded_by: null
evidence: "cy-final-verify report p99<200ms; diff abc123; issue_003 resolved"
---

## Context

<the original ADR's Context — why the decision was needed>

## Decision

<the original ADR's Decision — what was chosen>

## Alternatives

<the original ADR's Alternatives Considered>

## Consequences

<the original ADR's Consequences — trade-offs accepted>

## Reconciliation

<what execution proved vs. planned. State "implemented as designed" when the shipped result matched
the plan. Mark each divergence with a [DEVIATION] marker and cite the evidence that proves it.>
```

## Field Definitions

- **id**: `AD-NNN`, zero-padded, sequential, unique across the whole log. Never reused.
- **title**: One-line human summary of the decision. Keep it short and specific. Must not contain a
  literal `|`: it is copied verbatim into the pipe-delimited index line, so render any pipe as `/`
  (e.g. `JWT / opaque tokens`).
- **status**: Exactly one of `proven`, `candidate`, `superseded`.
  - `proven` — evidence-backed; appears in the index; loaded into agent memory.
  - `candidate` — relevance-gated but no evidence yet; excluded from the index; promotable to `proven`
    in place on a later capture when evidence appears.
  - `superseded` — replaced by a newer record; excluded from the index; keeps its file.
- **tags**: A YAML list of area/domain tags, e.g. `[orders, async]`. Forward-compat for scoped loading.
- **source_slug**: Provenance — the originating workflow slug this decision was reconciled from.
- **source_adr**: Provenance — the originating ADR path relative to the workflow tree
  (e.g. `adrs/adr-002.md`). This is the idempotent re-match key.
- **promoted_at**: The date the record was first written (`YYYY-MM-DD`).
- **supersedes**: A YAML list of `AD-NNN` ids this record replaces. `[]` when it replaces nothing.
- **superseded_by**: The single `AD-NNN` id that replaced this record, or `null` when active.
- **evidence**: A quoted string citing the concrete proof (verify result, diff ref, resolved issue).
  Required and non-empty when `status: proven`. May be empty for `candidate` (record the missing-evidence
  reason in the `Reconciliation` section instead).

## Rules

- Preserve the four original ADR body sections (`Context`, `Decision`, `Alternatives`, `Consequences`)
  and always append a `## Reconciliation` section — it is what distinguishes a captured decision from a
  raw ADR copy.
- A `proven` record MUST cite evidence; a record with no evidence MUST be `candidate`.
- Supersession links are bidirectional: the old record's `superseded_by` and the new record's
  `supersedes` must point at each other.
- Never rewrite an accepted decision in place to reverse it — create a new record and supersede the old
  one. In-place amend is only for UPDATE (same decision gaining status/evidence), which keeps the id.
- Keep `[DEVIATION]` markers concrete: name what diverged and cite the diff/review/verify evidence.
