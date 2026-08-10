# React

`@xstate/store-react` v2 re-exports everything from `@xstate/store`, so it is the only install needed. Importing bindings from `@xstate/store/react` is gone in v4.

```bash
npm install @xstate/store-react
```

```tsx
import { createStore, useSelector } from '@xstate/store-react';

const store = createStore({
  context: { count: 0 },
  on: {
    inc: (context, event: { by?: number }) => ({
      count: context.count + (event.by ?? 1),
    }),
  },
});

function Counter() {
  const count = useSelector(store, (state) => state.context.count);

  return (
    <div>
      <p>Count: {count}</p>
      <button onClick={() => store.trigger.inc()}>+1</button>
      <button onClick={() => store.trigger.inc({ by: 5 })}>+5</button>
    </div>
  );
}
```

Events are dispatched by calling `store.trigger.*` directly in handlers — no dispatch hook, no provider.

## `useSelector(store, selector?, compare?)`

Subscribes to a store, selection, or atom and re-renders when the selected value changes.

```tsx
const count = useSelector(store, (state) => state.context.count);

const user = useSelector(
  store,
  (state) => state.context.user,
  (prev, next) => prev?.id === next?.id,
);
```

The selector receives the **snapshot**, so context lives at `state.context`. Comparison defaults to `===`; a selector building a new object or array needs `shallowEqual` or a custom `compare`, otherwise it re-renders on every event. Selecting a primitive per hook call is the cheapest option.

`compare`'s first argument is typed `T | undefined` (there is no previous value on first render), so guard it with optional chaining.

Omitting the selector subscribes to the whole snapshot — convenient, and it re-renders on every event.

## `useStore(configOrLogic, input?)`

Creates a store scoped to the component, stable across re-renders. Accepts a config object or logic from `createStoreLogic(...)`.

```tsx
import { createStoreLogic, useSelector, useStore } from '@xstate/store-react';

const counterLogic = createStoreLogic({
  context: (input: { initialCount: number }) => ({ count: input.initialCount }),
  on: {
    inc: (context) => ({ count: context.count + 1 }),
    dec: (context) => ({ count: context.count - 1 }),
  },
});

function Counter() {
  const store = useStore(counterLogic, { initialCount: 0 });
  const count = useSelector(store, (state) => state.context.count);

  return (
    <div>
      <p>Count: {count}</p>
      <button onClick={() => store.trigger.inc()}>+</button>
      <button onClick={() => store.trigger.dec()}>-</button>
    </div>
  );
}
```

`input` is required when the logic's `context` function requires it, optional when it does not. Input is read when the store is created, so later prop changes do not reinitialize it — send an event to react to changing props.

Share a component-scoped store with descendants through React context, passing the store object itself.

## `useAtom(atomOrConfig, selectorOrInput?, compare?)`

Subscribes to an atom and returns its value.

```tsx
import { createAtom, useAtom } from '@xstate/store-react';

const countAtom = createAtom(0);

function Counter() {
  const count = useAtom(countAtom);

  return <button onClick={() => countAtom.set((prev) => prev + 1)}>{count}</button>;
}
```

With an existing atom, the second argument is a selector and the third a comparison. With an atom config, the second argument is the input:

```tsx
import { createAtomConfig, useAtom } from '@xstate/store-react';

const countConfig = createAtomConfig((input: { initialCount: number }) => input.initialCount);

function Counter() {
  const count = useAtom(countConfig, { initialCount: 0 });
  return <div>Count: {count}</div>;
}
```

## `useAtomState(atomOrConfig, input?)`

Returns `[value, atom]` — the `useState`-shaped form, and the way to create a component-scoped atom from a config.

```tsx
import { createAtomConfig, useAtomState } from '@xstate/store-react';

const countConfig = createAtomConfig((input: { initialCount: number }) => input.initialCount);

function Counter() {
  const [count, countAtom] = useAtomState(countConfig, { initialCount: 0 });

  return <button onClick={() => countAtom.set((c) => c + 1)}>{count}</button>;
}
```

## `createStoreHook(config)`

Builds a custom hook that creates the store and selects from it in one call, returning `[selectedValue, store]`.

```tsx
import { createStoreHook } from '@xstate/store-react';

const useCountStore = createStoreHook({
  context: { count: 0 },
  on: {
    inc: (context, event: { by: number }) => ({ count: context.count + event.by }),
  },
});

function Counter() {
  const [count, store] = useCountStore((s) => s.context.count);

  return (
    <div>
      <p>Count: {count}</p>
      <button onClick={() => store.trigger.inc({ by: 1 })}>+1</button>
    </div>
  );
}
```

Called with no selector it returns `[snapshot, store]`.

## Scope

| Scope | Pattern |
| --- | --- |
| App-wide state shared across routes | `createStore(...)` at module scope, `useSelector` where read |
| State per component instance | `createStoreLogic(...)` + `useStore(logic, input)` |
| A single local reactive value | `createAtomConfig(...)` + `useAtomState` |
| Local state with no cross-component reach | plain `useState` |

Module-scoped stores persist for the page's lifetime and are shared by every mount — which is the point for app state, and a leak for per-screen state. On the server they are shared across requests, so keep request-scoped data out of them.

## Emitted events in components

Subscribe in an effect and unsubscribe on cleanup:

```tsx
useEffect(() => {
  const sub = store.on('increased', (event) => {
    toast(`+${event.by}`);
  });
  return () => sub.unsubscribe();
}, []);
```

## Reading without subscribing

Inside event handlers, read the current value with `store.getSnapshot().context` or `selection.get()` rather than adding a `useSelector` whose only job is to feed a callback — that avoids re-rendering for a value the render output never uses.
