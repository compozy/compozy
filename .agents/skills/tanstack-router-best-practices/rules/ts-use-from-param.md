# ts-use-from-param: Use `from` Parameter for Type Narrowing

## Priority: CRITICAL

## Explanation

When using hooks like `useParams`, `useSearch`, or `useLoaderData`, provide the `from` parameter to get exact types for that route. Without it, TypeScript returns a union of all possible types across all routes.

## Bad Example

```tsx
// Without 'from' - TypeScript doesn't know which route's types to use
function PostDetail() {
  // params could be from ANY route - types are unioned
  const params = useParams();
  // params: { postId?: string; userId?: string; categoryId?: string; ... }

  // TypeScript can't guarantee postId exists
  console.log(params.postId); // postId: string | undefined
}

// Similarly for search params
function SearchResults() {
  const search = useSearch();
  // search: union of ALL routes' search params
}
```

## Good Example

```tsx
// With 'from' - exact types for this specific route
function PostDetail() {
  const params = useParams({ from: "/posts/$postId" });
  // params: { postId: string } - exactly what this route provides

  console.log(params.postId); // postId: string (guaranteed)
}

// Full path matching
function UserPost() {
  const params = useParams({ from: "/users/$userId/posts/$postId" });
  // params: { userId: string; postId: string }
}

// Search params with type narrowing
function SearchResults() {
  const search = useSearch({ from: "/search" });
  // search: exactly the validated search params for /search route
}

// Loader data with type inference
function PostPage() {
  const { post, comments } = useLoaderData({ from: "/posts/$postId" });
  // Exact types from your loader function
}
```

## Using Route.fullPath for Type Safety

```tsx
// routes/posts/$postId.tsx
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/posts/$postId")({
  loader: async ({ params }) => {
    const post = await fetchPost(params.postId);
    return { post };
  },
  component: PostComponent,
});

function PostComponent() {
  // Use Route.fullPath for guaranteed type matching
  const params = useParams({ from: Route.fullPath });
  const { post } = useLoaderData({ from: Route.fullPath });

  // Or use route-specific helper (preferred in same file)
  const { postId } = Route.useParams();
  const data = Route.useLoaderData();
}
```

## Using getRouteApi for Code-Split Components

```tsx
// components/PostDetail.tsx (separate file from route)
import { getRouteApi } from "@tanstack/react-router";

// Get type-safe access without importing the route
const postRoute = getRouteApi("/posts/$postId");

export function PostDetail() {
  const params = postRoute.useParams();
  // params: { postId: string }

  const data = postRoute.useLoaderData();
  // data: exact loader return type

  const search = postRoute.useSearch();
  // search: exact search param types
}
```

## When to Use strict: false

```tsx
// In shared components that work across multiple routes
function Breadcrumbs() {
  // strict: false returns union types but allows component reuse
  const params = useParams({ strict: false });
  const location = useLocation();

  // params may or may not have certain values
  return (
    <nav>
      {params.userId && <span>User: {params.userId}</span>}
      {params.postId && <span>Post: {params.postId}</span>}
    </nav>
  );
}
```

## Context

- Always use `from` in route-specific components for exact types
- Use `Route.useParams()` / `Route.useLoaderData()` within route files
- Use `getRouteApi()` in components split from route files
- Use `strict: false` only in truly generic, cross-route components
- The `from` path must match exactly (including params like `$postId`)
