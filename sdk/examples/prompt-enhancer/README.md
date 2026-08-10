# Prompt Enhancer

`prompt-enhancer` is the TypeScript reference extension for the subprocess extension architecture.

It demonstrates:

- a persistent runtime built with `@compozy/extension-sdk`
- a `prompt.post_assemble` hook that injects workspace context
- Host API usage through the SDK
- end-to-end capability denial handling when an extension attempts an ungranted write method

## Prerequisites

Install the repository dependencies once from the repository root:

```bash
bun install
```

## Build

From this directory:

```bash
npm run build
```

The build emits `dist/index.js`, which is used by both the persistent subprocess runtime and the one-shot hook executor.

## Using it outside this repository

The only CompozyOS import in this example is the published package
[`@compozy/extension-sdk`](https://www.npmjs.com/package/@compozy/extension-sdk). Inside the
repository it resolves through the `workspace:*` dependency so the example always tracks the
in-tree SDK.

To run it as a standalone project, copy this directory out of the repository and replace the
dependency with the published version matching your daemon:

```json
"dependencies": { "@compozy/extension-sdk": "^0.3.0-beta.1" }
```

Then `bun install` inside the copied directory so the runtime dependency is materialized under the
extension root. CompozyOS's managed installer rejects dependency symlinks that escape the package
boundary, which is what a repository `workspace:*` link produces.

## Declaration summary

- Provide surfaces: none
- Hook: `prompt.post_assemble`
- Host API permission: `sessions/list`
- Deliberately denied Host API call: `sessions/create`

## Optional Runtime Markers

The persistent runtime reads these optional environment variables:

- `COMPOZY_PROMPT_ENHANCER_HANDSHAKE_PATH`: writes the initialize request/response as JSON.
- `COMPOZY_PROMPT_ENHANCER_HOST_CALL_PATH`: writes the result of the `sessions/list` probe as JSON.
- `COMPOZY_PROMPT_ENHANCER_CAPABILITY_PATH`: writes the typed error returned by the intentionally denied `sessions/create` call.
- `COMPOZY_PROMPT_ENHANCER_SHUTDOWN_PATH`: appends one line when the daemon sends `shutdown`.

## Hook Behavior

The hook prepends the resolved workspace path to the assembled prompt:

```text
[Workspace: /absolute/workspace/path]

<original prompt>
```

If the workspace is unavailable in the payload, it falls back to `workspace_id`, then `unknown`.
