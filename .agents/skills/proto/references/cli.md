# CLI, JSON, exit codes and MCP

Contents: invocation · commands · JSON shapes · exit and error codes · MCP.

## Invocation

```sh
proto <command> [positionals] [options] [--json] [--app <dir>] [--verbose]
```

- `--json` prints exactly one JSON value on stdout, which is the contract. Human output is for
  people and may change. Exception: `proto mcp --json` startup failures write the JSON error
  envelope to stderr and leave stdout empty, because stdout belongs to JSON-RPC.
- App selection: `--app <dir>` (relative to the working directory, or `apps/<dir>`). Without it,
  the CLI walks up to the nearest `proto.config.ts`, then tries a single `apps/*/proto.config.ts`.
  Several candidates fail with `app_required`.
- Unknown flags, missing flag values and extra positionals exit 2.
- `proto --help`, `proto -h` and `proto help` list the commands. Add `--help` or `-h` after a
  command, or run `proto help dev` for the usage, arguments, flags, and defaults of a command.
  With `--json`, command help returns `{ command, summary, usage, arguments, options }`.
  Top-level `proto --help --json` returns `{ commands, plans }`. `commands` stays a `string[]`
  of command names. `plans` maps each local command name to `"free"` or `"pro"`. Cloud
  commands appear in `commands` and use their own service rules.
- `proto version` and `proto --version` print the installed version. Add `--json` for `{ version }`.
- Paths in output are app-relative, except `proto build`, which returns the absolute
  `dist-proto` path.
- `--remote` works only on the comment and review commands and on `url`. Every other command
  exits 2 with `This command does not support --remote`.

Local authoring, human review in the gallery and static builds are Free. Local `comments *`,
`screenshot`, `evidence` and every local MCP tool call are Pro operations. Proto Cloud
checks each operation online before it runs. An Editor seat in a Pro workspace covers
unlimited operations. Without a seat, the account has a one-time allowance of 100 free
operations that never resets. If Proto Cloud cannot confirm access, the operation stops before
reading or changing project data or writing captures. Starting `proto mcp` uses no operations.

Interactive `proto dev`, `proto start`, `proto init` and `proto mcp` startup need a free
account. Sign in with `proto login` using Google, GitHub or an email link. A stored login lets
these commands start when the sign-in check cannot reach Proto Cloud. A truthy `CI` skips only
the Free startup sign-in rule. Use `PROTOHQ_TOKEN` from a seat holder for Pro operations in CI.
Headless Free commands such as `proto check` and `proto build` and the Vite, Storybook and Vitest
integrations need no login or network access.

The published CLI uses the production login at `https://api.protohq.design` for agent checks
and allowance status, regardless of `PROTOHQ_API_URL`. The sign-in probe sends the CLI
version, platform and architecture, device kind, and hostname in `User-Agent`.

The probe checks account identity without an `X-Proto-Workspace` header. A rejected environment
token must be unset or replaced; `proto login` does not override `PROTOHQ_TOKEN`. If the stored
credentials file is corrupt, its error names the path and `proto login` replaces it after approval.

## Commands

Local CLI commands below are Free except `comments *`, `screenshot`,
`evidence` and the `mcp` entry, which help labels `(Pro)`. Use `--remote --share <slug>` for
hosted comments under the share's access rules. All MCP tool calls count as Pro operations,
including those with Free CLI equivalents.

Read and check:

| Command                                                               | Result                                               |
| --------------------------------------------------------------------- | ---------------------------------------------------- |
| `proto list [--spec <id>] [--kind draft\|component]`                  | `{ prototypes, flows }`                              |
| `proto check`                                                         | `{ problems, warnings, error? }`; exit 1 on problems |
| `proto url <slug> [--state] [--theme light\|dark] [--role] [--width]` | `{ url }` of the gallery screen                      |
| `proto url <slug> --state <id> --bare`                                | `{ url }` of the render origin document (`chrome=0`) |
| `proto flow list`                                                     | `{ flows }`, with nodes and edges                    |
| `proto flow show <id>`                                                | `{ id, title, steps, edges, problems, warnings }`    |
| `proto flow coverage`                                                 | `{ uncovered: [{ proto, state }] }`                  |

- `--width` only affects the gallery URL. The bare render document takes the window's width, so
  use the gallery URL or `screenshot --width` to see a specific width. With `--remote`, `url`
  ignores `--bare`.
