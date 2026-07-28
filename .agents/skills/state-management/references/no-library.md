# No Library

Use this reference when the user wants plain JavaScript or TypeScript patterns without committing to a state library.

## Strong default

Model state as:

- a state shape
- a list of events
- a pure transition function
- selectors for derived reads
- an imperative shell for effects

## Minimal pattern

```ts
type State = {
  status: 'idle' | 'editing' | 'submitting' | 'failure';
  draft: Draft;
  error: string | null;
};

type Event =
  | { type: 'draft.changed'; value: Draft }
  | { type: 'submit.requested' }
  | { type: 'submit.succeeded' }
  | { type: 'submit.failed'; error: string };

function transition(state: State, event: Event): State {
  switch (event.type) {
    case 'draft.changed':
      return { ...state, draft: event.value };
    case 'submit.requested':
      return { ...state, status: 'submitting', error: null };
    case 'submit.succeeded':
      return { ...state, status: 'idle' };
    case 'submit.failed':
      return { ...state, status: 'failure', error: event.error };
  }
}
```

This is often enough for modeling and review, even if the final implementation later moves to React, Redux, Zustand, or Vue.

## Guidance

- Keep transitions pure.
- Keep effects outside the transition function.
- Use selectors for derived state.
- Use discriminated unions when fields only make sense in certain modes.
- Reach for a fuller state machine only when the workflow truly needs it.
