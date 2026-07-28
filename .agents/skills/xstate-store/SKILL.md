---
name: xstate-store
description: XState Store (@xstate/store v4) event-driven state for TypeScript apps. Use when creating a store with createStore or createStoreLogic, writing transitions and enqueue effects, declaring Standard Schema contracts, wiring selectors or atoms, adding the persist, undo-redo, reset, or validateSchemas extensions, binding a store to React with @xstate/store-react, testing transitions or bridging a store into XState with fromStore, or migrating a v3 store to v4. Don't use for XState state machines and actors, Zustand or Redux stores, or TanStack Query server state.
---

# XState Store

Targets `@xstate/store` v4 and `@xstate/store-react` v2 (TypeScript 5.4+). Match the task to one or more Branches rows and read every listed file **in full** before producing output — those references are the contract; the tripwires below are only a final self-check.

## Pick the primitive first

| State | Primitive | Why |
| --- | --- | --- |
| Domain state updated through named events | `createStore({ context, on })` | Typed `trigger`, `can`, emitted events, replayable transitions |
| Per-instance state built from input | `createStoreLogic({ context: (input) => … })` | One definition, many instances; `selectors` come along |
| A single value set directly | `createAtom(value)` | No event vocabulary to justify a store |
| A value computed from other atoms or selectors | `createAtom(() => …)` | Read-only, recomputes on dependency change |
| Modes, guards, delays, hierarchical or parallel states | `xstate` machine | A store models data, not lifecycle |

*Done when:* the primitive is chosen from this table and its justification holds.

## Branches

| When you are… | Read in full |
| --- | --- |
| Creating a store, writing transitions, using `trigger`/`send`/`can`, or Immer | `references/store-core.md` |
| Enqueuing effects, emitting events, or doing async work | `references/effects-and-events.md` |
| Declaring `schemas` or turning on runtime validation | `references/schemas-and-validation.md` |
| Reading state with `store.select`, atoms, derived/async/reducer atoms | `references/selectors-and-atoms.md` |
| Building reusable/per-instance stores with `createStoreLogic`, input, or `selectors` | `references/store-logic-and-input.md` |
| Adding `persist`, `undoRedo`, `reset`, or composing `.with(...)` | `references/extensions.md` |
| Wiring a store into React components | `references/react.md` |
| Testing transitions, inspecting a store, or interoperating with XState | `references/testing-and-interop.md` |
| Upgrading a v3 store to v4, or porting an existing Zustand store over | `references/migration-v4.md` |

*Done when:* every matched reference was read, the code follows its patterns, and no tripwire is violated.

## Tripwires

**Transitions** — return the complete next context (spread the old one); return `undefined` to mark an event disallowed, which is what `store.can.*()` reports.

**Effects** — call `enqueue.effect`, `enqueue.emit.*`, and `enqueue.trigger.*` synchronously inside the transition; async work goes inside `enqueue.effect(async () => …)` and reports back by triggering another event.

**Imports** — framework bindings come from `@xstate/store-react` (and siblings), never `@xstate/store/react`; extensions come from `@xstate/store/persist`, `/undo`, `/reset`, `/validate`.

**Schemas** — `schemas` types the store; only `.with(validateSchemas())` validates at runtime.

**Atoms** — computed atoms read dependencies via `.get()`; their first parameter is the previous computed value, so annotate `createAtom<T>(…)` when using it.

**React** — module-scoped stores for app-wide state, `useStore(logic, input)` for component-scoped state; subscribe through `useSelector`/`useAtom` rather than `getSnapshot()`.
