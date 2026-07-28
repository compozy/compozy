# React 19 APIs

Covers React 19.0 ([blog](https://react.dev/blog/2024/12/05/react-19)) and 19.2 ([blog](https://react.dev/blog/2025/10/01/react-19-2)).

## Actions and forms

An async function passed to a transition is an **Action**: React handles pending state, errors, and optimistic updates for you.

### useActionState

```tsx
const [error, submitAction, isPending] = useActionState(
  async (previousState, formData) => {
    const error = await updateName(formData.get("name"));
    return error || null;
  },
  null,
);

<form action={submitAction}>
  <input name="name" />
  <button type="submit" disabled={isPending}>Update</button>
  {error ? <p role="alert">{error}</p> : null}
</form>;
```

Return order is `[state, submitAction, isPending]`. Form elements accept a function as `action`.

### useFormStatus

Reads the enclosing `<form>`'s status without prop drilling — the child must be inside the form.

```tsx
import { useFormStatus } from "react-dom";

function SubmitButton() {
  const { pending } = useFormStatus();
  return <button type="submit" disabled={pending}>{pending ? "Submitting…" : "Submit"}</button>;
}
```

### useOptimistic

```tsx
function TodoList({ todos, addTodo }: Props) {
  const [optimisticTodos, addOptimisticTodo] = useOptimistic(
    todos,
    (state, newTodo: Todo) => [...state, { ...newTodo, pending: true }],
  );

  async function handleAdd(formData: FormData) {
    const newTodo = { id: crypto.randomUUID(), text: String(formData.get("text")) };
    addOptimisticTodo(newTodo);
    await addTodo(newTodo);
  }

  return (
    <form action={handleAdd}>
      <input name="text" />
      <ul>
        {optimisticTodos.map((todo) => (
          <li key={todo.id} className={todo.pending ? "opacity-50" : ""}>{todo.text}</li>
        ))}
      </ul>
    </form>
  );
}
```

The optimistic value reverts automatically when the action settles.

## use()

Reads a promise or a context during render, suspending until it resolves.

```tsx
import { use, Suspense } from "react";

function UserProfile({ userPromise }: { userPromise: Promise<User> }) {
  const user = use(userPromise);
  return <div>{user.name}</div>;
}

<Suspense fallback={<Loading />}>
  <UserProfile userPromise={fetchUser(id)} />
</Suspense>;
```

Create the promise in a Server Component or a cached loader, not inline in a Client Component's render — a promise created during render is a new promise each time.

## Ref cleanup functions

A ref callback may now return a cleanup function, which runs on unmount:

```tsx
<input
  ref={(node) => {
    const observer = observe(node);
    return () => observer.disconnect();
  }}
/>
```

`ref` is also an ordinary prop now, with `forwardRef` deprecated — `components-and-types.md` → *React 19 TypeScript changes* covers the conversion and the implicit-return hazard this cleanup behaviour introduces.

## Context as provider

```tsx
// Deprecated
<ThemeContext.Provider value="dark">{children}</ThemeContext.Provider>

// React 19
<ThemeContext value="dark">{children}</ThemeContext>
```

## useDeferredValue with an initial value

```tsx
const deferred = useDeferredValue(value, initialValue); // initialValue used on the first render
```

## Document metadata and resources

`<title>`, `<meta>`, and `<link>` rendered anywhere in the tree are hoisted into `<head>`.

```tsx
function BlogPost({ post }) {
  return (
    <article>
      <title>{post.title}</title>
      <meta name="description" content={post.excerpt} />
      <link rel="canonical" href={post.url} />
      <h1>{post.title}</h1>
    </article>
  );
}
```

Stylesheets support a `precedence` attribute for ordering, and async `<script>` tags are deduplicated wherever they appear.

Preloading APIs from `react-dom`:

```tsx
import { prefetchDNS, preconnect, preload, preinit } from "react-dom";

preload("/fonts/inter.woff2", { as: "font" });
preinit("/analytics.js", { as: "script" });
```

## Error handling

Root options replace the old console-only behaviour, and errors are deduplicated with hydration-mismatch diffs:

```tsx
createRoot(container, {
  onCaughtError: (error, errorInfo) => report(error, errorInfo),   // caught by a boundary
  onUncaughtError: (error, errorInfo) => report(error, errorInfo), // reached the root
});
```

## Activity (19.2, stable)

Hides a subtree while **preserving its DOM and state**, instead of destroying both.

```tsx
// Before: state and DOM are thrown away
{isVisible && <Page />}

// After
<Activity mode={isVisible ? "visible" : "hidden"}>
  <Page />
</Activity>
```

While hidden: `display: none` is applied, effects are cleaned up, DOM and state are preserved, and children re-render at lower priority. Uses: restoring state across tab switches, preserving unsaved textarea contents, pre-rendering a likely-next route, selective hydration.

Caveats: text-only children have no DOM node to hide; `<video>`, `<audio>`, and `<iframe>` keep running when hidden, so pause them in effect cleanup; a hidden↔visible transition triggers `enter`/`exit` inside a `<ViewTransition>`.

## Server-side and RSC

- **Server Components** and **Server Actions** (`"use server"`) are stable.
- **Static APIs** — `prerender` and `prerenderToNodeStream` from `react-dom/static`.
- **Partial Pre-rendering** (19.2) — `prerender` returns `{ prelude, postponed }`; resume later with `resume` (to an SSR stream) or `resumeAndPrerender` (to static HTML).
- **`cacheSignal`** (19.2, RSC only) — learn when a `cache()` lifetime ends so in-flight work can abort.

```tsx
import { cache, cacheSignal } from "react";

const dedupedFetch = cache(fetch);
async function Component() {
  await dedupedFetch(url, { signal: cacheSignal() });
}
```

## Other 19.2 changes worth knowing

- **Performance Tracks** — Chrome DevTools custom tracks: *Scheduler* (priority lanes) and *Components* (per-component render and effect timing).
- **SSR Suspense reveals are batched** briefly, matching client behaviour and preparing for `<ViewTransition>`.
- **`useId` prefix changed from `:r:` to `_r_`** so generated ids are valid CSS and XML selectors. Breaking for any selector keyed on the old prefix.
- `renderToReadableStream` and `prerender` are available in Node.js, though the Node Streams APIs remain the recommendation there.

## Transitions

```tsx
const [isPending, startTransition] = useTransition();

function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
  setInputValue(e.target.value);              // urgent: keep the input responsive
  startTransition(() => setQuery(e.target.value)); // non-urgent: interruptible
}
```

## Code splitting

```tsx
const HeavyComponent = lazy(() => import("./heavy-component"));

<Suspense fallback={<Loading />}>
  <HeavyComponent />
</Suspense>;
```

The compiler does nothing for bundle size — splitting is still your job.
