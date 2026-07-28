# Effects and emitted events

The third transition parameter, `enqueue`, is how a transition schedules everything that is not a context update. Enqueued work runs after the transition returns, once the snapshot is committed.

## The synchronous rule

`enqueue.effect(...)`, `enqueue.emit.*(...)`, and `enqueue.trigger.*(...)` must be called during the transition's own synchronous execution. Calling them later — inside a promise callback, a `setTimeout`, an `await` continuation — does nothing, because the transition has already completed.

This is what keeps the store deterministic: every state change, effect, and emitted event is the direct result of one event being sent.

## `enqueue.effect` — side effects

```ts
const store = createStore({
  context: { count: 0 },
  on: {
    incrementDelayed: (context, event, enqueue) => {
      enqueue.effect(() => {
        setTimeout(() => {
          store.trigger.increment();
        }, 1000);
      });
      return context;
    },
    increment: (context) => ({ count: context.count + 1 }),
  },
});
```

## Async work

The effect callback itself may be async. It reports its result by triggering another event, so the resulting state change is still event-driven.

```ts
import { z } from 'zod';

const store = createStore({
  context: { data: null as string | null },
  schemas: {
    emitted: {
      loaded: z.object({ data: z.string() }),
    },
  },
  on: {
    fetchData: (context, event, enqueue) => {
      enqueue.effect(async () => {
        const result = await fetch('/api/data');
        const data = await result.json();
        store.trigger.dataLoaded({ data });
      });
      return context;
    },
    dataLoaded: (context, event: { data: string }, enqueue) => {
      enqueue.emit.loaded({ data: event.data });
      return { ...context, data: event.data };
    },
  },
});
```

Model the full lifecycle in context (`status: 'idle' | 'loading' | 'error'`) rather than leaving in-flight requests invisible. When the lifecycle grows guards, cancellation, or retries, that is the point to move to an XState machine.

## `enqueue.trigger` — chaining events

```ts
on: {
  inc: (context) => ({ count: context.count + 1 }),
  incTwice: (context, _event, enqueue) => {
    enqueue.trigger.inc();
    enqueue.trigger.inc();
    return context;
  },
}
```

Each enqueued event runs its own transition after the current one commits.

## `enqueue.emit` — outward notifications

Emitted events are the store's outbound channel: they tell listeners that something happened without putting that fact in context. Declare their payloads in `schemas.emitted`.

```ts
import { z } from 'zod';

const store = createStore({
  context: { count: 0 },
  schemas: {
    emitted: {
      increased: z.object({ by: z.number() }),
    },
  },
  on: {
    inc: (context, event: { by: number }, enqueue) => {
      enqueue.emit.increased({ by: event.by });
      return { ...context, count: context.count + event.by };
    },
  },
});
```

Listen with `store.on(type, handler)`, which returns a subscription:

```ts
const sub = store.on('increased', (event) => {
  console.log(`Count increased by ${event.by}`);
});
sub.unsubscribe();
```

`store.on('*', handler)` receives every emitted event.

Emit for one-shot occurrences a component should react to — a toast, an analytics call, a scroll-to-bottom. Keep durable facts in context, where selectors can read them.

`schemas.emitted` types the payload; runtime checking arrives only with `.with(validateSchemas())` (`references/schemas-and-validation.md`).
