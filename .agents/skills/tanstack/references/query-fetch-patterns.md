# Query Fetch Patterns

## Prefetch on User Intent

Prefetch on intent signals (hover, focus, touch) instead of waiting for the click, so likely-next data is warm before navigation eliminates the perceived load. Use `queryClient.prefetchQuery` with a `staleTime` so the prefetched entry isn't immediately refetched.

```tsx
import { useQueryClient } from '@tanstack/react-query'
import { postQueries } from '@/lib/queries'

function PostList({ posts }: { posts: Post[] }) {
  const queryClient = useQueryClient()

  const handlePrefetch = (postId: number) => {
    queryClient.prefetchQuery({
      ...postQueries.detail(postId),
      staleTime: 60 * 1000,
    })
  }

  return (
    <ul>
      {posts.map(post => (
        <li key={post.id}>
          <Link
            to={`/posts/${post.id}`}
            onMouseEnter={() => handlePrefetch(post.id)}
            onFocus={() => handlePrefetch(post.id)}
          >
            {post.title}
          </Link>
        </li>
      ))}
    </ul>
  )
}
```

| Trigger | When to Use |
|---------|-------------|
| `onMouseEnter` | Desktop, links/buttons user will likely click |
| `onFocus` | Keyboard navigation, accessibility |
| `onTouchStart` | Mobile, before navigation |
| Component mount | Likely next pages, wizard steps |
| Intersection Observer | Below-fold content |

TanStack Router prefetches natively — set `preload="intent"` on `<Link>`, or configure it router-wide:

```tsx
const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
  defaultPreloadDelay: 100,
})
```

For manual prefetch, debounce with a `setTimeout` on `onMouseEnter` cleared on `onMouseLeave` to skip quick pass-overs. Prefetched data uses `gcTime` for retention.

## Use useQueries for Dynamic Parallel Queries

Use `useQueries` when the number or identity of queries is dynamic — never loop `useQuery` (breaks the rules of hooks) or await sequentially in `useEffect` (waterfall). Queries run in parallel and are cached independently; an empty queries array is valid.

```tsx
import { useQueries } from '@tanstack/react-query'

function UserProfiles({ userIds }: { userIds: string[] }) {
  const userQueries = useQueries({
    queries: userIds.map(id => ({
      queryKey: ['users', id],
      queryFn: () => fetchUser(id),
      staleTime: 5 * 60 * 1000,
    })),
  })

  const isLoading = userQueries.some(q => q.isLoading)
  const isError = userQueries.some(q => q.isError)
  const users = userQueries.map(q => q.data).filter(Boolean)

  if (isLoading) return <Loading />
  if (isError) return <Error />

  return (
    <ul>
      {users.map(user => (
        <li key={user.id}>{user.name}</li>
      ))}
    </ul>
  )
}
```

Use the `combine` option to reduce all results into a single derived value in one place:

```tsx
const { data: users, isPending } = useQueries({
  queries: userIds.map(id => ({
    queryKey: ['users', id],
    queryFn: () => fetchUser(id),
  })),
  combine: (results) => ({
    data: results.map(r => r.data).filter(Boolean),
    isPending: results.some(r => r.isPending),
    isError: results.some(r => r.isError),
  }),
})
```

Notes: `useSuspenseQueries` is the Suspense variant (data guaranteed, no loading flags). For dependent parallel queries, gate the second batch on the first's results with `enabled: posts.length > 0`.

## Provide getNextPageParam for Infinite Queries

`useInfiniteQuery` requires `getNextPageParam` — omit it and pages can't advance. It receives the last page (and `allPages`) and returns the next page param, or `undefined` to set `hasNextPage` to `false`. Pair with `getPreviousPageParam` for bi-directional scroll; use `maxPages` to cap stored pages for memory.

