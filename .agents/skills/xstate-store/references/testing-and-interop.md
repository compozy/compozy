# Testing, inspection, and XState interop

## Pure transitions

`store.transition(snapshot, event)` computes `[nextSnapshot, effects]` without touching the live store. This is the testing seam: transitions are pure functions of `(snapshot, event)`, so assertions need no mounting, no timers, and no cleanup.

```ts
const snapshot = store.getSnapshot();

const [nextState, effects] = store.transition(snapshot, { type: 'inc', by: 1 });

nextState.context;           // { count: 1 }
effects;                     // [{ type: 'incremented', by: 1 }, Function]
store.getSnapshot().context; // { count: 0 } — unchanged
```

Effects come back as an array mixing emitted event objects and effect callbacks, so a test can assert what *would* happen without running it.

Start from a known baseline with `store.getInitialSnapshot()`:

```ts
const [nextState] = store.transition(store.getInitialSnapshot(), { type: 'inc', by: 1 });
```

Chain transitions to test a sequence, threading the snapshot through:

```ts
let snapshot = store.getInitialSnapshot();
for (const event of [{ type: 'inc', by: 1 }, { type: 'inc', by: 2 }] as const) {
  [snapshot] = store.transition(snapshot, event);
}
snapshot.context.count; // 3
```

`store.can.*(...)` is the other pure check — assert guard behaviour without driving state.

For integration-level tests, build a throwaway instance from store logic (`logic.createStore(input)`) so each test gets a fresh store instead of sharing a module-scoped one.

## Inspection

`store.inspect(observer)` receives a single `@xstate.transition` event per transition, carrying `event` and `snapshot`. Inspectors get the current snapshot immediately, since the store is already started.

```ts
const sub = store.inspect((inspectionEvent) => {
  inspectionEvent.event;
  inspectionEvent.snapshot;
});

sub.unsubscribe();
```

Wire it to the Stately Inspector for a visual trace:

```ts
import { createBrowserInspector } from '@statelyai/inspect';

const inspector = createBrowserInspector();
store.inspect(inspector);
```

## `fromStore` — store logic as an XState actor

`fromStore(...)` turns a store definition into XState-compatible actor logic for `createActor(...)`. It accepts `schemas` and infers context, event, and emitted types from them.

```ts
import { fromStore } from '@xstate/store';
import { createActor } from 'xstate';

const storeLogic = fromStore({
  context: { count: 0, incremented: false },
  on: {
    inc: {
      count: (context, event) => context.count + 1,
      // static values need no function
      incremented: true,
    },
  },
});

const actor = createActor(storeLogic);
actor.subscribe((snapshot) => console.log(snapshot));
actor.start();

actor.send({ type: 'inc' });
```

`fromStore` returns *logic*, not a running store — it must be passed to `createActor`, and started. Input arrives through a context function:

```ts
const storeLogic = fromStore({
  context: (initialCount: number) => ({ count: initialCount }),
  on: { /* … */ },
});

const actor = createActor(storeLogic, { input: 42 });
```

Use this to drop a store into an existing actor system: spawn it as a child, invoke it from a machine, or keep one shared inspector across both.

## Converting a store into a machine

When a store grows modes, guards, delays, or cancellation, it has outgrown data-shaped state. Convert it:

1. `createMachine(…)` from `xstate` replaces `createStore(…)`.
2. Assignments move into an `assign(…)` action under the transition's `actions`.
3. `context` and `event` arrive destructured from a single argument.

```ts
import { assign, createMachine } from 'xstate';

const machine = createMachine({
  context: { count: 0, name: 'David' },
  on: {
    inc: {
      actions: assign({
        count: ({ context, event }) => context.count + event.by,
      }),
    },
  },
});
```

Add `setup(...)` for strong typing:

```ts
import { setup } from 'xstate';

const machine = setup({
  types: {
    context: {} as { count: number; name: string },
    events: {} as { type: 'inc'; by: number },
  },
}).createMachine({
  // as above
});
```
