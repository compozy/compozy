# router-default-options: Configure Router Default Options

## Priority: HIGH

## Explanation

TanStack Router's `createRouter` accepts several default options that apply globally. Configure these for consistent behavior across your application including error handling, scroll restoration, and performance optimizations.

## Bad Example

```tsx
// Minimal router - missing useful defaults
const router = createRouter({
  routeTree,
  context: { queryClient },
});

// Each route must handle its own errors
// No scroll restoration on navigation
// No preloading configured
```

## Good Example: Full Configuration

```tsx
import { QueryClient } from "@tanstack/react-query";
import { createRouter } from "@tanstack/react-router";
import { setupRouterSsrQueryIntegration } from "@tanstack/react-router-ssr-query";
import { routeTree } from "./routeTree.gen";
import { DefaultCatchBoundary } from "@/components/DefaultCatchBoundary";
import { DefaultNotFound } from "@/components/DefaultNotFound";

export function getRouter() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false,
        staleTime: 1000 * 60 * 2,
      },
    },
  });

  const router = createRouter({
    routeTree,
    context: { queryClient, user: null },

    // Preloading
    defaultPreload: "intent", // Preload on hover/focus
    defaultPreloadStaleTime: 0, // Let Query manage freshness

    // Error handling
    defaultErrorComponent: DefaultCatchBoundary,
    defaultNotFoundComponent: DefaultNotFound,

    // UX
    scrollRestoration: true, // Restore scroll on back/forward

    // Performance
    defaultStructuralSharing: true, // Optimize re-renders
  });

  setupRouterSsrQueryIntegration({
    router,
    queryClient,
  });

  return router;
}
```

## Good Example: DefaultCatchBoundary Component

```tsx
// components/DefaultCatchBoundary.tsx
import { ErrorComponent, useRouter } from "@tanstack/react-router";

export function DefaultCatchBoundary({ error }: { error: Error }) {
  const router = useRouter();

  return (
    <div className="error-container">
      <h1>Something went wrong</h1>
      <ErrorComponent error={error} />
      <button onClick={() => router.invalidate()}>Try again</button>
    </div>
  );
}
```

## Good Example: DefaultNotFound Component

```tsx
// components/DefaultNotFound.tsx
import { Link } from "@tanstack/react-router";

export function DefaultNotFound() {
  return (
    <div className="not-found-container">
      <h1>404 - Page Not Found</h1>
      <p>The page you're looking for doesn't exist.</p>
      <Link to="/">Go home</Link>
    </div>
  );
}
```

## Router Options Reference

| Option                     | Type                                          | Default  | Description                              |
| -------------------------- | --------------------------------------------- | -------- | ---------------------------------------- |
| `defaultPreload`           | `false \| 'intent' \| 'render' \| 'viewport'` | `false`  | When to preload routes                   |
| `defaultPreloadStaleTime`  | `number`                                      | `30000`  | How long preloaded data stays fresh (ms) |
| `defaultErrorComponent`    | `Component`                                   | Built-in | Global error boundary                    |
| `defaultNotFoundComponent` | `Component`                                   | Built-in | Global 404 page                          |
| `scrollRestoration`        | `boolean`                                     | `false`  | Restore scroll on navigation             |
| `defaultStructuralSharing` | `boolean`                                     | `true`   | Optimize loader data re-renders          |

## Good Example: Route-Level Overrides

```tsx
// Routes can override defaults
export const Route = createFileRoute("/admin")({
  // Custom error handling for admin section
  errorComponent: AdminErrorBoundary,
  notFoundComponent: AdminNotFound,

  // Disable preload for sensitive routes
  preload: false,
});
```

## Good Example: With Pending Component

```tsx
const router = createRouter({
  routeTree,
  context: { queryClient },

  defaultPreload: "intent",
  defaultPreloadStaleTime: 0,
  defaultErrorComponent: DefaultCatchBoundary,
  defaultNotFoundComponent: DefaultNotFound,
  scrollRestoration: true,

  // Show during route transitions
  defaultPendingComponent: () => <div className="loading-bar" />,
  defaultPendingMinMs: 200, // Min time to show pending UI
  defaultPendingMs: 1000, // Delay before showing pending UI
});
```

## Context

- Set `defaultPreloadStaleTime: 0` when using TanStack Query
- `scrollRestoration: true` improves back/forward navigation UX
- `defaultStructuralSharing` prevents unnecessary re-renders
- Route-level options override router defaults
- Error/NotFound components receive route context
- Pending components help with perceived performance
