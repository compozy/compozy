# The Rules of React

The compiler's contract. Every rule here has a lint rule enforcing it, and every violation risks a **bail** — the compiler skipping optimization of that component or hook with a green build.

Source: [Rules of React](https://react.dev/reference/rules), [eslint-plugin-react-hooks reference](https://react.dev/reference/eslint-plugin-react-hooks).

## Rule map

| Lint rule (`react-hooks/…`) | Severity | Enforces |
| --- | --- | --- |
| `rules-of-hooks` | error | Hooks only at the top level of React functions |
| `exhaustive-deps` | warn | Dependency arrays list every dependency |
| `purity` | error | No known-impure calls during render |
| `immutability` | error | No mutating props, state, or other immutable values |
| `globals` | error | No assigning or mutating globals during render |
| `refs` | error | No reading or writing refs during render |
| `set-state-in-render` | error | No unconditional `setState` during render |
| `set-state-in-effect` | error | No synchronous `setState` in an effect |
| `static-components` | error | Components declared once, not recreated per render |
| `preserve-manual-memoization` | error | Manual memoization the compiler can match (exhaustive deps) |
| `use-memo` | error | Every `useMemo` callback returns a value |
| `error-boundaries` | error | Child render errors handled by an Error Boundary |
| `config` / `gating` | error | Valid compiler configuration |
| `unsupported-syntax` | warn | Syntax the compiler cannot compile |
| `incompatible-library` | warn | Library APIs unsafe to memoize |

`component-hook-factories` exists but is a deprecated no-op in 7.1.1; the pattern it described is covered by `static-components`. Additional off-by-default rules that surface silent bails are listed in `compiler.md`.

## Components and hooks must be pure

### Idempotence

Same props, state, and context in — same output out, every time.

```jsx
// INVALID: new Date() differs on every call, so the clock freezes at render time
function Clock() {
  const time = new Date();
  return <span>{time.toLocaleString()}</span>;
}

// VALID: the non-idempotent work lives in an initializer and an effect
function useTime() {
  const [time, setTime] = useState(() => new Date()); // initializer runs once
  useEffect(() => {
    const id = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(id);
  }, []);
  return time;
}
```

`purity` flags `Math.random()`, `Date.now()`, `new Date()`, `crypto.randomUUID()`, and `performance.now()` in render.

```jsx
// INVALID: const id = Math.random();
// VALID:
const [id] = useState(() => crypto.randomUUID());
```

### Side effects belong outside render

The only sanctioned homes are event handlers and effects.

```jsx
// INVALID: mutates the document during render
function ProductDetailPage({ product }) {
  document.title = product.title;
}

// VALID
function ProductDetailPage({ product }) {
  useEffect(() => {
    document.title = product.title;
  }, [product.title]);
}
```

### Props are immutable

```jsx
// INVALID
item.url = new Url(item.url, base);

// VALID
const url = new Url(item.url, base);
```

A destructured prop is still a prop. Reassigning it is the single most common real-world bail:

```jsx
// INVALID: destructure-then-reassign
function MyComponent({ value }) {
  value = value ?? someStateValue;
  value = normalizeValue(value);
}

// VALID: rename on destructure, derive a new const
function MyComponent({ value: valueFromProps }) {
  const value = normalizeValue(valueFromProps ?? someStateValue);
}
```

### State is immutable

Always through the setter, and always with a fresh reference. A mutation that keeps the reference produces no re-render at all.

```jsx
// INVALID
items.push(4);
setItems(items);
user.name = "Bob";
setUser(user);
setItems(items.sort()); // sort() mutates in place and returns the same array

// VALID
setItems([...items, 4]);
setUser({ ...user, name: "Bob" });
setItems([...items].sort()); // or items.toSorted()
```

### Hook arguments and return values are immutable

This is the rule most directly load-bearing for the compiler, because mutating across a hook boundary silently defeats memoization:

```jsx
// INVALID
style = useIconStyle(icon); // memoized on `icon`
icon.enabled = false;       // mutation the memoization cannot see
style = useIconStyle(icon); // returns the stale memoized result

// VALID
icon = { ...icon, enabled: false }; // new reference forces recompute
style = useIconStyle(icon);
```

### Values are immutable after being passed to JSX

React may evaluate JSX eagerly, so a value handed to an element is frozen from that point.

```jsx
// INVALID: styles was already given to <Header>
const styles = { colour, size: "large" };
const header = <Header styles={styles} />;
styles.size = "small";
const footer = <Footer styles={styles} />;

// VALID: one object per consumer
const headerStyles = { colour, size: "large" };
const header = <Header styles={headerStyles} />;
const footerStyles = { colour, size: "small" };
const footer = <Footer styles={footerStyles} />;
```

### Local mutation is fine

Mutating a value created during this render that never escapes is explicitly allowed:

```jsx
// VALID: `items` is born and consumed inside this render
function List({ source }) {
  const items = [];
  for (const row of source) items.push(row.label);
  return <ul>{items.map((label) => <li key={label}>{label}</li>)}</ul>;
}
```

The same code with `const items = []` hoisted to module scope is a `globals` violation.

## Globals stay out of render

Module-level mutable state read or written during render breaks purity, diverges between dev and prod, breaks Fast Refresh, and forfeits compilation.

```jsx
// INVALID
let renderCount = 0;
function Component() {
  renderCount++;
  return <div>{renderCount}</div>;
}

// INVALID
window.currentUser = userId;          // during render
moduleLevelArray.push(event);         // during render
if (!cache[id]) cache[id] = fetch(id); // ad-hoc render cache

// VALID: useState / useContext for values, useEffect for outward writes
useEffect(() => {
  document.title = title;
}, [title]);
```

## Refs are not readable during render

```jsx
// INVALID: read during render
const value = ref.current;
return <div>{value}</div>;

// INVALID: write during render
ref.current = value;

// VALID: one-time lazy initialization is explicitly blessed
const ref = useRef(null);
if (ref.current === null) {
  ref.current = expensiveComputation();
}

// VALID: read and write in effects and event handlers
useEffect(() => {
  observerRef.current = new IntersectionObserver(onIntersect);
  return () => observerRef.current?.disconnect();
}, [onIntersect]);
```

"One-time lazy init is fine; a ref used as a render-visible cache keyed on props is not" is the line. Note a long-standing gap: writing `ref.current` inside a custom-hook body (the `useStableRef` pattern) historically triggers neither the lint rules nor a compiler complaint ([facebook/react#29161](https://github.com/facebook/react/issues/29161)) — hold yourself to the rule anyway.

## Components are static

```jsx
// INVALID: a new component type every render — state resets, DOM is destroyed
function Parent() {
  const [theme, setTheme] = useState("light");
  function ThemedButton() {
    return <button className={theme}>Click me</button>;
  }
  return <ThemedButton />;
}

// VALID: module scope, data through props
function ThemedButton({ theme }) {
  return <button className={theme}>Click me</button>;
}
```

Factories that return components or hooks are the same defect one level up:

```jsx
// INVALID
function createComponent(x) {
  return function Component() { /* … */ };
}
function createCustomHook(url) {
  return function useData() { /* … */ };
}
```

Splitting a component *so that a memo boundary exists* is obsolete under the compiler. Splitting for readability and single responsibility is not — keep doing that.

## setState during render

Unconditional `setState` in render is an infinite loop:

```jsx
// INVALID
function C({ value }) {
  const [n, setN] = useState(0);
  setN(value);
  return <div>{n}</div>;
}
```

A *conditional* `setState` during render is legal, and is the sanctioned alternative to an adjust-state effect — `effects.md` → *Adjusting some state on a prop change* has the pattern.

## Error boundaries, not try/catch

```jsx
// INVALID
try {
  return <ChildComponent />;
} catch {
  return <div>Error</div>;
}

// VALID
<ErrorBoundary FallbackComponent={ErrorFallback}>
  <ChildComponent />
</ErrorBoundary>
```

JSX inside a `try` block is also a compiler bail in its own right (see `compiler.md`).

```tsx
import { ErrorBoundary, type FallbackProps } from "react-error-boundary";

function ErrorFallback({ error, resetErrorBoundary }: FallbackProps) {
  return (
    <div role="alert">
      <h2>Something went wrong</h2>
      <pre>{error.message}</pre>
      <button onClick={resetErrorBoundary}>Try again</button>
    </div>
  );
}
```

Narrow unknown errors rather than assuming `Error`:

```ts
function toMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return "An unknown error occurred";
}
```

## React calls components and hooks

- Render components as JSX (`<Component />`), never call them as functions (`Component()`).
- Never pass a hook around as a value; call it directly at the top level.
- Hooks run at the top level, before any early return, never inside loops, conditions, or nested functions.
