---
name: eng-data-boundaries
description: Data-boundary audit for Compozy route loaders, TanStack Query caches, paginated catalogs, filters, ordering, totals, live streams, workspace-scoped reads, and their backend APIs. Use when adding, changing, or debugging those paths. Don't use for presentational-only UI or backend work with no public read-model impact.
---

# Compozy Data Boundaries

Preserve the owner's scope, order, completeness, identity, and continuity across the changed read path.

- Identify the authoritative store/service/projection and whether the datum is global, workspace, session, or agent scoped. Distinguish complete populations from pages, samples, tails, and projections. Use `references/boundary-map.md` for a new or ambiguous contract; reuse an existing map for unchanged fields.
- Trace affected Web reads through loaders, query options/keys, adapters, contract, transport, and owner; trace live writes through cursor/frame, listener, cache merge, and view model. Repair the first boundary that loses truth and remove compensation made obsolete by that repair.
- For catalog/filter/sort/count/cursor changes, read the matching sections of `references/catalog-contract.md`. For pagination with SSE, polling, optimistic writes, or reconnect, use `references/live-cache-contract.md`. Read the complete contract when a change spans those coupled invariants.
- Preserve the API envelope in Query caches; flatten only at the view-model read boundary. Reuse canonical option/key factories across loaders, hooks, mutations, and stream writers. Do not reconstruct server scope, order, totals, cursors, or continuity from weaker client data.

Use relevant rows of `references/test-matrix.md` to identify the invariant, owning layer, and existing suite before changing coverage. Adversarial identities, scale, and reconnect cases should falsify the changed assumption; reuse existing cases for unchanged invariants. `eng-consolidate-test-suites` helps when ownership is unclear.

Public contract changes co-ship affected transports, generated types, consumers, docs, and `skills/compozy/`. Record that once in the owning `docs/_memory/change-impact.md` audit and update affected QA scenarios. Use the enclosing workstream's scoped verification and delivery gates without repeating them for each boundary.

## Unresolved boundaries

- Trace an unknown owner/population before designing its query; do not ask the user to resolve discoverable code facts.
- Exact totals need an aggregate/projection, not an unbounded rich read or a relabeled loaded count.
- A stream without stable identity or a reset fence remains a wake signal for a canonical reread; do not heuristically merge it.
- If a static rule cannot distinguish a valid view-model transformation from server-contract compensation, prove the invariant behaviorally instead of expanding a syntax ban.
