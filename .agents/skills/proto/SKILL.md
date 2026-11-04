---
name: proto
description: Author, set up and debug @compozy/proto prototypes, drafts, flows, existing Storybook stories and the canvas. Use when editing `*.proto.tsx`, `src/drafts/`, `*.flow.ts`, `proto.canvas.tsx` or `proto.config.ts` and its Vite, Storybook or Vitest guards, when running the `proto` CLI or MCP tools, or when `proto check`, `proto dev` or a capture fails. Not for UI edits that touch no prototype, nor for gallery review comments (proto-apply-comments).
---

# proto

`@compozy/proto` is a prototype gallery and review loop for React + Vite apps. A prototype renders
the app's **real, shipped views** with fixture props, one state at a time. People open the gallery,
pin comments to rendered elements, and each comment records the JSX source chain of what they
clicked. Agents fix that source and answer through the CLI or MCP. **People approve; agents never
do.** The point is that the reviewed screen _is_ the code that ships, so there is nothing to
translate from a mockup afterwards.

This skill is the shipped contract and is replaced by `proto init --skills` on upgrade. Put
project rules (which views exist, where fixtures live, which specs to use) in the project's
`AGENTS.md` or `CLAUDE.md`, not in this file.

## Mental model

- **Component prototype**: `<view>.proto.tsx` beside the view it composes. It imports the view
  and picks props per state. It contains no markup of its own: any lowercase JSX tag (`<div>`)
  is a `markup` problem. Layout belongs in the view.
- **Draft**: `src/drafts/<slug>/index.proto.tsx`, an exploration that may use free markup. Drafts
  never get the production `Frame`.
- **Flow**: `flows/<id>.flow.ts`, a journey across prototype states and external pages.
- **`meta` is data, never code.** `export const meta = definePrototype({...})` (and
  `export const flow = defineFlow({...})`) is read from the syntax tree and never executed. Only
  string, boolean, array and object literals are allowed: no variables, numbers, spreads,
  template strings or computed values.
- **Base origins.** The shell (gallery and review API) runs on `127.0.0.1:<dev.port>` (4400 by
  default). It is prebuilt and never imports app code. Prototypes render on the render origin,
  `port + 1`, compiled by the app's own `vite.config.ts` in the `proto-render` mode, one state
  per iframe document. The two talk only through a validated `postMessage` protocol. App CSS
  loads on the render origin only.
- **Canvas and Frame.** `proto.canvas.tsx` exports `Canvas` (providers around every state; it
  receives `theme` and `role`) and an optional `Frame` (production chrome around component
  prototypes that are not `standalone`).
- **Temporary sharing.** `proto share` sends a scoped built snapshot through Cloudflare Access.
  Use CLI-only `share connect`, `share <slug> --approver <email>` (or `--flow` / `--all`),
  `share status`, `share refresh`, `share stop`, and `share disconnect`; all accept `--json`.
  Start runs until stopped, and approvers require a clean committed tree. Agents never approve.
  See `references/cli.md` for invitations and output.
- **Review store.** `proto/review/<key>.json` per prototype (key = slug) or flow
  (key = `flow-<id>`), committed with the code. Comment ids are `<key>-<n>`. Each key is
  `in_review` or `approved`; approval records the commit and is a human act. A new comment on an
  approved prototype puts it back in review.
- **Guards.** The production boundary (`proto()` in `vite.config.ts`) fails the app build when
  prototype-only code reaches it. The network guard fails `proto check`, `proto build` and
  Storybook/Vitest with `protoPresentation()` when a prototype, draft or story reaches a module
  listed in `guards.forbidRuntimeImports`; it cannot see a global `fetch` or an unlisted client.
  Matching rules: `references/setup.md`, section Guards.

## Proto or Storybook

- **Proto**: screens and journeys that people review and approve: states of a real view inside
  the app chrome, flows across states, comments anchored to source, evidence for a spec.
- **Storybook**: the component catalog, args and controls, per-component interaction and a11y
  tests. Proto can host its existing CSF stories for browsing and review; addon panels and
  addon-vitest stay in Storybook. Root guards are explicitly configured per story root.
- **Neither**: a UI change with no prototype to update. Edit the view and run its own checks.

## Hard rules

1. **Never weaken a guard to make something render.** Do not remove `forbidRuntimeImports`
   entries, set `guards.productionBoundary: false`, drop `proto()` or `protoPresentation()`, or
   move a file out of a guarded path. A guard failure means a view reaches data access. Move
   that access to the route and pass props instead.
2. **Prototypes compose views with fixtures.** When a state needs something the view cannot show,
   add a prop to the view. Never copy the view's JSX into the `.proto.tsx`.
