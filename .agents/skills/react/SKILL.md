---
name: react
description: "Build React 19 components and hooks under React Compiler: state, Effects, memoization, props/refs, Actions, use(), compiler/lint setup, and Vitest tests. Excludes React Native, other frameworks, and backend-only Node.js."
allowed-tools: Read, Grep, Glob
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# React

Targets **React 19.2** and **React Compiler 1.0** (`babel-plugin-react-compiler`), linted by **`eslint-plugin-react-hooks` 7.1+**. Read the reference matched by the API or behavior being changed. Load adjacent references only for concerns the change crosses; reuse relevant context already read.

## The compiler is the optimizer

The compiler applies the equivalent of `memo` to every component and memoizes calculations inside components and hooks, at build time. It does this *more* granularly than hand-written hooks can — it memoizes after early returns, where `useMemo` is structurally forbidden.

The price is that the **Rules of React** stopped being style advice and became a build-time contract. When a component violates them, the compiler **bails**: it silently skips optimizing that component and moves on. The build stays green, the tests stay green, and the component is simply never optimized. Write code the compiler cannot bail on.

Two consequences that invert pre-compiler habits:

- Reflexive `useMemo`/`useCallback` on cheap values is now noise, and an **incomplete dependency array is worse than no memoization at all** — the compiler refuses to optimize a component whose manual memoization it cannot match.
- A rule violation costs optimization silently rather than raising an error, so the lint rules are the only thing standing between you and a quietly unoptimized app. Enable them before enabling the compiler.

## Decide memoization first

| Situation | Do |
| --- | --- |
| New value or callback consumed only by render/JSX | Write it plainly; the compiler memoizes it |
| Value feeds a `useEffect` (or another hook's) dependency array and identity must hold | Keep `useMemo`/`useCallback` — the escape hatch react.dev names by name |
| Value crosses to a non-compiled consumer (vanilla lib instance, `addEventListener`, uncompiled package) | Keep manual memoization |
| Manual memoization you keep for any reason | Make its deps exhaustive, or the component bails |
| Working memoization in existing code | Leave it — removing it changes compilation output; test before touching |
| Same expensive call repeated across components | Compiler memoization is per-component, never shared; hoist to a module-level cache or the data layer |
| Expensive work in a plain module-level util | Not memoized at all; the compiler only covers components and hooks |
| `memo()` with a custom `areEqual` comparator | Keep it; the compiler has no equivalent |

*Done when:* each memoization added or changed by this task traces to a row above, and no row was satisfied by adding a hook the compiler already covers.

## Branches

| When you are… | Reference |
| --- | --- |
| Writing or reviewing any component or hook body — purity, mutation, refs, globals | `references/rules-of-react.md` |
| Removing, keeping, or debating a `useMemo`/`useCallback`/`memo`, or a component was skipped by the compiler | `references/compiler.md` |
| Installing or configuring the compiler, the lint preset, or adopting it incrementally | `references/compiler-setup.md` |
| Reaching for `useEffect`, or deriving/syncing state | `references/effects.md` |
| Shaping components, composition, props, or TypeScript types | `references/components-and-types.md` |
| Choosing where state lives, wiring context or an external store | `references/state.md` |
| Using `use()`, Actions, `useOptimistic`, `<Activity>`, metadata, or other React 19 APIs | `references/react19-apis.md` |
| Writing or fixing component/hook tests | `references/testing.md` |

Apply the technical rules relevant to the changed behavior.

## Tripwires

**Purity** — render computes from props, state, and context only. `Math.random()`, `Date.now()`, `new Date()`, `crypto.randomUUID()`, and `performance.now()` belong in a state initializer, an event handler, or an effect.

**Mutation** — props, state, hook arguments, hook return values, and any value already handed to JSX are read-only; copy before changing (`setItems([...items].sort())`). A value created in this render that never escapes is yours to mutate freely.

**Reassignment** — treat a destructured prop as read-only: rename on destructure (`{ value: valueFromProps }`) and derive a new `const`.

**Refs** — read and write `ref.current` in effects and event handlers. During render the only sanctioned touch is one-time lazy init: `if (ref.current === null) ref.current = …`.

**Globals** — module scope holds constants; per-render values live in `useState`/`useContext` and outward writes in an effect. Counters, arrays, and ad-hoc caches touched during render break Fast Refresh and forfeit compilation.

**Components are static** — declare components and hooks at module scope. Defining one inside another component (or returning one from a factory) makes a new type every render, resetting state and tearing down DOM.

**Effects** — an effect exists to synchronize with something outside React. Synchronous `setState` in an effect to derive or copy state is an error-level lint violation; compute during render instead.

**Errors** — child render errors are caught by an Error Boundary, never by `try`/`catch` around JSX.

**Refs are props** — `forwardRef` is deprecated in 19; accept `ref` in the props type. Likewise `<Context value={…}>`, not `<Context.Provider>`.

**Escape hatch** — `"use no memo"` is a bisection tool carrying a written reason and a removal plan, not a fix.

## Related skills

`tanstack` for TanStack Query, Router, and Form. `xstate-store` for `@xstate/store`. `shadcn` for component primitives and CVA variants. `tailwindcss` for styling. `vitest` for runner configuration beyond `references/testing.md`.
