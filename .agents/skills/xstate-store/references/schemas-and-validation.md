# Schemas and validation

v4 accepts any [Standard Schema](https://standardschema.dev/) library — Zod, Valibot, ArkType. Schemas declare the store's contract in three slots: `context`, `events`, `emitted`.

By default schemas are a **type source and runtime-readable metadata**. They do not validate anything until `validateSchemas()` is applied.

## Declaring schemas

```ts
import { createStore } from '@xstate/store';
import { z } from 'zod';

const store = createStore({
  schemas: {
    context: z.object({
      count: z.number(),
      label: z.string(),
    }),
    events: {
      rename: z.object({ label: z.string() }),
      reset: z.object({}),
    },
    emitted: {
      renamed: z.object({ label: z.string() }),
    },
  },
  context: { count: 0, label: 'ready' },
  on: {
    rename: (context, event, enqueue) => {
      enqueue.emit.renamed({ label: event.label });
      return { ...context, label: event.label };
    },
  },
});

store.trigger.rename({ label: 'done' });
store.trigger.reset();
```

Event and emitted schemas describe payload **objects**; use `z.object({})` for a payload-free event.

When an event is declared in `schemas.events`, its handler in `on` is optional. A missing handler is a no-op, but the event still exists for typing and for `store.trigger` — useful for events that only extensions or future code will handle.

With `schemas.events` present, the transition's `event` parameter is already typed, so drop the inline `event: { … }` annotation.

## Reading schemas at runtime

```ts
store.schemas?.events?.inc;
```

The exposed schemas drive form generation, API docs, and dev tooling. `fromStore(...)` accepts `schemas` too and infers its context, event, and emitted types from them.

## Opting into validation

```ts
import { createStore } from '@xstate/store';
import { validateSchemas } from '@xstate/store/validate';
import { z } from 'zod';

const store = createStore({
  schemas: {
    context: z.object({ count: z.number() }),
    events: { increment: z.object({ by: z.number() }) },
  },
  context: { count: 0 },
  on: {
    increment: (context, event) => ({ count: context.count + event.by }),
  },
}).with(validateSchemas());

store.trigger.increment({ by: 1 });               // ok
store.trigger.increment({ by: 'two' as any });    // throws StoreValidationError
```

`validateSchemas()` checks, in order:

1. initial context at store creation,
2. incoming events before each transition,
3. context after each transition,
4. emitted events before effects execute.

`send(...)`, `trigger.*(...)`, and `transition(...)` throw on failure. `store.can.*(...)` stays boolean-only and returns `false` instead of throwing.

## Options

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `context` | `boolean` | `true` | Validate context at init and after each transition |
| `events` | `boolean` | `true` | Validate incoming events before transitions |
| `emitted` | `boolean` | `true` | Validate emitted events before effects |
| `unknownEvents` | `'throw' \| 'ignore'` | `'throw'` | Events absent from `schemas.events` |
| `unknownEmitted` | `'throw' \| 'ignore'` | `'throw'` | Emitted events absent from `schemas.emitted` |

```ts
store.with(
  validateSchemas({
    context: false,
    events: true,
    emitted: true,
    unknownEvents: 'ignore',
  }),
);
```

Set `unknownEvents: 'ignore'` while migrating a store whose event list is only partly declared.

## `StoreValidationError`

```ts
import { StoreValidationError } from '@xstate/store/validate';

try {
  store.trigger.inc({ by: 'bad' as any });
} catch (error) {
  if (error instanceof StoreValidationError) {
    error.reason;    // 'invalidEvent' | 'invalidContext' | 'invalidEmitted' | 'unknownEvent' | 'unknownEmitted'
    error.eventType; // 'inc'
    error.payload;   // { by: 'bad' }
    error.issues;    // issues from the schema library
  }
}
```

## Where to apply validation

Validation costs a schema parse per event. Apply it where untyped data crosses into the store — deserialized persistence, URL or query params, messages from a worker, third-party callbacks — and in test and development builds. For a store fed only by typed application code, the types already carry the contract.
