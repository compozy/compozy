# 🔄 Looper

**Drive the full lifecycle of AI-assisted development — from idea to shipped code.**

Looper is a Go module and CLI that orchestrates AI coding agents (Claude Code, Codex, Droid, Cursor) to process structured markdown workflows. It covers product ideation, technical specification, task breakdown with codebase-informed enrichment, and automated execution.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)

## 📑 Table of Contents

- [✨ Features](#-features)
- [🚀 Installation](#-installation)
- [📖 Usage](#-usage)
- [🔧 PRD Workflow](#-prd-workflow)
  - [How It Works](#how-it-works)
  - [CLI Usage](#cli-usage)
  - [Skills](#skills)
  - [Output Convention](#output-convention)
- [🔍 PR Review Workflow](#-pr-review-workflow)
  - [How It Works](#how-it-works-1)
  - [Prerequisites](#prerequisites)
  - [CLI Usage](#cli-usage-1)
  - [Skills](#skills-1)
- [🧩 Shared Skills](#-shared-skills)
- [📦 Go Package Usage](#-go-package-usage)
- [⌨️ CLI Reference](#️-cli-reference)
  - [`looper fetch-reviews`](#looper-fetch-reviews)
  - [`looper start`](#looper-start)
  - [`looper fix-reviews`](#looper-fix-reviews)
- [🏗️ Project Layout](#️-project-layout)
- [🛠️ Development](#️-development)
- [🔀 Migration Guide](#-migration-guide)
- [📄 License](#-license)

## ✨ Features

- **End-to-end PRD workflow** — go from a product idea to implemented code through structured phases
- **Provider-agnostic PR review automation** — fetch and remediate review feedback in tracked rounds under each PRD
- **Multi-agent support** — run tasks through Claude Code, Codex, Droid, or Cursor
- **Concurrent execution** — run multiple tasks in parallel with configurable batch sizes
- **Interactive mode** — guided form-based CLI for quick setup
- **Plain markdown artifacts** — all specs, tasks, and tracking are human-readable markdown files
- **Embeddable** — use as a standalone CLI or import as a Go package into your own tools

## 🚀 Installation

Install the CLI:

```bash
go install github.com/compozy/looper/cmd/looper@latest
```

Install the bundled skills that Looper prompts expect:

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

### Requirements

- Go 1.26+

## 📖 Usage

Looper exposes three workflow subcommands: **[PRD Tasks](#-prd-workflow)** via `looper start`, **review fetching** via `looper fetch-reviews`, and **[PR Review](#-pr-review-workflow)** remediation via `looper fix-reviews`. See each workflow section below for details.

Interactive mode prompts for workflow-specific options:

```bash
looper fetch-reviews --form
looper start --form
looper fix-reviews --form
```

---

## 🔧 PRD Workflow

The PRD workflow takes you from a product idea to implemented code through a structured pipeline. Each step produces plain markdown artifacts that feed into the next, and AI agents execute the final tasks automatically.

### How It Works

```
💡 Idea
   │
   ▼
/create-prd          ──▶  tasks/prd-<name>/_prd.md
   │
   ▼
/create-techspec     ──▶  tasks/prd-<name>/_techspec.md
   │
   ▼
/create-tasks        ──▶  tasks/prd-<name>/_tasks.md + task_01.md … task_N.md
   │
   ▼
looper start --name <name>  ──▶  AI agents execute each task
```

Each step is independent — you can start from any point. All artifacts are plain markdown files in `tasks/prd-<name>/`.

### CLI Usage

Execute tasks from a PRD directory:

```bash
looper start \
  --name multi-repo \
  --tasks-dir tasks/prd-multi-repo \
  --ide claude
```

Preview generated prompts without executing (dry run):

```bash
looper start \
  --name multi-repo \
  --tasks-dir tasks/prd-multi-repo \
  --dry-run
```

### Skills

| Skill | Phase | Purpose |
|-------|-------|---------|
| `create-prd` | Creation | Creates PRDs through 5-phase interactive brainstorming with parallel codebase and web research |
| `create-techspec` | Creation | Translates PRD business requirements into technical specifications with architecture exploration |
| `create-tasks` | Creation | Decomposes PRDs and TechSpecs into independently implementable task files enriched with codebase context |
| `execute-prd-task` | Execution | Executes one PRD task end-to-end: implement, validate, track, and optionally commit |

### Output Convention

All PRD workflow artifacts live in `tasks/prd-<name>/`:

```
tasks/prd-<name>/
  _prd.md           # Product Requirements Document
  _techspec.md      # Technical Specification
  _tasks.md         # Master task list
  task_01.md        # Individual executable task files
  task_02.md
  task_N.md
```

Files prefixed with `_` are meta documents. Task files (`task_*.md`) are the executable units Looper processes.

Each task file follows a structured format:

```markdown
## status: pending

<task_context>
  <domain>Backend</domain>
  <type>Feature Implementation</type>
  <scope>Full</scope>
  <complexity>medium</complexity>
  <dependencies>task_01</dependencies>
</task_context>

# Task 2: Implement User Authentication
...
```

---

## 🔍 PR Review Workflow

The PR review workflow automates the remediation of code review feedback. Review comments are fetched into versioned rounds under the PRD directory, then batched for parallel execution. Looper resolves provider threads automatically after a batch succeeds.

### How It Works

1. **Fetch** — `looper fetch-reviews --provider <provider> --pr <PR> --name <name>` creates `tasks/prd-<name>/reviews-NNN/`
2. **Batch** — Looper groups and batches `issue_NNN.md` files based on `--batch-size` and `--grouped`
3. **Execute** — AI agents process each batch concurrently, triaging issues, implementing fixes, and updating issue statuses
4. **Resolve** — after a successful batch, Looper resolves provider threads for issue files that changed to `## Status: resolved`
5. **Verify** — each batch is validated before completion or auto-commit

### Prerequisites

- `gh` CLI installed and authenticated for the target repository
- access to the target repository through `gh` (for example via `gh auth login`)

### CLI Usage

Fetch a new review round:

```bash
looper fetch-reviews \
  --provider coderabbit \
  --pr 259 \
  --name my-feature
```

Process batched review issues from the latest round:

```bash
looper fix-reviews \
  --name my-feature \
  --ide codex \
  --concurrent 2 \
  --batch-size 3 \
  --grouped
```

### Skills

| Skill | Purpose |
|-------|---------|
| `fix-reviews` | Processes review issue batches: triages issues, implements fixes, verifies results, and updates review tracking files |

---

## 🧩 Shared Skills

These skills are used across both workflows.

| Skill | Purpose |
|-------|---------|
| `verification-before-completion` | Enforces fresh verification evidence before any completion claim, commit, or PR creation |

## 📦 Go Package Usage

### PRD Tasks

Prepare work without executing any IDE process:

```go
prep, err := looper.Prepare(context.Background(), looper.Config{
    Name:     "multi-repo",
    TasksDir: "tasks/prd-multi-repo",
    Mode:     looper.ModePRDTasks,
    DryRun:   true,
})
```

### PR Review

Fetch a review round, then execute the remediation loop:

```go
_, _ = looper.FetchReviews(context.Background(), looper.Config{
    Name:     "my-feature",
    Provider: "coderabbit",
    PR:       "259",
})

_ = looper.Run(context.Background(), looper.Config{
    Name:            "my-feature",
    Mode:            looper.ModePRReview,
    IDE:             looper.IDECodex,
    ReasoningEffort: "medium",
})
```

### Embedding

Embed the Cobra command in another CLI:

```go
root := command.New()
_ = root.Execute()
```

## ⌨️ CLI Reference

### `looper fetch-reviews`

Fetch provider review comments into a PRD review round.

```bash
looper fetch-reviews [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--provider` | `string` | | Review provider name (for example: `coderabbit`) |
| `--pr` | `string` | | Pull request number |
| `--name` | `string` | | PRD workflow name (used for `tasks/prd-<name>`) |
| `--round` | `int` | `0` | Review round number. When omitted, Looper creates the next available round |
| `--form` | `bool` | `false` | Use interactive form to collect parameters |

### `looper start`

Execute PRD task files from a PRD workflow directory.

```bash
looper start [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | `string` | | PRD task workflow name (used for `tasks/prd-<name>`) |
| `--tasks-dir` | `string` | | Path to PRD tasks directory (`tasks/prd-<name>`) |
| `--ide` | `string` | `codex` | IDE tool to use: `claude`, `codex`, `cursor`, or `droid` |
| `--model` | `string` | *(per IDE)* | Model to use (default: `gpt-5.4` for codex/droid, `opus` for claude, `composer-1` for cursor) |
| `--concurrent` | `int` | `1` | Number of batches to process in parallel |
| `--reasoning-effort` | `string` | `medium` | Reasoning effort for codex/claude/droid (`low`, `medium`, `high`, `xhigh`) |
| `--timeout` | `string` | `10m` | Activity timeout duration (e.g., `5m`, `30s`). Job canceled if no output within this period |
| `--max-retries` | `int` | `0` | Retry failed or timed-out jobs up to N times before marking them failed |
| `--retry-backoff-multiplier` | `float` | `1.5` | Multiplier applied to activity timeout after each retry |
| `--tail-lines` | `int` | `30` | Number of log lines to show in UI for each job |
| `--add-dir` | `strings` | | Additional directory to allow for Codex and Claude (repeatable or comma-separated) |
| `--auto-commit` | `bool` | `false` | Include automatic commit instructions at task/batch completion |
| `--include-completed` | `bool` | `false` | Include completed tasks |
| `--dry-run` | `bool` | `false` | Only generate prompts; do not run IDE tool |
| `--form` | `bool` | `false` | Use interactive form to collect parameters |

### `looper fix-reviews`

Process review issue markdown files from a PRD review round and dispatch AI agents to remediate feedback.

```bash
looper fix-reviews [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | `string` | | PRD workflow name (used for `tasks/prd-<name>`) |
| `--round` | `int` | `0` | Review round number. When omitted, Looper uses the latest existing round |
| `--reviews-dir` | `string` | | Override path to a review round directory (`tasks/prd-<name>/reviews-NNN`) |
| `--ide` | `string` | `codex` | IDE tool to use: `claude`, `codex`, `cursor`, or `droid` |
| `--model` | `string` | *(per IDE)* | Model to use (default: `gpt-5.4` for codex/droid, `opus` for claude, `composer-1` for cursor) |
| `--batch-size` | `int` | `1` | Number of file groups to batch together |
| `--concurrent` | `int` | `1` | Number of batches to process in parallel |
| `--grouped` | `bool` | `false` | Generate grouped issue summaries in `reviews-NNN/grouped/` |
| `--include-resolved` | `bool` | `false` | Include review issues already marked as resolved |
| `--reasoning-effort` | `string` | `medium` | Reasoning effort for codex/claude/droid (`low`, `medium`, `high`, `xhigh`) |
| `--timeout` | `string` | `10m` | Activity timeout duration (e.g., `5m`, `30s`). Job canceled if no output within this period |
| `--max-retries` | `int` | `0` | Retry failed or timed-out jobs up to N times before marking them failed |
| `--retry-backoff-multiplier` | `float` | `1.5` | Multiplier applied to activity timeout after each retry |
| `--tail-lines` | `int` | `30` | Number of log lines to show in UI for each job |
| `--add-dir` | `strings` | | Additional directory to allow for Codex and Claude (repeatable or comma-separated) |
| `--auto-commit` | `bool` | `false` | Include automatic commit instructions at task/batch completion |
| `--dry-run` | `bool` | `false` | Only generate prompts; do not run IDE tool |
| `--form` | `bool` | `false` | Use interactive form to collect parameters |

## 🏗️ Project Layout

```
cmd/looper/              CLI entry point
command/                 Public Cobra wrapper for embedding
internal/cli/            Cobra flags, interactive form, CLI glue
internal/looper/         Internal facade for preparation and execution
  agent/                 IDE command validation and process construction
  model/                 Shared runtime data structures
  plan/                  Input discovery, filtering, grouping, batch prep
  prompt/                Prompt builders emitting runtime context + skill names
  run/                   Execution pipeline, logging, shutdown, Bubble Tea UI
internal/version/        Build metadata
skills/                  Bundled installable skills
tasks/docs/              Standalone document templates (PRD, TechSpec, ADR)
```

## 🛠️ Development

```bash
make deps      # Install tool dependencies
make fmt       # Format code
make lint      # Lint with zero-tolerance policy
make test      # Run tests with race detector
make build     # Compile binary
make verify    # Full pipeline: fmt → lint → test → build
```

## 🔀 Migration Guide

Migrating from a repository that currently vendors `scripts/markdown`:

1. Remove the copied script tree
2. Install the CLI and skills:
   ```bash
   go install github.com/compozy/looper/cmd/looper@latest
   npx skills add https://github.com/compozy/looper
   ```
3. Choose whether to shell out to `looper` or import `github.com/compozy/looper` / `github.com/compozy/looper/command`
4. Point Looper at the current PRD directory under `tasks/prd-<name>`
5. Fetch reviews into `tasks/prd-<name>/reviews-NNN/` with `looper fetch-reviews --provider <provider> --pr <PR> --name <name>`
6. Run `looper fix-reviews --name <name>` for review remediation or `looper start --name <name>` for PRD task execution

## 📄 License

Looper is licensed under the [MIT License](LICENSE).
