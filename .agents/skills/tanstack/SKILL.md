---
name: tanstack
description: TanStack Query, Router, and Form patterns for React. Use when writing useQuery/queryOptions, mutations, caching, file-based routes, search params, loaders, or TanStack Form validation. Don't use for TanStack Start, TanStack DB/collections, Zustand client state, or non-TanStack routing.
---

# TanStack

Match the task to one or more Branches rows and read every listed file **in full** before producing output — those references are the contract; the tripwires below are only a final self-check.

## Branches

| When you are… | Read in full |
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

*Done when:* every matched reference was read, the change follows its patterns, and no tripwire is violated.

## Tripwires

**Query** — array keys that include every dependency; invalidate (or optimistically update with rollback) after mutations; never put non-serializable values in keys.

**Router** — register the router type; validate search with defaults (`.catch()`); use `ensureQueryData` in loaders when pairing with Query; `throw redirect(...)` not `return`.

**Form** — complete `defaultValues` for inference; Zod at form/field level; debounce async validators (≥500ms); `role="alert"` on field errors; `preventDefault` on submit.
