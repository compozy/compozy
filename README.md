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
  - [`looper start`](#looper-start)
  - [`looper fix-reviews`](#looper-fix-reviews)
- [🏗️ Project Layout](#️-project-layout)
- [🛠️ Development](#️-development)
- [🔀 Migration Guide](#-migration-guide)
- [📄 License](#-license)

## ✨ Features

- **End-to-end PRD workflow** — go from a product idea to implemented code through structured phases
- **PR review automation** — process CodeRabbit review feedback in batches automatically
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

Looper exposes two workflow subcommands: **[PRD Tasks](#-prd-workflow)** via `looper start`, and **[PR Review](#-pr-review-workflow)** via `looper fix-reviews`. See each workflow section below for details.

Interactive mode prompts for workflow-specific options:

```bash
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

The PR review workflow automates the remediation of code review feedback. It takes review issues (e.g., from CodeRabbit), triages them, batches them for parallel processing, and dispatches AI agents to implement fixes and resolve review threads — all without manual intervention.

### How It Works

1. **Export** — use the `fix-coderabbit-review` skill to export review comments into structured issue files at `ai-docs/reviews-pr-<PR>/issues`
2. **Batch** — Looper groups and batches the issue files based on `--batch-size` and `--grouped` flags
3. **Execute** — AI agents process each batch concurrently, implementing fixes and resolving GitHub review threads
4. **Verify** — each fix is validated before the review thread is marked as resolved

### Prerequisites

- `gh` CLI installed and authenticated for the target repository
- `python3` available on `PATH`
- `GITHUB_TOKEN` available if `gh` uses environment-based authentication

### CLI Usage

Process batched PR review issues:

```bash
looper fix-reviews \
  --pr 259 \
  --ide codex \
  --concurrent 2 \
  --batch-size 3 \
  --grouped
```

### Skills

| Skill | Purpose |
|-------|---------|
| `fix-coderabbit-review` | Processes PR review feedback: exports CodeRabbit issues, triages fixes, implements changes, verifies, and resolves GitHub review threads |

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
    PR:        "multi-repo",
    IssuesDir: "tasks/prd-multi-repo",
    Mode:      looper.ModePRDTasks,
    DryRun:    true,
})
```

### PR Review

Execute the full review remediation loop:

```go
_ = looper.Run(context.Background(), looper.Config{
    PR:              "259",
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

Process CodeRabbit review issue markdown files and dispatch AI agents to remediate feedback.

```bash
looper fix-reviews [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--pr` | `string` | | Pull request number |
| `--issues-dir` | `string` | | Path to issues directory (`ai-docs/reviews-pr-<PR>/issues`) |
| `--ide` | `string` | `codex` | IDE tool to use: `claude`, `codex`, `cursor`, or `droid` |
| `--model` | `string` | *(per IDE)* | Model to use (default: `gpt-5.4` for codex/droid, `opus` for claude, `composer-1` for cursor) |
| `--batch-size` | `int` | `1` | Number of file groups to batch together |
| `--concurrent` | `int` | `1` | Number of batches to process in parallel |
| `--grouped` | `bool` | `false` | Generate grouped issue summaries in `issues/grouped/` directory |
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
4. Point at the same issue/task directories:
   - `ai-docs/reviews-pr-<PR>/issues` for PR reviews
   - `tasks/prd-<name>` for PRD task workflows
5. Use creation skills to define new work, then run `looper start --name <name>` to execute

## 📄 License

Looper is licensed under the [MIT License](LICENSE).
