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
providers: web/gallery/render/canvas.tsx
dev: bun run proto dev
preview: "http://127.0.0.1:4400/{slug}?chrome=0"
typecheck: bun run --cwd web typecheck
evidence: bun run proto screenshot {slug} --state <id> --theme light|dark --width <px>
verified: 2026-10-29
---

# Prototyping in the gallery

Compozy prototypes render the product's own components. A prototype is a `*.proto.tsx` file that
composes real `@compozy/ui` primitives and `web/src/systems/**` views with typed fixtures, inside a
local review gallery (`web/gallery/`). It replaces hand-copied HTML boards for new design work:
prototypes typecheck, follow the tokens, and cannot drift from production because they are
production views.

`bun run proto dev` serves the gallery at `http://127.0.0.1:4400` (port fixed, strict). It reuses
the web app's React Compiler, Tailwind and `@` alias through `web/vite.presentation.ts`, without
route generation, the daemon proxy or the production boundary. `bun run proto build` writes
`web/dist-gallery`; the production SPA build in `web/dist` never includes gallery code. Prototypes
never talk to a daemon.

## Two kinds of prototype

- **Spec prototypes** live beside the view they render, as `web/src/systems/**/<slug>.proto.tsx`
  or `web/src/components/**/<slug>.proto.tsx`. They contain **no intrinsic JSX** (`<div>`,
  `<button>`…): only components. Layout that a view lacks belongs in the view or in an
  `@compozy/ui` primitive (`PageContent`, `PageShell`…), not in the prototype. Discovery reports
  intrinsic elements as `markup` problems.
- **Drafts** live in `web/src/drafts/<slug>/index.proto.tsx` and may use free markup with tokens
  and `@compozy/ui`. They are the successor of `docs/design/opendesign/` boards: exploration
  before a view exists. The gallery labels them "Draft".

Both export a literal `meta` and a default component:

