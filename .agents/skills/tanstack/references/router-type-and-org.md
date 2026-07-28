# Router Type Safety and Organization

## File-Based Routing Structure

Route files map 1:1 to URL paths — the string in `createFileRoute('/posts/$postId')` must match the file location exactly, or the generated route tree breaks.

```
src/routes/
├── __root.tsx          # Root layout with providers
├── _authenticated.tsx  # Auth layout wrapper
├── index.tsx           # Home page (/)
├── about.tsx           # /about
├── posts/
│   ├── index.tsx       # /posts
│   └── $postId.tsx     # /posts/:postId (typed params)
└── settings/
    ├── _layout.tsx     # Settings layout
    ├── index.tsx       # /settings
    └── profile.tsx     # /settings/profile
```

| Pattern | Purpose |
| --- | --- |
| `index.tsx` | Index route for a directory |
| `$paramName.tsx` | Dynamic route parameter |
| `_layoutName.tsx` | Layout route (underscore prefix) |
| `_authenticated.tsx` | Protected route layout |
| `__root.tsx` | Root layout (double underscore) |
| `route.lazy.tsx` / `*.lazy.tsx` | Lazy-loaded route component |

```tsx
// routes/posts/$postId.tsx
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/posts/$postId')({
  component: PostDetail,
})

function PostDetail() {
  const { postId } = Route.useParams()
  return <div>Post: {postId}</div>
}
```

## Register Router Type for Global Inference

Register your router instance via TypeScript module declaration so hooks (`useNavigate`, `useParams`, `useSearch`, `Link`) infer your route tree; skip it and every route path is an untyped string with no autocomplete and no error on invalid routes. Do this once in your router config file — works for both file-based and code-based routing.

```tsx
// router.tsx
import { createRouter } from '@tanstack/react-router'
import { routeTree } from './routeTree.gen'

export const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

// Now fully typed across the app:
const navigate = useNavigate()
navigate({ to: '/posts/$postId', params: { postId: '123' } })
navigate({ to: '/invalid-route' }) // Error: not assignable to known routes
```

## Use `from` Parameter for Type Narrowing

Pass `from` to `useParams` / `useSearch` / `useLoaderData` to get exact types for one route; omit it and TypeScript widens to a union of every route's params (each field `string | undefined`). The `from` path must match exactly, params included (`$postId`).

```tsx
function PostDetail() {
  const params = useParams({ from: '/posts/$postId' })
  // params: { postId: string } — guaranteed, not string | undefined
  const { post, comments } = useLoaderData({ from: '/posts/$postId' })
  const search = useSearch({ from: '/posts' })
}
```

Within the route's own file, prefer `Route.fullPath` or the route helpers:

```tsx
// routes/posts/$postId.tsx
export const Route = createFileRoute('/posts/$postId')({
  loader: async ({ params }) => ({ post: await fetchPost(params.postId) }),
  component: PostComponent,
})

function PostComponent() {
  const { postId } = Route.useParams()               // in-file helper
  const data = Route.useLoaderData()
  const params = useParams({ from: Route.fullPath })  // equivalent via fullPath
}
```

For components code-split out of the route file, bind an API with `getRouteApi`:

```tsx
import { getRouteApi } from '@tanstack/react-router'

const postRoute = getRouteApi('/posts/$postId')

export function PostDetail() {
  const params = postRoute.useParams()   // { postId: string }
  const data = postRoute.useLoaderData()
  const search = postRoute.useSearch()
}
```

For truly generic cross-route components, use `strict: false` to read whatever params are present:

```tsx
function Breadcrumbs() {
  const params = useParams({ strict: false })
  return (
    <nav>
      {params.userId && <span>User: {params.userId}</span>}
      {params.postId && <span>Post: {params.postId}</span>}
    </nav>
  )
}
```

## Virtual File Routes

When a `.lazy.tsx` file has no matching main route file, TanStack Router auto-generates a virtual route in `routeTree.gen.ts` to anchor it — so don't hand-write an empty boilerplate main route file just to host a lazy component. Only works with file-based routing.

