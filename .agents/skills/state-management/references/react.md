# React

Use this reference when the task is implemented with React, React Native, or React-compatible component state patterns.

## Strong defaults

- Derive values during render when possible.
- Prefer event handlers over `useEffect` for logic triggered by user actions.
- Use `useReducer` when several fields change together or transitions depend on current state.
- Use `useRef` for mutable values that do not belong in rendering.
- Use `useSyncExternalStore` only for actual external stores or browser APIs, not as a workaround for local modeling problems.

## What to store

Prefer storing:

- IDs instead of whole selected objects
- a single finite `status` instead of several request booleans
- user input and durable local facts

Prefer deriving:

- filtered/sorted lists
- counts, labels, and validation summaries
- `isEmpty`, `hasItems`, `canSubmit`, `isDirty`, and similar booleans
- selected object lookup from `selectedId` plus entity collection

## When `useState` is enough

Use local `useState` when:

- the values are truly independent
- the state is owned by one component or a tight subtree
- transitions are simple and do not need coordinated event handling

Move to `useReducer` when:

- one event updates several fields
- current state determines whether an event is valid
- the code is drifting toward many handler branches and synchronization effects

## Common React smells

- `useEffect` that copies props into state
- `useEffect` that recalculates derived values and writes them back into state
- multiple booleans like `isLoading`, `isSuccess`, `isError`, `isIdle`
- storing fetched data, cache freshness, and local draft state in one component blob
- storing objects in state when IDs are the durable source of truth

## Review guidance

When refactoring React code:

1. Delete synchronization effects first.
2. Move derivations into render or small selectors/helpers.
3. Collapse invalid boolean combinations into one `status`.
4. Introduce `useReducer` only if it clarifies transitions.
5. Push server state into a query/cache tool when that is the real concern.

## Example shape

```ts
type Status = 'idle' | 'editing' | 'submitting' | 'success' | 'failure';

type State = {
  status: Status;
  draft: DraftForm;
  error: string | null;
};
```

If `error` only exists in `failure`, prefer a discriminated union instead of a loose nullable field.
