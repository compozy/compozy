# ctx-root-context: Define Context at Root Route

## Priority: LOW

## Explanation

Use `createRootRouteWithContext` to define typed context that flows through your entire route tree. This enables dependency injection for things like query clients, auth state, and services.

## Bad Example

```tsx
// No context - importing globals directly
// routes/__root.tsx
import { createRootRoute } from "@tanstack/react-router";
import { queryClient } from "@/lib/query-client"; // Global import

export const Route = createRootRoute({
  component: RootComponent,
});

// routes/posts.tsx
import { queryClient } from "@/lib/query-client"; // Import again

export const Route = createFileRoute("/posts")({
  loader: async () => {
    // Using global - harder to test, couples to implementation
    return queryClient.ensureQueryData(postQueries.list());
  },
});
```

## Good Example

```tsx
// routes/__root.tsx
import { createRootRouteWithContext, Outlet } from "@tanstack/react-router";
import { QueryClient } from "@tanstack/react-query";

// Define the context interface
interface RouterContext {
  queryClient: QueryClient;
  auth: {
    user: User | null;
    isAuthenticated: boolean;
  };
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootComponent,
});

function RootComponent() {
  return (
    <>
      <Header />
      <main>
        <Outlet />
      </main>
      <Footer />
    </>
  );
}

// router.tsx - Provide context when creating router
import { createRouter } from "@tanstack/react-router";
import { QueryClient } from "@tanstack/react-query";
import { setupRouterSsrQueryIntegration } from "@tanstack/react-router-ssr-query";
import { routeTree } from "./routeTree.gen";

export function getRouter(auth: RouterContext["auth"] = { user: null, isAuthenticated: false }) {
  const queryClient = new QueryClient();

  const router = createRouter({
    routeTree,
    context: {
      queryClient,
      auth,
    },
    defaultPreload: "intent",
    defaultPreloadStaleTime: 0,
    scrollRestoration: true,
  });

  setupRouterSsrQueryIntegration({ router, queryClient });

  return router;
}

// routes/posts.tsx - Use context in loaders
export const Route = createFileRoute("/posts")({
  loader: async ({ context: { queryClient } }) => {
    // Context is typed and injected
    return queryClient.ensureQueryData(postQueries.list());
  },
});
```

## Good Example: Auth-Protected Routes

```tsx
// routes/__root.tsx
interface RouterContext {
  queryClient: QueryClient;
  auth: AuthState;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootComponent,
});

// routes/_authenticated.tsx - Layout route for protected pages
export const Route = createFileRoute("/_authenticated")({
  beforeLoad: async ({ context, location }) => {
    if (!context.auth.isAuthenticated) {
      throw redirect({
        to: "/login",
        search: { redirect: location.href },
      });
    }
  },
  component: AuthenticatedLayout,
});

// routes/_authenticated/dashboard.tsx
export const Route = createFileRoute("/_authenticated/dashboard")({
  loader: async ({ context: { queryClient, auth } }) => {
    // We know user is authenticated from parent beforeLoad
    return queryClient.ensureQueryData(dashboardQueries.forUser(auth.user!.id));
  },
});
```

## Extending Context with beforeLoad

```tsx
// routes/posts/$postId.tsx
export const Route = createFileRoute("/posts/$postId")({
  beforeLoad: async ({ context, params }) => {
    // Extend context with route-specific data
    const post = await fetchPost(params.postId);

    return {
      post, // Available to this route and children
    };
  },
  loader: async ({ context }) => {
    // context now includes 'post' from beforeLoad
    const comments = await fetchComments(context.post.id);
    return { comments };
  },
});
```

## Context vs. Loader Data

| Context                                        | Loader Data                  |
| ---------------------------------------------- | ---------------------------- |
| Available in beforeLoad, loader, and component | Only available in component  |
| Set at router creation or in beforeLoad        | Returned from loader         |
| Good for services, clients, auth               | Good for route-specific data |
| Flows down to all children                     | Specific to route            |

## Context

- Type the context interface in `createRootRouteWithContext<T>()`
- Provide context when calling `createRouter({ context: {...} })`
- Context flows from root to all nested routes
- Use `beforeLoad` to extend context for specific subtrees
- Enables testability - inject mocks in tests
- Avoids global imports and singletons