- A flow with any problem is left out of `list`, `flow list` and `flow show`, which then fails
  with `Unknown flow`. Read its problems in `proto check --json`, filtered by `flow`.

Write files:

| Command                                                    | Result                                         |
| ---------------------------------------------------------- | ---------------------------------------------- |
| `proto new --in <view.tsx> [--spec <id>]`                  | `{ file, slug, url }` (component prototype)    |
| `proto new <slug> --draft [--spec <id>]`                   | `{ file, slug, url }` (draft)                  |
| `proto flow new <slug> [--spec <id>]`                      | `{ file, slug, url }`; the slug is the flow id |
| `proto screenshot <slug> [--state --theme --width --role]` | `{ path, width, height }` (PNG)                |
| `proto evidence <slug> [--state --theme --width --role]`   | `{ path }` (static HTML without scripts)       |

Captures default to the first state, `light` and a width of 1440, use a 900px tall viewport and
write `<evidence.dir>/<slug>/<state>--<theme>--<width>.png` or `<evidence.dir>/<slug>/index.html`.
They need Playwright Chromium. They reuse a ready project through its lock and verified instance,
even while Proto is stopped, or serve it temporarily without starting Proto in the background.
Evidence HTML uses source paths without local origins. Another capture or a startup can block
reuse for up to 60 seconds (`project_busy`); losing the project fails with `project_offline`.

Review comments use the local store by default; add `--remote --share <slug>` for the hosted share. Each
command returns `{ comment }` unless noted:

- `proto comments list [--status open|resolved|all] [--slug <key>] [--spec <id>] [--state <id>]
  [--author-kind agent|reviewer] [--waiting]` returns `{ comments }`. `--status` defaults to
  `open`.
- `proto comments show <id>` returns the comment with its whole thread.
- `proto comments add <slug> --selector <css> --note "…" [--source <path:line:col>] [--state]
  [--theme] [--role] [--width]` adds an element comment.
- `proto comments add flow-<id> [--node <node>] --note "…"` comments a flow step, or the whole
  flow without `--node`.
- `proto comments reply <id> --note "…"`.
- `proto comments resolve <id> [--note "…"]` and `proto comments reopen <id> [--note "…"]`.
- `proto comments edit <id> --note "…" [--message <n>]` edits your own message.
- `proto comments attach <id> <file>` and `proto comments detach <id> <attachment-id>` change your
  own message.
- `proto comments attachments <id> [--out <dir>]` downloads the images and returns `{ files }`
  (default `.tmp/proto/<id>/`).
- `proto comments delete <id>` returns `{ deleted }`. Run it only when a person asks.
- `proto review status <key>` returns `{ key, status, updatedAt, open, events }`.

- `--waiting` keeps threads whose last message is from an agent, which are waiting on a person.
- `--message <n>` is one-based. Without it, `edit`, `attach` and `detach` act on your latest
  message. Someone else's message fails with `message_not_owned`.
- Images are PNG, JPEG, WebP or GIF, at most 5 MB each and 6 per message.
- `--author` defaults to `agent`. Never pass `--author reviewer` to speak as a person.

Run by people, never by agents (none of these is an MCP tool):

| Command                                       | What                                                |
| --------------------------------------------- | --------------------------------------------------- |
| `proto review approve <key> [--confirm-open]` | Approve; open comments require `--confirm-open`     |
| `proto review reopen <key>`                   | Put an approved review back in review               |
| `proto review unlock <key> --force`           | Operator recovery of a lock left by another machine |
| `proto login` / `proto logout`                | Sign in to Proto Cloud or remove the CLI session    |
| `proto migrate --from <dir> [--dry-run]`      | Rewrite v1/v2 review files to v3                    |

Proto Cloud requests, including device login, report the CLI version in `X-Proto-Client`,
its platform and architecture in `X-Proto-Platform`, and its kind in `X-Proto-Device-Kind`.
The device approval screen and account device list display these self-reported facts. The kind
is `ci` for a truthy `CI` value other than `false` or `0`, `laptop` on macOS, Windows, or
Linux with a display, and `server` elsewhere.

Process lifecycle:

