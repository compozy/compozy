# TanStack Router Patterns and Best Practices

This document contains comprehensive patterns and best practices for using TanStack Router in this project.

## Core Principles

- **Type-Safe Routing**: Embrace type-safe routing as the primary benefit.
- **File-Based Routes**: Use file-based routing for scalability.
- **Generated Route Tree**: Leverage the generated route tree for type safety.
- **Layouts and Nesting**: Think in terms of layouts and nested routes.

## File-Based Routing Structure

```
src/routes/
├── __root.tsx          # Root layout with providers
├── _authenticated.tsx  # Auth layout wrapper
├── index.tsx          # Home page (/)
├── about.tsx          # /about
├── posts/
│   ├── index.tsx      # /posts
│   └── $postId.tsx    # /posts/:postId (typed params)
└── settings/
    ├── _layout.tsx    # Settings layout
    ├── index.tsx      # /settings
    └── profile.tsx    # /settings/profile
```

### File Naming Conventions

- `index.tsx` - Index route for a directory
- `$paramName.tsx` - Dynamic route parameter
- `_layoutName.tsx` - Layout route (underscore prefix)
- `_authenticated.tsx` - Protected route layout
- `__root.tsx` - Root layout (double underscore)
- `route.lazy.tsx` - Lazy-loaded route component

## Common Tasks

### Creating a Basic Route

Use `createFileRoute` with the route path matching the file location.

```typescript
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/posts/')({
  component: PostsList,
})

function PostsList() {
  return <div>Posts List</div>
}
```

### Route with Typed Parameters

Use `$paramName` in the file path for dynamic segments and access them via `Route.useParams()`.

```typescript
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

### Search Params with Zod Validation

Use `validateSearch` with a Zod schema for type-safe URL search parameters.

```typescript
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

const searchSchema = z.object({
  page: z.number().min(1).catch(1),
  search: z.string().optional(),
  sort: z.enum(['asc', 'desc']).catch('asc'),
})

export const Route = createFileRoute('/posts/')({
  validateSearch: searchSchema,
  component: PostsList,
})

function PostsList() {
  const { page, search, sort } = Route.useSearch()
  // Use search params...
}
```

### Data Loading with Loaders

Use the `loader` option for server-side data fetching and `loaderDeps` to control re-fetching.

```typescript
export const Route = createFileRoute('/posts/')({
  validateSearch: z.object({
    page: z.number().min(1).catch(1),
  }),
  loaderDeps: ({ search }) => ({ page: search.page }),
  loader: async ({ deps }) => {
    return await fetchPosts(deps.page)
  },
  component: PostsList,
})

function PostsList() {
  const posts = Route.useLoaderData()
  // Render posts...
}
```

### Authentication Layout Route

Use layout routes with `beforeLoad` for authentication checks.

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

Use `Link` component for declarative navigation and `useNavigate` for programmatic navigation.

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

### Error Handling

Use `errorComponent` and `notFoundComponent` for route-level error boundaries.

```typescript
export const Route = createFileRoute('/posts/$postId')({
  loader: async ({ params }) => {
    const post = await fetchPost(params.postId)
    if (!post) throw new Error('Post not found')
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

### Code Splitting with Lazy Routes

Use `createLazyFileRoute` for route-based code splitting.

```typescript
// routes/admin.lazy.tsx
import { createLazyFileRoute } from '@tanstack/react-router'

export const Route = createLazyFileRoute('/admin')({
  component: () => import('@/components/AdminDashboard'),
})
```

### Pending Component

Show loading state while data is being fetched.

```typescript
export const Route = createFileRoute('/posts/')({
  loader: async () => {
    return await fetchPosts()
  },
  pendingComponent: () => <div>Loading...</div>,
  component: PostsList,
})
```

## Advanced Patterns

### Context and Route Context

Pass context data through the route tree.

```typescript
// __root.tsx
import { createRootRouteWithContext } from '@tanstack/react-router'
import type { QueryClient } from '@tanstack/react-query'

interface RouterContext {
  queryClient: QueryClient
  auth: AuthState
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootComponent,
})
```

### Using Route Context in Child Routes

```typescript
export const Route = createFileRoute('/posts/')({
  beforeLoad: ({ context }) => {
    // Access context.queryClient, context.auth, etc.
    if (!context.auth.isAuthenticated) {
      throw redirect({ to: '/login' })
    }
  },
  loader: ({ context }) => {
    return context.queryClient.ensureQueryData({
      queryKey: ['posts'],
      queryFn: fetchPosts,
    })
  },
})
```

### Updating Search Params

Use `navigate` or `Link` to update search params while preserving current state.

```typescript
function PostsFilter() {
  const navigate = useNavigate()
  const { page, sort } = Route.useSearch()

  const handleSortChange = (newSort: 'asc' | 'desc') => {
    navigate({
      search: (prev) => ({ ...prev, sort: newSort }),
    })
  }

  return (
    <Link
      to="."
      search={(prev) => ({ ...prev, page: page + 1 })}
    >
      Next Page
    </Link>
  )
}
```

### Route Masks

Hide implementation details in the URL while maintaining full functionality.

```typescript
export const Route = createFileRoute('/posts/$postId')({
  validateSearch: z.object({
    modal: z.boolean().optional(),
  }),
})

// Navigate with mask
navigate({
  to: '/posts/$postId',
  params: { postId: '123' },
  search: { modal: true },
  mask: { to: '/posts' }, // URL shows /posts but component receives full params
})
```

### Prefetching Routes

Preload route data for faster navigation.

```typescript
import { usePrefetch } from '@tanstack/react-router'

function PostLink({ postId }: { postId: string }) {
  const prefetch = usePrefetch()

  return (
    <Link
      to="/posts/$postId"
      params={{ postId }}
      onMouseEnter={() => prefetch({ to: '/posts/$postId', params: { postId } })}
    >
      View Post
    </Link>
  )
}
```

## Integration with TanStack Query

For optimal data fetching, integrate TanStack Router with TanStack Query:

```typescript
export const Route = createFileRoute('/posts/$postId')({
  loader: async ({ params, context }) => {
    // Use queryClient from context
    await context.queryClient.ensureQueryData({
      queryKey: ['post', params.postId],
      queryFn: () => fetchPost(params.postId),
    })
  },
  component: PostDetail,
})

function PostDetail() {
  const { postId } = Route.useParams()
  const { data: post } = useQuery({
    queryKey: ['post', postId],
    queryFn: () => fetchPost(postId),
  })
  // Render post...
}
```

## Validation Checklist

Before finishing a task involving TanStack Router:

- [ ] Route path in `createFileRoute` matches file location.
- [ ] Search params use Zod validation with proper defaults (`.catch()`).
- [ ] Loader dependencies are correctly specified in `loaderDeps`.
- [ ] Authentication routes use `beforeLoad` with proper redirects.
- [ ] Navigation uses typed `Link` or `useNavigate` hooks.
- [ ] Error boundaries are implemented at route level.
- [ ] Lazy loading is used for large route components.
- [ ] Route context is properly typed and passed.
- [ ] Run type checks (`pnpm run typecheck`) and tests (`pnpm run test`).

## Common Mistakes to Avoid

1. **Incorrect route path**: The path in `createFileRoute` must match the file location exactly.
2. **Missing `.catch()` on search params**: Always provide defaults with `.catch()` for optional params.
3. **Not using `loaderDeps`**: If your loader depends on search params, specify them in `loaderDeps`.
4. **Throwing redirects incorrectly**: Use `throw redirect({...})` not `return redirect({...})`.
5. **Forgetting to type route context**: Always type your root route context for type safety.
