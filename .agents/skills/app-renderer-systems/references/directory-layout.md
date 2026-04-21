# System Directory Layout

## Canonical Structure

```
systems/<domain>/
├── index.ts                         # REQUIRED — public API barrel
├── types.ts                         # Domain types
│
├── adapters/
│   ├── <domain>-api.ts              # API service functions + error class
│   └── <domain>-api.test.ts
│
├── lib/
│   ├── query-keys.ts                # TanStack Query key factory
│   ├── query-options.ts             # Reusable queryOptions factories
│   ├── <domain>-schemas.ts          # Zod schemas (API validation + forms)
│   ├── <domain>-utils.ts            # Pure domain-specific utilities
│   └── constants.ts                 # Domain constants (timeouts, limits, etc.)
│
├── hooks/
│   ├── __tests__/                   # Subdirectory for complex/integration tests
│   │   └── use-<action>.test.tsx
│   ├── use-<action>.ts              # Query hooks (useQuery wrappers)
│   ├── use-create-<entity>.ts       # Mutation hooks (useMutation wrappers)
│   ├── use-update-<entity>.ts
│   ├── use-delete-<entity>.ts
│   └── use-<domain>-view-model.ts   # Composed view-model hook for a page/shell
│
├── contexts/
│   ├── <domain>-context.tsx         # Context + provider + re-exported consumer hooks
│   └── <domain>-context.test.tsx
│
├── stores/
│   ├── <domain>-store.ts            # XState store
│   └── <domain>-store.test.ts
│
├── components/
│   ├── <component-name>.tsx
│   ├── <component-name>.test.tsx
│   ├── stories/
│   │   └── <component-name>.stories.tsx
│   └── index.ts                     # Component barrel
│
└── guards/
    ├── <guard-name>.ts
    └── <guard-name>.test.ts
```

## File Naming Rules

| Layer         | Pattern                   | Example                     |
| ------------- | ------------------------- | --------------------------- |
| API service   | `<domain>-api.ts`         | `issues-api.ts`             |
| Types         | `types.ts`                | `types.ts`                  |
| Query keys    | `query-keys.ts`           | `query-keys.ts`             |
| Query options | `query-options.ts`        | `query-options.ts`          |
| Zod schema    | `<domain>-schemas.ts`     | `issue-schemas.ts`          |
| Hook          | `use-kebab-case.ts`       | `use-create-issue.ts`       |
| Context       | `<domain>-context.tsx`    | `issue-details-context.tsx` |
| Store         | `<domain>-store.ts`       | `api-key-store.ts`          |
| Component     | `kebab-case.tsx`          | `issue-list-item.tsx`       |
| Story         | `<component>.stories.tsx` | `issue-list.stories.tsx`    |
| Test          | `<file>.test.ts(x)`       | `use-delete-issue.test.tsx` |

## Folders That Are Optional

Only create these when the system actually needs them:

- `contexts/` — only when query data or state must be shared across a subtree without prop-drilling
- `stores/` — only for complex async state machines (multi-step flows, polling, event emission)
- `guards/` — only for route-level or access-control logic

## Index Barrel Convention

Use explicit named exports organized by labeled sections. No `export * from`:

```ts
// Types
export type { FooType, FooStatus } from "./types";

// Hooks — Queries
export { useFooList } from "./hooks/use-foo-list";
export { useFooDetail } from "./hooks/use-foo-detail";

// Hooks — Mutations
export { useCreateFoo } from "./hooks/use-create-foo";
export { useUpdateFoo } from "./hooks/use-update-foo";
export { useDeleteFoo } from "./hooks/use-delete-foo";

// Components
export { FooList, FooDetail } from "./components";

// Utilities
export { fooHelperFn } from "./lib/foo-utils";

// Query Keys & Options
export { fooKeys } from "./lib/query-keys";
export { fooListOptions, fooDetailOptions } from "./lib/query-options";

// API
export { fooApi, FooApiError } from "./adapters/foo-api";
```

## Component Barrel Convention

`components/index.ts` exports all public components by name:

```ts
export { FooCard } from "./foo-card";
export { FooList } from "./foo-list";
export { FooGuard } from "./foo-guard";
```

## Cross-System Imports

- Import from another system using its public barrel: `import { issuesApi } from "@/systems/issues"`.
- Never reach into another system's internals: `import { xxx } from "@/systems/issues/adapters/issues-api"` is forbidden.
- Shared utilities that multiple systems need belong in the project's shared `lib/` directory, not inside any system folder.
