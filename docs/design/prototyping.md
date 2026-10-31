---
target: library
library: ["@compozy/ui", "@/systems/*/components/**", "@/systems/*/mocks/*"]
catalog:
  - packages/ui/src/index.ts
  - packages/ui/src/primitives.ts
  - packages/ui/src/exports
  - web/src/systems/*/components
authority: [DESIGN.md, COPY.md, packages/ui/src/tokens.css]
host: vite-app
prototypes: web/src/drafts
entry: "{slug}/index.proto.tsx"
providers: web/proto.canvas.tsx
dev: bun run proto dev
preview: "http://127.0.0.1:4401/__proto/render/{slug}?chrome=0"
typecheck: bun run --cwd web typecheck
evidence: "bun run proto evidence {slug} --state <id>"
output: "web/docs/design/{slug}/"
verified: 2026-10-31
---

# Prototyping in the gallery

Compozy prototypes render the product's own components. A prototype is a `*.proto.tsx` file that
composes real `@compozy/ui` primitives and `web/src/systems/**` views with typed fixtures. The
gallery, review loop, guards and CLI come from the published
[`@compozy/proto`](https://github.com/compozy/proto) package (experimental 0.x, a `web`
devDependency pinned exactly). Prototypes typecheck, follow the tokens, and cannot drift from
production because they are production views.

Run every command from the repository root: `bun run proto <command>` runs the package CLI in
`web/` (the app root). `bun run proto --help` lists the commands; every command accepts `--json`.

| File                     | Role                                                                     |
| ------------------------ | ------------------------------------------------------------------------ |
| `web/proto.config.ts`    | Title, specs (read from `.compozy/tasks/`), network guard                 |
| `web/proto.canvas.tsx`   | `Canvas`: providers, theme and CSS around every rendered state            |
| `web/flows/*.flow.ts`    | Journeys across prototype states                                          |
| `web/proto/review/*.json`| Local review history, one file per prototype or flow, committed with code |
| `web/vite.config.ts`     | `proto()`: the production boundary of the SPA build                       |
| `web/vite.presentation.ts` | Rendering config shared by the app, Storybook and the render origin     |

`bun run proto dev` serves the gallery shell at `http://127.0.0.1:4400` and the render origin at
`http://127.0.0.1:4401`, which compiles prototypes with the web app's own Vite config (React
Compiler, Tailwind, `@` alias). The first render after a cold start waits for Vite's dependency
optimizer. `bun run proto build` writes a static, read-only gallery to `web/dist-proto/`; the SPA
build in `web/dist` never includes prototype or gallery code. Prototypes never talk to a daemon.

## Two kinds of prototype

- **Component prototypes** live beside the view they render, as
  `web/src/systems/**/<slug>.proto.tsx` or `web/src/components/**/<slug>.proto.tsx`. They contain
  **no intrinsic JSX** (`<div>`, `<button>`…): only components. Layout that a view lacks belongs
  in the view or in an `@compozy/ui` primitive (`PageContent`, `PageShell`…). `proto check`
  reports intrinsic elements as `markup` problems.
- **Drafts** live in `web/src/drafts/<slug>/index.proto.tsx` and may use free markup with tokens
  and `@compozy/ui`: exploration before a view exists, the successor of `docs/design/opendesign/`
  boards. The gallery labels them "Draft".

Both export a literal `meta` and a default component:

```tsx
import { definePrototype, type PrototypeProps } from "@compozy/proto";

export const meta = definePrototype({
  title: "Sessions board",
  spec: "sessions-kanban", // a .compozy/tasks/<id>/ directory
  surface: "S4", // the spec's UI-contract surface code
  route: "/sessions?view=board", // production route, for reviewers; never navigated
  states: [
    { id: "populated", label: "Populated" },
    { id: "empty", label: "Empty" },
  ],
});

