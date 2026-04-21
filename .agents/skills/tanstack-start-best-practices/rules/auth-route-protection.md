# auth-route-protection: Protect Routes with beforeLoad

## Priority: HIGH

## Explanation

Use `beforeLoad` in route definitions to check authentication before the route loads. This prevents unauthorized access, redirects to login, and can extend context with user data for child routes.

## Bad Example

```tsx
// Checking auth in component - too late, data may have loaded
function DashboardPage() {
  const user = useAuth();

  useEffect(() => {
    if (!user) {
      navigate({ to: "/login" }); // Redirect after render
    }
  }, [user]);

  if (!user) return null; // Flash of content possible

  return <Dashboard user={user} />;
}

// No protection on route
export const Route = createFileRoute("/dashboard")({
  loader: async () => {
    // Fetches sensitive data even for unauthenticated users
    return await fetchDashboardData();
  },
  component: DashboardPage,
});
```

## Good Example: Route-Level Protection

```tsx
// routes/_authenticated.tsx - Layout route for protected area
import { createFileRoute, redirect, Outlet } from "@tanstack/react-router";
import { getSessionData } from "@/lib/session.server";

export const Route = createFileRoute("/_authenticated")({
  beforeLoad: async ({ location }) => {
    const session = await getSessionData();

    if (!session) {
      throw redirect({
        to: "/login",
        search: {
          redirect: location.href,
        },
      });
    }

    // Extend context with user for all child routes
    return {
      user: session,
    };
  },
  component: AuthenticatedLayout,
});

function AuthenticatedLayout() {
  return (
    <div>
      <AuthenticatedNav />
      <main>
        <Outlet /> {/* Child routes render here */}
      </main>
    </div>
  );
}

// routes/_authenticated/dashboard.tsx
// This route is automatically protected by parent
export const Route = createFileRoute("/_authenticated/dashboard")({
  loader: async ({ context }) => {
    // context.user is guaranteed to exist
    return await fetchDashboardData(context.user.id);
  },
  component: DashboardPage,
});

function DashboardPage() {
  const data = Route.useLoaderData();
  const { user } = Route.useRouteContext();

  return <Dashboard data={data} user={user} />;
}
```

## Good Example: Role-Based Access

```tsx
// routes/_admin.tsx
export const Route = createFileRoute("/_admin")({
  beforeLoad: async ({ context }) => {
    // context.user comes from parent _authenticated route
    if (context.user.role !== "admin") {
      throw redirect({ to: "/unauthorized" });
    }
  },
  component: AdminLayout,
});

// File structure:
// routes/
//   _authenticated.tsx        # Requires login
//   _authenticated/
//     dashboard.tsx           # /dashboard - any authenticated user
//     settings.tsx            # /settings - any authenticated user
//     _admin.tsx              # Admin layout
//     _admin/
//       users.tsx             # /users - admin only
//       analytics.tsx         # /analytics - admin only
```

## Good Example: Preserving Redirect URL

```tsx
// routes/login.tsx
import { z } from "zod";

export const Route = createFileRoute("/login")({
  validateSearch: z.object({
    redirect: z.string().optional(),
  }),
  component: LoginPage,
});

function LoginPage() {
  const { redirect } = Route.useSearch();
  const loginMutation = useMutation({
    mutationFn: login,
    onSuccess: () => {
      // Redirect to original destination or default
      navigate({ to: redirect ?? "/dashboard" });
    },
  });

  return <LoginForm onSubmit={loginMutation.mutate} />;
}

// In protected routes
beforeLoad: async ({ location }) => {
  if (!session) {
    throw redirect({
      to: "/login",
      search: { redirect: location.href },
    });
  }
};
```

## Good Example: Conditional Content Based on Auth

```tsx
// Public route with different content for logged-in users
export const Route = createFileRoute("/")({
  beforeLoad: async () => {
    const session = await getSessionData();
    return { user: session?.user ?? null };
  },
  component: HomePage,
});

function HomePage() {
  const { user } = Route.useRouteContext();

  return (
    <div>
      <Hero />
      {user ? <PersonalizedContent user={user} /> : <SignUpCTA />}
    </div>
  );
}
```

## Context

- `beforeLoad` runs before route loading begins
- Throwing `redirect()` prevents route from loading
- Context from `beforeLoad` flows to loader and component
- Child routes inherit parent's `beforeLoad` protection
- Use pathless layout routes (`_authenticated.tsx`) for grouped protection
- Store redirect URL in search params for post-login navigation