```tsx
// Offset-based
const {
  data,
  fetchNextPage,
  hasNextPage,
  isFetchingNextPage,
} = useInfiniteQuery({
  queryKey: ['posts'],
  queryFn: ({ pageParam }) => fetchPosts({ page: pageParam, limit: 20 }),
  initialPageParam: 1,
  getNextPageParam: (lastPage, allPages) => {
    if (lastPage.length < 20) {
      return undefined
    }
    return allPages.length + 1
  },
})
```

Cursor-based returns the server's next cursor; bi-directional adds `getPreviousPageParam`:

```tsx
// Cursor-based
useInfiniteQuery({
  queryKey: ['posts'],
  queryFn: ({ pageParam }) => fetchPosts({ cursor: pageParam }),
  initialPageParam: undefined as string | undefined,
  getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
})

// Bi-directional
useInfiniteQuery({
  queryKey: ['messages', chatId],
  queryFn: ({ pageParam }) => fetchMessages({ chatId, cursor: pageParam }),
  initialPageParam: { direction: 'initial' } as PageParam,
  getNextPageParam: (lastPage) =>
    lastPage.hasMore ? { cursor: lastPage.nextCursor, direction: 'next' } : undefined,
  getPreviousPageParam: (firstPage) =>
    firstPage.hasPrevious ? { cursor: firstPage.prevCursor, direction: 'prev' } : undefined,
})
```

For total-count APIs, derive `Math.ceil(total / pageSize)` and return `page + 1` while `page < totalPages`. Flatten pages for render and drive a Load More button off `hasNextPage` / `isFetchingNextPage`:

```tsx
const allPosts = data?.pages.flatMap(page => page.posts) ?? []

return (
  <div>
    {allPosts.map(post => (
      <PostCard key={post.id} post={post} />
    ))}
    {hasNextPage && (
      <button onClick={() => fetchNextPage()} disabled={isFetchingNextPage}>
        {isFetchingNextPage ? 'Loading...' : 'Load More'}
      </button>
    )}
  </div>
)
```

## Implement Query Cancellation

TanStack Query passes an `AbortSignal` to `queryFn`; forward it to fetch/axios so in-flight requests cancel when the query goes stale or the component unmounts — omitting it leaks bandwidth and lets stale responses resolve. Cancelled queries do not trigger `onError`.

```tsx
const { data } = useQuery({
  queryKey: ['search', searchTerm],
  queryFn: async ({ signal }) => {
    const response = await fetch(`/api/search?q=${searchTerm}`, { signal })
    return response.json()
  },
})
```

| Scenario | Cancelled? |
|----------|------------|
| Query key changes | Yes |
| Component unmounts | Yes |
| `queryClient.cancelQueries()` called | Yes |
| Refetch triggered | Previous request cancelled |
| `enabled` becomes false | Yes |

Axios accepts the same `signal` (`axios.get(url, { signal })`). Cancel manually with `queryClient.cancelQueries({ queryKey: ['search'] })`, and always cancel before an optimistic update so an outstanding fetch can't overwrite it:

```tsx
const updateTodo = useMutation({
  mutationFn: (todo: Todo) => api.updateTodo(todo),
  onMutate: async (newTodo) => {
    await queryClient.cancelQueries({ queryKey: ['todos'] })
    const previousTodos = queryClient.getQueryData(['todos'])
    queryClient.setQueryData(['todos'], (old) => /* ... */)
    return { previousTodos }
  },
})
```

For non-fetch work (Web Workers, custom promises), honor the signal manually — check `signal.aborted` up front and reject on the `abort` event:

```tsx
const { data } = useQuery({
  queryKey: ['expensive-computation', params],
  queryFn: ({ signal }) => {
    return new Promise((resolve, reject) => {
      if (signal.aborted) {
        reject(new DOMException('Aborted', 'AbortError'))
        return
      }
      const worker = new Worker('computation.js')
      worker.postMessage(params)
      worker.onmessage = (e) => resolve(e.data)
      worker.onerror = (e) => reject(e)
      signal.addEventListener('abort', () => {
        worker.terminate()
        reject(new DOMException('Aborted', 'AbortError'))
      })
    })
  },
})
```
