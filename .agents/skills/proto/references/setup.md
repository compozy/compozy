# Setup, configuration and guards

Contents: install · `proto init` · `proto.config.ts` · guards · Vite, Storybook and Vitest ·
the render origin · agent skills.

## Install

Proto needs a React 19 + Vite 8 app and uses the app's own Vite and React installation.

```sh
bun add -d @compozy/proto     # or npm i -D / pnpm add -D
bunx proto init --skills      # from the app directory, or pass --app <dir>
bunx proto dev --open
```

Optional peers, installed only for the features you use:

| Feature                  | Peer                                    | Extra step                         |
| ------------------------ | --------------------------------------- | ---------------------------------- |
| `screenshot`, `evidence` | `playwright` (`^1.64.0`)                | `bunx playwright install chromium` |
| `proto mcp`              | `@modelcontextprotocol/sdk` (`^1.32.2`) |                                    |
| Postgres review storage  | `postgres` (`^3.4.0`)                   | in the backend that mounts it      |

A missing Playwright or MCP SDK exits 3, with `playwright_missing` or a missing-dependency
message.

## `proto init`

`init` needs a `vite.config.ts`, `.js` or `.mts` in the app root, or it fails with
`not_a_vite_app`. It writes:

- `proto.config.ts`, `proto.canvas.tsx`, `src/drafts/welcome/index.proto.tsx`;
- `docs/design/prototyping.md`, the design record used by open-design tooling. Many projects
  ignore `docs/`, so nothing in the skills depends on this file;
- `proto/review/.gitignore`, which ignores locks, temporary files and pending uploads;
- `proto()` added to the Vite config's `plugins` array, plus its import.

When `proto.config.ts` already exists, `init` writes none of these. `--force` rewrites all of them,
including your config and canvas, so do not use it on a configured app. When only some of the
files exist, `init` stops with `invalid_config` (`<file> exists; use --force`).

