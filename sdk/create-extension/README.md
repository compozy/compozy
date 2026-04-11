# @compozy/create-extension

Scaffold Compozy extensions from the published starter templates.

## Usage

```bash
npx @compozy/create-extension my-ext
```

The default template is `lifecycle-observer` and the default runtime is `typescript`.

## Templates

- `lifecycle-observer`
- `prompt-decorator`
- `review-provider`
- `skill-pack`

## Options

```text
create-extension <name> [options]

--template <name>    lifecycle-observer | prompt-decorator | review-provider | skill-pack
--runtime <name>     typescript | go
--module <path>      Go module path when --runtime go
--skip-install       Skip npm install / go mod init + go mod tidy
--help               Show help
```

## Runtime support

- `typescript` works with every template.
- `go` currently scaffolds `lifecycle-observer` and `prompt-decorator`.

## What the CLI does

- Copies the selected starter template into a new directory.
- Rewrites manifest and package tokens.
- Installs dependencies with `npm install` by default for TypeScript projects.
- Runs `go mod init` and `go mod tidy` by default for supported Go projects.

## Documentation

- [`Getting started`](/Users/pedronauck/Dev/compozy/_worktrees/ext/.compozy/docs/extensibility/getting-started.md)
- [`Hello world in TypeScript`](/Users/pedronauck/Dev/compozy/_worktrees/ext/.compozy/docs/extensibility/hello-world-ts.md)
- [`Hello world in Go`](/Users/pedronauck/Dev/compozy/_worktrees/ext/.compozy/docs/extensibility/hello-world-go.md)
