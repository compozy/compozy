# Router Code Splitting, Errors, and Context

## Use `.lazy.tsx` for Code Splitting

Split route components into `.lazy.tsx` files to shrink the initial bundle, rather than bundling heavy component code (chart libraries, data grids) into the main route file. The main route file keeps critical config (validateSearch, beforeLoad, loader, context); the lazy file holds the component plus its pending/error/notFound components, which load on-demand.

```tsx
// routes/dashboard.tsx - only critical config
export const Route = createFileRoute('/dashboard')({
  loader: async ({ context }) => {
    return context.queryClient.ensureQueryData(dashboardQueries.stats())
  },
})

// routes/dashboard.lazy.tsx
import { createLazyFileRoute } from '@tanstack/react-router'

export const Route = createLazyFileRoute('/dashboard')({
  component: DashboardPage,
  pendingComponent: DashboardSkeleton,
  errorComponent: DashboardError,
})

function DashboardPage() {
  const data = Route.useLoaderData()
  return (
    <div>
      <HeavyChartLibrary data={data} />
      <ComplexDataGrid />
    </div>
  )
}
```

| Main route file (`example.tsx`) | Lazy file (`example.lazy.tsx`) |
| --- | --- |
| validateSearch | component |
| beforeLoad (auth, redirects) | pendingComponent |
| loader / loaderDeps | errorComponent |
| context manipulation | notFoundComponent |

In a lazy component that isn't the route's own file, reach params/loader data via `getRouteApi`:

```tsx
// routes/posts/$postId.lazy.tsx
import { createLazyFileRoute, getRouteApi } from '@tanstack/react-router'

const route = getRouteApi('/posts/$postId')

export const Route = createLazyFileRoute('/posts/$postId')({
  component: PostPage,
})

function PostPage() {
  const { postId } = route.useParams()
  const data = route.useLoaderData()
  return <article>{/* ... */}</article>
}
```

Enable `autoCodeSplitting` in the Vite plugin to split without hand-authoring `.lazy.tsx` files:

```tsx
// vite.config.ts
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'

export default defineConfig({
  plugins: [
    TanStackRouterVite({ autoCodeSplitting: true }),
    react(),
  ],
})
```

- Loaders are NOT lazy — they run before rendering.
- `createLazyFileRoute` only accepts component-related options.
- Virtual routes auto-generate when only `.lazy.tsx` exists.

## Handle Not-Found Routes Properly

Configure `notFoundComponent` and throw `notFound()` for missing resources, instead of throwing a generic `Error` or omitting `defaultNotFoundComponent`. Handling works at multiple levels — root, route-specific, and programmatic via `notFound()` — and bubbles up the route tree when unhandled.

Root-level fallback: `notFoundComponent` on the root route plus `defaultNotFoundComponent` on the router.

```tsx
// routes/__root.tsx
export const Route = createRootRoute({
  component: RootComponent,
  notFoundComponent: GlobalNotFound,
})

// router.tsx
const router = createRouter({
  routeTree,
  defaultNotFoundComponent: DefaultNotFound,
})
```

Route-specific: throw `notFound()` in the loader and render a route-local `notFoundComponent`.

```tsx
import { createFileRoute, notFound } from '@tanstack/react-router'

export const Route = createFileRoute('/posts/$postId')({
  loader: async ({ params }) => {
    const post = await fetchPost(params.postId)
    if (!post) throw notFound()
    return post
  },
  notFoundComponent: PostNotFound,
  component: PostPage,
})

function PostNotFound() {
  const { postId } = Route.useParams()
  return <p>No post exists with ID: {postId}</p>
}
```

Attach `data` to `notFound()` to render context in the not-found component:

```tsx
if (!user) {
  throw notFound({
    data: {
      username: params.username,
      suggestions: await fetchSimilarUsernames(params.username),
    },
  })
}
```

`errorComponent` is separate from not-found and handles thrown errors with a `retry`:

