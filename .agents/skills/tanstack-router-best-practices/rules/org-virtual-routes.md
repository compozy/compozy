# org-virtual-routes: Understand Virtual File Routes

## Priority: LOW

## Explanation

Virtual routes are automatically generated placeholder routes in the route tree when you have a `.lazy.tsx` file without a corresponding main route file. They provide the minimal configuration needed to anchor lazy-loaded components.

## Bad Example

```tsx
// Creating unnecessary boilerplate main route files
// routes/settings.tsx - Just to have a file
export const Route = createFileRoute("/settings")({
  // Empty - no loader, no beforeLoad, nothing
});

// routes/settings.lazy.tsx - Actual component
export const Route = createLazyFileRoute("/settings")({
  component: SettingsPage,
});

// The main file is unnecessary boilerplate
```

## Good Example: Let Virtual Routes Handle It

```tsx
// Delete routes/settings.tsx entirely!

// routes/settings.lazy.tsx - Only file needed
export const Route = createLazyFileRoute("/settings")({
  component: SettingsPage,
  pendingComponent: SettingsLoading,
  errorComponent: SettingsError,
});

function SettingsPage() {
  return <div>Settings Content</div>;
}

// TanStack Router auto-generates a virtual route:
// {
//   path: '/settings',
//   // Minimal config to anchor the lazy file
// }
```

## Good Example: When You DO Need Main Route File

```tsx
// routes/dashboard.tsx - Need this for loader/beforeLoad
export const Route = createFileRoute("/dashboard")({
  beforeLoad: async ({ context }) => {
    if (!context.auth.isAuthenticated) {
      throw redirect({ to: "/login" });
    }
  },
  loader: async ({ context: { queryClient } }) => {
    await queryClient.ensureQueryData(dashboardQueries.stats());
  },
  // Component is in lazy file
});

// routes/dashboard.lazy.tsx
export const Route = createLazyFileRoute("/dashboard")({
  component: DashboardPage,
  pendingComponent: DashboardSkeleton,
});

// Main file IS needed here because we have loader/beforeLoad
```

## Decision Guide

| Route Has...                         | Need Main File? | Use Virtual? |
| ------------------------------------ | --------------- | ------------ |
| Only component                       | No              | Yes          |
| loader                               | Yes             | No           |
| beforeLoad                           | Yes             | No           |
| validateSearch                       | Yes             | No           |
| loaderDeps                           | Yes             | No           |
| Just pendingComponent/errorComponent | No              | Yes          |

## Good Example: File Structure with Virtual Routes

```
routes/
├── __root.tsx              # Always needed
├── index.tsx               # Has loader
├── about.lazy.tsx          # Virtual route (no main file)
├── contact.lazy.tsx        # Virtual route (no main file)
├── dashboard.tsx           # Has beforeLoad (auth)
├── dashboard.lazy.tsx      # Component
├── posts.tsx               # Has loader
├── posts.lazy.tsx          # Component
├── posts/
│   ├── $postId.tsx         # Has loader
│   └── $postId.lazy.tsx    # Component
└── settings/
    ├── index.lazy.tsx      # Virtual route
    ├── profile.lazy.tsx    # Virtual route
    └── security.tsx        # Has beforeLoad (requires re-auth)
```

## Good Example: Generated Route Tree

```tsx
// routeTree.gen.ts (auto-generated)
import { Route as rootRoute } from './routes/__root'
import { Route as aboutLazyRoute } from './routes/about.lazy'  // Virtual parent

export const routeTree = rootRoute.addChildren([
  // Virtual route created for about.lazy.tsx
  createRoute({
    path: '/about',
    getParentRoute: () => rootRoute,
  }).lazy(() => import('./routes/about.lazy').then(m => m.Route)),

  // Regular route with explicit main file
  dashboardRoute.addChildren([...]),
])
```

## Context

- Virtual routes reduce boilerplate for simple pages
- Only works with file-based routing
- Auto-generated in `routeTree.gen.ts`
- Main route file needed for any "critical path" config
- Critical: loader, beforeLoad, validateSearch, loaderDeps, context
- Non-critical (can be in lazy): component, pendingComponent, errorComponent
- Check generated route tree to verify virtual routes
