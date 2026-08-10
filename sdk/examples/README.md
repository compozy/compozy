# CompozyOS extension examples

Runnable extensions that import **only** a published CompozyOS SDK:

- TypeScript — [`@compozy/extension-sdk`](https://www.npmjs.com/package/@compozy/extension-sdk)
- Go — `github.com/compozy/compozy/sdk/go`

Nothing here imports `internal/*`. Daemon-only fixtures live under
`internal/extension/testdata/` and are not examples.

| Example                                | Language   | Shows                                                                                             |
| -------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------- |
| [`clarify-tool`](./clarify-tool)       | Go         | A tool provider that asks the operator one bounded question through `clarify/ask`.                |
| [`notes-commands`](./notes-commands)   | Go         | Contributed operator commands: a flat leaf, a declared group, a nested leaf, and projected flags. |
| [`prompt-enhancer`](./prompt-enhancer) | TypeScript | A persistent subprocess plus a `prompt.post_assemble` hook, and Host API usage.                   |

## Run one

The Go examples are code-first: you write the definition in Go and `compozy extension build`
generates the manifest. `prompt-enhancer` is a TypeScript protocol and hook reference with an
explicit manifest.

```bash
compozy extension dev ./notes-commands
compozy extension commands notes
```

Start from scratch instead with `compozy extension init <name> --template <template>`. Full
walkthrough: [Build your first extension](https://compozy.com/runtime/guides/build-your-first-extension).

## Working from a repository checkout

Go examples include a `go.mod` that requires the published SDK version and replaces it with the local
`sdk/go` module while they are inside this repository. After copying one out, remove that checkout-only
replacement with `go mod edit -dropreplace=github.com/compozy/compozy/sdk/go`, then run `go mod tidy`.
TypeScript examples resolve the SDK through the Bun workspace; replace `workspace:*` with the published
version when you copy them out.
