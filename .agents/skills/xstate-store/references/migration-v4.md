# Migrating to v4, and porting from Zustand

Current versions: `@xstate/store` 4.x, framework packages (`@xstate/store-react`, `-vue`, `-svelte`, `-solid`, `-angular`, `-preact`) 2.x. TypeScript 5.4+ is required.

## Removed APIs

**Positional `createStore(context, transitions)`** → config object.

```diff
- const store = createStore(
-   { count: 0 },
-   { inc: (context) => ({ count: context.count + 1 }) }
- );
+ const store = createStore({
+   context: { count: 0 },
+   on: { inc: (context) => ({ count: context.count + 1 }) }
+ });
```

**`createStoreWithProducer`** → call the producer inside the transition.

```diff
- import { createStoreWithProducer } from '@xstate/store';
+ import { createStore } from '@xstate/store';
  import { produce } from 'immer';

- const store = createStoreWithProducer(produce, {
+ const store = createStore({
    context: { count: 0 },
    on: {
-     inc: (context) => { context.count++; }
+     inc: (context) => produce(context, (draft) => { draft.count++; })
    }
  });
```

Transitions that disallow an event need care with producers — see the Immer section of `references/store-core.md`.

**Framework subpaths** → dedicated packages.

```diff
- import { useSelector } from '@xstate/store/react';
+ import { useSelector } from '@xstate/store-react';
```

`@xstate/store/react` and `@xstate/store/solid` no longer resolve. The framework package re-exports the core, so `@xstate/store` need not be imported separately.

**`undoRedo(config)` wrapper** → the extension form.

```diff
- const store = createStore(
-   undoRedo({ context: { count: 0 }, on: { /* … */ } })
- );
+ const store = createStore({ context: { count: 0 }, on: { /* … */ } })
+   .with(undoRedo());
```

**`store._snapshot`** → `store.getSnapshot()` or `store.get()`.

## Changed APIs

**`emits` → `schemas.emitted`.** Only the declaration moved; `enq.emit.*` is unchanged.

```diff
  const store = createStore({
    context: { count: 0 },
-   emits: {
-     increased: (_payload: { by: number }) => {}
-   },
+   schemas: {
+     emitted: { increased: z.object({ by: z.number() }) }
+   },
    on: {
      inc: (context, event: { by: number }, enq) => {
        enq.emit.increased({ by: event.by });
        return { count: context.count + event.by };
      }
    }
  });
```

**Computed atoms lost the `read` argument.** The first parameter is now the atom's previous value; dependencies are read with `.get()`.

```diff
- const doubled = createAtom((read) => read(countAtom) * 2);
+ const doubled = createAtom(() => countAtom.get() * 2);

- const withPrev = createAtom((read, prev) => read(countAtom) + (prev ?? 0));
+ const withPrev = createAtom<number>((prev) => countAtom.get() + (prev ?? 0));
```

**`store.inspect(...)` emits one event type.** `@xstate.actor`, `@xstate.event`, and `@xstate.snapshot` are replaced by a single `@xstate.transition` carrying `event` and `snapshot`.

```diff
  store.inspect((inspectionEvent) => {
-   // '@xstate.actor' | '@xstate.event' | '@xstate.snapshot'
+   // '@xstate.transition'
+   inspectionEvent.event;
+   inspectionEvent.snapshot;
  });
```

**`persist(...)` writes snapshots.** The persisted value is derived from the full store snapshot. `clearStorage(store)` and `flushStorage(store)` may return a `Promise` with async adapters.

## New in v4

Reach for these while migrating rather than after:

- `schemas` for context/events/emitted, plus `.with(validateSchemas())` for runtime checks — `references/schemas-and-validation.md`
- `createStoreLogic(...)` with `selectors` and `input` for per-instance stores — `references/store-logic-and-input.md`
- `persist`, `reset`, and `undoRedo` as composable `.with(...)` extensions — `references/extensions.md`
- `useStore`, `useAtomState`, and `createStoreHook` in `@xstate/store-react` v2 — `references/react.md`

## Migration order

1. Rewrite `createStore` calls to the config form and replace `createStoreWithProducer`.
2. Repoint framework imports at the dedicated packages.
3. Move `emits` into `schemas.emitted` and rewrite computed atom getters.
4. Convert `undoRedo(config)` and any wrapper-style setup into `.with(...)` chains.
5. Update `store._snapshot` readers and inspection handlers.
6. Typecheck; the compiler surfaces the remaining call-site changes.

## Porting from Zustand

| Zustand | XState Store |
| --- | --- |
| `create(...)` store state | `createStore({ context })` or `createAtom(...)` |
| action method | transition in `on` |
| `set(...)` | the context returned from a transition, or `atom.set(...)` |
| hook selector | `useSelector(store, selector)` |
| middleware | an extension, or explicit `enqueue.effect(...)` |

```ts
// Zustand
const useCountStore = create((set) => ({
  count: 0,
  inc: (by: number) => set((state) => ({ count: state.count + by })),
}));

// XState Store
const store = createStore({
  context: { count: 0 },
  on: {
    inc: (context, event: { by: number }) => ({ count: context.count + event.by }),
  },
});
store.trigger.inc({ by: 1 });
```

The structural difference: Zustand keeps state and actions in one object reached through a hook; XState Store keeps `context` as data and `on` as named transitions, with the core framework-neutral and the React binding a thin adapter. That buys typed `trigger`, `can` checks, emitted events, and pure `transition(...)` testing. Zustand stays the better fit when the team is already fluent in it, when direct setters model the state naturally, or when ecosystem breadth matters more than the event vocabulary.
