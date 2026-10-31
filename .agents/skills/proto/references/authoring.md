# Authoring prototypes, drafts, flows and the canvas

Contents: component prototypes · `meta` fields · fixtures and state · drafts · URL state ·
canvas and frame · flows. The check-and-render loop is in `SKILL.md`.

## Component prototypes

A component prototype lives beside the view it composes. Its slug is the file name without
`.proto.tsx` (`src/billing/invoice-list.proto.tsx` is `invoice-list`). Discovery uses the
`prototypes` glob (default `src/**/*.proto.tsx`), and files under the drafts directory count as
drafts.

Scaffold it from the view:

```sh
proto new --in src/billing/invoice-list.tsx [--spec billing]
```

The command writes `src/billing/invoice-list.proto.tsx`, which imports the component and has one
`default` state. The view file must end in `.tsx`, its first `export` must be the component (a
named function or `const`, default or not), and its base name must be a valid slug. `--spec` must
name a configured spec. The check is skipped only when the `specs` function returns `null`. Then
replace the generated title and add the real states:

```tsx
import { definePrototype, setParam, type PrototypeProps } from "@compozy/proto";
import { InvoiceList } from "./invoice-list";
import { invoices } from "./invoice-list.fixtures";

export const meta = definePrototype({
  title: "Invoices",
  spec: "billing",
  route: "/billing/invoices",
  states: [
    { id: "list", label: "Invoices" },
    { id: "empty", label: "No invoices" },
    { id: "failed", label: "Failed to load" },
  ],
});

export default function Prototype({ state }: PrototypeProps) {
  return (
    <InvoiceList
      invoices={state === "empty" ? [] : invoices}
      error={state === "failed" ? "Invoices could not be loaded." : null}
      onRetry={() => setParam("state", "list")}
      onOpen={() => {}}
    />
  );
}
```

Rules that `proto check` enforces:

- **No markup.** Any lowercase JSX tag in a component prototype is a `markup` problem, wrappers
  included. Fragments are fine. When a state needs layout or content the view cannot produce,
  add a prop or a child slot to the view. The fix belongs in the view because the view is what
  ships.
- **Slugs** match `^[a-z0-9]+(-[a-z0-9]+)*$`, are at most 120 characters, are unique in the app,
  and never start with `flow-`, which is reserved for flow review keys.

The default export is a component that receives `PrototypeProps`:
`{ state: string; role: string | null }`. `proto check` does not verify it, because it compiles
without type checking. A missing default export only fails when the state renders.

## `meta` fields

`meta` is parsed, not executed. Allowed values are string, boolean, array and object literals
only: no identifiers, numbers, spreads, template literals, function calls or duplicate keys. An
invalid literal makes the prototype disappear from the gallery and shows up as `invalid_meta`.

| Field        | Required | Meaning                                                                  |
| ------------ | -------- | ------------------------------------------------------------------------ |
| `title`      | yes      | Non-empty name in the gallery                                            |
| `states`     | yes      | Non-empty `{ id, label }[]`; ids unique. The first state is the default  |
| `spec`       | no       | A key of `specs` in `proto.config.ts` (`unknown_spec` otherwise)         |
| `roles`      | no       | Non-empty subset of the configured `roles` (`unknown_role` otherwise)    |
| `route`      | no       | The production route; passed to `Frame` (defaults to `"/"`)              |
| `surface`    | no       | A short surface label, such as a spec's screen id                        |
| `blocks`     | no       | String labels for building blocks                                        |
| `standalone` | no       | `true` renders a component prototype without `Frame` (full-page screens) |

With `roles` declared, the render host only renders those roles. Without it, the prototype renders
any configured role.

## Fixtures and state

- Keep fixtures as plain data beside the view, in `<view>.fixtures.ts`. The `.fixtures.` name is
  prototype-only, so the production boundary keeps it out of the app bundle. Stories can share
  the same fixtures.
- Pick states that change what people review: ready, empty, loading, error, permission or role
  differences, long content. Each state needs a stable `id` because comments and flows refer to
  it.
- Callbacks are no-ops, or move between states with `setParam("state", "<id>")`.
- Never fetch, create a query client, read auth or import API modules in a prototype. The data
  belongs to the route. The network guard only fails imports of modules listed in
  `guards.forbidRuntimeImports`, and a global `fetch` is invisible to it, so do not rely on it to
  catch everything.

