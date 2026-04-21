---
name: tanstack
description: Comprehensive guide for TanStack ecosystem in React - Query/DB for data fetching, Form for form handling, and Router for client-side routing. Use when working with collections, live queries, optimistic updates, forms, validation, routing, URL parameters, or navigation.
allowed-tools: Read, Grep, Glob
---

# TanStack Developer Guide

This skill provides comprehensive patterns and best practices for the TanStack ecosystem in React applications:

- **TanStack Query/DB**: Data fetching, caching, collections, live queries, and optimistic updates
- **TanStack Form**: Form state management, validation, and field components
- **TanStack Router**: File-based routing, type-safe navigation, and URL parameters

## Quick Start

For detailed examples and patterns, refer to the following files in the `references/` directory:

- `references/query-patterns.md` - TanStack Query and TanStack DB patterns
- `references/form-patterns.md` - TanStack Form patterns and components
- `references/router-patterns.md` - TanStack Router patterns and navigation

---

## TanStack Query/DB Overview

TanStack DB extends TanStack Query with collections, live queries, and optimistic mutations. Key principle: load data into typed collections and consume through live queries that auto-update on data changes.

### Critical Rules

1. **Never Use React Query Patterns with Collections** - Collections have built-in mutation handling. Do NOT use `useMutation` + `invalidateQueries`.

2. **Always Share Collection Instances** - Creating new collection instances for mutations causes "key not found" errors. The data-fetching hook must expose the collection, and mutation hooks must receive it as a parameter.

3. **Configure Persistence Handlers** - Put server writes in collection handlers (`onInsert`, `onUpdate`, `onDelete`), not mutation hooks.

4. **Single Canonical Collection Pattern** - Create ONE collection per entity type. Use live queries for filtered views.

5. **Check Field Changes Properly** - Verify fields actually changed in `onUpdate`, not just that they exist.

### Basic Collection Setup

```typescript
import { createCollection } from '@tanstack/react-db';
import { queryCollectionOptions } from '@tanstack/query-db-collection';
import { z } from 'zod';

const itemSchema = z.object({
  id: z.string(),
  name: z.string().min(1),
  status: z.enum(['active', 'archived']),
});

const itemCollection = createCollection(
  queryCollectionOptions({
    queryKey: ['items'],
    queryFn: async () => (await fetch('/api/items')).json(),
    queryClient,
    getKey: (item) => item.id,
    schema: itemSchema,
  })
);
```

### Sharing Collection Instance (Critical Pattern)

```typescript
// CORRECT - share the instance
export function useItems() {
  const collection = useMemo(() => createItemsCollection(), []);
  const { data } = useLiveQuery(collection);
  return { data, collection }; // Expose collection
}

export function useUpdateItem(collection: ItemsCollection) {
  return (id, data) => collection.update(id, data);
}
```

---

## TanStack Form Overview

TanStack Form provides headless form logic with automatic type inference and flexible validation.

### Core Principles

- **Type Safety**: Types are inferred from default values - avoid manual generic declarations.
- **Headless Design**: Build UI components to match your design system.
- **Schema-First Validation**: Use Zod for cleaner, more maintainable validation.

### Basic Form Setup with `createFormHook`

```typescript
import { createFormHookContexts, createFormHook } from '@tanstack/react-form'

export const { fieldContext, formContext, useFieldContext } =
  createFormHookContexts()

export const { useAppForm } = createFormHook({
  fieldContext,
  formContext,
  fieldComponents: {
    TextField,
    SelectField,
  },
  formComponents: {
    SubmitButton,
  },
})
```

### Form Initialization

```typescript
const form = useAppForm({
  defaultValues: {
    username: '',
    email: '',
    age: 0,
  },
  validators: {
    onChange: schema,
  },
  onSubmit: async ({ value }) => {
    // Handle submission
  },
})
```

### Async Validation with Debouncing

```typescript
<form.Field
  name="username"
  asyncDebounceMs={500}
  validators={{
    onChangeAsync: async ({ value }) => {
      const isAvailable = await checkUsernameAvailability(value)
      return isAvailable ? undefined : 'Username already taken'
    },
  }}
/>
```

---

## TanStack Router Overview

TanStack Router provides type-safe file-based routing with first-class TypeScript support.

### Core Principles

- **Type-Safe Routing**: Embrace type-safe routing as the primary benefit.
- **File-Based Routes**: Use file-based routing for scalability.
- **Generated Route Tree**: Leverage the generated route tree for type safety.

### File Structure

```
src/routes/
├── __root.tsx          # Root layout with providers
├── _authenticated.tsx  # Auth layout wrapper
├── index.tsx          # Home page (/)
├── posts/
│   ├── index.tsx      # /posts
│   └── $postId.tsx    # /posts/:postId (typed params)
└── settings/
    ├── _layout.tsx    # Settings layout
    └── profile.tsx    # /settings/profile
```

### Basic Route with Search Params

```typescript
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

const searchSchema = z.object({
  page: z.number().min(1).catch(1),
  search: z.string().optional(),
})

export const Route = createFileRoute('/posts/')({
  validateSearch: searchSchema,
  component: PostsList,
})

function PostsList() {
  const { page, search } = Route.useSearch()
  // Use search params...
}
```

### Authentication Layout

```typescript
// routes/_authenticated.tsx
import { createFileRoute, redirect, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    const isAuthenticated = checkAuth()
    if (!isAuthenticated) {
      throw redirect({
        to: '/login',
        search: { redirect: location.href },
      })
    }
  },
  component: () => <Outlet />,
})
```

### Type-Safe Navigation

```typescript
import { Link, useNavigate } from '@tanstack/react-router'

function Navigation() {
  const navigate = useNavigate()

  return (
    <>
      <Link
        to="/posts/$postId"
        params={{ postId: '123' }}
        search={{ tab: 'comments' }}
      >
        View Post
      </Link>

      <button onClick={() => navigate({ to: '/posts', search: { page: 1 } })}>
        Go to Posts
      </button>
    </>
  )
}
```

---

## Validation Checklist

Before finishing a task involving TanStack:

### Query/DB
- [ ] Collection instances are shared between data-fetching and mutation hooks
- [ ] Persistence handlers (`onInsert`, `onUpdate`, `onDelete`) are configured
- [ ] No `useMutation` + `invalidateQueries` patterns with collections
- [ ] One canonical collection per entity type
- [ ] Field changes properly verified in `onUpdate` handlers

### Form
- [ ] Use `createFormHook` with `useAppForm` instead of raw `useForm` for consistency
- [ ] Provide complete default values for proper type inference
- [ ] Use Zod schemas for validation when possible
- [ ] Debounce async validations (minimum 500ms recommended)
- [ ] Prevent default on form submission
- [ ] Display errors with proper accessibility (`role="alert"`)

### Router
- [ ] Route path in `createFileRoute` matches file location
- [ ] Search params use Zod validation with proper defaults (`.catch()`)
- [ ] Loader dependencies are correctly specified in `loaderDeps`
- [ ] Authentication routes use `beforeLoad` with proper redirects
- [ ] Navigation uses typed `Link` or `useNavigate` hooks
- [ ] Error boundaries are implemented at route level

### General
- [ ] Run `pnpm run typecheck` and `pnpm run test`

---

For complete examples, edge cases, and advanced patterns, see the reference files in this skill directory.
