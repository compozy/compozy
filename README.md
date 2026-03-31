# 🔄 Looper

**Drive the full lifecycle of AI-assisted development — from idea to shipped code.**

Looper is a Go module and CLI that orchestrates AI coding agents (Claude Code, Codex, Droid, Cursor) to process structured markdown workflows. It covers product ideation, technical specification, task breakdown with codebase-informed enrichment, and automated execution.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)

## 📑 Table of Contents

- [🔄 Looper](#-looper)
  - [📑 Table of Contents](#-table-of-contents)
  - [✨ Features](#-features)
  - [🚀 Installation](#-installation)
    - [Requirements](#requirements)
  - [📖 Usage](#-usage)
  - [🛠️ Skills Setup](#️-skills-setup)
    - [How It Works](#how-it-works)
    - [Supported Agents](#supported-agents)
    - [Installation Modes](#installation-modes)
  - [🔧 PRD Workflow](#-prd-workflow)
    - [How It Works](#how-it-works-1)
    - [CLI Usage](#cli-usage)
    - [Skills](#skills)
    - [Output Convention](#output-convention)
  - [🔍 PR Review Workflow](#-pr-review-workflow)
    - [How It Works](#how-it-works-2)
    - [Prerequisites](#prerequisites)
    - [CLI Usage](#cli-usage-1)
    - [Skills](#skills-1)
  - [🧩 Shared Skills](#-shared-skills)
  - [📦 Go Package Usage](#-go-package-usage)
    - [PRD Tasks](#prd-tasks)
    - [PR Review](#pr-review)
    - [Embedding](#embedding)
  - [⌨️ CLI Reference](#️-cli-reference)
    - [`looper setup`](#looper-setup)
    - [`looper fetch-reviews`](#looper-fetch-reviews)
    - [`looper start`](#looper-start)
    - [`looper fix-reviews`](#looper-fix-reviews)
  - [🏗️ Project Layout](#️-project-layout)
  - [🛠️ Development](#️-development)
  - [📄 License](#-license)

## ✨ Features

- **Built-in skills installer** — `looper setup` installs bundled skills directly into 30+ AI agents with auto-detection, no external tools needed
- **End-to-end PRD workflow** — go from a product idea to implemented code through structured phases
- **Provider-agnostic PR review automation** — fetch and remediate review feedback in tracked rounds under each PRD
- **Multi-agent support** — run tasks through Claude Code, Codex, Droid, Cursor, and many more
- **Concurrent review execution** — run review batches in parallel with configurable batch sizes
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
looper setup
```

Non-interactive example:

```bash
looper setup \
  --agent codex \
  --agent claude \
  --skill create-prd \
  --skill create-techspec \
  --yes
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

Looper exposes four subcommands: **[Skills Setup](#️-skills-setup)** via `looper setup`, **[PRD Tasks](#-prd-workflow)** via `looper start`, **review fetching** via `looper fetch-reviews`, and **[PR Review](#-pr-review-workflow)** remediation via `looper fix-reviews`. See each section below for details.

Interactive mode prompts for workflow-specific options:

```bash
looper setup
looper fetch-reviews --form
looper start --form
looper fix-reviews --form
```

---

## 🛠️ Skills Setup

Looper bundles all the skills that its workflows depend on. The `looper setup` command installs them directly into the configuration directories of your AI coding agents — **no external package manager or `npx` required**.

### How It Works

1. **Detect** — Looper scans your system for installed agents (Claude Code, Codex, Cursor, etc.)
2. **Select** — In interactive mode you pick which skills and agents to install; in non-interactive mode flags or `--all` drive the selection
3. **Preview** — A summary of every file that will be created is shown before confirmation
4. **Install** — Skills are copied (or symlinked) into each agent's skills directory

```bash
# Interactive — walks you through skill/agent selection
looper setup

# Install everything to every detected agent, no prompts
looper setup --all

# Target specific agents and skills
looper setup --agent claude --agent codex --skill create-prd --skill create-techspec --yes

# Install globally (user-level) instead of per-project
looper setup --global --yes

# List available bundled skills without installing
looper setup --list
```

### Supported Agents

Looper auto-detects and supports **30+ agents and editors**, including:

| Agent            | Project Directory  | Notes                                         |
| ---------------- | ------------------ | --------------------------------------------- |
| Claude Code      | `.claude/skills`   | Also accepts `--agent claude` alias           |
| Codex            | `.agents/skills`   | Universal layout                              |
| Cursor           | `.agents/skills`   | Universal layout                              |
| Droid            | `.factory/skills`  |                                               |
| Gemini CLI       | `.agents/skills`   | Universal layout                              |
| GitHub Copilot   | `.agents/skills`   | Universal layout                              |
| Windsurf         | `.windsurf/skills` |                                               |
| Amp              | `.agents/skills`   | Universal layout                              |
| Continue         | `.continue/skills` |                                               |
| Goose            | `.goose/skills`    |                                               |
| Roo Code         | `.roo/skills`      |                                               |
| Augment          | `.augment/skills`  |                                               |
| _…and many more_ |                    | Run `looper setup` to see all detected agents |

### Installation Modes

When installing to **multiple agents**, Looper offers two modes:

- **Symlink** _(recommended)_ — One canonical copy in `.agents/skills/`, with symlinks from each agent directory. Keeps all agents in sync automatically.
- **Copy** — Duplicates skill files into each agent directory independently. Use `--copy` when symlinks are not supported (e.g., some CI environments).

When installing to a **single agent**, Looper copies directly since symlinking offers no benefit.

---

## 🔧 PRD Workflow

The PRD workflow takes you from a product idea to implemented code through a structured pipeline. Each step produces plain markdown artifacts that feed into the next, and AI agents execute the final tasks automatically.

### How It Works

```
💡 Idea
   │
   ▼
/create-prd          ──▶  tasks/<name>/_prd.md
   │
   ▼
/create-techspec     ──▶  tasks/<name>/_techspec.md
   │
   ▼
/create-tasks        ──▶  tasks/<name>/_tasks.md + task_01.md … task_N.md
   │
   ▼
looper start --name <name>  ──▶  AI agents execute each task sequentially
```

Each step is independent — you can start from any point. All artifacts are plain markdown files in `tasks/<name>/`.

### CLI Usage

Execute tasks from a PRD directory:

```bash
looper start \
  --name multi-repo \
  --tasks-dir tasks/multi-repo \
  --ide claude
```

Preview generated prompts without executing (dry run):

```bash
looper start \
  --name multi-repo \
  --tasks-dir tasks/multi-repo \
  --dry-run
```

### Skills

| Skill              | Phase     | Purpose                                                                                                  |
| ------------------ | --------- | -------------------------------------------------------------------------------------------------------- |
| `create-prd`       | Creation  | Creates PRDs through 5-phase interactive brainstorming with parallel codebase and web research           |
| `create-techspec`  | Creation  | Translates PRD business requirements into technical specifications with architecture exploration         |
| `create-tasks`     | Creation  | Decomposes PRDs and TechSpecs into independently implementable task files enriched with codebase context |
| `execute-prd-task` | Execution | Executes one PRD task end-to-end: implement, validate, track, and optionally commit                      |

### Output Convention

All PRD workflow artifacts live in `tasks/<name>/`:

```
tasks/<name>/
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

1. **Review** (optional) — `/review-round` performs a manual code review and creates `tasks/<name>/reviews-NNN/` with issue files
2. **Fetch** (alternative) — `looper fetch-reviews --provider <provider> --pr <PR> --name <name>` creates `tasks/<name>/reviews-NNN/` from a provider
3. **Batch** — Looper groups and batches `issue_NNN.md` files based on `--batch-size` and `--grouped`
4. **Execute** — AI agents process each batch concurrently, triaging issues, implementing fixes, and updating issue statuses
5. **Resolve** — after a successful batch, Looper resolves provider threads for issue files that changed to `## Status: resolved`
6. **Verify** — each batch is validated before completion or auto-commit

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

| Skill          | Purpose                                                                                                                           |
| -------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `review-round` | Performs a comprehensive code review of a PRD implementation and generates a review round directory compatible with `fix-reviews` |
| `fix-reviews`  | Processes review issue batches: triages issues, implements fixes, verifies results, and updates review tracking files             |

---

## 🧩 Shared Skills

These skills are used across both workflows.

| Skill                            | Purpose                                                                                  |
| -------------------------------- | ---------------------------------------------------------------------------------------- |
| `verification-before-completion` | Enforces fresh verification evidence before any completion claim, commit, or PR creation |

## 📦 Go Package Usage

### PRD Tasks

Prepare work without executing any IDE process:

```go
prep, err := looper.Prepare(context.Background(), looper.Config{
    Name:     "multi-repo",
    TasksDir: "tasks/multi-repo",
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

### `looper setup`

Install Looper bundled public skills for supported agents/editors.

```bash
looper setup [flags]
```

| Flag             | Type      | Default | Description                                                               |
| ---------------- | --------- | ------- | ------------------------------------------------------------------------- |
| `--agent`, `-a`  | `strings` |         | Target agent/editor name (repeatable)                                     |
| `--skill`, `-s`  | `strings` |         | Bundled skill name to install (repeatable)                                |
| `--global`, `-g` | `bool`    | `false` | Install to the user directory instead of the project                      |
| `--copy`         | `bool`    | `false` | Copy files instead of symlinking to agent directories                     |
| `--list`, `-l`   | `bool`    | `false` | List bundled public skills without installing                             |
| `--yes`, `-y`    | `bool`    | `false` | Skip confirmation prompts                                                 |
| `--all`          | `bool`    | `false` | Install all bundled public skills to all supported agents without prompts |

### `looper fetch-reviews`

Fetch provider review comments into a PRD review round.

```bash
looper fetch-reviews [flags]
```

| Flag         | Type     | Default | Description                                                                |
| ------------ | -------- | ------- | -------------------------------------------------------------------------- |
| `--provider` | `string` |         | Review provider name (for example: `coderabbit`)                           |
| `--pr`       | `string` |         | Pull request number                                                        |
| `--name`     | `string` |         | PRD workflow name (used for `tasks/<name>`)                            |
| `--round`    | `int`    | `0`     | Review round number. When omitted, Looper creates the next available round |
| `--form`     | `bool`   | `false` | Use interactive form to collect parameters                                 |

### `looper start`

Execute PRD task files from a PRD workflow directory.

```bash
looper start [flags]
```

| Flag                         | Type      | Default     | Description                                                                                   |
| ---------------------------- | --------- | ----------- | --------------------------------------------------------------------------------------------- |
| `--name`                     | `string`  |             | PRD task workflow name (used for `tasks/<name>`)                                          |
| `--tasks-dir`                | `string`  |             | Path to PRD tasks directory (`tasks/<name>`)                                              |
| `--ide`                      | `string`  | `codex`     | IDE tool to use: `claude`, `codex`, `cursor`, or `droid`                                      |
| `--model`                    | `string`  | _(per IDE)_ | Model to use (default: `gpt-5.4` for codex/droid, `opus` for claude, `composer-1` for cursor) |
| `--reasoning-effort`         | `string`  | `medium`    | Reasoning effort for codex/claude/droid (`low`, `medium`, `high`, `xhigh`)                    |
| `--timeout`                  | `string`  | `10m`       | Activity timeout duration (e.g., `5m`, `30s`). Job canceled if no output within this period   |
| `--max-retries`              | `int`     | `0`         | Retry failed or timed-out jobs up to N times before marking them failed                       |
| `--retry-backoff-multiplier` | `float`   | `1.5`       | Multiplier applied to activity timeout after each retry                                       |
| `--tail-lines`               | `int`     | `30`        | Number of log lines to show in UI for each job                                                |
| `--add-dir`                  | `strings` |             | Additional directory to allow for Codex and Claude (repeatable or comma-separated)            |
| `--auto-commit`              | `bool`    | `false`     | Include automatic commit instructions at task/batch completion                                |
| `--include-completed`        | `bool`    | `false`     | Include completed tasks                                                                       |
| `--dry-run`                  | `bool`    | `false`     | Only generate prompts; do not run IDE tool                                                    |
| `--form`                     | `bool`    | `false`     | Use interactive form to collect parameters                                                    |

### `looper fix-reviews`

Process review issue markdown files from a PRD review round and dispatch AI agents to remediate feedback.

```bash
looper fix-reviews [flags]
```

| Flag                         | Type      | Default     | Description                                                                                   |
| ---------------------------- | --------- | ----------- | --------------------------------------------------------------------------------------------- |
| `--name`                     | `string`  |             | PRD workflow name (used for `tasks/<name>`)                                               |
| `--round`                    | `int`     | `0`         | Review round number. When omitted, Looper uses the latest existing round                      |
| `--reviews-dir`              | `string`  |             | Override path to a review round directory (`tasks/<name>/reviews-NNN`)                    |
| `--ide`                      | `string`  | `codex`     | IDE tool to use: `claude`, `codex`, `cursor`, or `droid`                                      |
| `--model`                    | `string`  | _(per IDE)_ | Model to use (default: `gpt-5.4` for codex/droid, `opus` for claude, `composer-1` for cursor) |
| `--batch-size`               | `int`     | `1`         | Number of file groups to batch together                                                       |
| `--concurrent`               | `int`     | `1`         | Number of batches to process in parallel                                                      |
| `--grouped`                  | `bool`    | `false`     | Generate grouped issue summaries in `reviews-NNN/grouped/`                                    |
| `--include-resolved`         | `bool`    | `false`     | Include review issues already marked as resolved                                              |
| `--reasoning-effort`         | `string`  | `medium`    | Reasoning effort for codex/claude/droid (`low`, `medium`, `high`, `xhigh`)                    |
| `--timeout`                  | `string`  | `10m`       | Activity timeout duration (e.g., `5m`, `30s`). Job canceled if no output within this period   |
| `--max-retries`              | `int`     | `0`         | Retry failed or timed-out jobs up to N times before marking them failed                       |
| `--retry-backoff-multiplier` | `float`   | `1.5`       | Multiplier applied to activity timeout after each retry                                       |
| `--tail-lines`               | `int`     | `30`        | Number of log lines to show in UI for each job                                                |
| `--add-dir`                  | `strings` |             | Additional directory to allow for Codex and Claude (repeatable or comma-separated)            |
| `--auto-commit`              | `bool`    | `false`     | Include automatic commit instructions at task/batch completion                                |
| `--dry-run`                  | `bool`    | `false`     | Only generate prompts; do not run IDE tool                                                    |
| `--form`                     | `bool`    | `false`     | Use interactive form to collect parameters                                                    |

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
internal/setup/          Bundled skill installer (agent detection, symlink/copy)
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

## 📄 License

Looper is licensed under the [MIT License](LICENSE).
