# Extension Authoring

Write a Compozy extension. Lifecycle mechanics — generation handles, instance scoping, dev overlays,
reload semantics, logs, install trust, publish credentials — live in
`references/extensions.md`. Native tool IDs and risk flags live in
`references/native-tools.md`. Read this file for what goes in the code.

## Contents

- Code-first model
- Templates
- Declaration shape
- Ready callback lifecycle
- Permissions and consent
- Provide surfaces
- Contributed commands
- Generated manifest
- Structured workflows

## Code-First Model

The SDK declaration is the single source of truth. `compozy extension build` starts the built binary
with the `__describe` argument, reads the contract it prints, and generates
`dist/gen-<hash>/extension.toml`. Never hand-write or hand-edit a manifest for a subprocess
extension: the next build overwrites it. Hand-written manifests are for resource-only extensions,
which run no code.

There is no second place to declare identity, schemas, permissions, tools, hooks, or commands, so
schema-digest drift cannot occur. Manifest generation is deterministic; identical source produces a
byte-identical manifest.

Published SDKs, both version-matched to the daemon: `@compozy/extension-sdk` on npm and
`github.com/compozy/compozy/sdk/go`. `build` stamps `min_compozy_version` from the SDK, so an
extension cannot claim compatibility its SDK does not have.

## Templates

`compozy extension init <name> --template <template> [--dir <path>]` writes source plus
`package.json` or `go.mod` and nothing else.

| Template               | Use for                                       |
| ---------------------- | --------------------------------------------- |
| `tool-provider-ts`     | Agent-callable tools in TypeScript (default). |
| `tool-provider-go`     | Agent-callable tools in Go.                   |
| `hook-ts`              | A runtime hook that returns a payload patch.  |
| `memory-backend-ts`    | The `memory.backend` provide surface.         |
| `loop-watch-source-go` | The `loop.watch_source` provide surface.      |

## Declaration Shape

Go:

```go
extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
	Name: "hello", Version: "0.1.0", Description: "…",
	Subprocess:  compozysdk.DescribeSubprocess{Command: "./bin"},
	Permissions: compozysdk.PermissionsConfig{Requires: []compozysdk.HostAPIMethod{"sessions/list"}},
})
compozysdk.Tool[searchInput](extension, "search", compozysdk.ToolOptions{
	Description: "…", ReadOnly: true, InputSchema: searchInputSchema,
}, handleSearch)
extension.Run(context.Background())
```

TypeScript uses `new Extension({...})`, `extension.tool<TInput>(name, options, handler)`,
`extension.handle("execute_hook", …)` for hook events declared in `supported_hook_events`, and
`extension.start()`.

Registering a tool adds `tool.provider` to `capabilities.provides` automatically; do not declare it.
The handler name plus the extension name produce the tool ID `ext__<extension>__<tool>`: each segment
lowercased, `[a-z0-9]` kept, every other run of characters collapsed to a single `_`, and `__`
reserved as the separator.

`InputSchema` is a JSON Schema object. Provide `OutputSchema` only when the tool has one; the digest
of each is computed by the SDK and re-verified by the daemon.

Registration closes at `initialize`. Register every tool, hook, watch source, and command group
before `Run`/`start`.

## Ready Callback Lifecycle

Go `OnReady` callbacks start after initialization. Register them before `Run`, treat `ctx.Done()` as
the stop signal, and return promptly. On shutdown, the SDK cancels accepted callbacks and takes one
bounded drain snapshot at the request deadline or negotiated runtime timeout. Observable completion
wins when completion and deadline are both ready; `Run` reports non-cancellation failures captured
by that snapshot, while failures recorded after a deadline decision are excluded. A callback that
outlives that decision may outlive `Run`. Registrations after shutdown or runtime closure are ignored.

## Permissions And Consent

`permissions.requires` is the single authored list: the Host API method paths the extension calls,
validated against the closed Host API method set at build, validate, install, and load. An unknown value is a
hard error, never a silent no-op. Both SDKs export the method names as typed constants.

