---
name: app-renderer-systems
description: "Create, extend, or debug frontend domain modules organized under systems/. Covers adapters, Query hooks, local state, and public exports; excludes generic components and backend implementation."
allowed-tools: Read, Grep, Glob
---
# Feature Systems

A `systems/<domain>/` module owns its domain's UI-facing adapters, data access, hooks, components, and public barrel. Extend the existing system; create only layers the feature actually needs. A local component edit does not scaffold CRUD adapters, a store, context providers, or tests for every layer.

Read `references/directory-layout.md` for placement or a new system, and `references/patterns.md` for the changed API/query/hook pattern. Related skills are conditional: React for component/hook behavior, TanStack for Query/Router/Form, XState Store for that store API, and the existing test runner's guidance when editing tests. Use debugging guidance for an unresolved failure, not every system edit.

- Keep dependency direction toward lower layers: components consume hooks, hooks consume query/options and adapters; adapters do not import UI. Cross-system consumers use the public barrel.
- Preserve API envelopes in adapters and query caches; flatten into component view models at the read boundary. Derive types from the public contract and validate untrusted inputs at the owning boundary.
- Reuse canonical query options/keys across loaders, hooks, mutations, and stream writers. Include stable scope and filters; for infinite queries, continuation cursors belong in `pageParam`.
- Pass `AbortSignal` through adapters and represent errors with the project's typed error contract. Guard missing scope at the request boundary; `enabled` alone is not a type proof for a non-null assertion.
- Follow existing freshness defaults. Reconcile mutations through canonical keys; invalidate only when the owning read model requires a server reread. Add optimistic callbacks only for an optimistic contract; cancel conflicting reads, snapshot complete state, and restore on failure.
- Add context only for state that must be shared through a subtree. A nullable required-provider pattern fits some contexts; split providers only for demonstrated ownership or update-frequency needs.
- Keep server state in Query. Use XState Store for event-driven local data; explicit lifecycle modes/guards may need a state machine instead. Do not introduce a store merely because the system has async calls.
- Export supported operations explicitly from the barrel. Reuse the existing suite that owns each changed invariant instead of mirroring the directory tree with tests.

For Compozy data-boundary changes, apply `eng-data-boundaries` and `web/CLAUDE.md`; preserve workspace scope and canonical cache ownership.
