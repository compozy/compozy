# Troubleshooting

Find the symptom, then fix the cause. Never "fix" a guard failure by loosening the guard.

## Command resolves to moonrepo

Run `bun run proto`, `bunx proto` or `npx proto` from the project with `@protohq/cli`
installed. These resolve the local `proto` binary; a bare global `proto` may instead run
moonrepo's toolchain manager.

## `proto check` problems

- **`markup`**: a component prototype contains lowercase JSX. Move it into the view, or add a
  prop.
- **`invalid_meta`**: one of these:
  - `meta` is not a literal (a variable, number, spread or template string);
  - it lacks `title` or `states`;
  - a draft is not at `<drafts>/<slug>/index.proto.tsx`.
- **`invalid_slug`**: the prototype's own file name is not kebab-case. Component slugs come from
  the `.proto.tsx` file name, so `login-form.proto.tsx` is `login-form`, even beside a view called
  `LoginForm.tsx`.
- **`reserved_slug`**: prototype slugs cannot start with `flow-`.
- **`duplicate_slug`**: two prototypes share a file name, or two drafts share a directory name.
- **`unknown_spec` / `unknown_role`**: add the id to `specs` or `roles` in `proto.config.ts`, or
  fix the typo.
- **`network_import`**: read `chain`. The first import that reaches a forbidden module is the
  one to cut. Usually a view imports a hook or client, or the prototype imports a barrel that
  re-exports routes and data modules. Import the view file directly, and move data access to the
  route.
- **`unknown_prototype` / `unknown_state` / `unknown_node`**: a flow names something that does not
  exist. `proto list --json` shows the real slugs and states.