```tsx
export const Route = createFileRoute('/posts/$postId')({
  loader: async ({ params }) => {
    const post = await fetchPost(params.postId)
    if (!post) throw notFound()
    return post
  },
  errorComponent: ({ error, retry }) => (
    <div>
      <p>Error: {error.message}</p>
      <button onClick={retry}>Retry</button>
    </div>
  ),
  notFoundComponent: () => <div>Post not found</div>,
})
```

Catch-all route (`routes/$.tsx`) matches any unmatched path via the `_splat` param:

```tsx
// routes/$.tsx
export const Route = createFileRoute('/$')({
  component: CatchAllNotFound,
})

function CatchAllNotFound() {
  const { _splat } = Route.useParams()
  return <p>No page exists at: /{_splat}</p>
}
```

A child route with no `notFoundComponent` bubbles its `notFound()` to the nearest parent that defines one:

```tsx
// routes/posts.tsx - catches child 404s too
export const Route = createFileRoute('/posts')({
  notFoundComponent: PostsNotFound,
})

// routes/posts/$postId.tsx - no notFoundComponent, bubbles to parent
export const Route = createFileRoute('/posts/$postId')({
  loader: async ({ params }) => {
    const post = await fetchPost(params.postId)
    if (!post) throw notFound()
    return post
  },
})
```

- `notFound()` throws a special error caught by the nearest `notFoundComponent` — distinct from error boundaries, which handle general errors.

## Define Context at Root Route

Use `createRootRouteWithContext` to define typed context (query client, auth, services) that flows through the whole route tree, instead of importing global singletons into each route. Context is available in `beforeLoad`, `loader`, and components, and makes dependencies injectable in tests.

```tsx
// routes/__root.tsx
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import { QueryClient } from '@tanstack/react-query'

interface RouterContext {
  queryClient: QueryClient
  auth: {
    user: User | null
    isAuthenticated: boolean
  }
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <>
      <Header />
      <main><Outlet /></main>
      <Footer />
    </>
  ),
})

// router.tsx
export function getRouter(auth: RouterContext['auth'] = { user: null, isAuthenticated: false }) {
  const queryClient = new QueryClient()
  return createRouter({
    routeTree,
    context: { queryClient, auth },
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
    scrollRestoration: true,
  })
}

// routes/posts.tsx
export const Route = createFileRoute('/posts')({
  loader: async ({ context: { queryClient } }) => {
    return queryClient.ensureQueryData(postQueries.list())
  },
})
```

Extend context for a subtree by returning values from `beforeLoad`:

```tsx
export const Route = createFileRoute('/posts/$postId')({
  beforeLoad: async ({ context, params }) => {
    const post = await fetchPost(params.postId)
    return { post } // available to this route and children
  },
  loader: async ({ context }) => {
    const comments = await fetchComments(context.post.id)
    return { comments }
  },
})
```

| Context | Loader Data |
| --- | --- |
| Available in beforeLoad, loader, and component | Only available in component |
| Set at router creation or in beforeLoad | Returned from loader |
| Good for services, clients, auth | Good for route-specific data |
| Flows down to all children | Specific to route |

- Type the interface in `createRootRouteWithContext<T>()` to keep context type-safe and avoid global imports.

## Authentication with `beforeLoad`

Guard authenticated areas with a layout route whose `beforeLoad` throws `redirect()` when the user isn't authenticated — `throw redirect({...})`, never `return`. Child routes then read `context.auth` freely.

```tsx
// routes/_authenticated.tsx
import { createFileRoute, redirect, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location, context }) => {
    if (!context.auth.isAuthenticated) {
      throw redirect({
        to: '/login',
        search: { redirect: location.href },
      })
    }
  },
  component: () => <Outlet />,
})

// routes/_authenticated/dashboard.tsx
export const Route = createFileRoute('/_authenticated/dashboard')({
  loader: async ({ context: { queryClient, auth } }) => {
    return queryClient.ensureQueryData(dashboardQueries.forUser(auth.user!.id))
  },
})
```

- Use `throw redirect({...})`, not `return redirect({...})`.
- Pass `redirect: location.href` in search params for post-login return navigation.
- An individual route can run the same `beforeLoad` auth check inline instead of nesting under a layout route.
