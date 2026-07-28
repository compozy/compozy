# Selectors and atoms

## Selectors

`store.select(selector, equalityFn?)` returns a `Selection`: a readable, subscribable view of one slice of context. Subscribers fire only when the selected value changes, so unrelated events cost nothing.

```ts
const store = createStore({
  context: { position: { x: 0, y: 0 }, name: 'John', age: 30 },
  on: {
    positionUpdated: (context, event: { position: { x: number; y: number } }) => ({
      ...context,
      position: event.position,
    }),
  },
});

const position = store.select((context) => context.position);

position.get(); // { x: 0, y: 0 }
const sub = position.subscribe((value) => console.log('Position:', value));
sub.unsubscribe();
```

Selections are the unit to pass around: a module can export `store.select(...)` views and keep the store itself private.

### Equality

The default comparison is `===`, so a selector returning a fresh object or array notifies on every event. Either select a primitive, or pass an equality function.

```ts
import { shallowEqual } from '@xstate/store';

const position = store.select((context) => context.position, shallowEqual);

// Or a field-specific comparison
const byX = store.select(
  (context) => context.position,
  (prev, next) => prev.x === next.x,
);
```

## Atoms

An atom is a standalone reactive value: `get()`, `set()`, `subscribe()`. Atoms suit state with no event vocabulary — a toggle, a hovered id, a computed value bridging other sources. State that needs named transitions or `can` checks belongs in a store.

```ts
import { createAtom } from '@xstate/store';

const countAtom = createAtom(0);
const userAtom = createAtom({ name: 'David', count: 100 });

countAtom.get();                  // 0
countAtom.set(1);                 // 1
countAtom.set((prev) => prev + 1); // 2

const sub = countAtom.subscribe((value) => console.log('Count:', value));
sub.unsubscribe();
```

`createAtom(value, { compare })` accepts a custom equality function to suppress no-op notifications.

### Derived atoms

Passing a getter creates a **read-only** computed atom. It reads its dependencies with `.get()` and recomputes when any of them change.

```ts
const nameAtom = createAtom('David');
const ageAtom = createAtom(30);

const userAtom = createAtom(() => ({
  name: nameAtom.get(),
  age: ageAtom.get(),
}));

nameAtom.set('John');
userAtom.get(); // { name: 'John', age: 30 }
```

To update several values as one unit, use a store — derived atoms are read-only by design.

### Previous value in computed atoms

The getter's first parameter is the atom's own previous computed value, `undefined` on first computation. TypeScript cannot infer it, so supply the type parameter.

```ts
const countAtom = createAtom(0);

const totalAtom = createAtom<number>((prev) => countAtom.get() + (prev ?? 0));

totalAtom.get(); // 0
countAtom.set(5);
totalAtom.get(); // 5
countAtom.set(3);
totalAtom.get(); // 8
```

### Reducer atoms

`createReducerAtom(initialValue, reducer)` gives an atom an event-driven update path via `send`.

```ts
import { createReducerAtom } from '@xstate/store';

const countAtom = createReducerAtom(0, (state, event: { type: 'inc' }) => {
  if (event.type === 'inc') {
    return state + 1;
  }
  return state;
});

countAtom.send({ type: 'inc' });
```

### Atom configs

`createAtomConfig(...)` defines an inert atom that is instantiated later — the atom equivalent of store logic, and what React's `useAtom`/`useAtomState` use for component-scoped atoms.

```ts
import { createAtomConfig } from '@xstate/store';

const countConfig = createAtomConfig((input: { initialCount: number }) => input.initialCount);

const countAtom = countConfig.createAtom({ initialCount: 0 });
```

### Async atoms

`createAsyncAtom(getValue, options?)` wraps an async getter. Its value is a tagged state, and the getter receives an `AbortSignal` that is aborted when the atom recomputes, so stale resolutions are discarded.

```ts
import { createAsyncAtom } from '@xstate/store';

const userAtom = createAsyncAtom(async ({ signal }) => {
  const response = await fetch('/api/user', { signal });
  return response.json();
});

userAtom.subscribe((snapshot) => {
  if (snapshot.status === 'pending') {
    // {}
  } else if (snapshot.status === 'done') {
    snapshot.data;
  } else if (snapshot.status === 'error') {
    snapshot.error;
  }
});
```

Async atoms cover a value loaded once. Server state that needs caching, invalidation, and refetching belongs in TanStack Query.

## Bridging stores and atoms

A selection is a `Readable`, so a computed atom can read it with `.get()`:

```ts
const countSelector = store.select((context) => context.count);
const doubleCountAtom = createAtom(() => 2 * countSelector.get());

doubleCountAtom.get(); // 0
store.trigger.increment();
doubleCountAtom.get(); // 2
```

This is the seam for combining evented domain state with local reactive values without merging them into one store.