```tsx
// routes/settings.lazy.tsx — the only file needed; no routes/settings.tsx
export const Route = createLazyFileRoute('/settings')({
  component: SettingsPage,
  pendingComponent: SettingsLoading,
  errorComponent: SettingsError,
})
```

You still need a main route file for any critical-path config (loader, beforeLoad, validateSearch, loaderDeps, context) — the lazy file then carries only the component:

```tsx
// routes/dashboard.tsx — main file for beforeLoad/loader
export const Route = createFileRoute('/dashboard')({
  beforeLoad: async ({ context }) => {
    if (!context.auth.isAuthenticated) throw redirect({ to: '/login' })
  },
  loader: async ({ context: { queryClient } }) => {
    await queryClient.ensureQueryData(dashboardQueries.stats())
  },
})

// routes/dashboard.lazy.tsx
export const Route = createLazyFileRoute('/dashboard')({
  component: DashboardPage,
  pendingComponent: DashboardSkeleton,
})
```

| Route Has... | Need Main File? | Use Virtual? |
| --- | --- | --- |
| Only component | No | Yes |
| loader | Yes | No |
| beforeLoad | Yes | No |
| validateSearch | Yes | No |
| loaderDeps | Yes | No |
| Just pendingComponent/errorComponent | No | Yes |

```
routes/
├── __root.tsx
├── index.tsx               # Has loader
├── about.lazy.tsx          # Virtual route (no main file)
├── dashboard.tsx           # Has beforeLoad (auth)
├── dashboard.lazy.tsx
├── posts.tsx               # Has loader
├── posts.lazy.tsx
├── posts/
│   ├── $postId.tsx         # Has loader
│   └── $postId.lazy.tsx
└── settings/
    ├── index.lazy.tsx      # Virtual route
    └── security.tsx        # Has beforeLoad
```

## Configure Router Default Options

Set global defaults on `createRouter` so behavior is consistent instead of every route re-handling its own errors, preloading, and scroll; route-level options override the defaults. Set `defaultPreloadStaleTime: 0` when using TanStack Query so Query owns freshness.

```tsx
import { QueryClient } from '@tanstack/react-query'
import { createRouter } from '@tanstack/react-router'
import { routeTree } from './routeTree.gen'
import { DefaultCatchBoundary } from '@/components/DefaultCatchBoundary'
import { DefaultNotFound } from '@/components/DefaultNotFound'

export function getRouter() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: 1000 * 60 * 2 } },
  })

  return createRouter({
    routeTree,
    context: { queryClient, user: null },

    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0, // let Query manage freshness

    defaultErrorComponent: DefaultCatchBoundary,
    defaultNotFoundComponent: DefaultNotFound,

    scrollRestoration: true,
    defaultStructuralSharing: true,

    defaultPendingComponent: () => <div className="loading-bar" />,
    defaultPendingMinMs: 200,
    defaultPendingMs: 1000,
  })
}
```

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `defaultPreload` | `false \| 'intent' \| 'render' \| 'viewport'` | `false` | When to preload routes |
| `defaultPreloadStaleTime` | `number` | `30000` | How long preloaded data stays fresh (ms) |
| `defaultErrorComponent` | `Component` | Built-in | Global error boundary |
| `defaultNotFoundComponent` | `Component` | Built-in | Global 404 page |
| `scrollRestoration` | `boolean` | `false` | Restore scroll on navigation |
| `defaultStructuralSharing` | `boolean` | `true` | Optimize loader data re-renders |

A default error component wires `ErrorComponent` plus `router.invalidate()` to retry:

```tsx
import { ErrorComponent, useRouter } from '@tanstack/react-router'

export function DefaultCatchBoundary({ error }: { error: Error }) {
  const router = useRouter()
  return (
    <div>
      <ErrorComponent error={error} />
      <button onClick={() => router.invalidate()}>Try again</button>
    </div>
  )
}
```

Route-level overrides win over router defaults:

```tsx
export const Route = createFileRoute('/admin')({
  errorComponent: AdminErrorBoundary,
  notFoundComponent: AdminNotFound,
  preload: false, // disable preload for sensitive routes
})
```