`--skills` installs the agent skills even when the app is already set up. See
[Agent skills](#agent-skills).

## `proto.config.ts`

The app root's `proto.config.ts` is the only config file. The CLI, MCP and both Vite plugins load
it through Vite's config loader, so it may import Node modules.

```ts
import { defineProtoConfig } from "@compozy/proto/config";

export default defineProtoConfig({
  title: "Acme",
  specs: { auth: "Authentication", billing: "Billing" },
  roles: { member: "Member", admin: "Admin" },
  guards: {
    forbidRuntimeImports: ["src/lib/api.ts", "src/lib/gateway-*.ts", "@tanstack/react-query"],
  },
});
```

| Key                           | Default                | Meaning                                                        |
| ----------------------------- | ---------------------- | -------------------------------------------------------------- |
| `title`                       | (required)             | Gallery title                                                  |
| `prototypes`                  | `"src/**/*.proto.tsx"` | Glob or globs of prototype files                               |
| `drafts`                      | `"src/drafts"`         | Drafts directory, inside the `prototypes` glob                 |
| `flows`                       | `"flows"`              | Directory of `<id>.flow.ts` files                              |
| `canvas`                      | `"proto.canvas.tsx"`   | File exporting `Canvas` and optional `Frame`                   |
| `specs`                       | `{}`                   | `{ id: label }`, or `(root) => Record<string, string> \| null` |
| `roles`                       | `{}`                   | `{ id: label }` offered by the role picker                     |
| `siteUrl`                     | none                   | Origin that resolves path `url`s of external flow nodes        |
| `locale`                      | `"en"`                 | Shell language: `"en"` or `"pt-BR"`                            |
| `guards.forbidRuntimeImports` | `[]`                   | Modules that prototypes, drafts and stories must not reach     |
| `guards.productionBoundary`   | `true`                 | Fail the app build when prototype-only code reaches it         |
| `review.remote.url`           | none                   | Deployed gallery used by `--remote` and `build --remote`       |
| `evidence.dir`                | `"docs/design"`        | Screenshot and evidence output, inside the app root            |
| `build.includeReview`         | `false`                | Ship review history in static `proto build` output             |
| `dev.port`                    | `4400`                 | Shell port; the render origin uses `port + 1`                  |
| `dev.poll`                    | `false`                | Polling file watcher                                           |
| `dev.allowedHosts`            | `[]`                   | Extra hostnames, without ports, accepted by both dev servers   |

- Spec and role ids match `^[a-z0-9][a-z0-9_-]*$`.
- `specs` as a function runs in Node with the app root. Returning `null` skips spec validation:
  `proto check` warns `specs_unavailable` and accepts any `spec`. Use `null` when the catalog
  comes from files that can be absent, such as an ignored directory in CI or a fresh worktree.
  Returning `{}` makes every `spec` unknown. Throwing is `invalid_config`.
- Environment overrides: `PROTO_PORT`, `PROTO_POLL=1`, `PROTO_REMOTE_URL`, `PROTO_REMOTE_TOKEN`.

## Guards

**Production boundary.** `proto()` in the app's `vite.config.ts` fails `vite build` with
`production_boundary: Prototype-only module reached the production graph: <id>` when any of these
reach the app bundle:

- `.proto`, `.fixtures` and `.stories` modules with a `js`, `jsx`, `ts` or `tsx` extension
  (optionally `c`/`m`), so `x.fixtures.ts` counts and `x.fixtures.json` does not;
- the drafts and flows directories, `preview/` and `.storybook/`;
- the canvas file and `@compozy/proto` itself.

The boundary steps aside in the `proto-render` mode that proto uses for its own builds. Keep
`guards.productionBoundary` on.

**Network guard.** `guards.forbidRuntimeImports` (empty by default) lists data-layer modules,
such as API clients, query libraries, auth clients, env and config modules, generated SDKs and
mocks. The guard knows only these modules: an unlisted client, or a global `fetch`, passes. An
entry matches in one of these ways:

- an exact app-relative path (`src/lib/api.ts`);
- a path prefix (`src/lib/api` also matches `src/lib/api/...`);
- a glob (`src/lib/gateway-*.ts`);
- a package name, matched after the last `node_modules/` (`better-auth`);
- a `RegExp`.

The guard follows runtime imports from every `*.proto.*`, `*.stories.*` and `src/drafts/` module,
after types are erased, so `import type` is fine. A match fails with:

```text
network_import: src/x.proto.tsx reaches src/lib/api.ts: src/x.proto.tsx → src/x.tsx → src/lib/api.ts. Stories and prototypes render views; move data access to the route container.
```

It runs on the render origin (`proto dev`, `proto check`, `proto build`, captures) and wherever
`protoPresentation()` is installed. The chain names the import to cut. Usually that is a view
importing a hook or client, or a barrel `index.ts` that re-exports both views and data modules.

## Vite, Storybook and Vitest

Split the Vite config so that each consumer gets only what it should run:

```text
vite.shared.ts    React (+ compiler), Tailwind, aliases, optimizeDeps. No proto(), no app-only plugins
vite.config.ts    mergeConfig(shared, { plugins: [...appOnlyPlugins, proto()] })
.storybook/main.ts  builder.viteConfigPath -> vite.shared.ts; viteFinal adds protoPresentation()
vitest.config.ts  browser/story projects extend vite.shared.ts and add protoPresentation()
```

```ts
// .storybook/main.ts
import { protoPresentation } from "@compozy/proto/vite";
import type { StorybookConfig } from "@storybook/react-vite";
import { fileURLToPath } from "node:url";
import { mergeConfig } from "vite";

const config: StorybookConfig = {
  stories: ["../src/**/*.stories.tsx"],
  framework: {
    name: "@storybook/react-vite",
    options: {
      builder: { viteConfigPath: fileURLToPath(new URL("../vite.shared.ts", import.meta.url)) },
    },
  },
  viteFinal: config => mergeConfig(config, { plugins: protoPresentation() }),
};
export default config;
```

Storybook loads `vite.config.ts` by default. With `proto()` in that file, the production boundary
treats stories and `.storybook/` as prototype-only modules, and `storybook build` fails.
`protoPresentation({ sourceTags: true })` also stamps `data-proto-src` source attributes; the
render origin always does.

## The render origin uses the app's Vite config

`proto dev`, `proto check`, `proto build`, `screenshot` and `evidence` compile prototypes with the
app's own `vite.config.*`, from the app root, in the `proto-render` mode. Proto adds its render
plugins and the network guard, and `proto()` disables itself. **Every other app plugin runs
too**: route generators, devtools, Sentry or other upload plugins, PWA plugins and dev proxies.
Gate the ones prototypes do not need on the mode:

```ts
export default defineConfig(({ mode }) => {
  const appOnly = mode === "proto-render" ? [] : [tanstackRouter(), devtools(), sentry()];
  return mergeConfig(shared, { plugins: [...appOnly, proto()] });
});
```

`loadEnv(mode, ...)` in that config reads the `.env.proto-render` files in this mode. Proto sets
its own server host, port, `strictPort`, `allowedHosts` and watcher on top of the app config.

## Agent skills

`proto init --skills` copies the package's skills into `.agents/skills/` at the repository root
(the Git top level), or at the app root outside Git:

- `proto`: concepts, authoring, setup, CLI and troubleshooting (this skill);
- `proto-apply-comments`: the review-comment loop.

Run it again after upgrading `@compozy/proto`. The command is idempotent: it reports each skill as
`installed`, `updated` or `unchanged`, and it never touches `proto.config.ts` or other files in an
app that is already set up. An `updated` skill is replaced exactly, so local edits to the
installed copy are lost. Keep project rules in `AGENTS.md` or `CLAUDE.md`.

Copies of the 0.1.x skills (`author-prototype`, `apply-prototype-comments`) are reported under
`legacySkills` and are never deleted. Remove them, including any `.claude/skills/` links that
point at them, so agents do not load two versions of the same instructions. Harnesses that read
`.claude/skills/` need a link or copy of each new skill there too.
