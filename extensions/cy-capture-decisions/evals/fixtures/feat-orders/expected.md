# Expected outcome — feat-orders

Canonical run: fresh (empty) log, scopable diff, completed task, resolved review.

## Index (`.compozy/DECISIONS.md`)

Exactly one active-proven line:

```
AD-001 | Event-sourcing for orders | proven | [orders, async] | audit + replay | feat-orders
```

## Body (`.compozy/decisions/AD-001.md`)

- Frontmatter: `id: AD-001`, `status: proven`, `source_slug: feat-orders`,
  `source_adr: adrs/adr-002.md`, non-empty `evidence` citing the verify/diff/issue, `supersedes: []`,
  `superseded_by: null`, `tags` includes `orders`.
- Body has the four original ADR sections plus a `## Reconciliation` section stating "implemented as
  designed" (no `[DEVIATION]`) with cited evidence (verify p99, diff, issue_001 resolved).

## Skipped (relevance gate)

- `adr-001` (table prefix) — feature-local, obvious from schema.
- `adr-003` (page size default) — not cross-feature-durable.

## Assertions

- Exactly one `AD-*.md` body created; exactly one index line.
- No log files written under `.compozy/tasks/feat-orders/`.
- Re-running with no further changes → zero file changes, summary "no changes" (idempotency).