- **`unreachable` / `dead_end`**: add the missing edge, or set `end: true` on terminal nodes.
- **`error`** (a string, exit 1): the render build failed for another reason, such as a syntax
  error, an unresolved import or a failing app plugin in a prototype, the canvas or the app
  config. `check` does not type-check, so type errors never show up here; run the project's
  typecheck. Read the message, and see [Render origin](#render-origin).
- **`network_import` only appears after other problems are fixed**: `check` compiles only when
  the registry has no problems.

## `proto check` warnings (never fail)

- **`specs_unavailable`**: `specs` is a function that returned `null`, so spec validation is
  skipped. This is expected when the catalog comes from files that are absent, such as an
  ignored `.compozy/tasks` in CI or in a fresh worktree. Leave it. Do not make the function return
  `{}` (every spec would become unknown), and do not strip `spec` from prototypes.
- **`canvas_missing`**: there is no `proto.canvas.tsx` (or configured `canvas`), so states render
  without providers or CSS. Create the canvas.
- **`uncovered_state`**: a flow touches the spec, but no flow reaches this state. Add a node or
  ignore it.

## Scaffolding and paths

- **`View must export a named component`** from `proto new --in`: the first `export` in the view
  file is not the component (for example `export type Props` comes first). Move the component
  export above it.
- **`Use exactly one of --draft or --in`**, **`--in must name the view .tsx file`**, or
  **`Unknown spec`** (`--spec` is not configured): fix the arguments.
- **`Invalid slug`** (`invalid_config`) from `proto new`: the slug, or the `--in` file name, is
  not kebab-case. `--in` takes the slug from the view's file name, so for `LoginForm.tsx` write
  `login-form.proto.tsx` by hand.
- **`This command does not support --remote`** (exit 2): only the comment and review commands and
  `url` take `--remote`. Run `check`, `flow show`, `screenshot` and the rest locally.
- **`Unknown flow`** from `flow show`, or a flow missing from `flow list`: the flow has a
  problem. Read `proto check --json`.
- **`slug_taken`**: a prototype, flow or target file with that slug already exists.
- **`Path must stay inside the app root (invalid_source)`**: a path resolves outside the app root.
  This applies to `evidence.dir`, `comments attachments --out`, `comments attach <file>` and
  `new --in`. Keep `evidence.dir` inside the app (the default is `docs/design`) and copy the
  output elsewhere afterwards when needed.

## Comments

- **`selector is required`** (exit 2) from `proto comments add <slug>`: element comments need
  `--selector <css>` (and preferably `--source <path:line:col>`). Flow comments use the key
  `flow-<id>` with an optional `--node <node>`. Agents add comments only when asked to; reviewers
  normally comment in the gallery.
- **`message_not_owned`**: `edit`, `attach` and `detach` only touch your own messages. Reply
  instead.
- **`comment_not_found`**: check the id with `proto comments list --status all --json`, and
  check that `--app` selects the app that owns the review store.
- **`key_busy`**: another writer holds the review lock. Retry. Run `review unlock` only when a
  person asks, as operator recovery.

## Render origin

`proto dev`, `check`, `build`, `screenshot` and `evidence` compile prototypes with the app's own
`vite.config.*` in the `proto-render` mode, so every app plugin runs there.

- **A route generator rewrites files, devtools inject UI, Sentry or another upload plugin runs or
  asks for credentials, or a dev proxy starts**: gate app-only plugins with
  `defineConfig(({ mode }) => ...)` and skip them when `mode === "proto-render"`. See
  `setup.md`. Also check `.env.proto-render` when env-dependent plugins misbehave.
- **The prototype renders unstyled**: the canvas does not import the app CSS. CSS loads on the
  render origin only, through `proto.canvas.tsx`.
- **One theme looks wrong**: the render host toggles `.dark` and `color-scheme` on `<html>`. A
  design system with another convention (for example `:root` = dark plus a `.light` class) must
  paint the theme in `Canvas` from its `theme` prop. See `authoring.md`.
- **A render error appears in the frame, or a capture fails with `Prototype failed to render.`**:
  open `proto url <slug> --state <id> --bare` and read the browser console. A view that needs a
  provider (router, i18n, toasts) gets it from `Canvas`; a view that needs data gets props.
- **`host_not_allowed`**: open the gallery at `127.0.0.1`, or add the hostname (without the port)
  to `dev.allowedHosts`.
- **"Proto shell is not built"**: only in a source checkout of proto. The published package
  ships the prebuilt shell, so run the package's `build:shell` in that checkout.

## Local serving and connection

Run `proto status --json` first; it starts nothing. Inspect
`~/.config/protohq/daemon/daemon.log` for startup details.

- **`port_in_use`**: another program or another user's Proto holds the address. Stop your own
  Proto first if necessary, then set `PROTOHQ_PORT=4500 proto start`. The default is 4400; the
  last port is remembered. For preview exhaustion, reservations are freed when project folders
  disappear or have not run for 30 days; for occupied previews, read the reported port list.
- **`daemon_unavailable`**: Proto did not start within 10 seconds (read the log), or its PID does
  not answer (run `proto stop`, then `proto start`, as the error directs).
- **`daemon_version_mismatch`**: upgrade `@protohq/cli` in this project; the running Proto needs a
  newer protocol. Ordinary version skew warns on stderr; newer project packages upgrade Proto
  and running projects reconnect. `status` marks older project versions as `behind`.
- **`state_unavailable`**: Proto cannot use `~/.config/protohq/daemon`. Read the reason (permissions,
  unreadable data or a held state lock); preserve state and fix that cause before retrying.
- **`project_busy`**: another capture is serving the project or `proto dev` is still starting
  after 60 seconds. Wait for that command, or keep `proto dev` running to share a ready server.
- **`plan_required` / `seat_required` on agent operations**: stop the paid step and show the
  error message and action URL from `next`. Continue with Free commands such as `proto check`
  or `proto url`.
- **`network_unavailable` on agent operations**: restore connectivity to Proto Cloud and try
  again. The operation needs an online decision before it runs.
- **`rate_limited` on agent operations**: honor `retryAfter` (seconds). Each retry needs a new
  online check.
- **`cli_outdated`**: install `@protohq/cli@latest` (0.3.0 or later) and rerun
  `proto init --skills` to refresh the agent instructions.
- **`project_offline`**: start this project with the `cd <folder> && proto dev` command in the
  message. For `proto url <slug> --bare`, start it once to reserve its first preview address.
- **`project_not_found`**: open home at `http://127.0.0.1:4400/` and select a known project.
- **A connection overlay or stopped project page**: Ctrl-C stops only that project's terminal.
  Restart it with `proto dev`. `proto stop` instead leaves projects alive and waiting for
  `proto start`. Captures can still use their previews. An unexpected Proto stop triggers
  automatic recovery; if it fails, project terminals retry and print the error.

Discover gallery links with `proto url <slug>` (under `/p/<label>/`) and separate previews with
`proto url <slug> --bare`. These commands never start anything. Preview and story addresses are
reserved per project; use `proto status --json` for running root addresses. `dev.allowedHosts`
applies only to project URLs; the home remains on 127.0.0.1. Lifecycle commands have `--json`
forms; `dev` emits one stdout JSON line and sends subsequent updates to stderr.

## Storybook and Vitest

- **`storybook build` fails with `production_boundary: Prototype-only module reached the
  production graph: …stories.tsx`**: Storybook loaded the app's `vite.config.ts`, which contains
  `proto()`, and the production boundary treats stories and `.storybook/` as prototype-only.
  Move the shared rendering config (React, compiler, Tailwind, aliases) to `vite.shared.ts`, point
  Storybook at it with `framework.options.builder.viteConfigPath`, and add `protoPresentation()`
  in `viteFinal`. Do not remove `proto()` from the app config. See `setup.md`.
- **A story fails with `network_import`**: the same guard as prototypes. Render the view with
  props.

## Installing and upgrading

- **`playwright_missing`** (exit 3): `playwright` cannot be imported or Chromium cannot launch.
  Use the printed commands to install the peer and Chromium. Each Playwright version needs its
  own browser build, so run it again after changing versions.
- **An unmet peer warning for `playwright`**: the peer range is `^1.64.0`, but capture only uses
  long-stable Playwright APIs, and projects run it on `1.63.0`. The warning alone is not a
  failure. Upgrade when you can instead of pinning proto down.
- **A new `@protohq/cli` release is not installable yet**: package-manager policies with a
  minimum release age reject versions published within the window. Exempt this package for the
  install, for example `bun add -d @protohq/cli@<version> --minimum-release-age-excludes=@protohq/cli`.
  With Bun, that flag replaces the `bunfig.toml` exclusion list for the run. Do not turn the
  policy off globally.
- **Two copies of the comment skill load, or old instructions mention `author-prototype` or
  `apply-prototype-comments`**: those are the 0.1.x skills. `proto init --skills` installs the
  current ones and lists the legacy directories under `legacySkills`; delete them, and their
  `.claude/skills/` links.
- **`not_a_vite_app`**: run `init` in the app directory, or pass `--app <dir>`.
- **`not_initialized`**: run `proto init` in the app, or pass `--app <dir>` to select an initialized app.
- **`app_required`**: several `apps/*/proto.config.ts` exist; pass `--app <dir>`.
