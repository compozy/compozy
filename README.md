# Looper

Looper is a reusable Go module and CLI for running markdown-driven AI work loops.

It replaces the copied `scripts/markdown` trees that were spread across multiple repositories and supports two execution modes:

- `pr-review`: process PR review issue markdown files in batches
- `prd-tasks`: process PRD task markdown files one task at a time

## Install

Install the CLI:

```bash
go install github.com/compozy/looper/cmd/looper@latest
```

Install the bundled skills that looper prompts expect:

```bash
npx skills add https://github.com/compozy/looper
```

Or build from source:

```bash
git clone git@github.com:compozy/looper.git
cd looper
make verify
go build ./cmd/looper
```

## Required Skills

Looper prompts are intentionally small and rely on bundled skills for reusable workflow doctrine.

- `fix-coderabbit-review`: required for PR review remediation flows
- `verification-before-completion`: required before completion claims or automatic commits
- `execute-prd-task`: required for PRD task execution flows

The supported install flow is:

```bash
npx skills add https://github.com/compozy/looper
```

PR review automation also expects:

- `gh` installed and authenticated for the target repository
- `python3` available on `PATH`
- `GITHUB_TOKEN` available if `gh` is using environment-based authentication in your setup

## CLI Usage

Interactive mode:

```bash
looper --form
```

PR review mode:

```bash
looper \
  --pr 259 \
  --mode pr-review \
  --ide codex \
  --concurrent 2 \
  --batch-size 3 \
  --grouped
```

If the review issue files do not exist yet, use the installed `fix-coderabbit-review` skill to export them into `ai-docs/reviews-pr-<PR>/issues` before running looper.

PRD task mode:

```bash
looper \
  --pr multi-repo \
  --mode prd-tasks \
  --issues-dir tasks/prd-multi-repo \
  --ide claude
```

## Go Package Usage

Prepare work without executing any IDE process:

```go
package main

import (
	"context"
	"fmt"

	"github.com/compozy/looper"
)

func main() {
	prep, err := looper.Prepare(context.Background(), looper.Config{
		PR:       "multi-repo",
		IssuesDir: "tasks/prd-multi-repo",
		Mode:     looper.ModePRDTasks,
		DryRun:   true,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(len(prep.Jobs))
}
```

Execute the loop from your own program:

```go
package main

import (
	"context"

	"github.com/compozy/looper"
)

func main() {
	_ = looper.Run(context.Background(), looper.Config{
		PR:              "259",
		Mode:            looper.ModePRReview,
		IDE:             looper.IDECodex,
		ReasoningEffort: "medium",
	})
}
```

Embed the Cobra command in another CLI:

```go
package main

import (
	"github.com/compozy/looper/command"
)

func main() {
	root := command.New()
	_ = root.Execute()
}
```

## Migration Guide

When migrating a repository that currently vendors `scripts/markdown`:

1. Remove the copied script tree.
2. Install the looper CLI and bundled skills:
   - `go install github.com/compozy/looper/cmd/looper@latest`
   - `npx skills add https://github.com/compozy/looper`
3. Choose whether the repo should:
   - shell out to the `looper` binary, or
   - import `github.com/compozy/looper` / `github.com/compozy/looper/command`
4. Point the repo at the same issue/task directories it already uses:
   - `ai-docs/reviews-pr-<PR>/issues`
   - `tasks/prd-<name>`
5. Keep repo-specific wrappers around looper if needed, but stop copying the looper engine or its skills into each project.

## Development

```bash
make deps
make fmt
make lint
make test
make build
make verify
```

## Project Layout

```text
cmd/looper/             Standalone CLI entry point
command/                Public Cobra wrapper
internal/cli/           Cobra flags, interactive form collection, CLI glue
internal/looper/        Internal facade for reusable preparation and execution
internal/looper/agent/  IDE command validation and process construction
internal/looper/model/  Shared runtime data structures
internal/looper/plan/   Input discovery, filtering, grouping, and batch preparation
internal/looper/prompt/ Thin prompt builders that emit runtime context plus required skill names
internal/looper/run/    Execution pipeline, logging, shutdown, and Bubble Tea UI
internal/version/       Build metadata
skills/                 Bundled installable skills consumed by looper-generated prompts
```

## License

BSL-1.1
