---
name: agh-data-boundaries
description: Data-boundary audit for AGH route loaders, TanStack Query caches, paginated catalogs, filters, ordering, totals, live streams, workspace-scoped reads, and their backend APIs. Use when adding, changing, or debugging those paths. Don't use for presentational-only UI or backend work with no public read-model impact.
---

# AGH Data Boundaries

Preserve the owner's scope, order, completeness, identity, and continuity across every boundary.

## Procedure

### Step 1: Declare truth

1. Read `.agents/skills/agh/agh-data-boundaries/references/boundary-map.md` and complete one boundary map for every changed list, metric, detail read, or stream.
2. Name the authoritative owner and classify every datum as global, workspace, session, or agent scoped.
3. Distinguish exact populations from pages, samples, tails, and projections.

*Done when:* every changed read model has an explicit owner, scope, population, order, completeness, identity, and lifecycle.

### Step 2: Trace the vertical

1. Trace each Web read through route, loader, option factory, query key, adapter, generated contract, transport, core/service, and store/projection.
2. Trace each live read through durable source, projection cursor, transport frame, listener, cache writer, page merge, and view model.
3. Include HTTP, UDS, CLI, native tools, Extension Host, and the official AGH skill whenever the contract is public.
4. Repair the first boundary that loses or guesses truth; remove downstream compensation made obsolete by that repair.

*Done when:* scope, ordering, completeness, page metadata, and fences remain traceable without a client heuristic or hidden fallback.

### Step 3: Apply the matching contract

1. For a catalog, filter, sort, count, metric, or cursor change, read `.agents/skills/agh/agh-data-boundaries/references/catalog-contract.md` in full and apply every matching invariant.
2. For pagination combined with SSE, polling, optimistic writes, or reconnect, read `.agents/skills/agh/agh-data-boundaries/references/live-cache-contract.md` in full and apply every matching invariant.
3. Keep the canonical API envelope in TanStack Query; flatten only at the view-model read boundary.
4. Reuse the same option factory and key factory in route loaders, hooks, mutations, and stream writers.

*Done when:* no consumer reconstructs scope, server order, totals, cursors, or stream continuity from a weaker representation.

### Step 4: Prove the invariant

1. Activate `consolidate-test-suites` before editing coverage.
2. Read `.agents/skills/agh/agh-data-boundaries/references/test-matrix.md`, select only the rows matching the changed contract, and name the invariant, owning layer, and canonical suite.
3. Reproduce a bug before its fix and keep tests at the lowest layer that owns each distinct failure mode.
4. Use scale and adversarial identities that can falsify page completeness, ordering, scope, and reconnect assumptions.

*Done when:* every changed contract has a canonical test that fails on the lossy behavior and no duplicate invariant was added at another layer.

### Step 5: Close every surface

1. Co-ship contract, OpenAPI, generated TypeScript, CLI/UDS/native/Extension surfaces, docs, and `skills/agh/` when public behavior changes.
2. Complete the AGH Impact Audit for native tools, extensibility/hooks, workspace data isolation, and the official AGH skill.
3. Flag user-visible behavior in `docs/qa/scenarios/`; declare a pure contributor-tooling change when no runtime surface changed.
4. Run the touched lanes during iteration and the repository completion gate once after source freeze.

*Done when:* every public surface agrees, workspace isolation is proved, QA impact is recorded, and the final gate is green.

## Error handling

- If the owner or population cannot be identified, stop implementation and trace the store/service contract before designing the UI query.
- If a required exact total would need scanning an unbounded rich read, design a projection or aggregate instead of relabeling a loaded count.
- If a stream lacks a stable identity or reset fence, keep it as a wake signal and reread a canonical projection instead of merging raw frames heuristically.
- If a static gate cannot distinguish valid view-model transformation from server-contract compensation, enforce the invariant behaviorally rather than broadening the syntax ban.
