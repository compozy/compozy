# Extensions

Extensions add behaviour to a store through `.with(extension)`, which returns a new store type carrying the extra triggers. Each ships from its own subpath.

```ts
import { createStore } from '@xstate/store';
import { undoRedo } from '@xstate/store/undo';
import { persist } from '@xstate/store/persist';
import { reset } from '@xstate/store/reset';
import { validateSchemas } from '@xstate/store/validate';

const store = createStore({ context: { count: 0 }, on: { /* … */ } })
  .with(undoRedo())
  .with(persist({ name: 'counter' }));
```

Each extension contributes its own event types (`undo`, `redo`, `reset`, …). In development builds, applying an extension whose internal event type collides with an existing store event throws with the conflicting names — rename the application event.

## `reset`

Adds `store.trigger.reset()`, returning context to its initial value.

```ts
const store = createStore({
  context: { count: 0 },
  on: { inc: (context) => ({ count: context.count + 1 }) },
}).with(reset());

store.trigger.inc();
store.trigger.reset();
store.getSnapshot().context.count; // 0
```

Pass `to` for a partial reset that keeps chosen fields:

```ts
.with(
  reset({
    to: (initial, current) => ({ ...initial, user: current.user }),
  }),
)
```

| Option | Type | Description |
| --- | --- | --- |
| `to` | `(initialContext, currentContext) => TContext` | Custom reset. Defaults to the initial context |

## `undoRedo`

Adds `store.trigger.undo()` and `store.trigger.redo()`.

```ts
const store = createStore({
  context: { count: 0 },
  on: { inc: (context) => ({ count: context.count + 1 }) },
}).with(undoRedo());

store.trigger.inc();  // 1
store.trigger.inc();  // 2
store.trigger.undo(); // 1
store.trigger.redo(); // 2
```

**Event strategy (default)** keeps the event log and rebuilds state by replaying from the initial snapshot. It is compact, and it requires transitions to be deterministic — a transition reading `Date.now()`, `Math.random()`, or module state replays to a different result.

**Snapshot strategy** stores past and future snapshots, so undo is instant at the cost of memory. Bound it with `historyLimit`.

```ts
.with(undoRedo({ strategy: 'snapshot', historyLimit: 50 }))
```

| Option | Type | Default | Strategy | Description |
| --- | --- | --- | --- | --- |
| `strategy` | `'event' \| 'snapshot'` | `'event'` | both | History storage strategy |
| `getTransactionId` | `(event, snapshot) => string \| null` | – | both | Group related events into one undo step |
| `skipEvent` | `(event, snapshot) => boolean` | – | both | Exclude events from history |
| `historyLimit` | `number` | `Infinity` | snapshot | Max snapshots kept |
| `compare` | `(past, current) => boolean` | – | snapshot | Skip duplicate snapshots |

Group a burst of events into a single undo step by returning a shared transaction id:

```ts
.with(undoRedo({ getTransactionId: (event) => event.batchId ?? null }))
```

Keep view-only state such as theme or selection out of history:

```ts
.with(undoRedo({ skipEvent: (event) => event.type === 'setTheme' }))
```

With the snapshot strategy, `compare` suppresses history entries when the meaningful part of state did not change:

```ts
.with(undoRedo({
  strategy: 'snapshot',
  compare: (past, current) => past.context.count === current.context.count,
}))
```

## `persist`

Saves and restores state, defaulting to `localStorage` with automatic hydration at store creation.

```ts
const store = createStore({
  context: { count: 0 },
  on: { inc: (context) => ({ count: context.count + 1 }) },
}).with(persist({ name: 'counter' }));
```

**Snapshot strategy (default)** writes the derived state on each event. **Event strategy** writes the event log and replays it on hydration, which suits event sourcing and audit trails; exceeding `maxEvents` writes a checkpoint and drops the oldest events.

```ts
.with(persist({ name: 'my-store', strategy: 'event', maxEvents: 100 }))
```

