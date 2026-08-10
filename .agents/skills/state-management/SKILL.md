---
name: state-management
description: Model, review, and refactor application state so source state stays minimal, derived state is computed instead of synchronized, impossible states are not representable, and each piece of state lives in the right place. Use this whenever the user mentions state management, reducers, stores, Redux, Zustand, React state, Vue state, Pinia, selectors, derived state, duplicated state, boolean flags, invalid states, async status, useEffect soup, forms, wizards, filters, URL state, server state, or confusing UI logic, even if they do not explicitly ask for a "state model."
---

# State Management

Use this skill for **state modeling first** and library mapping second.

The goal is not to recommend a favorite library. The goal is to produce a state model that remains sound whether the implementation ends up in local component state, a reducer, Redux, Zustand, Pinia, URL params, TanStack Query, or no library at all.

Your job:

- reduce state to the smallest valid source of truth
- move derived values into selectors, getters, computed values, or render-time calculations
- replace boolean soup with explicit finite states when modes are mutually exclusive
- define state changes in terms of events and transitions rather than arbitrary setter calls
- place each piece of state in the right home
- review existing code for invalid states, duplication, synchronization bugs, and misplaced effects
- map the resulting model to the user's chosen library only after the model is clear

## First pass

In an existing codebase:

1. Inspect local `package.json` files and imports.
2. Read the component, store, reducer, and selector/getter code near the problem.
3. Identify which state is local, global, remote, URL-backed, or external.
4. Separate current implementation details from the actual domain model before suggesting changes.

If the user wants a new design from scratch, model the domain directly and only then choose implementation.

## Task mode

Choose a mode before answering:

- **Design**: build a new state model from requirements.
- **Review**: identify bugs, risks, invalid states, and misplaced state.
- **Refactor**: preserve behavior while simplifying the model and tightening invariants.
- **Library mapping**: translate an already-sound model into React, Redux, Zustand, Vue/Pinia, or plain functions.

Do not jump straight to API advice when the real problem is a broken model.

## Core taxonomy

Classify every value before deciding where it belongs.

- **Source state**: authoritative facts the app must store.
- **Derived state**: values that can be computed from source state and props/input. Do not store these unless there is a proven boundary reason.
- **Finite state**: mutually exclusive modes such as `idle`, `editing`, `submitting`, `success`, `failure`.
- **Server state**: remote data with loading, freshness, cache, and invalidation concerns.
- **URL state**: values that should survive refresh, support sharing, or participate in back/forward navigation.
- **Ephemeral mutable values**: timers, DOM handles, `AbortController`s, previous snapshots, subscriptions, and other non-visual mutable values.
- **History state**: undo/redo stacks, audit logs, previous states, and snapshots.

If a value does not clearly fit, you probably have not understood its role yet.

## Core principles

- Keep source state minimal.
- Prefer one source of truth for each fact.
- Compute derived values instead of synchronizing them.
- Represent mutually exclusive modes with one finite `status` field or a discriminated union, not several booleans.
- Make invalid states hard or impossible to represent.
- Model updates as domain events and transitions.
- Keep effects at the boundary. Effects should react to state transitions, not serve as the primary coordination mechanism.
- Put state as low as possible, but as high as necessary.
- Distinguish client state from server cache and URL state.
- If several values always change together, model that relationship explicitly.

## Workflow

Use this sequence unless the user explicitly asks for something narrower.

1. Inventory all current or proposed state values.
2. Classify each value using the taxonomy above.
3. Remove duplication:
   - delete mirrored props
   - delete values that can be derived
   - avoid storing both an object and its selected ID unless there is a clear cache boundary
4. Define invariants:
   - what combinations are valid?
   - what combinations are impossible?
   - what must always be true in each mode?
5. Identify finite modes:
   - where booleans actually describe one status, collapse them into a finite state
6. List domain events:
   - what can happen?
   - who or what causes it?
   - what data does the event carry?
7. Sketch transitions:
   - from which states is each event valid?
   - what changes?
   - what side effects happen at the boundary?