Compozy derives operator-facing consent areas (`area:access`, such as `sessions:read` or
`memory:write`) from that list. Consent areas are a display and policy projection and are never
authored. `compozy extension validate <dir> -o json` returns `consent_areas`; show that to the user
before proposing an install.

Grant ceilings by install source: dev links (`workspace`), local-path installs (`user`), and bundled
extensions get every declared method. Published installs (`curated`, `github`, `git`) run under the
marketplace tier, limited to `logs.read`, `memory.read`, `observe.read`, `session.read`,
`skills.read`, and `tool.read`; anything outside is dropped at grant time with a recorded diagnostic.
Design a mutating extension as a local or dev-linked install, not a published one.

### Durable session input

An extension that guides a busy session must use the daemon-owned input operations. `sessions/prompt`
accepts `mode` (`queue`, `interrupt`, or `steer`). Both `interrupt` and `steer` require
`expected_turn_id`; its result reports
`status` and `delivery`. Treat those fields as the admission decision. A transcript marker only records
history; it is not an action result.

| Intent                                          | Host API method           | Permission      |
| ----------------------------------------------- | ------------------------- | --------------- |
| Read pending input in dispatch order            | `sessions/inputs/list`    | `session.read`  |
| Replace a queued input and its durable identity | `sessions/inputs/replace` | `session.write` |
| Remove a queued input                           | `sessions/inputs/cancel`  | `session.write` |
| Promote a queued input into steering            | `sessions/inputs/promote` | `session.write` |

Every mutating input operation names `workspace_id`, `session_id`, and `queue_entry_id`. Replace and
promote also require fresh `text`, `message_id`, and `idempotency_key`; promote requires the current
`expected_turn_id`. Read the list again after an action instead of maintaining a private queue or
assuming an input was delivered from its position alone.

### Durable session runtime

Use `sessions/runtime/set` and `sessions/runtime/clear` with `session.write` when an extension must
change next-prompt runtime intent. Both require `workspace_id`, `session_id`, and the current
`expected_revision`; set also requires the complete runtime selection. They return `SessionStatus`,
do not start or reconfigure ACP, and reject a stale revision. Read session status again after a
conflict rather than retrying the old selection.

## Provide Surfaces

Closed set, validated at build, install, and load.

| Provide             | Extension must implement                         | Public |
| ------------------- | ------------------------------------------------ | ------ |
| `tool.provider`     | `provide_tools`, `tools/call`                    | yes    |
| `memory.backend`    | `memory/store`, `memory/recall`, `memory/forget` | yes    |
| `model.source`      | `models/list`                                    | yes    |
| `loop.watch_source` | `watch/poll`                                     | yes    |
| `bridge.adapter`    | `bridges/deliver`, `bridges/targets/snapshot`    | no     |

Missing a required service method fails the build. `bridge.adapter` is excluded from the public
surface: an installed third-party manifest declaring it is rejected deterministically, because
external bridge authoring is a planned follow-up program. Never scaffold one for a user.

Every `watch/poll` response requires a stable `event_key`. The runtime trims it and normalizes its
Unicode to NFC, one canonical byte form. Invalid UTF-8 and values over 256 bytes are rejected before
a Loop starts. Redelivery of the same source key and event key returns the existing Loop run as a
structured suppression instead of starting duplicate work.

For a host bridge adapter, `delivery_id` is opaque: acknowledge the exact received value and sequence,
without trimming or reformatting it. Cursor diagnostics identify ownership with a structured
`{kind, workspace_id}` scope, never a joined string; `kind` is closed to `global` or `workspace`,
and `workspace_id` is likewise opaque and byte-exact.

## Contributed Commands

A command is presentation metadata on a tool. Add a `command` block to the tool registration
(`verb`, `summary`, optional `example`, and a `flags` map of CLI flag name to top-level input-schema
field), and declare parent groups with `extension.CommandGroup(path, summary)` /
`extension.commandGroup(path, summary)`.

