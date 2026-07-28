# Query SSR and Offline

## Use Dehydrate/Hydrate Pattern for SSR

For SSR, prefetch on the server, `dehydrate` the cache to a serializable state, ship it to the client, and hydrate it — don't pass fetched data straight through props, which bypasses the cache and refetches on mount (content flash + duplicate request). Create a fresh `QueryClient` per request so data never leaks between users.

Next.js App Router — prefetch, then wrap in `HydrationBoundary`:

```tsx
// app/posts/page.tsx
import { dehydrate, HydrationBoundary, QueryClient } from '@tanstack/react-query'
import { postQueries } from '@/lib/queries'

export default async function PostsPage() {
  const queryClient = new QueryClient()
  await queryClient.prefetchQuery(postQueries.list())

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <PostList /> {/* 'use client', reads via useSuspenseQuery(postQueries.list()) */}
    </HydrationBoundary>
  )
}
```

TanStack Router — prefetch in the route `loader` via `ensureQueryData`:

```tsx
import { createFileRoute } from '@tanstack/react-router'
import { postQueries } from '@/lib/queries'

export const Route = createFileRoute('/posts')({
  loader: ({ context: { queryClient } }) => queryClient.ensureQueryData(postQueries.list()),
  component: () => {
    const { data: posts } = useSuspenseQuery(postQueries.list())
    return <PostList posts={posts} />
  },
})
```

Manual SSR — `dehydrate` on the server, `hydrate` on the client:

```tsx
// server: prefetch, then serialize dehydrated state into the HTML
const queryClient = new QueryClient({ defaultOptions: { queries: { staleTime: 60 * 1000 } } })
await queryClient.prefetchQuery({ queryKey: ['posts'], queryFn: fetchPosts })
const html = renderToString(
  <QueryClientProvider client={queryClient}><App /></QueryClientProvider>,
)
// inject: window.__DEHYDRATED_STATE__ = <safe-serialized dehydrate(queryClient)>

// client
const queryClient = new QueryClient()
hydrate(queryClient, window.__DEHYDRATED_STATE__)
hydrateRoot(el, <QueryClientProvider client={queryClient}><App /></QueryClientProvider>)
```

Set `staleTime > 0` on the server so the client doesn't immediately refetch. Use a safe serializer, not `JSON.stringify`, to prevent XSS. Failed queries aren't dehydrated by default — override with `shouldDehydrateQuery`. `HydrationBoundary` nests for route-level prefetching.

## Configure Network Mode for Offline Support

`networkMode` controls how queries and mutations behave with no connection. The default `'online'` pauses (`fetchStatus: 'paused'`) but gives no feedback — always surface the paused state to the user:

```tsx
function TodoList() {
  const { data, fetchStatus } = useQuery({
    queryKey: ['todos'],
    queryFn: fetchTodos,
    networkMode: 'online',
  })
  return (
    <div>
      {fetchStatus === 'paused' && <Banner>You're offline. Showing cached data.</Banner>}
      <TodoItems todos={data} />
    </div>
  )
}
```

For offline-first, set `networkMode: 'always'` (globally via `QueryClient` defaults) so the `queryFn` always runs and can fall back to local storage:

```tsx
const queryClient = new QueryClient({
  defaultOptions: {
    queries: { networkMode: 'always' },
    mutations: { networkMode: 'always' },
  },
})
// queryFn: try fetchTodosFromServer() + saveToLocalDB(), catch → getFromLocalDB()
```

| Mode | Behavior | Use Case |
|------|----------|----------|
| `'online'` (default) | Pauses when offline, resumes when online | Most apps, show offline state |
| `'always'` | Always runs queryFn regardless of network | Offline-first apps, local-only data |
| `'offlineFirst'` | Tries once, then waits for network if fails | Best-effort offline |

Mutations queue while paused; expose the backlog with `useMutationState` and optimistic `onMutate`:

