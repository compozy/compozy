# ssr-hydration-safety: Prevent Hydration Mismatches

## Priority: MEDIUM

## Explanation

Hydration errors occur when server-rendered HTML doesn't match what the client expects. This causes React to discard server HTML and re-render, losing SSR benefits. Ensure consistent rendering between server and client.

## Bad Example

```tsx
// Using Date.now() - different on server and client
function Timestamp() {
  return <span>Generated at: {Date.now()}</span>;
}

// Using Math.random() - always different
function RandomGreeting() {
  const greetings = ["Hello", "Hi", "Hey"];
  return <h1>{greetings[Math.floor(Math.random() * 3)]}</h1>;
}

// Checking window - doesn't exist on server
function DeviceInfo() {
  return <span>Width: {window.innerWidth}px</span>; // Error on server
}

// Conditional render based on time
function TimeBasedContent() {
  const hour = new Date().getHours();
  return hour < 12 ? <Morning /> : <Evening />;
  // Server might render Morning, client renders Evening
}
```

## Good Example: Consistent Server/Client Rendering

```tsx
// Pass data from server to avoid mismatch
export const Route = createFileRoute("/dashboard")({
  loader: async () => {
    return {
      generatedAt: Date.now(),
    };
  },
  component: Dashboard,
});

function Dashboard() {
  const { generatedAt } = Route.useLoaderData();
  // Both server and client use same value
  return <span>Generated at: {generatedAt}</span>;
}
```

## Good Example: Client-Only Components

```tsx
// Use lazy loading for client-only features
import { lazy, Suspense } from "react";

const ClientOnlyMap = lazy(() => import("./Map"));

function LocationPage() {
  return (
    <div>
      <h1>Our Location</h1>
      <Suspense fallback={<MapPlaceholder />}>
        <ClientOnlyMap />
      </Suspense>
    </div>
  );
}

// Or use useEffect for client-only state
function WindowSize() {
  const [size, setSize] = useState<{ width: number; height: number } | null>(null);

  useEffect(() => {
    setSize({
      width: window.innerWidth,
      height: window.innerHeight,
    });
  }, []);

  if (!size) {
    return <span>Loading dimensions...</span>;
  }

  return (
    <span>
      {size.width} x {size.height}
    </span>
  );
}
```

## Good Example: Stable Random Values

```tsx
// Generate random value on server, pass to client
export const Route = createFileRoute('/onboarding')({
  loader: () => ({
    welcomeVariant: Math.floor(Math.random() * 3),
  }),
  component: Onboarding,
})

function Onboarding() {
  const { welcomeVariant } = Route.useLoaderData()
  const messages = ['Welcome aboard!', 'Let's get started!', 'Great to have you!']

  return <h1>{messages[welcomeVariant]}</h1>  // Same on server and client
}
```

## Good Example: Handling Time Zones

```tsx
// Pass formatted date from server
export const Route = createFileRoute("/posts/$postId")({
  loader: async ({ params }) => {
    const post = await getPost(params.postId);
    return {
      ...post,
      // Format on server to avoid timezone mismatch
      formattedDate: new Intl.DateTimeFormat("en-US", {
        dateStyle: "long",
        timeStyle: "short",
        timeZone: "UTC", // Consistent timezone
      }).format(post.createdAt),
    };
  },
  component: PostPage,
});

// Or use client-only formatting
function RelativeTime({ date }: { date: Date }) {
  const [formatted, setFormatted] = useState<string>("");

  useEffect(() => {
    // Format in user's timezone after hydration
    setFormatted(formatDistanceToNow(date, { addSuffix: true }));
  }, [date]);

  // Show absolute date initially (same server/client)
  return <time dateTime={date.toISOString()}>{formatted || date.toISOString().split("T")[0]}</time>;
}
```

## Common Hydration Mismatch Causes

| Issue                       | Solution                              |
| --------------------------- | ------------------------------------- |
| `Date.now()` / `new Date()` | Pass timestamp from loader            |
| `Math.random()`             | Generate on server, pass to client    |
| `window` / `document`       | Use useEffect or lazy loading         |
| User timezone differences   | Use UTC or client-only formatting     |
| Browser-specific APIs       | Check `typeof window !== 'undefined'` |
| Extension-injected content  | Use `suppressHydrationWarning`        |

## Debugging Hydration Errors

```tsx
// React 18+ provides detailed hydration error messages
// Check the console for:
// - "Text content does not match"
// - "Hydration failed because"
// - The specific DOM element causing the issue

// For difficult cases, use suppressHydrationWarning sparingly
function UserContent({ html }: { html: string }) {
  return <div suppressHydrationWarning dangerouslySetInnerHTML={{ __html: html }} />;
}
```

## Context

- Hydration compares server HTML with client render
- Mismatches force full client re-render (slow, flash)
- Use loaders to pass dynamic data consistently
- Defer client-only content with useEffect or Suspense
- Test SSR by disabling JavaScript and checking render
- Development mode shows hydration warnings in console
