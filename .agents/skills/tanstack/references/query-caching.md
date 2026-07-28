# Query Caching

## Set Appropriate staleTime

`staleTime` is how long data stays fresh before refetching on a new mount. Default is `0` (immediately stale → refetch every mount). Match it to how often the data actually changes: set a sensible `QueryClient` default and override per query.

```tsx
const { data: profile } = useQuery({
  queryKey: ['user-profile', userId],
  queryFn: () => fetchUserProfile(userId),
  staleTime: 5 * 60 * 1000, // profile rarely changes
})

// Sensible default, override per query
const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 60 * 1000 } },
})
```

| Data type | staleTime |
| --- | --- |
| Real-time (stocks, live feeds) | `0` |
| Frequently changing (notifications) | 30s–1min |
| User-generated content | 1–5min |
| Reference data (categories, config) | 10–30min |
| Static content | `Infinity` |

Stale data is still returned instantly — the refetch runs in the background. For SSR, raise `staleTime` to avoid an immediate client refetch.

## Configure gcTime for Inactive Query Retention

`gcTime` (formerly `cacheTime`) is how long an *inactive* query stays cached before garbage collection; the countdown starts when the last observer unmounts. Default is 5min. Longer gcTime = instant return on revisit; shorter = less memory. Avoid `Infinity` (memory leak) and `0` (loses the cache on unmount).

```tsx
const { data } = useQuery({
  queryKey: ['dashboard-stats'],
  queryFn: fetchDashboardStats,
  gcTime: 30 * 60 * 1000, // user returns to the dashboard often
})
```

Lifecycle: `mount → fresh (staleTime) → stale → unmount → gcTime countdown → garbage collected`. Remounting before `gcTime` expires returns cached data instantly.

| Scenario | gcTime |
| --- | --- |
| Frequently revisited routes | 15–30min |
| Detail pages (viewed once) | 2–5min |
| Large payloads | 1–2min |
| SSR hydration | ≥ 2s (never `0`) |

`gcTime < staleTime` is rarely useful.

## Use Targeted Invalidation

Invalidation marks matching queries stale, triggering a background refetch when they're next used. Match precisely: invalidate every query the mutation affected (list, detail, dependent counts) and nothing more. Bare `invalidateQueries()` nukes the whole cache.

```tsx
const mutation = useMutation({
  mutationFn: addComment,
  onSuccess: (data, { postId }) => {
    queryClient.invalidateQueries({ queryKey: ['posts', postId, 'comments'] })
    queryClient.invalidateQueries({ queryKey: ['posts', postId, 'comment-count'] })
  },
})
```

Matching modes:

```tsx
queryClient.invalidateQueries({ queryKey: ['todos'] })              // prefix: ['todos'], ['todos', 1], …
queryClient.invalidateQueries({ queryKey: ['todos'], exact: true }) // only ['todos']
queryClient.invalidateQueries({ predicate: (q) => q.queryKey.includes('user-generated') })
```

`refetchType` controls what actually refetches: `'active'` (default) refetches only queries with observers; use `'all'`, `'inactive'`, or `'none'` (mark stale without refetching) for the rest. For a value you already hold, prefer `setQueryData` over invalidation.

## Placeholder Data vs Initial Data

Both show data before the fetch resolves, but `initialData` is written to the cache as real data (respects `staleTime`, shared with other observers) while `placeholderData` is temporary — not cached, always refetches, flagged by `isPlaceholderData`.

```tsx
// initialData: known-good data (e.g. SSR) — persists to cache
useQuery({
  queryKey: ['posts', serverData.id],
  queryFn: () => fetchPost(serverData.id),
  initialData: serverData,
  initialDataUpdatedAt: serverData.fetchedAt, // for correct stale calc
})

// placeholderData: temporary preview from a list — does not persist
const { data, isPlaceholderData } = useQuery({
  queryKey: ['posts', postId],
  queryFn: () => fetchPost(postId),
  placeholderData: () =>
    queryClient.getQueryData<Post[]>(['posts'])?.find((p) => p.id === postId),
})
```

| Behavior | `initialData` | `placeholderData` |
| --- | --- | --- |
| Persisted to cache | Yes | No |
| `staleTime` applies | Yes | No (always fetches) |
| `isPlaceholderData` | `false` | `true` |
| Shown to other components | Yes | No |
| Use case | SSR, complete known data | Preview, previous page |

For pagination, `placeholderData: keepPreviousData` keeps the last page visible while the next loads — check `isPlaceholderData` to dim the UI.