Build and manifest load both enforce: `/`-joined paths with maximum depth 2; groups non-executable
and never leafless; unique paths per extension; no leaf that is also a group prefix; no group-level
flags; and flag names outside the reserved host set `agent, approval-token, cmd, help, input, json,
o, output, session, workspace`.

Flag projection accepts only scalars (`string`, `boolean`, `integer`, `number`), nullable scalars,
enums of the projected scalar, and arrays whose `items` is one supported scalar (repeatable flag).
Objects, nested arrays, tuples, `oneOf`/`anyOf`/`allOf`/`not`, conditional schemas, and unresolvable
`$ref` are rejected at build with an `--input` remediation.

`compozy extension exec <ext> --cmd <path>` performs exactly one
`POST /api/tools/ext__<ext>__<tool>/invoke`, so policy, approvals, risk gates, and
`trusted_workspace` apply unchanged. Agents do not need `exec` — call the tool ID directly.

## Generated Manifest

What `build` writes, for reading rather than editing: `[extension]` (`name`, `version`,
`description`, `min_compozy_version`, `requires_env`), `[capabilities] provides`,
`[permissions] requires`, `[subprocess]` (`command`, `args`, `env`, `secret_env`,
`health_check_interval`, `shutdown_timeout`), `[resources.tools.<handler>]` (id, handler, backend
kind `extension_host`, canonical `input_schema`/`output_schema`, risk metadata, optional `command`),
`[[resources.hooks]]`, and `[[resources.command_groups]]`.

Resource-only extensions additionally hand-write `resources.skills|agents|loops|automation|layouts|mcp_servers`
and `[resources.publish]` (families plus `max_scope`). Resource paths resolve inside the extension
root; `{{config_dir}}` is that root and `{{env:NAME}}` reads the daemon process environment.

Static kit resources stay inert after install. Enable publishes the instance-owned resources; disable
removes them. Use extension inventory to compare shipped and live resources, and preview to inspect
added, changed, or removed resources before enable. Required environment variables are names in the manifest; bind them
to Vault references through the extension secrets surface, which never returns values or references.
When enable or update returns a Network confirmation digest, retry only with that exact digest.

## Structured Workflows

Every command supports `-o json`. Report the structured field, never prose.

New extension, first success (three operator actions after the daemon is running):

```
compozy extension init hello --template tool-provider-go -o json   -> directory, template, files
compozy extension dev hello -o json                                 -> dev, workspace_id, generation_hash
compozy tool invoke ext__hello__search --workspace . --input '{"query":"x"}' -o json
```

`dev`, `reload`, `logs`, and `remove` infer the workspace from the working directory. `tool invoke`,
`extension list`, and `extension status` default to global scope: pass `--workspace` to reach a
dev-linked instance's tools, and read dev instances through `GET /api/extensions?workspace=<id>` or
an agent caller whose session binds the workspace — a dev-only instance never appears in
`extension list`.

Pre-install review:

```
compozy extension build <dir> -o json      -> generation_hash, generation_dir, manifest_path
compozy extension validate <gen-dir> -o json -> issues[] (path/line/column/field/severity), consent_areas[]
```

Iterate:

```
compozy extension reload <name> <dir> -o json   -> new generation_hash
compozy extension logs <name> -o json           -> sequence, timestamp, message, generation_hash
compozy extension status <name> -o json         -> published instance: state, health, consecutive_failures, restart_backoff_ms
```

Publish and consume:

```
compozy extension publish <gen-dir> --repository <owner/name> --tag <tag> -o json
compozy extension install github:<owner>/<name>@<tag> --allow-unverified --yes -o json
compozy extension commands <name> -o json
compozy extension exec <name> --cmd <path> [projected flags] -o json
```

Agent-native equivalents are `compozy__extensions_{init,build,validate,dev,reload,logs,publish}`.
`compozy__extensions_dev` and `compozy__extensions_reload` never build: call
`compozy__extensions_build` first and pass its `generation_hash`.

Stop and report instead of guessing when `validate` returns any `error` severity, when a permission
is rejected as unknown, when a build reports a command-tree or flag-projection failure, or when a
reload reports `activation_failed` (the last-good generation is still serving).
