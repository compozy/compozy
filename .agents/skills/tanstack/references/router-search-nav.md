# Router Search Params and Navigation

## Validate Search Params

Search params come from the URL — user-controlled input. Parse, coerce, and default them with `validateSearch` instead of reading `window.location.search`/`URLSearchParams` manually (which yields `NaN` pages and unchecked `as` casts).

```tsx
export const Route = createFileRoute('/products')({
  validateSearch: (search: Record<string, unknown>) => ({
    page: Number(search.page) || 1,
    sort: search.sort === 'desc' ? 'desc' : 'asc',
    category: typeof search.category === 'string' ? search.category : undefined,
  }),
  component: ProductsPage,
})

function ProductsPage() {
  const { page, sort, category } = Route.useSearch()
}
```

With Zod, always provide defaults via `.catch()` for optional params — missing `.catch()` is a common mistake that throws on bad input:

```tsx
import { z } from 'zod'

const productSearchSchema = z.object({
  page: z.number().min(1).catch(1),
  limit: z.number().min(1).max(100).catch(20),
  sort: z.enum(['name', 'price', 'date']).catch('name'),
  order: z.enum(['asc', 'desc']).catch('asc'),
  category: z.string().optional(),
  search: z.string().optional(),
})

export const Route = createFileRoute('/products')({
  validateSearch: search => productSearchSchema.parse(search),
  component: ProductsPage,
})
```

Update params with a `search` updater function to preserve the others:

```tsx
navigate({ to: '.', search: prev => ({ ...prev, ...newFilters, page: 1 }) })
```

Search params are inherited by child routes. Validation runs on every navigation — keep it fast.

## Configure Custom Search Param Serializers

TanStack Router serializes search params as JSON by default (`?data=%7B...%7D`, ugly URLs). For cleaner URLs or external-API compatibility, provide a global custom serializer via `search.serialize`/`parse`.

```tsx
import { createRouter } from '@tanstack/react-router'
import queryString from 'query-string'

const router = createRouter({
  routeTree,
  search: {
    serialize: search =>
      queryString.stringify(search, { arrayFormat: 'bracket', skipNull: true }),
    parse: searchString =>
      queryString.parse(searchString, {
        arrayFormat: 'bracket',
        parseBooleans: true,
        parseNumbers: true,
      }),
  },
})
// URL: /products?category=electronics&inStock=true&tags[]=sale
```

For deep nesting, swap in `qs` (`serialize: qs.stringify(search, { encodeValuesOnly: true, arrayFormat: 'brackets' })`, `parse: qs.parse(searchString, { ignoreQueryPrefix: true })`) → `?filters[category]=electronics&filters[price][min]=100`.

| Library | URL Style | Best For |
| --- | --- | --- |
| Default (JSON) | `?data=%7B...%7D` | TypeScript safety |
| jsurl2 | `?~(key~'value)` | Compact, readable |
| query-string | `?key=value&arr[]=1` | Traditional APIs |
| qs | `?obj[nested]=value` | Deep nesting |
| Base64 | `?eyJrZXkiOiJ2YWx1ZSJ9` | Opaque, compact |

Serializers apply globally to all routes; route-level `validateSearch` still runs after parsing. Watch URL length limits (~2000 chars for safe cross-browser).

## Prefer Link Component for Navigation

Use `<Link>` for navigation instead of `onClick={() => navigate(...)}` on a div — Links render real `<a>` tags with valid `href`, so right-click/open-in-new-tab, SEO, and screen-reader announcement all work.

```tsx
import { Link } from '@tanstack/react-router'

function PostCard({ post }: { post: Post }) {
  return (
    <Link to="/posts/$postId" params={{ postId: post.id }} className="post-card">
      <h2>{post.title}</h2>
    </Link>
  )
}
```

Drive active styling from `activeProps`/`inactiveProps`/`activeOptions` rather than manual pathname checks:

```tsx
function NavLink({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <Link
      to={to}
      activeProps={{ className: 'nav-link-active', 'aria-current': 'page' }}
      inactiveProps={{ className: 'nav-link' }}
      activeOptions={{ exact: true }}
    >
      {children}
    </Link>
  )
}
```

Reserve `useNavigate` for programmatic navigation with no clickable element — after form submit, after auth, or conditional redirects:

```tsx
// After mutation
onSuccess: data => navigate({ to: '/posts/$postId', params: { postId: data.id } })

// Guarded redirect
if (!isAuthenticated) {
  navigate({ to: '/login', search: { redirect: location.pathname } })
}
```

In loaders/`beforeLoad`, use `throw redirect({...})`, not `return redirect({...})`.

## Use Route Masks for Modal URLs

Route masks display one URL while internally routing to another — use them for modals/sheets/overlays instead of local `useState`, which leaves the URL unchanged (no shareable link, back button won't close).

```tsx
// routes/posts.tsx — link masks the detail route as the list URL
<Link
  to="/posts/$postId"
  params={{ postId: post.id }}
  mask={{ to: '/posts' }}
>
  {post.title}
</Link>

// routes/posts/$postId.tsx — the masked route renders as a modal
function PostModal() {
  const { postId } = Route.useParams()
  const navigate = useNavigate()
  return (
    <Modal onClose={() => navigate({ to: '/posts' })}>
      <PostDetail postId={postId} />
    </Modal>
  )
}
```

Masks accept search too: `mask={{ to: '/posts', search: { modal: post.id } }}`.

| Scenario | URL Shown | Actual Route |
| --- | --- | --- |
| Click masked link | Masked URL | Real route |
| Share/copy URL | Real URL | Real route |
| Direct navigation | Real URL | Real route |
| Browser refresh | Depends on URL in bar | Matches URL |

Masks are client-side only — a shared/copied URL and direct navigation resolve to the real route (full page), bypassing the mask.

## Enable Intent-Based Preloading

Set `defaultPreload: 'intent'` on the router to preload routes on hover/focus; without it users wait after every navigation. Preloading loads route code AND executes loaders.

```tsx
const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
  defaultPreloadDelay: 50,
  defaultPreloadStaleTime: 0, // Let TanStack Query manage cache
  context: { queryClient },
})

declare module '@tanstack/react-router' {
  interface Register { router: typeof router }
}
```

| Strategy | Behavior | Use Case |
| --- | --- | --- |
| `'intent'` | Preload on hover/focus | Default for most links |
| `'render'` | Preload when Link mounts | Critical next pages |
| `'viewport'` | Preload when Link enters viewport | Below-fold content |
| `false` | No preloading | Heavy, rarely-visited pages |

Override per link with `preload`/`preloadDelay` (e.g. `<Link to="/heavy-page" preload={false}>`), or prefetch manually via `usePrefetch`:

```tsx
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

`preloadDelay` prevents excessive requests on quick mouse movements. On mobile, prefer `'viewport'` since hover isn't available.