| Command                           | What                                                                 |
| --------------------------------- | -------------------------------------------------------------------- |
| `proto init [--skills] [--force]` | Scaffold the app and/or install the agent skills (`setup.md`)        |
| `proto start`                     | Start Proto in the background; `{ url, started, pid, version }`      |
| `proto stop`                      | Stop Proto; `{ stopped, waiting }`; project terminals stay alive     |
| `proto status`                    | Inspect without starting anything; `{ daemon, projects }`            |
| `proto dev [--open]`              | Project gallery on the shared Proto address                          |
| `proto build [--remote]`          | `dist-proto/`: read-only static gallery, or the remote build         |
| `proto mcp`                       | Start the Pro local MCP server over stdio without using an operation |

`proto whoami --json` includes a read-only `agentAccess` field. Its shape is
`{ via: "seat", workspace: { slug } }` for a seat holder, or
`{ via: "allowance" | "none", allowance: { granted, used, remaining }, next: { kind, url } }`.
The seat may belong to another Pro workspace. If only the agent status read fails, the field
is `null` and the rest of the response still describes the signed-in account.

## Project URLs and lifecycle JSON

`proto dev` prints a project URL `http://127.0.0.1:4400/p/<label>/` and home
`http://127.0.0.1:4400/`. Each project's foreground terminal owns its server; Ctrl-C stops only
that project. `start` and `stop` are idempotent, return at once and support `--json`.
`proto status --json` returns `daemon: null` when Proto is stopped and project states including
`waiting` for live terminals. Running rows carry `cliVersion`, `behind`, `renderOrigin` and
`storyRoots` preview addresses, plus identity, folder, URL, PID and timestamps.

```console
$ proto dev --json
{"url":"http://127.0.0.1:4400/p/acme-web/","home":"http://127.0.0.1:4400/","label":"acme-web","projectId":"9f2c41d07ab3e655","state":"running","renderOrigin":"http://127.0.0.1:24400"}
$ proto start --json
{"url":"http://127.0.0.1:4400/","started":false,"pid":4242,"version":"0.2.0"}
$ proto stop --json
{"stopped":true,"waiting":["acme-web"]}
```

With no known projects and Proto stopped, `proto status --json` returns
`{"daemon":null,"projects":[]}`. `dev` keeps serving after its one stdout JSON line; later
lifecycle messages and warnings use stderr.
A second `dev` exits 0 with `state: "already_running"` and may have `renderOrigin: null` while
startup finishes. After `stop`, projects wait for `start` and reconnect; an unexpected stop
triggers recovery on the same port.

`PROTOHQ_PORT` chooses Proto's port (default 4400), remembered when unset. To change it, stop
Proto, then run `PROTOHQ_PORT=4500 proto start`. `proto url <slug>` and `proto stories url <id>`
never start anything and return `/p/<label>/?p=<slug>` and
`/p/<label>/?c=<component>&story=<id>` at that address. `proto url <slug> --bare` returns a
reserved preview URL after the first project start, or `project_offline` before it. The preview
address stays with the project; never derive it from Proto's port.

## JSON shapes worth knowing

- `check` problems: `{ code, file, message, slug?, line?, column?, flow?, node?, chain? }`. Codes:
  `duplicate_slug`, `reserved_slug`, `invalid_slug`, `invalid_meta`, `duplicate_state`,
  `markup`, `unknown_spec`, `unknown_role`, `network_import`, and the flow
  codes `flow_outside_root`, `invalid_flow_id`, `missing_flow`, `invalid_flow`,
  `unknown_prototype`, `unknown_state`, `duplicate_node`, `invalid_node`, `unknown_node`,
  `duplicate_edge`, `unreachable`, `dead_end`. A compile failure that is not a guard violation
  comes back as `error` (a string), also with exit 1.
- `check` warnings never fail: `uncovered_state`, `canvas_missing`, `specs_unavailable`.
- A comment: `{ id, slug, status, createdAt, updatedAt, resolvedAt, context, target, thread }`.
  - `context`: `{ commit, state, theme, role, viewport: { width, height } }`.
  - `target` for an element or region: `{ kind: "element" | "region", selector, testId, tag,
    text, source, sourceChain, rect, rectNormalized, offset }`. For a flow step:
    `{ kind: "node", node, label }`, where `node: null` means the whole flow.
  - `source` is the nearest app-relative `path:line:column`. `sourceChain` (up to 8) walks from
    the clicked element out to the composing prototype. Only paths under `src/`, the flows
    directory and the canvas file appear, so design-system internals in `node_modules` never
    show up.
  - `thread`: `[{ id, at, editedAt, body, author: "reviewer" | "agent", authorName?,
    attachments }]`.

