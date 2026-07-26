# Prompt Enhancer

`prompt-enhancer` is the TypeScript reference extension for the subprocess extension architecture.

It demonstrates:

- a persistent runtime built with `@agh/extension-sdk`
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

## Managed-install boundary

This example uses `@agh/extension-sdk` through the repository's `workspace:*` dependency. Its
`node_modules/@agh/extension-sdk` entry resolves outside this extension root, and the built
JavaScript keeps that runtime import. AGH's managed installer intentionally rejects dependency
symlinks that escape the source package.

Use this directory to exercise the SDK from the repository checkout. Do not present it as an
installable standalone package until the SDK dependency is published or materialized inside the
extension root.

## Manifest Summary

- Capability: `prompt.provider`
- Hook: `prompt.post_assemble`
- Host API action: `sessions/list`
- Security grant: `session.read`

## Optional Runtime Markers

The persistent runtime reads these optional environment variables:

- `AGH_PROMPT_ENHANCER_HANDSHAKE_PATH`: writes the initialize request/response as JSON.
- `AGH_PROMPT_ENHANCER_HOST_CALL_PATH`: writes the result of the `sessions/list` probe as JSON.
- `AGH_PROMPT_ENHANCER_CAPABILITY_PATH`: writes the typed error returned by the intentionally denied `sessions/create` call.
- `AGH_PROMPT_ENHANCER_SHUTDOWN_PATH`: appends one line when the daemon sends `shutdown`.

## Hook Behavior

The hook prepends the resolved workspace path to the assembled prompt:

```text
[Workspace: /absolute/workspace/path]

<original prompt>
```

If the workspace is unavailable in the payload, it falls back to `workspace_id`, then `unknown`.
