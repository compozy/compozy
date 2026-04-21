# preload-intent: Enable Intent-Based Preloading

## Priority: MEDIUM

## Explanation

Configure `defaultPreload: 'intent'` to preload routes when users hover or focus links. This loads data before the click, making navigation feel instant.

## Bad Example

```tsx
// No preloading configured - data loads after click
const router = createRouter({
  routeTree,
  // No defaultPreload - user waits after every navigation
});

// Each navigation shows loading state
function PostList({ posts }: { posts: Post[] }) {
  return (
    <ul>
      {posts.map(post => (
        <li key={post.id}>
          <Link to="/posts/$postId" params={{ postId: post.id }}>
            {post.title}
          </Link>
          {/* Click → wait for data → render */}
        </li>
      ))}
    </ul>
  );
}
```

## Good Example

```tsx
// router.tsx - Enable preloading by default
const router = createRouter({
  routeTree,
  defaultPreload: "intent", // Preload on hover/focus
  defaultPreloadDelay: 50, // Wait 50ms before starting
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

// Links automatically preload on hover
function PostList({ posts }: { posts: Post[] }) {
  return (
    <ul>
      {posts.map(post => (
        <li key={post.id}>
          <Link to="/posts/$postId" params={{ postId: post.id }}>
            {post.title}
          </Link>
          {/* Hover → preload starts → click → instant navigation */}
        </li>
      ))}
    </ul>
  );
}
```

## Preload Options

```tsx
// Router-level defaults
const router = createRouter({
  routeTree,
  defaultPreload: 'intent',       // 'intent' | 'render' | 'viewport' | false
  defaultPreloadDelay: 50,        // ms before preload starts
  defaultPreloadStaleTime: 30000, // 30s - how long preloaded data stays fresh
})

// Link-level overrides
<Link
  to="/heavy-page"
  preload={false}  // Disable for this specific link
>
  Heavy Page
</Link>

<Link
  to="/critical-page"
  preload="render"  // Preload immediately when Link renders
>
  Critical Page
</Link>
```

## Preload Strategies

| Strategy     | Behavior                          | Use Case                    |
| ------------ | --------------------------------- | --------------------------- |
| `'intent'`   | Preload on hover/focus            | Default for most links      |
| `'render'`   | Preload when Link mounts          | Critical next pages         |
| `'viewport'` | Preload when Link enters viewport | Below-fold content          |
| `false`      | No preloading                     | Heavy, rarely-visited pages |

## Good Example: With TanStack Query Integration

```tsx
// When using TanStack Query, disable router cache
const router = createRouter({
  routeTree,
  defaultPreload: "intent",
  defaultPreloadStaleTime: 0, // Let TanStack Query manage cache
  context: {
    queryClient,
  },
});

// Route loader uses TanStack Query
export const Route = createFileRoute("/posts/$postId")({
  loader: async ({ params, context: { queryClient } }) => {
    // ensureQueryData respects TanStack Query's staleTime
    await queryClient.ensureQueryData(postQueries.detail(params.postId));
  },
});
```

## Context

- Preloading loads route code AND executes loaders
- `preloadDelay` prevents excessive requests on quick mouse movements
- Preloaded data is garbage collected after `preloadStaleTime`
- Works with both router caching and external caching (TanStack Query)
- Mobile: Consider `'viewport'` since hover isn't available
- Monitor network tab to verify preloading works correctly
