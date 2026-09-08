---
name: tanstack
description: TanStack Query, Router, and Form patterns for React. Use when writing useQuery/queryOptions, mutations, caching, file-based routes, search params, loaders, or TanStack Form validation. Don't use for TanStack Start, TanStack DB/collections, Zustand client state, or non-TanStack routing.
---

# TanStack

Read the reference matched by the API or behavior being changed. Load adjacent references only for concerns the change crosses; reuse relevant context already read.

## Branches

| When you are… | Reference |
| --- | --- |
| Defining or changing query keys / factories | `references/query-keys.md` |
| Setting staleTime, gcTime, invalidation, placeholder/initial data | `references/query-caching.md` |
| Writing mutations, optimistic updates, or mutation state | `references/query-mutations.md` |
| Prefetching, parallel queries, infinite queries, or cancellation | `references/query-fetch-patterns.md` |
| SSR dehydrate/hydrate, offline networkMode, or query persistence | `references/query-ssr-offline.md` |
| Query error boundaries or select/performance | `references/query-errors-performance.md` |
| Registering the router, `from` typing, virtual routes, or router defaults | `references/router-type-and-org.md` |
| Route loaders, ensureQueryData, or parallel route loading | `references/router-data-loading.md` |
| Search params, Link/navigate, route masks, or preload | `references/router-search-nav.md` |
| Lazy routes, not-found, route context, or auth beforeLoad | `references/router-split-errors-context.md` |
| TanStack Form hooks, Zod validators, or field components | `references/form.md` |

Apply the technical rules relevant to the changed behavior.

## Tripwires

**Query** — array keys that include every dependency; reconcile mutations through canonical keys: update from authoritative responses or invalidate when the owning read model requires a server reread; optimistic cache writes need rollback; never put non-serializable values in keys.

**Router** — register the router type; validate search with defaults (`.catch()`); use `ensureQueryData` in loaders when pairing with Query; `throw redirect(...)` not `return`.

**Form** — complete `defaultValues` for inference; Zod at form/field level; debounce async validators to fit endpoint cost and interaction needs; `role="alert"` on field errors; `preventDefault` on submit.