## Diagnose local serving

Inspect `proto status --json` and `~/.config/protohq/daemon/daemon.log`. `port_in_use` means an
address is occupied or preview ports cannot be allocated; `daemon_unavailable` means Proto did
not start or answer; `daemon_version_mismatch` requires upgrading this project's package;
`state_unavailable` means the private state cannot be used. `project_busy` means another command
or startup holds the project; `project_offline` means it stopped or lacks a preview address.
See `troubleshooting.md` and the generated `errors.md` for exact recovery and messages.

## Exit and error codes

| Exit | Meaning                     | Typical codes                                                                                                                                                                                                                                                                                |
| ---- | --------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0    | ok (warnings allowed)       |                                                                                                                                                                                                                                                                                              |
| 1    | problems or user error      | `invalid_config`, `slug_taken`, `comment_not_found`, `message_not_owned`, `invalid_source`, `port_in_use`, `daemon_unavailable`, `daemon_version_mismatch`, `state_unavailable`, `project_busy`, `project_offline`, `not_a_vite_app`, `key_busy`, `attachment_*`, `review_has_open_comments` |
| 1    | project setup               | `not_initialized`                                                                                                                                                                                                                                                                            |
| 2    | usage                       | unknown command or flag, a missing required argument (`selector is required`), `app_required`                                                                                                                                                                                                |
| 3    | missing optional dependency | `playwright_missing`; the MCP SDK for `proto mcp`                                                                                                                                                                                                                                            |
| 4    | remote or auth              | `not_logged_in`, `auth_required`, `invalid_credential`, `service_unavailable`, `scope_denied`, `plan_required`, `seat_required`, `cli_outdated`                                                                                                                                              |

Errors print `{ "error": { "code", "message", ... } }` with `--json`. Usage errors have only a
`message`. `key_busy` means another writer holds the review lock; retry.

For agent operations, `plan_required` and `seat_required` include `allowance` and
`next: { kind, url }`. Human output ends with the next action. Relay the message and URL, then
stop the paid step. The action is `upgrade`, `ask_owner_upgrade` or `ask_owner_seat`.
An agent check's `rate_limited` (exit 1) includes `retryAfter` in seconds. Wait at least that
long before retrying. The CLI issues one check per operation. For `proto login --json`,
`rate_limited` instead includes `retryAfterSeconds`.
`login_interrupted` means a token exchange may have completed during a timeout or outage. Run
`proto login` again.

## MCP

`proto mcp` (needs `@modelcontextprotocol/sdk`) serves the app selected at startup over stdio. Use
`--app` in the server command. Stdout belongs to the protocol.

- Tool names (generated from the catalog): `proto_list`, `proto_stories_list`, `proto_stories_run`, `proto_stories_url`, `proto_check`, `proto_url`, `proto_flow_list`, `proto_flow_show`, `proto_flow_coverage`, `proto_new`, `proto_flow_new`, `proto_comments_list`, `proto_comments_show`, `proto_comments_add`, `proto_comments_reply`, `proto_comments_resolve`, `proto_comments_reopen`, `proto_comments_attach`, `proto_comments_detach`, `proto_comments_edit`, `proto_comments_attachments`, `proto_comments_delete`, `proto_review_status`, `proto_screenshot`, `proto_evidence`.
- Inputs use the CLI flag names as fields, and positionals keep their catalog names: `slug`
  (also the flow id of `flow new`), `id` (a comment id, or the flow id of `flow show`), `key`,
  `file` and `attachment`. For example, `{ "id": "login-form-7", "note": "Fixed." }`. Store tools
  accept `"remote": true`.
- Results are the same JSON as `--json`. Errors come back as `isError` with the same `error`
  body. The last text item can be an allowance notice prefixed with `[Proto]`. Relay that
  notice once per stage (first use, low, used up). On the CLI, notices go to stderr.
- Not exposed: `init`, `dev`, `start`, `stop`, `status`, `build`, `mcp`, `migrate`, `review approve`, `review reopen`,
  `review unlock` and cloud management commands. `proto_comments_attachments` always writes to
  `.tmp/proto/<id>/`.

Remote store operations require `--remote --share <slug>` and a cloud session from `proto login`.
Local MCP uses `remote: true` with `share`. Bearers cannot approve or delete remote reviews.
Read [hosted operations](hosted.md) before publishing or using remote MCP, and the
[generated registry](errors.md) for all error codes and exit codes.
