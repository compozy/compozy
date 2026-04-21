# split-lazy-routes: Use .lazy.tsx for Code Splitting

## Priority: MEDIUM

## Explanation

Split route components into `.lazy.tsx` files to reduce initial bundle size. The main route file keeps critical configuration (path, loaders, search validation), while lazy files contain components that load on-demand.

## Bad Example

```tsx
// routes/dashboard.tsx - Everything in one file
import { createFileRoute } from "@tanstack/react-router";
import { HeavyChartLibrary } from "heavy-chart-library";
import { ComplexDataGrid } from "complex-data-grid";
import { AnalyticsWidgets } from "./components/AnalyticsWidgets";

export const Route = createFileRoute("/dashboard")({
  loader: async ({ context }) => {
    return context.queryClient.ensureQueryData(dashboardQueries.stats());
  },
  component: DashboardPage, // Entire component in main bundle
});

function DashboardPage() {
  // Heavy components loaded even if user never visits dashboard
  return (
    <div>
      <HeavyChartLibrary data={useLoaderData()} />
      <ComplexDataGrid />
      <AnalyticsWidgets />
    </div>
  );
}
```

## Good Example

```tsx
// routes/dashboard.tsx - Only critical config
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/dashboard")({
  loader: async ({ context }) => {
    return context.queryClient.ensureQueryData(dashboardQueries.stats());
  },
  // No component - it's in the lazy file
});

// routes/dashboard.lazy.tsx - Lazy-loaded component
import { createLazyFileRoute } from "@tanstack/react-router";
import { HeavyChartLibrary } from "heavy-chart-library";
import { ComplexDataGrid } from "complex-data-grid";
import { AnalyticsWidgets } from "./components/AnalyticsWidgets";

export const Route = createLazyFileRoute("/dashboard")({
  component: DashboardPage,
  pendingComponent: DashboardSkeleton,
  errorComponent: DashboardError,
});

function DashboardPage() {
  const data = Route.useLoaderData();
  return (
    <div>
      <HeavyChartLibrary data={data} />
      <ComplexDataGrid />
      <AnalyticsWidgets />
    </div>
  );
}

function DashboardSkeleton() {
  return <div className="dashboard-skeleton">Loading dashboard...</div>;
}

function DashboardError({ error }: { error: Error }) {
  return <div>Failed to load dashboard: {error.message}</div>;
}
```

## What Goes Where

```tsx
// Main route file (routes/example.tsx)
// - path configuration (implicit from file location)
// - validateSearch
// - beforeLoad (auth checks, redirects)
// - loader (data fetching)
// - loaderDeps
// - context manipulation
// - Static route data

// Lazy file (routes/example.lazy.tsx)
// - component
// - pendingComponent
// - errorComponent
// - notFoundComponent
```

## Using getRouteApi in Lazy Components

```tsx
// routes/posts/$postId.lazy.tsx
import { createLazyFileRoute, getRouteApi } from "@tanstack/react-router";

const route = getRouteApi("/posts/$postId");

export const Route = createLazyFileRoute("/posts/$postId")({
  component: PostPage,
});

function PostPage() {
  // Type-safe access without importing main route file
  const { postId } = route.useParams();
  const data = route.useLoaderData();

  return <article>{/* ... */}</article>;
}
```

## Automatic Code Splitting

```tsx
// vite.config.ts - Enable automatic splitting
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";

export default defineConfig({
  plugins: [
    TanStackRouterVite({
      autoCodeSplitting: true, // Automatically splits all route components
    }),
    react(),
  ],
});

// With autoCodeSplitting, you don't need .lazy.tsx files
// The plugin handles the splitting automatically
```

## Context

- Lazy loading reduces initial bundle size significantly
- Loaders are NOT lazy - they need to run before rendering
- `createLazyFileRoute` only accepts component-related options
- Use `getRouteApi()` for type-safe hook access in lazy files
- Consider `autoCodeSplitting: true` for simpler setup
- Virtual routes auto-generate when only .lazy.tsx exists
