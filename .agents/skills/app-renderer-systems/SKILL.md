---
name: app-renderer-systems
description: Guides creation and modification of domain feature systems organized under a systems/ directory. Covers directory layout, API service layer patterns, TanStack Query hooks (queries, mutations, optimistic updates), React context and XState store conventions, hook organization, and public API barrel exports. Use when adding a new domain system, extending an existing one, or fixing bugs in a system-layer codebase. Don't use for generic React component work, backend API implementation, or codebases not organized around a systems/ domain pattern.
allowed-tools: Read, Grep, Glob
---

# Feature Systems Guide

A "system" is a self-contained, domain-driven module that owns everything related to one domain: its API calls, query layer, hooks, components, and public API. Systems live under a `systems/<domain>/` directory.

Read `references/directory-layout.md` for the full directory structure and naming conventions.
Read `references/patterns.md` for annotated implementation patterns per layer.

## Quick Reference

### Mandatory Companion Skills

Activate alongside this skill — systems span multiple technical domains:

| Situation             | Activate                                  |
| --------------------- | ----------------------------------------- |
| Any hook or component | `react` + `tanstack-query-best-practices` |
| Data fetching/caching | `tanstack-query-best-practices`           |
| Mutations             | `tanstack-query-best-practices`           |
| XState store          | `xstate`                                  |
| Utility functions     | `es-toolkit`                              |
| Writing/fixing tests  | `test-antipatterns` + `vitest`            |
| Bug fix               | `systematic-debugging` + `no-workarounds` |

### System Directory at a Glance

```
systems/<domain>/
├── index.ts               # Public API barrel — required for every system
├── types.ts               # TypeScript types for this domain
├── adapters/              # API service layer (HTTP calls, error types)
│   └── <domain>-api.ts
├── lib/                   # Pure utilities, schemas, constants, query keys
│   ├── query-keys.ts      # TanStack Query key factory
│   ├── query-options.ts   # Reusable queryOptions / mutationOptions
│   ├── <domain>-schemas.ts
│   └── constants.ts
├── hooks/                 # React hooks (queries, mutations, view-models)
│   ├── __tests__/
│   ├── use-<action>.ts    # Query hooks
│   ├── use-create-<entity>.ts  # Mutation hooks
│   ├── use-update-<entity>.ts
│   ├── use-delete-<entity>.ts
│   └── use-<domain>-view-model.ts
├── contexts/              # React contexts + providers
│   └── <domain>-context.tsx
├── stores/                # XState stores (complex async state machines)
│   └── <domain>-store.ts
├── components/            # React UI components
│   ├── stories/
│   └── index.ts
└── guards/                # Route guards / access checks
```

## Step-by-Step: Creating a New System

### Step 1 — Define types.ts

- Export clean domain types; never expose raw API response shapes.
- Derive from the project's API contract types when available.
- Document complex aggregated types with JSDoc explaining derivation rules and invariants.

### Step 2 — Build the API service layer

- Create `adapters/<domain>-api.ts`.
- Use the project's HTTP client for API calls.
- Export a single namespace object: `export const <domain>Api = { list, create, update, delete }`.
- Export a typed error class: `export class <Domain>ApiError extends Error { ... }`.
- Accept `signal?: AbortSignal` on every function to support query cancellation.
- Keep all internal helpers (error extraction, response normalization) private to the module.

### Step 3 — Add lib/query-keys.ts

```ts
export const <domain>Keys = {
  all: ["<domain>"] as const,
  lists: () => [...<domain>Keys.all, "list"] as const,
  list: (scopeId: string | null) => [...<domain>Keys.lists(), scopeId] as const,
  details: () => [...<domain>Keys.all, "detail"] as const,
  detail: (id: string) => [...<domain>Keys.details(), id] as const,
};
```

- Use hierarchical key structure for granular invalidation.
- Scope keys with any identifier (userId, orgId, etc.) that isolates the cache correctly.
- Use `as const` on every key tuple.

### Step 4 — Add lib/query-options.ts

```ts
import { queryOptions } from "@tanstack/react-query";
import { <domain>Api } from "../adapters/<domain>-api";
import { <domain>Keys } from "./query-keys";

export function <domain>ListOptions(scopeId: string | null) {
  return queryOptions({
    queryKey: <domain>Keys.list(scopeId),
    queryFn: ({ signal }) => <domain>Api.list(scopeId!, signal),
    staleTime: 60_000,
    enabled: Boolean(scopeId),
  });
}

export function <domain>DetailOptions(id: string) {
  return queryOptions({
    queryKey: <domain>Keys.detail(id),
    queryFn: ({ signal }) => <domain>Api.get(id, signal),
    enabled: Boolean(id),
  });
}
```

