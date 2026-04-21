# TanStack Query and TanStack DB Patterns

This document provides comprehensive patterns and best practices for data fetching with TanStack Query and TanStack DB in React applications.

## Core Architecture

TanStack DB extends TanStack Query with collections, live queries, and optimistic mutations. Key principle: load data into typed collections and consume through live queries that auto-update on data changes.

## Critical Rules

### 1. Never Use React Query Patterns with Collections

Collections have built-in mutation handling. Do NOT use `useMutation` + `invalidateQueries`.

```typescript
// WRONG
const mutation = useMutation({
  mutationFn: async (data) => api.update(data),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['items'] })
});

// CORRECT
collection.update(itemId, (draft) => {
  draft.status = newStatus;
});
```

### 2. Always Share Collection Instances

Creating new collection instances for mutations causes "key not found" errors.

```typescript
// WRONG - creates separate instances
export function useItems() {
  const collection = useMemo(() => createItemsCollection(), []);
  return { data: useLiveQuery(collection).data };
}

export function useUpdateItem() {
  const collection = createItemsCollection(); // NEW instance - FAILS!
  return (id, data) => collection.update(id, data);
}

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

### 3. Configure Persistence Handlers

Put server writes in collection handlers, not mutation hooks.

```typescript
const collection = createCollection(
  queryCollectionOptions({
    queryKey: ["items"],
    queryFn: () => api.items.list(),
    queryClient,
    getKey: (item) => item.id,
    schema: itemSchema,

    onInsert: async ({ transaction }) => {
      await Promise.all(
        transaction.mutations.map((m) => api.items.create(m.modified))
      );
    },

    onUpdate: async ({ transaction }) => {
      await Promise.all(
        transaction.mutations.map((m) =>
          api.items.update(m.original.id, m.changes)
        )
      );
    },

    onDelete: async ({ transaction }) => {
      await Promise.all(
        transaction.mutations.map((m) => api.items.delete(m.original.id))
      );
    },
  })
);
```

### 4. Single Canonical Collection Pattern

Create ONE collection per entity type. Use live queries for filtered views.

```typescript
// WRONG - multiple collections
const projectItems = createCollection({ queryKey: ["items", { projectId }] });
const userItems = createCollection({ queryKey: ["items", { userId }] });

// CORRECT - one collection, multiple live queries
const itemsCollection = createCollection({ queryKey: ["items"] });

function ProjectItems({ projectId }) {
  const { data } = useLiveQuery((q) =>
    q.from({ item: itemsCollection })
      .where(({ item }) => eq(item.projectId, projectId))
  );
}
```

### 5. Check Field Changes Properly in onUpdate

Verify fields actually changed, not just that they exist.

```typescript
// WRONG
if (changes.archived !== undefined) { ... }

// CORRECT
if (changes.archived !== undefined && changes.archived !== original.archived) { ... }
```

## Collection Setup

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

## Live Query Patterns

```typescript
import { useLiveQuery } from '@tanstack/react-db';
import { eq } from '@tanstack/db';

// Basic query
const { data } = useLiveQuery((q) =>
  q.from({ item: itemCollection })
    .where(({ item }) => eq(item.status, 'active'))
    .orderBy(({ item }) => item.createdAt, 'desc')
);

// Query with joins
const { data } = useLiveQuery((q) =>
  q.from({ item: itemCollection })
    .join({ user: userCollection }, ({ item, user }) => eq(item.userId, user.id), 'inner')
    .select(({ item, user }) => ({
      id: item.id,
      name: item.name,
      userName: user.name
    }))
);
```

## Optimistic Updates

Mutations apply optimistically by default with automatic rollback on errors.

```typescript
// Optimistic (default)
collection.update(itemId, (draft) => {
  draft.status = 'completed';
});

// Non-optimistic for critical operations
const tx = collection.delete(itemId, { optimistic: false });
await tx.isPersisted.promise;
```

## Query Key Management

Structure keys from generic to specific:

```typescript
const itemKeys = {
  all: ['items'] as const,
  lists: () => [...itemKeys.all, 'list'] as const,
  list: (filters: string) => [...itemKeys.lists(), { filters }] as const,
  details: () => [...itemKeys.all, 'detail'] as const,
  detail: (id: string) => [...itemKeys.details(), id] as const,
};
```

## QueryClient Configuration

```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5,  // 5 minutes
      gcTime: 1000 * 60 * 30,    // 30 minutes
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});
```

## Dependent Queries

```typescript
const { data: user } = useQuery({
  queryKey: ['user', userId],
  queryFn: () => getUser(userId),
});

const { data: projects } = useQuery({
  queryKey: ['projects', user?.id],
  queryFn: () => getProjects(user!.id),
  enabled: !!user?.id, // Only run when user exists
});
```

## Testing

```typescript
const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }) => (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );
};

test('fetches items', async () => {
  const { result } = renderHook(() => useItems(), {
    wrapper: createWrapper(),
  });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
});
```

## Common Anti-Patterns

### Anti-Pattern 1: Creating Collections in Component Scope

```typescript
// WRONG - new collection on every render
function ItemList() {
  const collection = createCollection({ ... }); // BAD!
  const { data } = useLiveQuery(collection);
}

// CORRECT - stable collection reference
function ItemList() {
  const collection = useMemo(() => createCollection({ ... }), []);
  const { data } = useLiveQuery(collection);
}
```

### Anti-Pattern 2: Mixing Query and Collection Patterns

```typescript
// WRONG - mixing paradigms
const { data } = useQuery({ queryKey: ['items'] });
collection.update(id, changes); // Which source of truth?

// CORRECT - use collections consistently
const { data } = useLiveQuery(collection);
collection.update(id, changes);
```

### Anti-Pattern 3: Not Handling Loading States

```typescript
// WRONG - assumes data exists
function ItemList() {
  const { data } = useLiveQuery(collection);
  return data.map(item => ...); // Error if data is undefined!
}

// CORRECT - handle loading
function ItemList() {
  const { data, isPending } = useLiveQuery(collection);
  if (isPending) return <Skeleton />;
  return data?.map(item => ...) ?? null;
}
```

## Advanced Patterns

### Computed Collections

```typescript
const itemsWithStatus = useLiveQuery((q) =>
  q.from({ item: itemCollection })
    .select(({ item }) => ({
      ...item,
      isOverdue: item.dueDate < new Date(),
      priority: calculatePriority(item),
    }))
);
```

### Aggregations

```typescript
const stats = useLiveQuery((q) =>
  q.from({ item: itemCollection })
    .groupBy(({ item }) => item.status)
    .select(({ item }) => ({
      status: item.status,
      count: count(),
    }))
);
```

### Pagination with Collections

```typescript
const { data, hasMore, loadMore } = useLiveQuery((q) =>
  q.from({ item: itemCollection })
    .orderBy(({ item }) => item.createdAt, 'desc')
    .limit(pageSize)
    .offset(page * pageSize)
);
```

## Validation Checklist

Before finishing a task involving TanStack Query/DB:

- [ ] Collection instances are shared between data-fetching and mutation hooks
- [ ] Persistence handlers (`onInsert`, `onUpdate`, `onDelete`) are configured
- [ ] No `useMutation` + `invalidateQueries` patterns with collections
- [ ] One canonical collection per entity type
- [ ] Field changes properly verified in `onUpdate` handlers
- [ ] Run `pnpm run typecheck` and `pnpm run test`