3. **Import the view's own module.** Do not import a barrel that also re-exports routes, hooks or
   API clients. It drags the data layer into the graph, and the network guard fails it as
   `network_import` once those modules are listed.
4. **Agents never approve or reopen reviews**, never run `review unlock`, and delete comments only
   when a person asks. Resolving a comment is not approval.
5. **Never edit `proto/review/**` by hand.** Use the CLI, MCP or the gallery.

## Working loop

1. Orient: `proto list --json`, `proto flow list --json`, and `proto check --json` for the
   current problems. Add `--app <dir>` in a monorepo.
2. Scaffold: `proto new --in src/<path>/<view>.tsx` (component prototype),
   `proto new <slug> --draft` (draft), or `proto flow new <id>` (flow).
3. Edit, then run `proto check --json` until it exits 0, with an empty `problems` and no `error`.
   Warnings never fail. The compile step, and with it `network_import`, only runs once the
   registry problems are gone.
4. Look at every state you touched: with `proto dev` running, open
   `proto url <slug> --state <id> --bare`, or capture `proto screenshot <slug> --state <id>`
   (`--theme dark`, `--width <px>`), which starts a temporary server when none is running.
5. Run the project's typecheck and the view's own stories or tests: `check` does not type-check.

The work is done when `check` passes and the touched states render as intended. Approval stays
with people.

Invoke the binary the way the project does: `bunx proto`, `npx proto`, or a package script such as
`bun run proto <command> --app app`.

## References

Read only the one you need:

- `references/authoring.md`: before writing or editing a prototype, draft, flow or canvas
  (meta fields, states, fixtures, URL state, flow nodes and edges).
- `references/setup.md`: when installing proto, editing `proto.config.ts`, or wiring Vite,
  Storybook or Vitest (config keys and defaults, guards, the render origin, skills install).
- `references/cli.md`: when you need a command's flags, JSON output, exit codes, error codes or
  the MCP tools.
- `references/troubleshooting.md`: when `check`, `build`, `dev`, Storybook or a capture fails,
  or the gallery renders something unexpected.

For gallery review comments (a comment id like `login-form-7`), use the `proto-apply-comments`
skill.

## Existing Storybook stories

Add `stories.roots` to `proto.config.ts`: each root requires `id`, `stories` globs, `preview`
and `vite`; optional `dir` defaults to `.`, `title` to id, `themeGlobal` to `theme`, `static`
and `guards.forbidRuntimeImports` to empty. Paths are relative to `dir`. The root Vite config
installs React, CSS and aliases without `proto()`; `.storybook/main.ts` is never executed.
Install `storybook` and `@storybook/react-vite` at `~10.6.0` in each root. Never rewrite CSF.
The two base origins are joined by one origin per root at `port + 2 + index`; clear old MSW
workers/site data after reordering roots. Stories use their preview, not `proto.canvas.tsx`.
`sb.mock` is inert (`story_mock_unsupported`); do not claim module mocking works.

- `proto stories list --root <id> --filter '<story-id-glob>' --json` returns roots and files.
  No config returns empty arrays (human: `No stories configured.`), exit 0.
- `proto stories run --root <id> --filter '<glob>' --theme dark --json` runs selected roots only.
  Options: `--concurrency` (6), `--timeout` ms (30000, clamped 1000–120000), `--port`.
  JSON: `summary` and `stories` rows (`id`, `root`, `stage`, `status`, `hasPlay`, `durationMs`,
  `error`). Registry problems do not prevent selected stories running, but exit 1.
  Exit 0 all pass, 1 failure/empty filter, 3 missing Playwright. Existing servers must match
  `projectId`; interrupt closes only owned resources without a partial report.
- `proto stories url <id>` links `/?c=<component>&story=<id>`.
- MCP: `proto_stories_list`, `proto_stories_run` (requires root or filter), `proto_stories_url`.
- `check` validates story config/index only. Guards run in the story server/runner, without
  inheriting the app prototype guard. Local builds include lazy story roots under
  `__stories/<root>/`, including local `story--` review snapshots when requested; story problems
  and missing/unsupported peers fail them. MSW needs a base-aware worker URL and scope confined
  to that root (see the package README). Remote builds omit stories with `stories_not_built`;
  story problems/missing peers do not fail remote builds.
  Reference `static` mounts and `publicDir` assets base-relative (e.g. via `import.meta.env.BASE_URL`) so they work in local static builds.
- Component comments use `story--<slug>`; `comments add` requires `--state <story-id>` from that
  component plus `--note` and `--selector`. Remote review does not accept story keys.

Errors: `invalid_input` exit 2 (unknown root, missing/invalid state); `story_not_found` exit 1;
`project_mismatch` exit 1 (pass another `--port`); `storybook_missing` / `storybook_unsupported`
exit 3 (install supported peers); `playwright_missing` exit 3 (install Chromium).
