# Store logic, input, and named selectors

`createStore(...)` produces one live store. `createStoreLogic(...)` produces a reusable **definition** that creates stores on demand — the shape needed for per-instance state (one store per row, per dialog, per component) and for framework hooks.

```ts
import { createStoreLogic } from '@xstate/store';

const counterLogic = createStoreLogic({
  context: (input: { initialCount: number }) => ({ count: input.initialCount }),
  selectors: {
    count: (context) => context.count,
    doubled: (context) => context.count * 2,
  },
  on: {
    inc: (context) => ({ count: context.count + 1 }),
  },
});

const store = counterLogic.createStore({ initialCount: 2 });

store.selectors.count.get();   // 2
store.selectors.doubled.get(); // 4
```

## Input

Declaring `context` as a function of input makes each instance start differently. The input type is inferred from that function's parameter.

```ts
const counterLogic = createStoreLogic({
  context: (input: { initialCount: number }) => ({ count: input.initialCount }),
  on: {
    inc: (context) => ({ count: context.count + 1 }),
  },
});

const store = counterLogic.createStore({ initialCount: 0 });
```

Input is required wherever the logic is instantiated, including React's `useStore(counterLogic, { initialCount: 0 })` (`references/react.md`). Logic whose `context` is a plain object takes no input and `createStore()` is called bare.

## Named selectors

The `selectors` config resolves each entry into a `Selection` on `store.selectors`, with the same `get()`/`subscribe()` API as `store.select(...)`. Defining selectors on the logic keeps the derivation next to the state it derives from and gives every instance the same vocabulary — preferable to re-creating `store.select(...)` at each call site.

## `createStoreConfig`

`createStoreConfig(config)` returns the config object unchanged, typed. It exists to get inference and editor feedback on a config declared apart from its `createStore` call — for example a config shared between a live store and a test store.

```ts
import { createStore, createStoreConfig } from '@xstate/store';

export const counterConfig = createStoreConfig({
  context: { count: 0 },
  on: { inc: (context) => ({ count: context.count + 1 }) },
});

const store = createStore(counterConfig);
```

## Choosing between them

| Situation | Use |
| --- | --- |
| One app-wide store, created at module scope | `createStore(config)` |
| Many instances, or initial state from props/route/row data | `createStoreLogic({ context: (input) => … })` |
| A config reused across live and test stores | `createStoreConfig(config)` |

A store logic instantiated per component keeps state scoped to that component's lifetime, so unmounting discards it — the fix for module-scoped stores leaking values between mounts.

## Extensions and selectors

`.with(...)` preserves `selectors`, so extension order is free:

```ts
const store = counterLogic.createStore({ initialCount: 0 }).with(undoRedo());
store.selectors.doubled.get();
```
