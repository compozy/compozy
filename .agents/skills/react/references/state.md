# State management

## Hierarchy

Reach for the first row that fits.

| Priority | Tool | Use case |
| --- | --- | --- |
| 1 | `useState` / `useReducer` | Component-specific UI state |
| 2 | Context | Shared state read by a subtree, changing infrequently |
| 3 | Zustand / XState Store / Jotai | Shared client state across unrelated components |
| 4 | TanStack Query | Server state, caching, synchronization |
| 5 | URL state | Shareable, linkable application state (TanStack Router) |

Server data does not belong in a client store. Derived values do not belong in state at all — compute them during render.

```tsx
type State = { items: Item[]; filter: string; sortBy: SortKey };
type Action =
  | { type: "ADD_ITEM"; item: Item }
  | { type: "SET_FILTER"; filter: string }
  | { type: "SET_SORT"; sortBy: SortKey };

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case "ADD_ITEM":
      return { ...state, items: [...state.items, action.item] };
    case "SET_FILTER":
      return { ...state, filter: action.filter };
    case "SET_SORT":
      return { ...state, sortBy: action.sortBy };
  }
}
```

Every branch returns a new object. A reducer that mutates `state` is an `immutability` violation.

## Context under the compiler

The compiler stabilizes the provider's `value` object automatically, so the classic boilerplate is gone:

```tsx
// No longer needed
const value = useMemo(() => ({ state, dispatch }), [state, dispatch]);

// Just this
<StoreContext value={{ state, dispatch }}>{children}</StoreContext>
```

What the compiler does **not** do is make context subscriptions fine-grained. Compiler memoization behaves like `memo`, and a memoized component still re-renders when a context it consumes changes. So splitting contexts by change frequency remains correct:

```tsx
// Still bad: one monolithic context mixing slow and fast values
<AppContext value={{ theme, user, searchFilters, notifications }}>

// Good: split along subscription boundaries
<ThemeContext value={theme}>
  <UserContext value={user}>
    <FiltersContext value={filters}>
```

Short version: drop the hand-written `value` memo, keep the split contexts. There is still no first-party context selector — [RFC 119 (`useContextSelector`)](https://github.com/reactjs/rfcs/pull/119) remains open.

Context is for reading shared data in a subtree, not a cure for every prop chain. Passing a prop through two levels is fine; threading it through six is the signal.

## External stores

Zustand, Redux, Jotai, and XState Store are all built on `useSyncExternalStore`, which is the compiler-idiomatic pattern. None of them appears in the compiler's incompatible-library list, and the compiler changes no advice about them.

Two facts that predate the compiler and still hold:

- **Selectors do not need memoizing for subscription correctness.** react-redux moved `useSelector` onto `useSyncExternalStore` in v8, which supports unstable inline selectors without re-subscribing; Zustand removed its "memoizing selectors" docs section for the same reason. Memoize a selector only to avoid recomputation (reselect, proxy-memoize).
- **A selector returning a new object reference re-renders every time**, because comparison is `Object.is`. The compiler cannot fix this — the value is produced inside the store's own selector call, outside any component the compiler compiled. Select primitives, or pass a shallow comparator.

```tsx
// Re-renders on every store change: new object every call
const { a, b } = useStore((s) => ({ a: s.a, b: s.b }));

// Select primitives separately
const a = useStore((s) => s.a);
const b = useStore((s) => s.b);

// Or supply a comparator (XState Store)
const value = useSelector(actor, (s) => s.context.item, shallowEqual);
```

Calling `useSyncExternalStore` directly is covered in `effects.md` → *useSyncExternalStore over manual subscriptions*.

For `@xstate/store` specifically, use the `xstate-store` skill; for TanStack Query and Router, the `tanstack` skill.

## Libraries with interior mutability

The compiler's `incompatible-library` rule (severity **warn**) flags APIs that are unsafe to memoize. The cause is always **interior mutability**: an object or function holding hidden state that changes while its reference stays stable. React compares references only, so memoization freezes the UI.

The compiler's authoritative list contains exactly three modules:

| Module | Flagged API | Fix |
| --- | --- | --- |
| `react-hook-form` | `useForm().watch` only | Use `useWatch({ control, name })` |
| `@tanstack/react-table` | `useReactTable` | No compatible API yet — `"use no memo"` on the consuming component |
| `@tanstack/react-virtual` | `useVirtualizer`, `useWindowVirtualizer` | Same |

```tsx
// INVALID: watch() cannot be memoized safely — the value never updates
const { watch } = useForm();
const name = watch("name");

// VALID
const { control, register } = useForm();
const name = useWatch({ control, name: "name" });
```

**MobX** is incompatible via the `observer` HOC in `mobx-react` / `mobx-react-lite`, not MobX core, and the linter does not yet detect it ([mobxjs/mobx#3874](https://github.com/mobxjs/mobx/issues/3874)). Options: the `mobx-react-observer` Babel/SWC plugin, or hook-based observation.

Because `incompatible-library` is only a warning and can be suppressed by an unrelated `eslint-disable`, heavy TanStack Table or Virtual use quietly leaves whole components uncompiled. Audit with the healthcheck.

**If you author a library:** its APIs should be safe to memoize with `useMemo`. Return immutable state whose reference changes when the value changes, rather than a stable handle wrapping mutable state.
