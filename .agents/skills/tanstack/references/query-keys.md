# Query Keys

## Always Use Arrays for Query Keys

Query keys must always be arrays at the top level — never a bare string (`'todos'`) or object (`{ id: 1 }`), which cause cache misses and unexpected behavior. Arrays enable caching, invalidation matching, and deduplication.

```tsx
const { data } = useQuery({
  queryKey: ['todos'],
  queryFn: fetchTodos,
})

const { data: user } = useQuery({
  queryKey: ['user', 1],
  queryFn: () => fetchUser(1),
})

// Objects inside arrays are fine
const { data: filteredTodos } = useQuery({
  queryKey: ['todos', { status: 'done', page: 1 }],
  queryFn: () => fetchTodos({ status: 'done', page: 1 }),
})
```

Arrays enable prefix-based invalidation (`invalidateQueries({ queryKey: ['todos'] })` matches all todo queries). Object property order inside arrays does not matter for matching; array element order does: `['todos', 1]` !== `['1', 'todos']`.

## Include All Variables the Query Depends On

Every variable the query function uses must appear in the query key — omitting one (e.g. keying `['posts']` while fetching by `userId`) makes all users share one cache entry and skips refetching when the dependency changes. Including it gives each combination its own cache entry and auto-refetches on change.

```tsx
function UserPosts({ userId }: { userId: string }) {
  const { data } = useQuery({
    queryKey: ['posts', userId],
    queryFn: () => fetchPostsByUser(userId),
  })
  return <PostList posts={data} />
}

// Multiple deps: bundle them in an object
const { data } = useQuery({
  queryKey: ['todos', { status, page }],
  queryFn: () => fetchTodos({ status, page }),
})
```

Even with a long `staleTime`, changing the key triggers a new fetch.

## Organize Keys Hierarchically

Structure keys general → specific (entity type → id → modifiers/filters) rather than flat concatenated strings like `['todo-5-comments']`, which can't be invalidated by level. Hierarchy enables invalidation at any specificity and predictable cache organization.

```tsx
const { data: todos } = useQuery({ queryKey: ['todos'], queryFn: fetchTodos })
const { data: todo } = useQuery({ queryKey: ['todos', 5], queryFn: () => fetchTodo(5) })
const { data: comments } = useQuery({
  queryKey: ['todos', 5, 'comments'],
  queryFn: () => fetchTodoComments(5),
})
const { data: filteredTodos } = useQuery({
  queryKey: ['todos', { status: 'done', page: 1 }],
  queryFn: () => fetchTodos({ status: 'done', page: 1 }),
})

// Invalidate at any level:
queryClient.invalidateQueries({ queryKey: ['todos'] })                 // All todos
queryClient.invalidateQueries({ queryKey: ['todos', 5] })             // Todo 5 + sub-resources
queryClient.invalidateQueries({ queryKey: ['todos', 5, 'comments'] }) // Just comments
```

Recommended hierarchy pattern:

```
['entity']                                  // List
['entity', id]                              // Single item
['entity', id, 'sub-resource']              // Related data
['entity', { filters }]                     // Filtered list
['entity', id, 'sub-resource', { filters }] // Filtered sub-resource
```

## Use Query Key Factories

For apps with many queries, centralize key definitions in factory functions instead of scattering ad-hoc keys across files (where `'todo'` vs `'todos'` typos break invalidation). Factories give consistency, autocomplete, and type-safe refactors.

```tsx
// file: lib/query-keys.ts
export const todoKeys = {
  all: ['todos'] as const,
  lists: () => [...todoKeys.all, 'list'] as const,
  list: (filters: TodoFilters) => [...todoKeys.lists(), filters] as const,
  details: () => [...todoKeys.all, 'detail'] as const,
  detail: (id: number) => [...todoKeys.details(), id] as const,
  comments: (id: number) => [...todoKeys.detail(id), 'comments'] as const,
}

// Usage
const { data } = useQuery({
  queryKey: todoKeys.detail(id),
  queryFn: () => fetchTodo(id),
})
queryClient.invalidateQueries({ queryKey: todoKeys.all })       // Everything
queryClient.invalidateQueries({ queryKey: todoKeys.detail(5) }) // Specific todo + comments
```

Combine with `queryOptions` for full type-safe query definitions reusable across `useQuery`, `prefetchQuery`, etc.:

```tsx
import { queryOptions } from '@tanstack/react-query'

export const todoQueries = {
  detail: (id: number) => queryOptions({
    queryKey: todoKeys.detail(id),
    queryFn: () => fetchTodo(id),
    staleTime: 5 * 60 * 1000,
  }),
}

const { data } = useQuery(todoQueries.detail(5))
await queryClient.prefetchQuery(todoQueries.detail(5))
```

## Ensure All Key Parts Are JSON-Serializable

Keys are hashed via JSON serialization, so non-serializable parts — functions, class instances (lose their prototype), symbols, `new Date()` (new value each render), Map/Set, circular refs — break caching. Use primitives, plain objects, and stable string representations instead.

```tsx
const filters = { status: 'active', priority: 'high' } // plain objects are fine
const { data: todos } = useQuery({
  queryKey: ['todos', filters],
  queryFn: () => fetchTodos(filters),
})

// For dates, use a stable ISO string
const dateKey = date.toISOString().split('T')[0] // '2024-01-15'
const { data: events } = useQuery({
  queryKey: ['events', dateKey],
  queryFn: () => fetchEvents(date),
})
```

| Safe to use | Avoid |
| --- | --- |
| Strings, numbers, booleans, null | Functions |
| Plain objects (no prototype methods) | Class instances |
| Arrays of serializable values | Symbols |
| `undefined` (stripped but handled) | Date objects (use ISO strings) |
| | Map/Set (use arrays/objects) |
| | Circular references |

Hashing is deterministic: object property order does not matter (`{ a: 1, b: 2 }` equals `{ b: 2, a: 1 }`), and `undefined` properties are normalized (`{ a: 1, b: undefined }` equals `{ a: 1 }`). Sanity check: `JSON.stringify(queryKey)` should run without errors.
