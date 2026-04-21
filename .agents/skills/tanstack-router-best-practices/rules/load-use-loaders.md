# load-use-loaders: Use Route Loaders for Data Fetching

## Priority: HIGH

## Explanation

Route loaders execute before the route renders, enabling data to be ready when the component mounts. This prevents loading waterfalls, enables preloading, and integrates with the router's caching layer.

## Bad Example

```tsx
// Fetching in component - creates waterfall
function PostsPage() {
  const [posts, setPosts] = useState<Post[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Route renders, THEN data fetches, THEN UI updates
    fetchPosts().then(data => {
      setPosts(data);
      setLoading(false);
    });
  }, []);

  if (loading) return <Loading />;
  return <PostList posts={posts} />;
}

// No preloading possible - user sees loading state on navigation
```

## Good Example

```tsx
// routes/posts.tsx
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/posts")({
  loader: async () => {
    const posts = await fetchPosts();
    return { posts };
  },
  component: PostsPage,
});

function PostsPage() {
  // Data is ready when component mounts - no loading state needed
  const { posts } = Route.useLoaderData();
  return <PostList posts={posts} />;
}
```

## Good Example: With Parameters

```tsx
// routes/posts/$postId.tsx
export const Route = createFileRoute("/posts/$postId")({
  loader: async ({ params }) => {
    // params are type-safe and guaranteed to exist
    const post = await fetchPost(params.postId);
    const comments = await fetchComments(params.postId);
    return { post, comments };
  },
  component: PostDetailPage,
});

function PostDetailPage() {
  const { post, comments } = Route.useLoaderData();
  const { postId } = Route.useParams();

  return (
    <article>
      <h1>{post.title}</h1>
      <PostContent content={post.content} />
      <CommentList comments={comments} />
    </article>
  );
}
```

## Good Example: With TanStack Query

```tsx
// routes/posts/$postId.tsx
import { queryOptions } from "@tanstack/react-query";

const postQueryOptions = (postId: string) =>
  queryOptions({
    queryKey: ["posts", postId],
    queryFn: () => fetchPost(postId),
  });

export const Route = createFileRoute("/posts/$postId")({
  loader: async ({ params, context: { queryClient } }) => {
    // Ensure data is in cache before render
    await queryClient.ensureQueryData(postQueryOptions(params.postId));
  },
  component: PostDetailPage,
});

function PostDetailPage() {
  const { postId } = Route.useParams();
  // useSuspenseQuery because loader guarantees data exists
  const { data: post } = useSuspenseQuery(postQueryOptions(postId));

  return <PostContent post={post} />;
}
```

## Loader Context Properties

```tsx
export const Route = createFileRoute("/posts")({
  loader: async ({
    params, // Route path parameters
    context, // Route context (queryClient, auth, etc.)
    abortController, // For cancelling stale requests
    cause, // 'enter' | 'preload' | 'stay'
    deps, // Dependencies from loaderDeps
    preload, // Boolean: true if preloading
  }) => {
    // Use abortController for fetch cancellation
    const response = await fetch("/api/posts", {
      signal: abortController.signal,
    });

    // Different behavior for preload vs navigation
    if (preload) {
      // Lighter data for preload
      return { posts: await response.json() };
    }

    // Full data for actual navigation
    const posts = await response.json();
    const stats = await fetchStats();
    return { posts, stats };
  },
});
```

## Context

- Loaders run during route matching, before component render
- Supports parallel loading across nested routes
- Enables preloading on link hover/focus
- Built-in stale-while-revalidate caching
- For complex caching needs, integrate with TanStack Query
- Use `beforeLoad` for auth checks and redirects
