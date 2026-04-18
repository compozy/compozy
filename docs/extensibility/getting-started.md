# Getting Started

This is the fastest path to a working extension.

## Prerequisites

- Node 18+ for TypeScript or JavaScript extensions
- Go 1.24+ for Go extensions
- A Compozy build that includes the v1 extension runtime

## Scaffold a TypeScript extension

```bash
npx @compozy/create-extension my-ext --template lifecycle-observer
cd my-ext
npm run build
```

The scaffolder runs `npm install` by default. Use `--skip-install` if you want to manage dependencies yourself.

## Install and enable it

```bash
compozy ext install --yes .
compozy ext enable my-ext
compozy ext list
```

Important behavior:

- user and workspace extensions are discoverable but disabled by default
- bundled extensions are always enabled
- install prints the capability list before activation

## Run it

The lifecycle-observer template listens for `run.post_shutdown`, so any normal run path works:

```bash
compozy exec "Summarize the repository state"
```

If you want a visible side effect while iterating, set a record file and run Compozy from the same shell:

```bash
export COMPOZY_TS_RECORD_PATH="$PWD/records.jsonl"
compozy exec "Summarize the repository state"
cat records.jsonl
```

## Scaffold a Go extension

The scaffolder also supports Go for the executable templates:

```bash
npx @compozy/create-extension my-go-ext --template prompt-decorator --runtime go --module example.com/my-go-ext
cd my-go-ext
go run .
```

For real Compozy runs, build or run the program through the manifest entrypoint and then install it with the same `compozy ext install` and `compozy ext enable` flow.

## Recommended project loop

1. Start from the closest template.
2. Keep the manifest small and explicit.
3. Write unit tests with `@compozy/extension-sdk/testing`.
4. Add at least one real subprocess smoke test before publishing the extension to other users.

## Next reading

- [`Architecture overview`](./architecture.md)
- [`Hook reference`](./hook-reference.md)
- [`Testing`](./testing.md)