8. Decide placement:
   - local component state
   - reducer/store
   - URL
   - server cache/query layer
   - ref or external mutable object
9. Map the model to the chosen library.

When requirements are fuzzy, show the model before the code.

## Modeling questions

Use these questions before writing or revising code:

- What are the smallest facts we actually need to store?
- Which values are derived views of those facts?
- Which modes are mutually exclusive?
- What states are impossible but currently representable?
- What events can happen, and which ones are valid in each state?
- Does this state need to survive navigation or refresh?
- Is this really application state, or is it server cache?
- Is this value needed for rendering, or is it an ephemeral mutable handle?
- Who owns this state, and who is allowed to change it?
- Are we storing implementation details instead of domain truth?
- Are effects being used to synchronize state that should be derived?

## Choosing the representation

Prefer the simplest representation that preserves invariants.

- **Independent local values**: use local state only when fields are actually independent.
- **Finite modes**: use a string literal status or discriminated union, not several booleans.
- **Coordinated updates**: use a reducer when several fields change together or event handling depends on current state.
- **Complex workflows**: use a state machine or actor model when there are guards, retries, cancellation, concurrency, or child processes.
- **Relational collections**: normalize entities when nested updates become awkward or inconsistent.
- **Derived reads**: use selectors, getters, computed values, or render-time calculations.
- **Server data**: prefer a query/cache tool instead of forcing remote lifecycle concerns into a UI store.
- **Non-rendering mutable data**: use refs or external mutable objects.

Do not recommend a more complex abstraction unless the simpler one cannot enforce the needed invariants.

## Placement guidance

Choose where state lives by asking:

- Who reads it?
- Who writes it?
- How long must it live?
- Must it survive refresh?
- Must it be shareable by URL?
- Is it authoritative app state or remote cache?

Strong defaults:

- keep transient view-only state local
- keep cross-component client state in a reducer/store only when multiple consumers truly need it
- keep shareable navigation/filtering state in the URL
- keep fetched data in a server-state/cache layer
- keep non-visual handles out of render state

## Anti-patterns

Flag these aggressively in reviews and refactors:

- derived state stored and resynchronized with effects
- mirrored props in local state
- many booleans describing one mode
- storing both `selectedItem` and `selectedItemId` without a clear reason
- duplicated state across component, store, and URL
- action names that describe setters instead of domain events
- deep nested relational state that should be normalized
- `useEffect` chains that coordinate business logic
- putting most form state in a global store without a strong reason
- nullable field soup used instead of explicit states
- mixing server state, client state, and request lifecycle in one undifferentiated blob

## Reference routing

Open only the relevant references for the current task:

- React: `references/react.md`
- Redux and Redux Toolkit: `references/redux.md`
- Zustand: `references/zustand.md`
- Vue and Pinia: `references/vue-pinia.md`
- Plain JavaScript or library-agnostic implementation: `references/no-library.md`
- TypeScript modeling and exhaustiveness: `references/typescript-modeling.md`
- Async lifecycle modeling: `references/async-state.md`
- Review pass and smell detection: `references/review-checklist.md`

If the problem is specifically about XState or actor systems in a codebase that already uses them, prefer the dedicated `xstate-v5` skill for implementation details.

## Output format

Prefer this answer shape:

1. **State inventory**: what exists now or what must exist.
2. **Findings or invariants**: duplication, impossible states, and design constraints.
3. **Recommended model**:
   - source state
   - derived state
   - finite states or discriminated union
   - events and transitions
   - placement
4. **Library mapping**: only if the user asked for implementation in a specific stack.
5. **Code**: only after the model is clear enough.

For reviews, lead with concrete problems first. For design tasks, lead with the model first.

## Final self-check

Before answering, verify:

- every stored value really needs to be stored
- derived values are not being mirrored as source state
- mutually exclusive modes are not represented as unrelated booleans
- invalid states are reduced or eliminated
- state ownership and placement are explicit
- effects are not doing the work of selectors or transitions
- async status is modeled explicitly
- the recommended abstraction is not more complex than the problem requires
- library-specific advice matches the user's actual stack