```tsx
const addTodo = useMutation({
  mutationFn: createTodo,
  onMutate: async (newTodo) => {
    await queryClient.cancelQueries({ queryKey: ['todos'] })
    const previous = queryClient.getQueryData(['todos'])
    queryClient.setQueryData(['todos'], (old: Todo[]) => [...old, newTodo])
    return { previous }
  },
  onError: (err, newTodo, ctx) => queryClient.setQueryData(['todos'], ctx?.previous),
  onSettled: () => queryClient.invalidateQueries({ queryKey: ['todos'] }),
})

const paused = useMutationState({ filters: { status: 'pending' } }).filter(m => m.state.isPaused)
// paused.length → "N changes waiting to sync"
```

Track connectivity with `onlineManager` (subscribe via `useSyncExternalStore`; `onlineManager.setOnline(false)` forces offline for testing):

```tsx
import { onlineManager } from '@tanstack/react-query'

function NetworkStatus() {
  const isOnline = useSyncExternalStore(onlineManager.subscribe, () => onlineManager.isOnline())
  return <div className={isOnline ? 'online' : 'offline'}>{isOnline ? 'Connected' : 'Offline'}</div>
}
```

## Configure Query Persistence for Offline Support

TanStack Query can persist the cache to storage and restore it on load for offline support and faster startup — a plain `QueryClientProvider` loses the cache on every reload. Wrap the app in `PersistQueryClientProvider` (from `@tanstack/react-query-persist-client`) and set `gcTime` higher than the 5-min default so entries survive long enough to persist.

localStorage (synchronous persister):

```tsx
import { createSyncStoragePersister } from '@tanstack/query-sync-storage-persister'
import { PersistQueryClientProvider } from '@tanstack/react-query-persist-client'

const queryClient = new QueryClient({
  defaultOptions: { queries: { gcTime: 1000 * 60 * 60 * 24, staleTime: 1000 * 60 * 5 } },
})
const persister = createSyncStoragePersister({ storage: window.localStorage, key: 'REACT_QUERY_CACHE' })

function App() {
  return (
    <PersistQueryClientProvider
      client={queryClient}
      persistOptions={{ persister, maxAge: 1000 * 60 * 60 * 24 }}
    >
      <MyApp />
    </PersistQueryClientProvider>
  )
}
```

IndexedDB (asynchronous persister); `buster` invalidates the whole cache on version bumps:

```tsx
import { createAsyncStoragePersister } from '@tanstack/query-async-storage-persister'
import { get, set, del } from 'idb-keyval'

const persister = createAsyncStoragePersister({
  storage: {
    getItem: async (key) => await get(key),
    setItem: async (key, value) => await set(key, value),
    removeItem: async (key) => await del(key),
  },
  key: 'REACT_QUERY_CACHE',
})
// persistOptions: { persister, maxAge: 1000 * 60 * 60 * 24 * 7, buster: APP_VERSION }
```

React Native reuses the async persister with `storage: AsyncStorage` from `@react-native-async-storage/async-storage`.

Filter what gets written with `shouldDehydrateQuery` — never persist sensitive or real-time data:

```tsx
import { persistQueryClient } from '@tanstack/react-query-persist-client'

persistQueryClient({
  queryClient,
  persister,
  dehydrateOptions: {
    shouldDehydrateQuery: (query) => {
      if (query.queryKey[0] === 'user-session') return false
      if (query.queryKey[0] === 'notifications') return false
      return query.state.status === 'success'
    },
  },
})
```

Gate UI on restoration via the provider's `isRestoring`:

```tsx
<PersistQueryClientProvider client={queryClient} persistOptions={{ persister }}>
  <PersistQueryClientProvider.Consumer>
    {({ isRestoring }) => (isRestoring ? <SplashScreen /> : <MainApp />)}
  </PersistQueryClientProvider.Consumer>
</PersistQueryClientProvider>
```

| Option | Purpose |
|--------|---------|
| `maxAge` | Maximum cache age before considered invalid |
| `buster` | String to invalidate cache (use app version) |
| `dehydrateOptions.shouldDehydrateQuery` | Filter which queries to persist |
| `hydrateOptions.shouldHydrate` | Filter which queries to restore |

Restored data is still subject to `staleTime` checks.