## Drafts

```sh
proto new review-digest --draft
```

This writes `src/drafts/review-digest/index.proto.tsx`. The `drafts` directory is configurable,
but it must stay inside the `prototypes` glob, or the gallery never discovers the drafts.
Drafts are for exploring before a view exists: free markup is allowed, `meta` follows the same
rules, and `Frame` never wraps them. When the idea settles, extract the markup into a real view,
add a component prototype for it, and delete the draft. Review history stays with the draft's
key.

## URL state

The query string is the prototype's state. The render host reads `state`, `theme` and `role` from
it. Comments record the state the reviewer saw, and every state has a shareable link.
Runtime helpers from `@compozy/proto`:

- `useSearch()` returns the current query string and re-renders when it changes.
- `setParam(name, value | null)` and `setParams({ ... }, { push? })` update the query; `push`
  adds a history entry.
- `protoHref(slug, state?)` links to another prototype and keeps theme, role, width and chrome.

## Canvas and frame

`proto.canvas.tsx` at the app root (`canvas` in the config):

```tsx
import "./src/styles.css"; // app CSS loads on the render origin only
import type { CanvasProps, FrameProps } from "@compozy/proto";

/** Providers around every rendered state. */
export function Canvas({ children, theme, role }: CanvasProps) {
  return <AppProviders theme={theme}>{children}</AppProviders>;
}

/** Production chrome around component prototypes (not drafts, not `standalone`). */
export function Frame({ route, role, state, children }: FrameProps) {
  return <AppShellView activeRoute={route}>{children}</AppShellView>;
}
```

- Canvas holds presentation providers only: CSS, theme, i18n, toasts, and a memory-history router
  when views render links. Do not create query clients, API clients or auth sessions here.
  Prototypes must render from fixtures, and the network guard only inspects the graphs of
  prototypes, drafts and stories, so it does not catch a client in the canvas.
- Frame receives `route` (`meta.route` or `"/"`), `role` and `state`. Feed the app chrome from
  fixtures, not from session data.
- Theme: the render host toggles `.dark` and `color-scheme` on `<html>` for `?theme=dark`. If the
  design system uses another convention, paint it in `Canvas`. For example, when `:root` holds
  dark tokens and light mode needs `.light`:

  ```tsx
  useEffect(() => {
    const root = document.documentElement;
    root.classList.remove("light", "dark");
    root.classList.add(theme);
    root.style.colorScheme = theme;
  }, [theme]);
  ```

- Without a canvas file, prototypes render bare and `proto check` warns `canvas_missing`.

## Flows

```sh
proto flow new checkout [--spec billing]
```

This writes `flows/checkout.flow.ts` (the `flows` directory is configurable). The flow id is the
file name, and its review key is `flow-checkout`.

```ts
import { defineFlow } from "@compozy/proto/flow";

export const flow = defineFlow({
  title: "Checkout",
  spec: "billing",
  start: ["site"],
  nodes: {
    site: { external: "site", url: "/" },
    cart: { proto: "cart", state: "filled" },
    declined: { proto: "cart", state: "declined" },
    paid: { proto: "receipt", state: "default", end: true },
  },
  edges: [
    ["site", "cart", { on: "Open cart" }],
    ["cart", "declined", { on: "Card declined", kind: "overlay" }],
    ["declined", "paid", { on: "Retry", kind: "redirect" }],
  ],
});
```

- **Prototype nodes** are `{ proto, state, role?, end? }`. The prototype, state and role must
  exist.
- **External nodes** are `{ external, label?, url?, end? }`. `external` is one of `site`, `auth`,
  `payment`, `email` or `other`, and `other` needs a `label`. `url` is an absolute HTTPS URL, or
  a path when the config sets `siteUrl`.
- **Edges** are `[from, to, { on, kind? }]`. `kind` is `navigate` (the default), `overlay`,
  `redirect` or `external`.
- Node ids follow the slug pattern. `start` is a non-empty list of declared nodes. Every node
  must be reachable from `start` (`unreachable`), and a node without outgoing edges needs
  `end: true` (`dead_end`).
- Coverage: once a flow touches a spec, every state of that spec's prototypes should appear in
  some flow. `proto flow coverage --json` lists the gaps as warnings.