### Shared options

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | `string` | (required) | Storage key |
| `storage` | `StateStorage` | `localStorage` | Storage adapter |
| `version` | `string \| number` | `0` | Schema version for migrations |
| `throttle` | `number` | `0` | Minimum ms between writes |
| `skipHydration` | `boolean` | `false` | Skip automatic hydration (for async storage) |
| `onDone` | `(data) => void` | – | Called after a successful write |
| `onError` | `(error) => void` | – | Called on read/write failure |

### Snapshot-strategy options

| Option | Type | Description |
| --- | --- | --- |
| `filter` | `(event) => boolean` | Return `false` to skip persisting for an event |
| `pick` | `(context) => Partial<TContext>` | Persist only part of context |
| `migrate` | `(persistedContext, version) => TContext` | Version upgrade |
| `merge` | `(persisted, current) => TContext` | Rehydration merge; defaults to shallow |
| `serialize` / `deserialize` | `(value) => string` / `(str) => value` | Default `JSON.stringify` / `JSON.parse` |

### Event-strategy options

| Option | Type | Description |
| --- | --- | --- |
| `maxEvents` | `number` | Max events kept before a checkpoint |
| `migrate` | `(events, version) => events` | Version upgrade |
| `serialize` / `deserialize` | `(value) => string` / `(str) => value` | Custom codecs |

### Persisting a subset

Persist only what should survive a reload, and skip high-frequency events:

```ts
.with(persist({
  name: 'my-store',
  pick: (context) => ({ count: context.count }),
  filter: (event) => event.type !== 'hover',
  throttle: 1000,
}))
```

Throttled writes are pending until flushed:

```ts
import { flushStorage } from '@xstate/store/persist';

window.addEventListener('beforeunload', () => {
  flushStorage(store);
});
```

### Migrations

Bump `version` whenever the persisted shape changes, and map old payloads forward:

```ts
.with(persist({
  name: 'my-store',
  version: 2,
  migrate: (persisted, version) => {
    if (version === 1) {
      return { ...persisted, newField: 'default' };
    }
    return persisted;
  },
}))
```

### Async storage

`createJSONStorage` is SSR-safe (it returns noop storage when the backing store is unavailable). Async adapters — React Native `AsyncStorage`, IndexedDB — need `skipHydration` plus an explicit `rehydrateStore`.

```ts
import { persist, rehydrateStore, createJSONStorage } from '@xstate/store/persist';

const store = createStore({ /* … */ }).with(
  persist({
    name: 'my-store',
    storage: createJSONStorage(() => AsyncStorage),
    skipHydration: true,
  }),
);

await rehydrateStore(store);
```

Gate UI on `isHydrated(store)` while rehydration is in flight.

### Cross-tab sync

```ts
import {
  persist,
  createBroadcastStorage,
  subscribeToBroadcastStorage,
  createJSONStorage,
} from '@xstate/store/persist';

const storage = createBroadcastStorage(createJSONStorage(() => localStorage));

const store = createStore({ /* … */ }).with(persist({ name: 'my-store', storage }));

const unsubscribe = subscribeToBroadcastStorage(store);
```

When any tab writes, the others rehydrate.

### Helpers

All from `@xstate/store/persist`:

| Function | Description |
| --- | --- |
| `clearStorage(store)` | Remove persisted data |
| `flushStorage(store)` | Force pending throttled write |
| `isHydrated(store)` | Whether the store has hydrated |
| `rehydrateStore(store)` | Manual rehydration for async storage |
| `createJSONStorage(getStorage)` | SSR-safe adapter factory |
| `createBroadcastStorage(storage, options?)` | Cross-tab wrapper |
| `subscribeToBroadcastStorage(store)` | Listen for cross-tab updates |

`clearStorage` and `flushStorage` may return a `Promise` when the adapter is async.

## `validateSchemas`

Runtime checking of schema-declared context, events, and emitted events — see `references/schemas-and-validation.md`.
