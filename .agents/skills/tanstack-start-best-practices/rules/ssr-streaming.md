# ssr-streaming: Implement Streaming SSR for Faster TTFB

## Priority: MEDIUM

## Explanation

Streaming SSR sends HTML chunks to the browser as they're ready, rather than waiting for all data to load. This improves Time to First Byte (TTFB) and perceived performance by showing content progressively.

## Bad Example

```tsx
// Blocking SSR - waits for everything
export const Route = createFileRoute("/dashboard")({
  loader: async ({ context: { queryClient } }) => {
    // All of these must complete before ANY HTML is sent
    await Promise.all([
      queryClient.ensureQueryData(userQueries.profile()), // 200ms
      queryClient.ensureQueryData(dashboardQueries.stats()), // 500ms
      queryClient.ensureQueryData(activityQueries.recent()), // 300ms
      queryClient.ensureQueryData(notificationQueries.all()), // 400ms
    ]);
    // TTFB: 500ms (slowest query)
  },
});
```

## Good Example: Stream Non-Critical Content

```tsx
// routes/dashboard.tsx
export const Route = createFileRoute("/dashboard")({
  loader: async ({ context: { queryClient } }) => {
    // Only await critical above-the-fold data
    await queryClient.ensureQueryData(userQueries.profile());

    // Start fetching but don't await
    queryClient.prefetchQuery(dashboardQueries.stats());
    queryClient.prefetchQuery(activityQueries.recent());
    queryClient.prefetchQuery(notificationQueries.all());

    // HTML starts streaming immediately after profile loads
    // TTFB: 200ms
  },
  component: DashboardPage,
});

function DashboardPage() {
  // Critical data - ready immediately (from loader)
  const { data: user } = useSuspenseQuery(userQueries.profile());

  return (
    <div>
      <Header user={user} />

      {/* Non-critical - streams in with Suspense */}
      <Suspense fallback={<StatsSkeleton />}>
        <DashboardStats />
      </Suspense>

      <Suspense fallback={<ActivitySkeleton />}>
        <RecentActivity />
      </Suspense>

      <Suspense fallback={<NotificationsSkeleton />}>
        <NotificationsList />
      </Suspense>
    </div>
  );
}

// Each section loads independently and streams when ready
function DashboardStats() {
  const { data: stats } = useSuspenseQuery(dashboardQueries.stats());
  return <StatsDisplay stats={stats} />;
}
```

## Good Example: Nested Suspense Boundaries

```tsx
function DashboardPage() {
  const { data: user } = useSuspenseQuery(userQueries.profile());

  return (
    <div>
      <Header user={user} />

      <div className="grid grid-cols-2 gap-4">
        {/* Left column streams together */}
        <Suspense fallback={<LeftColumnSkeleton />}>
          <LeftColumn />
        </Suspense>

        {/* Right column streams independently */}
        <Suspense fallback={<RightColumnSkeleton />}>
          <RightColumn />
        </Suspense>
      </div>
    </div>
  );
}

function LeftColumn() {
  // These load together (same Suspense boundary)
  const { data: stats } = useSuspenseQuery(dashboardQueries.stats());
  const { data: chart } = useSuspenseQuery(dashboardQueries.chartData());

  return (
    <div>
      <StatsCard stats={stats} />
      <ChartDisplay data={chart} />
    </div>
  );
}
```

## Good Example: Progressive Enhancement

```tsx
export const Route = createFileRoute("/posts/$postId")({
  loader: async ({ params, context: { queryClient } }) => {
    // Critical: post content (await)
    await queryClient.ensureQueryData(postQueries.detail(params.postId));

    // Start but don't block: comments, related posts
    queryClient.prefetchQuery(commentQueries.forPost(params.postId));
    queryClient.prefetchQuery(postQueries.related(params.postId));
  },
  component: PostPage,
});

function PostPage() {
  const { postId } = Route.useParams();
  const { data: post } = useSuspenseQuery(postQueries.detail(postId));

  return (
    <article>
      {/* Streams immediately */}
      <PostHeader post={post} />
      <PostContent content={post.content} />

      {/* Streams when ready */}
      <Suspense fallback={<CommentsSkeleton />}>
        <CommentsSection postId={postId} />
      </Suspense>

      <Suspense fallback={<RelatedSkeleton />}>
        <RelatedPosts postId={postId} />
      </Suspense>
    </article>
  );
}
```

## Good Example: Error Boundaries with Streaming

```tsx
function DashboardPage() {
  return (
    <div>
      <Header />

      {/* Each section handles its own errors */}
      <ErrorBoundary fallback={<StatsError />}>
        <Suspense fallback={<StatsSkeleton />}>
          <DashboardStats />
        </Suspense>
      </ErrorBoundary>

      <ErrorBoundary fallback={<ActivityError />}>
        <Suspense fallback={<ActivitySkeleton />}>
          <RecentActivity />
        </Suspense>
      </ErrorBoundary>
    </div>
  );
}
```

## Streaming Timeline

```
Traditional SSR:
Request → [Wait for all data...] → Send complete HTML → Render

Streaming SSR:
Request → Send shell HTML → Stream chunk 1 → Stream chunk 2 → Stream chunk 3 → Done
          ↓                  ↓                ↓                ↓
          Browser renders    Shows content    More content     Complete
          skeleton          progressively
```

## Context

- Suspense boundaries define streaming chunks
- Place boundaries around slow or non-critical content
- Critical path data should still be awaited in loader
- Each Suspense boundary can error independently
- Works with React 18's streaming SSR
- Monitor TTFB to verify streaming is working
- Consider network conditions - too many chunks can slow total load