export default function SessionsBoardPrototype({ state }: PrototypeProps) {
  /* render the real view for `state` from fixtures */
}
```

`meta` is read statically and never executed. The slug is the file stem, or the parent directory
for `index.proto.tsx`; it is unique and cannot start with `flow-`. `useSearch`, `setParam(s)` and
`protoHref` from `@compozy/proto` keep extra prototype state in the URL. `spec` is checked against
`.compozy/tasks/<id>/` (the gallery title is the `_spec.md` H1); that directory is local workflow
state Git ignores, so in CI and fresh worktrees the specs function returns `null` and spec checks
are skipped (`specs_unavailable`). Scaffold with:

```sh
bun run proto new sessions-board --in src/systems/session/components/session-board --spec sessions-kanban
bun run proto new review-digest --draft --spec sessions-kanban
```

## Views, containers and fixtures

A prototype renders a **view**: a component that takes data and callbacks as props. When the view
you need is a container (it reads queries, stores bound to the daemon, preload hooks or menus that
pull the OS shell), split a pure view out of it first, behaviour-preserving, and keep the container
as the production entry point. Existing examples:

| Container (production) | View (prototype)                             | What the container binds                               |
| ---------------------- | -------------------------------------------- | ------------------------------------------------------ |
| `SessionBoardCard`     | `SessionBoardCardView`                       | runtime label, entry preload, row menus                |
| `SessionsBoard`        | `SessionsBoardView` + `SessionBoardCardSlot` | live cards, Group-by lanes, bulk selection             |
| `SessionListRow`       | `SessionListRowView`                         | runtime label, entry preload, `…` and right-click menus |

Hooks follow the same split: the board's views read the selection through
`use-sessions-board-view-selection.ts` (context only), while `use-sessions-board-selection.ts`
owns the data-bound bulk selection. Prefer deep imports of leaf components over a system barrel
inside views: barrels re-export hooks that reach the network layer.

Fixtures are shared with stories, never duplicated. They live in the owning system's `mocks/`
(`web/src/systems/session/mocks/sessions-board-fixtures.ts`, `sessions-board-view-fixtures.ts`,
`session-pull-request-fixtures.ts`, `session-row-fixtures.ts`). Builders take an optional `handler`
factory, so stories pass Storybook's `fn` to log actions while prototypes keep inert defaults.
Storybook documents components in isolation; the gallery reviews screens, states and journeys.

## Isolation rules

- **Network guard.** A prototype or draft whose runtime imports reach a module listed in
  `guards.forbidRuntimeImports` (`src/lib/api-client.ts`, the ticketed event source and web
  socket, `src/lib/gateway-*.ts`, `@compozy/client`, `@tanstack/react-query`) fails `proto check`,
  `proto build` and the render origin of `proto dev`, with the import chain. Type-only imports are
  fine. Fix the chain (view/container split, deep import); never loosen the guard. Storybook and
  Vitest do not load `protoPresentation()`: the package guard also covers `*.stories.tsx`, and
  CompozyOS route stories deliberately exercise containers against MSW.
- **Production boundary.** `proto()` in `web/vite.config.ts` fails the SPA build when a
  `*.proto.*` file, a draft, a flow, the canvas or `@compozy/proto` reaches it, and an oxlint
  `no-restricted-imports` rule rejects those imports in `web/src/**`. Storybook builds from
  `web/vite.presentation.ts`, because the boundary also treats stories as prototype-only.
- **Providers.** Every canvas gets a fresh stack (`web/proto.canvas.tsx`): `UIProvider` with the
  user's reduced-motion preference, `TooltipProvider`, a memory-history TanStack router, the
  toaster in the canvas theme, the theme through `applyTheme` (`data-theme` + `.dark`), and a reset
  of module-level client stores (`web/src/storybook/reset-client-stores.ts`, shared with
  Storybook). There is no `QueryClient`. There is no `Frame`: prototypes compose their own page
  container (`PageContent`).
- **Origins.** Prototypes render in iframes on the separate render origin and talk to the shell
  only through the package's validated `postMessage` protocol.

## Lifecycle

1. Explore in `web/src/drafts/<slug>/` with free markup.
2. When the direction is approved, build the real view in its system, write `<slug>.proto.tsx`
   beside it with the **same slug** (comments and links keep working), and delete the draft.
   Handle open comments before graduating; keep equivalent states, and note any dropped state that
   the production view cannot express.
3. The component prototype stays as the review surface for later changes to that view.

## URLs

`bun run proto url <slug> [--state <id>] [--bare]` prints the gallery URL, or with `--bare` the
render URL (`http://127.0.0.1:4401/__proto/render/<slug>?chrome=0&state=<id>`), which is the
preview and capture contract. The gallery keeps the whole view in its URL (`?p=<slug>&state=<id>`,
`?flow=<id>`, `&theme=light|dark`), so every screen, state and flow step has a shareable link.

## Flows

Journeys live in `web/flows/<id>.flow.ts` (the file name is the id):

```ts
import { defineFlow } from "@compozy/proto/flow";

export const flow = defineFlow({
  title: "Board to pull request",
  spec: "sessions-kanban",
  start: ["board"],
  nodes: {
    board: { proto: "sessions-board", state: "populated" },
    failing: { proto: "session-pull-request-chip", state: "checks-failing" },
    github: {
      external: "other",
      label: "Pull request on GitHub",
      url: "https://github.com/compozy/compozy/pull/932",
      end: true,
    },
  },
  edges: [
    ["board", "failing", { on: "Topbar PR chip — web slice" }],
    ["failing", "github", { on: "Open PR", kind: "external" }],
  ],
});
```

- Prototype nodes need an existing `proto` and `state`. External nodes use `site`, `auth`,
  `email`, `payment` or `other` (which needs a `label`), with an optional absolute `https://` URL.
- Edges are `[from, to, { on, kind? }]` with `navigate` (default), `overlay` (same screen),
  `redirect` or `external`. `on` is the **exact text of the real button or link**.
- Every node is reachable from `start`; a node without outgoing edges declares `end: true`.
- Coverage warnings (`uncovered_state`) list prototype states no flow reaches. They never fail a
  check.

`bun run proto flow new|show|list|coverage` scaffold and inspect flows.

## Comments and review

Reviewers press C in the gallery, click an element or drag a region, and write; each comment
records the selector, text, rects and the JSX source chain of what was clicked, plus the commit,
state, theme, role and viewport. Review files live in `web/proto/review/<key>.json` (flow keys are
`flow-<id>`), written under a per-key lock by the dev server and the CLI. Agents apply comments
with the shipped `apply-prototype-comments` skill (`.agents/skills/apply-prototype-comments/`) and
author prototypes with `author-prototype`:

```sh
bun run proto comments list [--status open|resolved|all] [--json]
bun run proto comments show <id> --json
bun run proto comments reply <id> --note "…"
bun run proto comments resolve <id> --note "what changed"
bun run proto comments attach <id> <file…> · attachments <id> --out <dir>
```

Approval (`proto review approve`, or the shell's Approve button) is for people; agents never
approve. CompozyOS had no review history in the previous in-repo gallery, so nothing was migrated;
`bun run proto migrate --from <dir>` upgrades older v1/v2 review files when needed.

## Screenshots and evidence

`bun run proto screenshot <slug> [--state <id>] [--theme light|dark] [--width <px>]` saves a PNG
and `bun run proto evidence <slug> [--state <id>]` a script-free HTML snapshot, both under
`web/docs/design/<slug>/` (Playwright Chromium; `bunx playwright install chromium` once). They
need a running `proto dev`. Capture both themes for visual changes and inspect the images.
`eng-ui-screenshot` covers visual contract bundles.

Known gap: `@compozy/proto` 0.1.1 confines `evidence.dir` to the app root (`web/`), while the
open-design `lint_artifact` tool only reads workspace `docs/design/`. Until the package accepts a
workspace-relative evidence directory, copy the evidence HTML into `docs/design/<slug>/` for lint.

## Checks

- `bun run proto check [--json]`: problems (slugs, metadata, markup, specs, flows, network guard)
  plus a render build that writes nothing. Prints coverage warnings without failing.
- `bun run proto list --json`: prototypes and flows as agents read them.
- Prototypes and drafts follow the web lint and design-system rules; the 500-line source limit
  applies.

## Citing prototypes in specs

Specs and visual contracts cite a prototype by slug and state, with its gallery URL:
`session-board-card` · `needs-you` (`/?p=session-board-card&state=needs-you`), and journeys by flow
id (`/?flow=board-to-pull-request`). A UI spec that adds prototypes also adds or updates the flows
its UI/UX contract describes. Existing `docs/design/opendesign/` boards stay as historical
references; production (`packages/ui` + `web/`) wins on any conflict.

Verified 2026-10-31 on `refactor/migrate-to-compozy-proto`: `proto check` (0 problems),
`proto list --json`, `proto dev` (both origins; sessions prototypes, the review-digest draft and
the flow canvas rendered in Chromium), `proto url --bare`, `proto screenshot`, `proto evidence`,
`proto build`, the SPA `vite build`, Storybook build, the web Vitest suite and typecheck.
