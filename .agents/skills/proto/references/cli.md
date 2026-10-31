# CLI, JSON, exit codes and MCP

Contents: invocation · commands · JSON shapes · exit and error codes · MCP.

## Invocation

```sh
proto <command> [positionals] [--flags] [--json] [--app <dir>] [--verbose]
```

- `--json` prints exactly one JSON value on stdout, which is the contract. Human output is for
  people and may change.
- App selection: `--app <dir>` (relative to the working directory, or `apps/<dir>`). Without it,
  the CLI walks up to the nearest `proto.config.ts`, then tries a single `apps/*/proto.config.ts`.
  Several candidates fail with `app_required`.
- Unknown flags, missing flag values and extra positionals exit 2.
- `proto --help` lists the commands, and `proto --version` prints the installed version.
- Paths in output are app-relative, except `proto build`, which returns the absolute
  `dist-proto` path.
- `--remote` works only on the comment and review commands and on `url`. Every other command
  exits 2 with `This command does not support --remote`.

## Commands

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
They need Playwright Chromium and start a temporary dev server when none is running.

Review comments use the local store by default; add `--remote` for the deployed gallery. Each
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
| `proto auth login\|logout --remote <url>`     | Store or remove a remote credential                 |
| `proto migrate --from <dir> [--dry-run]`      | Rewrite v1/v2 review files to v3                    |

Process lifecycle:

| Command                           | What                                                             |
| --------------------------------- | ---------------------------------------------------------------- |
| `proto init [--skills] [--force]` | Scaffold the app and/or install the agent skills (`setup.md`)    |
| `proto dev [--port <n>] [--open]` | Shell on `port` (4400), render origin on `port + 1`; both strict |
| `proto build [--remote]`          | `dist-proto/`: read-only static gallery, or the remote build     |
| `proto mcp`                       | MCP server over stdio                                            |

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

## Exit and error codes

| Exit | Meaning                     | Typical codes                                                                                                                                                                       |
| ---- | --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0    | ok (warnings allowed)       |                                                                                                                                                                                     |
| 1    | problems or user error      | `invalid_config`, `slug_taken`, `comment_not_found`, `message_not_owned`, `invalid_source`, `port_in_use`, `not_a_vite_app`, `key_busy`, `attachment_*`, `review_has_open_comments` |
| 2    | usage                       | unknown command or flag, a missing required argument (`selector is required`), `app_required`                                                                                       |
| 3    | missing optional dependency | `playwright_missing`; the MCP SDK for `proto mcp`                                                                                                                                   |
| 4    | remote or auth              | `auth_required`, `invalid_credential`, `credential_revoked`, `scope_denied`, `insecure_remote`                                                                                      |

Errors print `{ "error": { "code", "message", ... } }` with `--json`. Usage errors have only a
`message`. `key_busy` means another writer holds the review lock; retry.

## MCP

`proto mcp` (needs `@modelcontextprotocol/sdk`) serves the app selected at startup over stdio. Use
`--app` in the server command. Stdout belongs to the protocol.

- Tool names are `proto_` plus the CLI words with underscores: `proto_list`, `proto_check`,
  `proto_url`, `proto_flow_list`, `proto_flow_show`, `proto_flow_coverage`, `proto_new`,
  `proto_flow_new`, `proto_comments_list`, `proto_comments_show`, `proto_comments_add`,
  `proto_comments_reply`, `proto_comments_resolve`, `proto_comments_reopen`,
  `proto_comments_attach`, `proto_comments_detach`, `proto_comments_edit`,
  `proto_comments_attachments`, `proto_comments_delete`, `proto_review_status`,
  `proto_screenshot`, `proto_evidence`.
- Inputs use the CLI flag names as fields, and positionals keep their catalog names: `slug`
  (also the flow id of `flow new`), `id` (a comment id, or the flow id of `flow show`), `key`,
  `file` and `attachment`. For example, `{ "id": "login-form-7", "note": "Fixed." }`. Store tools
  accept `"remote": true`.
- Results are the same JSON as `--json`. Errors come back as `isError` with the same `error`
  body.
- Not exposed: `init`, `dev`, `build`, `mcp`, `migrate`, `review approve`, `review reopen`,
  `review unlock` and `auth *`. `proto_comments_attachments` always writes to
  `.tmp/proto/<id>/`.

Remote review uses `review.remote.url` or `PROTO_REMOTE_URL`, plus a credential stored by
`proto auth login --remote <url>` (or `PROTO_REMOTE_TOKEN`). Credentials only have the `comment`
scope, so remote approval or deletion fails with `scope_denied`.
