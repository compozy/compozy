# Web Frontend

React SPA using Vite, TanStack Router/Query, Tailwind, shadcn/ui, XState Store, and Zod. Root `CLAUDE.md` owns compatibility and delivery; this file adds web conventions.

## Ownership and Contracts

- Domain features live in `src/systems/<domain>/`. Dependencies flow `adapters → lib → hooks → components`; cross-system imports use the public barrel. Generic primitives come from `@compozy/ui`.
- TanStack Query owns server state, including envelopes, totals/facets, cursors, and stream fences. Flatten at the view-model boundary. Authorize, scope, filter, sort, count, and page on the server before the cut.
- Co-locate query keys/functions in `lib/query-options.ts`; loaders, hooks, mutations, and streams reuse these factories. Continuation stays in `pageParam`. Optimistic updates restore the full envelope; invalidation follows the owner's reread contract.
- Adapters expose typed errors and propagate `AbortSignal`. Components consume view models; pages/routes and owning hooks coordinate data access.
- XState Store owns domain interaction state. Context injects store handles; `useStore` binds component stores. Reserve `useStoreBinding` for pure pre-commit identity/baseline replacement; current callbacks enter as events.
- Use kebab-case files, named exports, functional components, and `@/*` imports. Native-element wrappers extend intrinsic props, merge `className`, forward React 19 `ref`, and spread remaining props. Do not edit `routeTree.gen.ts`.

## Design and Copy

- Read `packages/ui/src/index.ts` before adding UI; compose existing primitives and domain components. Domain variants use domain-prefixed names.
- `tokens.css` and `DESIGN.md` own visual grammar: flat content surfaces; tokenized glass/blur/window shadows only for OS-shell chrome. Fonts, signal colors, and spacing come from tokens.
- Structural micro-labels use `Eyebrow` or the `eyebrow` utility; uppercase is opt-in through `variant="caps"`. Preserve the canonical token contract instead of reconstructing it with utility tuples.
- `COPY.md` owns labels and public wording; runtime truth owns available controls, metrics, and states. Named prototypes constrain visual language; actual content, primitives, and host chrome keep their production owners.

## Focused References

Use `app-renderer-systems` for a new or restructured domain system; `eng-data-boundaries` for changed authorization, pagination, cache, or stream contracts; `xstate-store` for store lifecycle changes; `react` for React-specific patterns. Load other library references only for the API or pattern in question.

Use `eng-design` for design-system/redesign work, `eng-ui-screenshot` for named-reference comparison, and `storybook-stories` for stories. Routine edits do not require the entire design/React/testing skill stack.

## Validation

Run from the repository root; `make gate` selects delivery lanes. During iteration:

```bash
bunx turbo run typecheck --filter=./web
bunx turbo run test --filter=./web
```

Use the existing route/hook/component/E2E suite that owns the changed behavior. Storybook configuration is infrastructure: prefer build, `list-stories`, or rendered capture over assertions about decorator arrays or globs. Repeat checks only when their inputs change or a failure needs investigation.

`make web-dev`, `make web-build`, and `make web-fmt` are local development commands; package-local test/typecheck runs do not replace Turbo evidence. Isolated QA derives `COMPOZY_WEB_API_PROXY_TARGET` from its bootstrap manifest and follows root teardown rules.