```tsx
import { definePrototype, type PrototypeProps } from "#gallery/contract";

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

`meta` must be a pure literal: discovery reads it from the syntax tree and never runs the module.
The slug is the file stem, or the parent directory for `index.proto.tsx`; it matches
`^[a-z0-9]+(-[a-z0-9]+)*$`, is unique across the repository, and cannot start with `flow-`
(reserved for flow comments). `spec` is checked against `.compozy/tasks/<id>/` when that local
directory exists (the gallery title is the `_spec.md` H1); in CI and fresh worktrees the directory
is absent and spec validation is skipped. Scaffold with:

```sh
bun run proto new sessions-board --in web/src/systems/session/components/session-board --spec sessions-kanban
bun run proto new review-digest --draft --spec sessions-kanban
```

## Views, containers and fixtures

A prototype renders a **view**: a component that takes data and callbacks as props. When the view
you need is a container (it reads queries, stores bound to the daemon, preload hooks or menus that
pull the OS shell), split a pure view out of it first, behaviour-preserving, and keep the container
as the production entry point. Existing examples:

| Container (production) | View (prototype) | What the container binds |
| --- | --- | --- |
| `SessionBoardCard` | `SessionBoardCardView` | runtime label, entry preload, row menus |
| `SessionsBoard` | `SessionsBoardView` + `SessionBoardCardSlot` | live cards, Group-by lanes |
| `SessionListRow` | `SessionListRowView` | runtime label, entry preload, `…` and right-click menus |

Prefer deep imports of leaf components (`@/systems/profiles/components/profile-owner-tag`) over a
system barrel inside views: barrels re-export hooks that reach the network layer.

Fixtures are shared with stories, never duplicated. They live in the owning system's `mocks/`
(`web/src/systems/session/mocks/sessions-board-fixtures.ts`, `sessions-board-view-fixtures.ts`,
`session-pull-request-fixtures.ts`, `session-row-fixtures.ts`). Builders take an optional `handler`
factory, so stories pass Storybook's `fn` to log actions while prototypes keep inert defaults.
Storybook documents components in isolation; the gallery reviews screens, states and journeys.

## Isolation rules

- **Network guard.** A prototype or draft whose runtime import graph reaches
  `web/src/lib/api-client.ts`, the ticketed event source or web socket, the gateway modules or the
  `@tanstack/react-query` runtime fails `proto check` and shows an error in `proto dev`, with the
  import chain. Type-only imports are fine. Fix the chain (view/container split, deep import);
  never suppress the guard.
- **Production boundary.** `web/src/**` cannot import `*.proto.*`, `web/src/drafts/**`,
  `#gallery`/`web/gallery/**` or flows: an oxlint `no-restricted-imports` rule and the
  `compozy:production-boundary` plugin in `web/vite.config.ts` both enforce it, and
  `routeTree.gen.ts` never picks up gallery files.
- **Providers.** Every canvas gets a fresh stack (`web/gallery/render/canvas.tsx`): `UIProvider`
  with the user's reduced-motion preference, `TooltipProvider`, a memory-history TanStack router,
  the toaster in the canvas theme, and a reset of module-level client stores
  (`web/src/storybook/reset-client-stores.ts`, shared with Storybook). There is no `QueryClient`.
  Themes set `data-theme` and `.dark` on `<html>`; dark is the default.
- **Frames.** The canvas iframe isolates styles and theme, not untrusted code: it runs
  repository-owned prototypes on the gallery origin so comments can inspect their DOM.

## Lifecycle

1. Explore in `web/src/drafts/<slug>/` with free markup.
2. When the direction is approved, build the real view in its system, write
   `<slug>.proto.tsx` beside it with the **same slug** (comments and links keep working), and
   delete the draft. Handle open comments before graduating; keep equivalent states, and note any
   dropped state that the production view cannot express.
3. The spec prototype stays as the review surface for later changes to that view.

## URL contract

The standalone prototype is `/<slug>?state=<id>&theme=light|dark&chrome=0` (`chrome=0` hides the
floating toolbar); it is the canvas URL and the screenshot contract. The gallery at `/` keeps the
whole view in its URL, so every level has a copyable link:

| Level | URL |
| --- | --- |
| Specs index | `/` |
| Spec overview (prototypes, flows, crossing flows) | `/?spec=sessions-kanban` |
| Prototype (first state) / one state | `/?p=session-board-card` · `&state=merged` |
| Prototype view options | `&theme=light` · `&w=390` (320–3840) · `&compare=themes` |
| Comments | `&comments=1` · `&comment=<id>` |
| Flow / one step | `/?flow=board-to-pull-request` · `&node=card` |

`spec`, `p` and `flow` are exclusive; links clear the other selections' parameters and keep the
theme. An existing `flow` wins over `p`, which wins over `spec`; unknown values fall back one level
up (a missing flow shows "Flow not found"), and the URL is rewritten to name what is on screen.
`bun run proto url <slug> [--state] [--theme] [--gallery]` and `bun run proto flow url <id>
[--node]` print links. Keyboard: J/K prototypes, [ ] states, T theme, C comment mode (comments
panel on a flow), / search, ? help, Cmd/Ctrl+B sidebar.

## Flows

Journeys live in `web/gallery/flows/<id>.flow.ts` (flat directory; the file name is the id):

```ts
import { defineFlow } from "#gallery/flow";

export const flow = defineFlow({
  title: "Board to pull request",
  spec: "sessions-kanban",
  start: ["board"],
  nodes: {
    board: { proto: "sessions-board", state: "populated" },
    failing: { proto: "session-pull-request-chip", state: "checks-failing" },
    github: { external: "github", url: "https://github.com/compozy/compozy/pull/932", end: true },
  },
  edges: [
    ["board", "failing", { on: "Topbar PR chip — web slice" }],
    ["failing", "github", { on: "Open PR", kind: "external" }],
  ],
});
```

- Literal only, AST-parsed, never executed. Prototype nodes need `proto` and `state` (both must
  exist); external nodes use `github`, `terminal`, `desktop`, `docs` or `other` (which needs a
  `label`), with an optional absolute `https://` URL. External pages are never framed.
- Edges are `[from, to, { on, kind? }]` with `navigate` (default), `overlay` (same screen),
  `redirect` or `external`. `on` is the **exact text of the real button or link**.
- Every node is reachable from `start`; a node without outgoing edges declares `end: true`;
  repeated `[from, to, on]` edges are rejected. Steps are numbered breadth-first.
- Coverage warnings list prototype states, in specs a flow owns or crosses, that no flow reaches.
  They never fail a check.

The canvas (`?flow=`) is read-only: ELK lays it out, prototype cards mount inert live previews at
60 % zoom or more once the viewport settles (three at a time, twelve kept), and the side panel
shows the selected step (arrives from, leads to, also in) or the flow's comments. The prototype
toolbar's **In N flows** menu jumps to every step that shows the state on screen.

```sh
bun run proto flow new <id> [--spec <id>] [--title <text>]
bun run proto flow list [--spec <id>] [--json]
bun run proto flow show <id> [--json]
bun run proto flow coverage [--spec <id>] [--json]
bun run proto flow url <id> [--node <id>]
```

## Comments

Reviewers press C (or the comment button), click an element or drag an area of the canvas, and
write; pins follow the element through source tags (`data-proto-src`), selectors, test ids and a
React fiber fallback. Flow comments are written in the flow's side panel about the selected step,
or the whole flow. Each key has `web/gallery/comments/<key>/comments.json` (version 2: thread,
`context{commit,state,theme,viewport}`, element/region/node target) plus its images (PNG, JPEG,
WebP or GIF, at most 5 MB each and six per message). Flow keys are `flow-<id>`. Comments are local
only and served by the dev server's `/__comments` middleware; edits from the CLI or by hand reach
the open gallery through HMR. The sidebar shows open counts per prototype, spec and flow.

```sh
bun run proto comments list [--slug <key>] [--status open|resolved|all] [--json]
bun run proto comments show <id>
bun run proto comments reply <id> --note "…" [--author agent|reviewer]
bun run proto comments resolve <id> --note "what changed"
bun run proto comments reopen <id> [--note "why"] · delete <id>
bun run proto comments edit <id> --note "…" [--message <n>]
bun run proto comments attach <id> <file…> · detach <id> <attachment id> · attachments <id> --out <dir>
```

Agents apply comments with the `eng-prototype-comments` skill
(`.agents/skills/eng/eng-prototype-comments/`): read the comment, change the owning view or
draft, verify the same state, theme and width, then reply and resolve through the CLI.

## Screenshots and evidence

`bun run proto screenshot <slug> [--state <id>] [--theme light|dark] [--width <px>] [--out <file>]`
saves a full-page PNG of the standalone URL in Playwright's Chromium, reusing a running `proto dev`
or starting and stopping one for the capture (default `.tmp/proto/<slug>--<state>--<theme>--<width>.png`).
Capture both themes for visual changes and inspect the images. `eng-ui-screenshot` covers visual
contract bundles.

## Checks

- `bun run proto check [--json]`: discovery (slugs, metadata, markup, specs), flows, and a gallery
  build that writes nothing (network guard). Prints coverage warnings without failing.
- Harness tests: `web/gallery/__tests__/` (discovery, flows, comments store, URL view, network
  guard), run by the web Vitest project.
- Prototypes and drafts follow the web lint and design-system rules; the 500-line source limit
  applies.

## Citing prototypes in specs

Specs and visual contracts cite a prototype by slug and state, with its gallery URL:
`session-board-card` · `needs-you` (`/?p=session-board-card&state=needs-you`), and journeys by
flow id (`/?flow=board-to-pull-request`). A UI spec that adds prototypes also adds or updates the
flows its UI/UX contract describes. Existing `docs/design/opendesign/` boards stay as historical
references; production (`packages/ui` + `web/`) wins on any conflict.

Verified 2026-10-29 in the `proto-gallery` worktree: `bun run proto dev`, `proto check`,
`proto screenshot` (sessions board and review digest in both themes), `proto flow
list|show|coverage|url`, `proto comments list`, `bun run --cwd web typecheck`.