- Co-locate `queryKey` and `queryFn` via `queryOptions` for type safety and reuse.
- Export each option factory for use in hooks, prefetching, and route loaders.
- Always pass `signal` from the query context through to the API layer.

### Step 5 — Write hooks

- **Query hooks**: Wrap `useQuery` with the `queryOptions` factories; accept a scope ID + optional `{ enabled? }`.
- **Mutation hooks**: Use `useMutation` with proper `onMutate` / `onError` / `onSettled` callbacks for optimistic updates.
- **View-model hooks**: Compose multiple hooks for a page/shell component; return a flat object.
- Place tests in `hooks/__tests__/` or co-locate as `use-xxx.test.tsx`.

Read `references/patterns.md` for complete mutation and optimistic update patterns.

### Step 6 — (Optional) Add context

Create `contexts/<domain>-context.tsx` when query data or combined state must be shared across a component subtree without prop-drilling.

```ts
// Always nullable context — consumer hook throws if used outside provider
export const <Domain>Context = createContext<<Domain>ContextValue | null>(null);
```

- Export the context, provider component, and re-export consumer hooks from the same file.
- For performance-sensitive trees, split into Core / UI / Operations sub-contexts.

### Step 7 — (Optional) Add an XState store

Create `stores/<domain>-store.ts` for complex async state machines (multi-step flows, polling, event emission).

```ts
export const <domain>Store = createStore({
  context: { ... } as <Domain>Context,
  emits: { ... },
  on: {
    someEvent: (context, event, enqueue) => {
      enqueue.effect(async () => { ... });
      return { ...context, isLoading: true };
    },
  },
});
```

### Step 8 — Wire up index.ts

Organize the barrel with labeled sections and explicit named exports:

```ts
// Types
export type { <Domain>Type } from "./types";

// Hooks
export { use<Domain>List, use<Domain>Detail } from "./hooks";
export { useCreate<Domain>, useUpdate<Domain>, useDelete<Domain> } from "./hooks";

// Components
export { <Domain>Component } from "./components";

// Utilities
export { <domain>HelperFn } from "./lib/<domain>-utils";

// Query Keys & Options
export { <domain>Keys } from "./lib/query-keys";
export { <domain>ListOptions, <domain>DetailOptions } from "./lib/query-options";

// API
export { <domain>Api, <Domain>ApiError } from "./adapters/<domain>-api";
```

## Critical Rules

1. **Use `queryOptions` for co-location.** Co-locate `queryKey` and `queryFn` in reusable option factories. Never scatter the same query key across multiple files.
2. **Unidirectional dependency flow.** `adapters -> lib -> hooks -> components`. Adapters never import from hooks or components.
3. **Scope query keys.** Any query depending on an authenticated scope (user, org, tenant) must include that scope ID in its key to prevent stale cross-scope data.
4. **Typed errors in the API layer.** Never throw raw errors from adapters. Use a typed error class so consumers can distinguish error types without inspecting message strings.
5. **AbortSignal propagation.** Pass `signal` from the `queryFn` context through to every API call for proper query cancellation.
6. **Always invalidate after mutations.** Use `queryClient.invalidateQueries` in `onSettled` to ensure eventual consistency with the server.
7. **Optimistic updates require rollback.** When using cache-based optimistic updates, snapshot previous data in `onMutate` and restore in `onError`.
8. **Cancel outgoing queries before optimistic updates.** Call `queryClient.cancelQueries` in `onMutate` to prevent refetches from overwriting optimistic state.
9. **Zod schemas in lib/.** Place all Zod schemas in `lib/<domain>-schemas.ts` for runtime validation at API boundaries.

## Error Handling

- **API layer throws typed error**: TanStack Query catches and exposes it via `query.error`.
- **Mutation fails with optimistic update**: `onError` callback rolls back the cache to the snapshot from `onMutate`, then `onSettled` invalidates to refetch fresh data.
- **Stale cross-scope data**: Add the scope ID to the query key and verify that `enabled` guards check `Boolean(scopeId)`.
- **Query cancellation on unmount**: TanStack Query automatically cancels in-flight queries via the `signal` when a component unmounts — ensure `signal` is propagated to the API layer.

## Detailed References

- `references/directory-layout.md` — Full directory structure, file naming, and barrel conventions
- `references/patterns.md` — Annotated code patterns for the API layer, query options, hooks, mutations, optimistic updates, contexts, and stores
