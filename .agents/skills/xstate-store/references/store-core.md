# Store core

`createStore(config)` builds a started store. The config takes `context` (initial state), `on` (event type → transition function), and optionally `schemas`.

```ts
import { createStore } from '@xstate/store';

const store = createStore({
  context: { count: 0, name: 'David' },
  on: {
    inc: (context, event: { by: number }) => ({
      ...context,
      count: context.count + event.by,
    }),
  },
});
```

## Transitions return the whole context

A transition receives `(context, event, enqueue)` and returns the **complete** next context. Spread the previous context so untouched fields survive.

```ts
on: {
  rename: (context, event: { name: string }) => ({
    ...context,
    name: event.name,
  }),
}
```

Type the event payload inline on the second parameter (`event: { by: number }`) — that is what makes `store.trigger.inc({ by: 1 })` type-safe. Declare payloads in `schemas.events` instead when the contract should also exist at runtime (`references/schemas-and-validation.md`).

## Sending events

```ts
store.trigger.inc({ by: 2 });      // typed, preferred
store.send({ type: 'inc', by: 2 }); // equivalent object form
```

`trigger` is the default: it needs no hand-built event object and it autocompletes. Reach for `send` when the event object is already built (forwarding, replay, generic plumbing).

## Disallowing an event

Return `undefined` (or a bare `return;`) to say the event does not apply in the current context. `store.can.*()` reports that decision without mutating anything.

```ts
const store = createStore({
  context: { count: 0 },
  on: {
    inc: (context, event: { by: number }) => {
      if (context.count + event.by > 10) {
        return;
      }
      return { count: context.count + event.by };
    },
  },
});

store.can.inc({ by: 4 });  // true
store.can.inc({ by: 11 }); // false
```

Returning the *same* context object is still an allowed transition. A transition that enqueues an effect, an emitted event, or a triggered event is allowed too. Use `can` to drive disabled buttons and menu items instead of duplicating the guard in the view.

## Reading state

```ts
store.getSnapshot().context; // { count: 0, name: 'David' }
store.get();                 // same snapshot, `Readable` form
store.getInitialSnapshot();  // snapshot the store started from
```

For anything that re-renders or re-runs on change, subscribe through a selector rather than polling `getSnapshot()` — see `references/selectors-and-atoms.md`.

```ts
const sub = store.subscribe((snapshot) => {
  console.log(snapshot.context);
});
sub.unsubscribe();
```

The store is started on creation, so subscribers receive updates from the next event onward; read the current value with `get()` when the initial value is needed.

## Immer

`produce(...)` is called inside the transition and its result returned. There is no `createStoreWithProducer` in v4.

```ts
import { produce } from 'immer';

const store = createStore({
  context: { count: 0, todos: [] as string[] },
  on: {
    addTodo: (context, event: { todo: string }) =>
      produce(context, (draft) => {
        draft.todos.push(event.todo);
      }),
    resetCount: (context) => ({ ...context, count: 0 }),
  },
});
```

To disallow the event, return before calling `produce`:

```ts
eatDonut: (context) => {
  if (context.donuts === 0) {
    return;
  }
  return produce(context, (draft) => {
    draft.donuts--;
  });
},
```

Immer treats a producer that returns `undefined` as "no explicit return". When `produce(...)` itself must yield `undefined`, return Immer's `nothing` token from the producer.

## Naming

Event types read as things that happened or commands issued (`inc`, `todoAdded`, `filterChanged`), not as setters (`setCount`). A store whose events are all `setX` is a signal that atoms (`references/selectors-and-atoms.md`) fit better.
