# Setup, configuration and guards

Interactive `proto dev`, `proto start`, `proto init` and `proto mcp` startup need a free
account. Sign in with `proto login` using Google, GitHub or an email link. A stored login lets
these commands start when the sign-in check cannot reach Proto Cloud. A truthy `CI` skips only
the Free startup sign-in rule. Use `PROTOHQ_TOKEN` from a seat holder for Pro operations in CI.
Headless Free commands such as `proto check` and `proto build` and the Vite, Storybook and Vitest
integrations need no login or network access.

Local `comments *`, `screenshot`, `evidence` and each local MCP tool call need an online
check with Proto Cloud. Without a Pro Editor seat, they use the account's one-time allowance of
100 free operations. See [CLI](cli.md) for the access and output contract.

Contents: install · `proto init` · `proto.config.ts` · guards · Vite, Storybook and Vitest ·
the render origin · agent skills.

## Install

Proto needs a React 19 + Vite 8 app and uses the app's own Vite and React installation.

```sh
bun add -d @protohq/cli     # or npm i -D / pnpm add -D
bunx proto login             # Google, GitHub or email; free account
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

When `proto.config.ts` already exists, `init` preserves configuration and rewrites an older
preview record to `preview: "proto url {slug} --bare"`, reporting the changed file. `--force` rewrites all of them,
including your config and canvas, so do not use it on a configured app. When only some of the
files exist, `init` stops with `invalid_config` (`<file> exists; use --force`).

`--skills` installs the agent skills even when the app is already set up. See
[Agent skills](#agent-skills).

## `proto.config.ts`

The app root's `proto.config.ts` is the only config file. The CLI, MCP and both Vite plugins load
it through Vite's config loader, so it may import Node modules.

```ts
import { defineProtoConfig } from "@protohq/cli/config";

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
| `evidence.dir`                | `"docs/design"`        | Screenshot and evidence output, inside the app root            |
| `build.includeReview`         | `false`                | Ship review history in static `proto build` output             |
| `dev.poll`                    | `false`                | Polling file watcher                                           |
| `dev.allowedHosts`            | `[]`                   | Extra hostnames, without ports, for this project's URLs        |

- Spec and role ids match `^[a-z0-9][a-z0-9_-]*$`.
- `specs` as a function runs in Node with the app root. Returning `null` skips spec validation:
  `proto check` warns `specs_unavailable` and accepts any `spec`. Use `null` when the catalog
  comes from files that can be absent, such as an ignored directory in CI or a fresh worktree.
  Returning `{}` makes every `spec` unknown. Throwing is `invalid_config`.
- Environment overrides: `PROTOHQ_PORT`, `PROTOHQ_POLL=1`, `PROTOHQ_TOKEN`, `PROTOHQ_WORKSPACE`, `PROTOHQ_API_URL` (loopback, or exactly `https://staging-api.protohq.design` for staging).

## Shared Proto address

`proto dev` serves this project at `http://127.0.0.1:4400/p/<label>/` and starts Proto in the
background when needed. `/` lists projects. Every app keeps its own foreground terminal; Ctrl-C
stops only that app. `proto stop` leaves those terminals waiting; `proto start` reconnects them.
`proto status` never starts anything. All three support `--json`, as does `dev` (one stdout JSON
line followed by lifecycle messages on stderr).

`PROTOHQ_PORT` selects Proto's port, default 4400. Proto remembers the last port when the variable
is absent. Stop it before using `PROTOHQ_PORT=4500 proto start` to move the address; a different
variable does not move an already running Proto. `PROTOHQ_POLL=1` enables polling for this project.
`dev.allowedHosts` applies only to this project's URLs; home and the project list stay on the
loopback address. Localhost redirects to 127.0.0.1.

Prototypes and story roots have separate reserved preview origins; reordering roots preserves
addresses. Discover a prototype's address with `proto url <slug> --bare` after its first start,
and running story addresses with `proto status --json`. Never construct preview URLs from the
Proto port. Capture commands need no background process and can temporarily serve the project.
Startup diagnostics live in `~/.config/protohq/daemon/daemon.log`; see `troubleshooting.md` for
`port_in_use`, `daemon_unavailable`, `daemon_version_mismatch`, `state_unavailable`, `project_busy`
and `project_offline`.

## Guards

**Production boundary.** `proto()` in the app's `vite.config.ts` fails `vite build` with
`production_boundary: Prototype-only module reached the production graph: <id>` when any of these
reach the app bundle:

- `.proto`, `.fixtures` and `.stories` modules with a `js`, `jsx`, `ts` or `tsx` extension
  (optionally `c`/`m`), so `x.fixtures.ts` counts and `x.fixtures.json` does not;
- the drafts and flows directories, `preview/` and `.storybook/`;
- the canvas file and `@protohq/cli` itself.

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
import { protoPresentation } from "@protohq/cli/vite";
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

Run it again after upgrading `@protohq/cli`. The command is idempotent: it reports each skill as
`installed`, `updated` or `unchanged`, and it never touches `proto.config.ts` or other files in an
app that is already set up. An `updated` skill is replaced exactly, so local edits to the
installed copy are lost. Keep project rules in `AGENTS.md` or `CLAUDE.md`.

Copies of the 0.1.x skills (`author-prototype`, `apply-prototype-comments`) are reported under
`legacySkills` and are never deleted. Remove them, including any `.claude/skills/` links that
point at them, so agents do not load two versions of the same instructions. Harnesses that read
`.claude/skills/` need a link or copy of each new skill there too.
