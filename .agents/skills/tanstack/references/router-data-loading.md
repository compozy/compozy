# Router Data Loading

## Use Route Loaders for Data Fetching

Define a route `loader` to fetch data before the route renders, instead of fetching in a `useEffect` after mount — a `useState`/`useEffect` fetch can't preload, shows a loading state on every navigation, and bypasses the router's cache. Loaders run before the component mounts, so data is ready at first render.

```tsx
export const Route = createFileRoute('/posts')({
  loader: async () => {
    const posts = await fetchPosts()
    return { posts }
  },
  component: PostsPage,
})

function PostsPage() {
  const { posts } = Route.useLoaderData()
  return <PostList posts={posts} />
}
```

Access route params in the loader for parameterized fetches:

```tsx
export const Route = createFileRoute('/posts/$postId')({
  loader: async ({ params }) => {
    const post = await fetchPost(params.postId)
    const comments = await fetchComments(params.postId)
    return { post, comments }
  },
  component: PostDetailPage,
})
```

Declare search params in `loaderDeps` — the loader only re-runs when a declared dep changes:

```tsx
export const Route = createFileRoute('/posts/')({
  validateSearch: z.object({
    page: z.number().min(1).catch(1),
  }),
  loaderDeps: ({ search }) => ({ page: search.page }),
  loader: async ({ deps }) => fetchPosts(deps.page),
  component: PostsList,
})
```

Render `pendingComponent` for the loading state while the loader resolves: `pendingComponent: () => <div>Loading...</div>`.

## Use `ensureQueryData` with TanStack Query

When integrating TanStack Router with TanStack Query, call `queryClient.ensureQueryData()` in loaders, not `prefetchQuery()` — `prefetchQuery` returns void, never throws, and swallows errors, so failures escape your error boundaries. `ensureQueryData` respects the cache, awaits data when missing, and throws on error.

```tsx
import { queryOptions, useSuspenseQuery } from '@tanstack/react-query'

const postQueryOptions = (postId: string) =>
  queryOptions({
    queryKey: ['posts', postId],
    queryFn: () => fetchPost(postId),
    staleTime: 5 * 60 * 1000,
  })

export const Route = createFileRoute('/posts/$postId')({
  loader: async ({ params, context: { queryClient } }) => {
    await queryClient.ensureQueryData(postQueryOptions(params.postId))
  },
  component: PostPage,
})

function PostPage() {
  const { postId } = Route.useParams()
  const { data: post } = useSuspenseQuery(postQueryOptions(postId))
  return <PostContent post={post} />
}
```

Fetch independent queries in parallel with `Promise.all`:

```tsx
export const Route = createFileRoute('/dashboard')({
  loader: async ({ context: { queryClient } }) => {
    await Promise.all([
      queryClient.ensureQueryData(statsQueries.overview()),
      queryClient.ensureQueryData(activityQueries.recent()),
      queryClient.ensureQueryData(notificationQueries.unread()),
    ])
  },
})
```

Await sequentially only when one query depends on another's result:

```tsx
export const Route = createFileRoute('/users/$userId/posts')({
  loader: async ({ params, context: { queryClient } }) => {
    const user = await queryClient.ensureQueryData(userQueries.detail(params.userId))
    await queryClient.ensureQueryData(postQueries.byAuthor(user.id))
  },
})
```

| Method | Returns | Throws | Awaits | Use Case |
| --- | --- | --- | --- | --- |
| `ensureQueryData` | Data | Yes | Yes | Route loaders (recommended) |
| `prefetchQuery` | void | No | Yes | Background prefetching |
| `fetchQuery` | Data | Yes | Yes | When you need data immediately |

Wire the `queryClient` into router context and set `defaultPreloadStaleTime: 0` so TanStack Query owns cache freshness:

```tsx
const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 60 * 1000 },
  },
})

export const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreloadStaleTime: 0, // Let TanStack Query manage cache
  Wrap: ({ children }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  ),
})
```

Use the `queryOptions()` factory for type-safe, reusable definitions, and pair with `useSuspenseQuery` in components for guaranteed data.

## Leverage Parallel Route Loading

TanStack Router loads nested route data in parallel, not sequentially. Fetch independent data with `Promise.all` inside a loader rather than awaiting each call in turn — chained `await`s create an artificial waterfall (three 200ms calls become 600ms instead of 200ms).

```tsx
export const Route = createFileRoute('/dashboard')({
  beforeLoad: async () => {
    const [user, config] = await Promise.all([
      fetchUser(),
      fetchAppConfig(),
    ])
    return { user, config }
  },
  loader: async ({ context }) => {
    const [stats, activity, notifications] = await Promise.all([
      fetchDashboardStats(context.user.id),
      fetchRecentActivity(context.user.id),
      fetchNotifications(context.user.id),
    ])
    return { stats, activity, notifications }
  },
})
```

Parent and child route loaders also run in parallel automatically — navigating to `/posts/123` costs `max(parentLoader, childLoader)`, not their sum. The same nesting parallelism applies to `ensureQueryData` loaders.

```tsx
// routes/posts.tsx
export const Route = createFileRoute('/posts')({
  loader: async () => ({ categories: await fetchCategories() }),
})

// routes/posts/$postId.tsx — runs concurrently with the parent loader
export const Route = createFileRoute('/posts/$postId')({
  loader: async ({ params }) => {
    const post = await fetchPost(params.postId)
    const comments = await fetchComments(params.postId)
    return { post, comments }
  },
})
```

Stream non-critical data by starting a `prefetchQuery` without awaiting it, then reading it via `useQuery` in the component:

```tsx
export const Route = createFileRoute('/posts/$postId')({
  loader: async ({ params, context: { queryClient } }) => {
    const post = await queryClient.ensureQueryData(postQueries.detail(params.postId))
    // Non-critical - start but don't await
    queryClient.prefetchQuery(commentQueries.forPost(params.postId))
    return { post }
  },
  component: PostPage,
})

function PostPage() {
  const { post } = Route.useLoaderData()
  const { postId } = Route.useParams()
  const { data: comments, isLoading } = useQuery(commentQueries.forPost(postId))
  return (
    <article>
      <PostContent post={post} />
      {isLoading ? <CommentsSkeleton /> : <Comments data={comments} />}
    </article>
  )
}
```

- `beforeLoad` runs before `loader` (auth, redirects, context setup); nested route loaders run in parallel by default.
- Use `Promise.all` for parallel fetches within a single loader; prefetch non-critical data without awaiting for streaming UX.

## Loader Dependencies and Context

Loaders receive a single argument exposing `params`, `context`, `abortController`, `cause`, `deps`, and `preload` — use these instead of reaching for globals. Pass `abortController.signal` to `fetch` so stale requests cancel, and branch on `preload` to keep preloads cheap.

```tsx
export const Route = createFileRoute('/posts')({
  loader: async ({
    params,           // Route path parameters
    context,          // Route context (queryClient, auth, etc.)
    abortController,  // For cancelling stale requests
    cause,            // 'enter' | 'preload' | 'stay'
    deps,             // Dependencies from loaderDeps
    preload,          // Boolean: true if preloading
  }) => {
    const response = await fetch('/api/posts', {
      signal: abortController.signal,
    })
    if (preload) {
      return { posts: await response.json() }
    }
    const posts = await response.json()
    const stats = await fetchStats()
    return { posts, stats }
  },
})
```

- Loaders run during route matching, before component render; use `beforeLoad` for auth checks and redirects.
- If your loader depends on search params, specify them in `loaderDeps`.
