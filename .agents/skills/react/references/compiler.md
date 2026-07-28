# React Compiler: memoization policy and bail-outs

Targets React Compiler **1.0** (stable since 2025-10-07). Works with React 17+; 17 and 18 additionally need `react-compiler-runtime` and `target` set accordingly.

## What the compiler does

Two optimizations, applied at build time ([Introduction](https://react.dev/learn/react-compiler/introduction)):

1. **Skips cascading re-renders** — the equivalent of wrapping every component in `memo`, automatically.
2. **Memoizes expensive calculations** inside components and hooks.

Its advantage over hand-written hooks is precision, not just convenience: it can memoize **after early returns**, which the Rules of Hooks forbid `useMemo`/`useCallback` from doing, and it handles optional chains and array indices as dependencies where `exhaustive-deps` struggles.

Meta reported up to 12% faster initial load and navigation, and some interactions more than 2.5× faster, on the Quest Store, with neutral memory ([1.0 blog](https://react.dev/blog/2025/10/07/react-compiler-1)).

## What the compiler does not do

Design limits, not bugs:

- **Only components and hooks are compiled.** An expensive function in a plain module-level util is never memoized. `compilationMode: 'infer'` matches PascalCase-named or `use`-prefixed functions that create JSX and/or call hooks.
- **Memoization is not shared across components.** The same expensive call in five components runs five times. If profiling justifies it, write your own cache or push the work into the data layer.
- **Class components are out of scope** — not "unsupported syntax", simply never matched. No directive or mode memoizes a class.
- **Context subscriptions are not made fine-grained.** Compiler memoization behaves like `memo`, and a memoized component still re-renders when a context it consumes changes. See `state.md`.
- **Nothing architectural.** Data-fetching waterfalls, bundle size, unvirtualized lists, and over-fetching are untouched. Compiling a wrong architecture faster does not help.

## Manual memoization policy

react.dev deliberately walked back its original "you can remove your memos" wording ([reactjs/react.dev#7953](https://github.com/reactjs/react.dev/pull/7953)) because it "was coming off a bit too strong and makes it incorrectly seem like the manual memos and compiler memos are 1:1". The current guidance:

- **New code** — rely on the compiler. Use `useMemo`/`useCallback` only for precise control.
- **Existing code** — leave working memoization in place, or test carefully before removing it, because removal changes compilation output.

The escape hatch react.dev names explicitly:

> "A common use-case for this is if a memoized value is used as an **effect dependency**, in order to ensure that an effect does not fire repeatedly even when its dependencies do not meaningfully change."

The 1.0 blog reinforces this with a forward-compatibility warning: future compiler versions may memoize more granularly, and because a previously-memoized value may be feeding a `useEffect` somewhere in the tree, a change in memoization can cause that effect to over- or under-fire.

Still-legitimate reasons to write a memo ([useMemo docs](https://react.dev/reference/react/useMemo)):

1. A noticeably slow calculation whose dependencies rarely change.
2. A value passed to a component still wrapped in `memo`.
3. Preventing an effect from firing too often.
4. A value later used as a dependency of another hook.

Plus the standing caveat: memoization is only ever a performance optimization. If the code does not *work* without it, the underlying bug is elsewhere.

`memo()` with a custom `areEqual` comparator has no compiler equivalent — keep those.

### Reflexive memoization is noise

```jsx
// Pre-compiler ceremony
const ExpensiveComponent = memo(function ExpensiveComponent({ data, onClick }) {
  const processedData = useMemo(() => expensiveProcessing(data), [data]);
  const handleClick = useCallback((item) => onClick(item.id), [onClick]);
  return <List items={processedData} onItemClick={handleClick} />;
});

// Same optimization, no ceremony
function ExpensiveComponent({ data, onClick }) {
  const processedData = expensiveProcessing(data);
  const handleClick = (item) => onClick(item.id);
  return <List items={processedData} onItemClick={handleClick} />;
}
```

```jsx
// Noise on cheap values, and a bail risk
const total = useMemo(() => items.reduce((s, i) => s + i.price, 0), [items]);
const onClose = useCallback(() => setOpen(false), []);
<Dialog open={open} onClose={onClose} />;

// Plain
const total = items.reduce((s, i) => s + i.price, 0);
<Dialog open={open} onClose={() => setOpen(false)} />;
```

### An incomplete dependency array is worse than no memo

`preserve-manual-memoization` (error) is the enforcement. The compiler compiles a component only if its own inference matches or exceeds the manual memoization already there; when deps are incomplete it cannot follow the data flow and **bails on the whole component**.

```jsx
// INVALID: 'filter' missing — the component stops being compiled at all
const filtered = useMemo(() => data.filter(filter), [data]);

// VALID: exhaustive deps, compiler preserves and keeps optimizing around it
const filtered = useMemo(() => data.filter(filter), [data, filter]);

// BEST for new code
const filtered = data.filter(filter);
```

If you keep a manual memo, its deps must be exhaustive or you pay twice.

### The useRef-as-cache hack

```jsx
// INVALID: a render-visible cache the compiler cannot reason about
const cacheRef = useRef();
if (!cacheRef.current) cacheRef.current = expensive(props.x);
const value = cacheRef.current;
```

One-time lazy init (`if (ref.current === null) ref.current = …`) remains valid — see `rules-of-react.md`.

## The bail catalogue

### Hard `UnsupportedSyntax` errors

Only three, verified in the compiler's `BuildHIR.ts`:

```js
// with statements
with (Math) { return <div>{sin(PI / 2)}</div>; }

// eval
const result = eval(code);

// class declarations inside a component or hook — move them to module scope
function Component() { class Foo {} /* … */ }
```

Two claims that circulate widely and are **false**: labeled statements are supported (including labeled loops with `continue`), and the `arguments` object is not in the unsupported set.

### Silent skips (`Todo` reasons, no diagnostic under the `recommended` preset)

This is the dangerous class — extracted from the compiler's `BuildHIR.ts`:

| Pattern | Compiler reason |
| --- | --- |
| `for await (…)` | Handle for-await loops |
| `try` with no `catch` | Handle TryStatement without a catch clause |
| `try`/`catch`/`finally` | Handle TryStatement with a finalizer ('finally') |
| `throw` as an expression | Throw expressions are not supported |
| `i++` on a variable captured in a lambda | Handle UpdateExpression to variables captured within lambdas |
| `i++` on a global | Support UpdateExpression where argument is a global |
| `for (;;)` with an empty test | Handle empty test in ForStatement |
| non-`const` hoisted declarations | Handle non-const declarations for hoisting |
| computed keys in object destructuring | Handle computed properties in ObjectPattern |
| rest element in a nested `ObjectPattern` | Handle `...` rest element in ObjectPattern |
| pipeline operator `\|>` | Pipe operator not supported |
| a JSX tag declared locally | tags should be module-level imports |
| meta properties other than `import.meta` | Handle MetaProperty expressions other than import.meta |
| reassigning a `const` binding | Expect const declaration not to be reassigned |

The "try/catch breaks the compiler" complaint is real but mis-stated: what bites is **catch-less `try`**, **`finally`**, and **JSX inside `try`** (which is separately an `error-boundaries` violation) — not conditionals or ordinary `try`/`catch`.

### Off-by-default rules that surface these

Enable these to convert silent skips into visible diagnostics:

| Rule (`react-hooks/…`) | Default | Surfaces |
| --- | --- | --- |
| `todo` | Hint, off | Compiler `Todo` bail-outs — the genuinely silent class |
| `capitalized-calls` | error, off | Calling a capitalized function instead of rendering it |
| `memoized-effect-dependencies` | error, off | Effect deps relying on memoized identity |
| `exhaustive-effect-dependencies` | error, off | Missing effect deps, compiler-inferred |
| `no-deriving-state-in-effects` | error, off | Derived state computed in an effect |
| `hooks` | error, off | Overlaps `rules-of-hooks` |
| `void-use-memo` | error | `recommended-latest` only |

Experimental rules ship via `reactHooks.configs.flat['recommended-latest']`.

### Suppression traps

- An `eslint-disable` for an *unrelated* rule can suppress `incompatible-library`, turning a debuggable skip into a silent one ([facebook/react#35105](https://github.com/facebook/react/issues/35105), open).
- Disabling hooks rules produces: "React Compiler has skipped optimizing this component because one or more React ESLint rules were disabled."
- `incompatible-library` is only a `warn`; `state.md` lists which library APIs trip it.

## Directives

Two string-literal directives ([Directives](https://react.dev/reference/react-compiler/directives)):

```jsx
function OptimizedComponent() { "use memo"; return <div />; }
function UnoptimizedComponent() { "use no memo"; return <div />; }
```

Module level (before imports) applies to the whole file; function level overrides module level. `"use no memo"` wins over every `compilationMode`. It must be the first statement in the function body (comments are fine), use quotes rather than backticks, and match exactly; `"use no forget"` is an accepted alias.

Interaction with `compilationMode`: `annotation` compiles only `"use memo"`; `infer` lets the compiler decide with directives overriding; `all` compiles everything and `"use no memo"` excludes.

Treat it as temporary, with the reason written down:

```jsx
function DataGrid() {
  "use no memo"; // TODO: remove after fixing dynamic row heights (JIRA-123)
}
```

The count of `"use no memo"` directives in a codebase is a health metric — left unmanaged it becomes `@ts-ignore`.

## Detecting what compiled

1. **React DevTools** — compiled components carry a **"Memo ✨"** badge beside the name.
2. **Build output** — look for `import { c as _c } from "react/compiler-runtime";` and `const $ = _c(n);`.
3. **`npx react-compiler-healthcheck@latest`** — reports how many components compile, StrictMode usage, and incompatible libraries. `--src <glob>` and `--verbose` are the useful flags; confirm with `--help`. Caveats: the presence of a `babel.config.js` can make it report `0 out of 0` ([#29135](https://github.com/facebook/react/issues/29135)), and StrictMode detection has false negatives ([#29075](https://github.com/facebook/react/issues/29075)). It does not tell you *which* components failed — ESLint does.
4. **ESLint** — the plugin surfaces compiler diagnostics without the compiler installed at all.
5. **React Performance Tracks** (19.2) — Chrome DevTools custom Scheduler and Components tracks for render and effect timing.

## Debugging loop

Compiler *errors* are rare, since the compiler prefers to skip. Runtime misbehaviour is the common case and is almost always a subtle Rules of React violation. The three breaking patterns react.dev names: effects depending on referential equality, unstable dependency arrays causing over-firing or infinite loops, and conditional logic keyed on reference checks.

1. Add `"use no memo"` to the suspect component.
2. Bug disappears → it is a Rules violation, not a compiler defect.
3. Now also remove the local `useMemo`/`useCallback`/`memo`. If the bug persists with *no* memoization at all, that is conclusive.
4. Fix the violation, remove the directive, confirm the ✨ badge returns.

Selective opt-out is safe because the compiler only looks at code inside a component or hook body.
