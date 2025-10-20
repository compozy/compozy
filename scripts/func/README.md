# Function Utilities CLI

The `scripts/func` package bundles several AST-powered utilities that enforce Compozy's Go function guidelines. All tools are exposed through a Cobra CLI.

## Quick Start

Run a tool from the repository root:

```bash
go run scripts/func/main.go < command > [path]
```

If no path is provided, the current directory (`.`) is used.

## Available Commands

### `length`

Report functions whose bodies exceed the configured line limit (30 lines).

```bash
go run scripts/func/main.go length ./engine/agent
```

### `spacing`

Detect or remove blank lines between statements inside function bodies.

```bash
go run scripts/func/main.go spacing --fix ./engine/agent
```

### `comments`

Detect or remove comments that appear inside function bodies (excluding `// TODO` notes).

```bash
go run scripts/func/main.go comments --fix ./engine/agent
```

## Shared Features

- ✅ Walks Go source files using the Go parser and AST packages
- ✅ Skips tests and common vendor/build directories
- ✅ Returns non-zero exit codes when violations are found
- ✅ Supports targeted execution by directory
- ✅ Provides optional `--fix` flags where automated cleanup is available
