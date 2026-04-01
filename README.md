# 🔄 Compozy

**Drive the full lifecycle of AI-assisted development — from idea to shipped code.**

Compozy is a Go module and CLI that orchestrates AI coding agents (Claude Code, Codex, Droid, Cursor) to process structured markdown workflows. It covers product ideation, technical specification, task breakdown with codebase-informed enrichment, and automated execution.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)

## 📑 Table of Contents

- [🔄 Compozy](#-compozy)
  - [📑 Table of Contents](#-table-of-contents)
  - [✨ Features](#-features)
  - [🚀 Installation](#-installation)
    - [Requirements](#requirements)
  - [📖 Usage — Complete Workflow](#-usage--complete-workflow)
    - [Step 1: Install skills into your AI agents](#step-1-install-skills-into-your-ai-agents)
    - [Step 2: Create a PRD](#step-2-create-a-prd)
    - [Step 3: Create a TechSpec](#step-3-create-a-techspec)
    - [Step 4: Break down into tasks](#step-4-break-down-into-tasks)
    - [Step 5: Execute tasks with AI agents](#step-5-execute-tasks-with-ai-agents)
    - [Step 6: Review the implementation](#step-6-review-the-implementation)
    - [Step 7: Fix review issues](#step-7-fix-review-issues)
    - [Step 8: Iterate until clean](#step-8-iterate-until-clean)
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
    - [`compozy setup`](#compozy-setup)
    - [`compozy fetch-reviews`](#compozy-fetch-reviews)
    - [`compozy start`](#compozy-start)
    - [`compozy fix-reviews`](#compozy-fix-reviews)
  - [🏗️ Project Layout](#️-project-layout)
  - [🛠️ Development](#️-development)
  - [📄 License](#-license)

## ✨ Features

- **Built-in skills installer** — `compozy setup` installs bundled skills directly into 30+ AI agents with auto-detection, no external tools needed
- **End-to-end PRD workflow** — go from a product idea to implemented code through structured phases
- **Provider-agnostic PR review automation** — fetch and remediate review feedback in tracked rounds under each PRD
- **Multi-agent support** — run tasks through Claude Code, Codex, Droid, Cursor, and many more
- **Concurrent review execution** — run review batches in parallel with configurable batch sizes
- **Interactive mode** — guided form-based CLI for quick setup
- **Plain markdown artifacts** — all specs, tasks, and tracking are human-readable markdown files
- **Embeddable** — use as a standalone CLI or import as a Go package into your own tools

## 🚀 Installation

#### Homebrew

```bash
brew tap compozy/compozy
brew install --cask compozy
```

#### NPM

```bash
npm install -g @compozy/cli
```

#### Go Install

```bash
go install github.com/compozy/compozy/cmd/compozy@latest
```

#### From Source

```bash
git clone git@github.com:compozy/compozy.git
cd compozy
make verify
go build ./cmd/compozy
```

### Post-install: Set Up Skills

Install the bundled skills that Compozy prompts expect:

```bash
compozy setup
```

Non-interactive example:

```bash
compozy setup \
  --agent codex \
  --agent claude \
  --skill create-prd \
  --skill create-techspec \
  --yes
```

### Requirements

- Go 1.26+ (only for `go install` or building from source)

## 📖 Usage — Complete Workflow

This walkthrough builds a feature called **user-auth** from idea to shipped code. Each step feeds into the next — all artifacts are plain markdown files in `tasks/user-auth/`.

> **Tip:** Every CLI command supports `--form` for interactive mode, which guides you through all options.

```
compozy setup                          Install skills (once per project)
   │
   ▼
/create-prd user-auth                 ──▶  tasks/user-auth/_prd.md
   │
   ▼
/create-techspec user-auth            ──▶  tasks/user-auth/_techspec.md
   │
   ▼
/create-tasks user-auth               ──▶  tasks/user-auth/task_01.md … task_N.md
   │
   ▼
compozy start --name user-auth         ──▶  AI agents execute each task
   │
   ▼
/review-round user-auth               ──▶  tasks/user-auth/reviews-001/
   │  (or compozy fetch-reviews)
   ▼
compozy fix-reviews --name user-auth   ──▶  Issues triaged, fixed, resolved
   │
   ▼
🔁 Repeat review → fix until clean   ──▶  Ship it
```

### Step 1: Install skills into your AI agents

Compozy bundles all the skills its workflows depend on. Run `setup` once per project to install them into your AI agents (Claude Code, Codex, Cursor, etc.):

```bash
compozy setup --all --yes
```

This auto-detects installed agents and copies (or symlinks) skills into their configuration directories. See [Skills Setup](#️-skills-setup) for agent-specific options.

### Step 2: Create a PRD

Inside your AI agent, invoke the `/create-prd` skill to brainstorm and produce a Product Requirements Document:

```
/create-prd user-auth
```

The skill runs an interactive session — it asks clarifying questions about what you're building, who it's for, and why. It also spawns parallel agents to research your codebase and the web for context. The result is a business-focused PRD:

```
tasks/user-auth/
  _prd.md            # Product Requirements Document
  adrs/              # Architecture Decision Records from brainstorming
```

### Step 3: Create a TechSpec

Next, translate the business requirements into a technical specification:

```
/create-techspec user-auth
```

The skill reads your PRD, explores the codebase architecture, and asks technical clarification questions (how to implement, which technologies, where components live). Output:

```
tasks/user-auth/
  _techspec.md       # Technical Specification (architecture, APIs, data models)
```

### Step 4: Break down into tasks

Decompose the PRD and TechSpec into independently implementable tasks enriched with codebase context:

```
/create-tasks user-auth
```

The skill analyzes both documents, explores your codebase for relevant files and patterns, and produces individually executable task files. You can review and edit them before execution:

```
tasks/user-auth/
  _tasks.md          # Master task list
  task_01.md         # Individual tasks with status, context, and acceptance criteria
  task_02.md
  task_N.md
```

### Step 5: Execute tasks with AI agents

Now hand the tasks to AI agents for automated implementation:

```bash
compozy start --name user-auth --ide claude
```

Compozy processes each pending task sequentially — the agent reads the task spec, implements the code, validates it, and updates the task status from `pending` to `completed`.

Use `--dry-run` to preview generated prompts without executing. Add `--auto-commit` to automatically commit after each task. See [CLI Reference — `compozy start`](#compozy-start) for all flags.

### Step 6: Review the implementation

Once tasks are complete, review the code before shipping. You have two options:

**Option A** — AI-powered manual review using the `/review-round` skill inside your agent:

```
/review-round user-auth
```

This performs a comprehensive code review across security, correctness, performance, error handling, and more. It generates structured issue files without modifying any source code.

**Option B** — Fetch review comments from an external provider (CodeRabbit, GitHub, etc.):

```bash
compozy fetch-reviews --provider coderabbit --pr 42 --name user-auth
```

Both options produce the same output format:

```
tasks/user-auth/
  reviews-001/
    _meta.md         # Round metadata
    issue_001.md     # Individual review issues with severity and context
    issue_002.md
```

See [PR Review Workflow](#-pr-review-workflow) for details.

### Step 7: Fix review issues

Dispatch AI agents to triage and fix all review issues:

```bash
compozy fix-reviews --name user-auth --ide claude --concurrent 2 --batch-size 3
```

Each agent reads the issue files, triages them as valid or invalid, implements fixes for valid issues, and updates the issue status to `resolved`. Compozy automatically resolves provider threads (CodeRabbit, GitHub) for resolved issues.

### Step 8: Iterate until clean

Repeat steps 6 and 7 until no issues remain. Each cycle creates a new review round directory (`reviews-002/`, `reviews-003/`, etc.), preserving full history. When the implementation is clean — merge and ship.

---

## 🛠️ Skills Setup

Compozy bundles all the skills that its workflows depend on. The `compozy setup` command installs them directly into the configuration directories of your AI coding agents — **no external package manager or `npx` required**.

### How It Works

1. **Detect** — Compozy scans your system for installed agents (Claude Code, Codex, Cursor, etc.)
2. **Select** — In interactive mode you pick which skills and agents to install; in non-interactive mode flags or `--all` drive the selection
3. **Preview** — A summary of every file that will be created is shown before confirmation
4. **Install** — Skills are copied (or symlinked) into each agent's skills directory

```bash
# Interactive — walks you through skill/agent selection
compozy setup

# Install everything to every detected agent, no prompts
compozy setup --all

# Target specific agents and skills
compozy setup --agent claude --agent codex --skill create-prd --skill create-techspec --yes

# Install globally (user-level) instead of per-project
compozy setup --global --yes

# List available bundled skills without installing
compozy setup --list
```

### Supported Agents

Compozy auto-detects and supports **30+ agents and editors**, including:

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
| _…and many more_ |                    | Run `compozy setup` to see all detected agents |

### Installation Modes

When installing to **multiple agents**, Compozy offers two modes:

- **Symlink** _(recommended)_ — One canonical copy in `.agents/skills/`, with symlinks from each agent directory. Keeps all agents in sync automatically.
- **Copy** — Duplicates skill files into each agent directory independently. Use `--copy` when symlinks are not supported (e.g., some CI environments).

When installing to a **single agent**, Compozy copies directly since symlinking offers no benefit.

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
compozy start --name <name>  ──▶  AI agents execute each task sequentially
```

Each step is independent — you can start from any point. All artifacts are plain markdown files in `tasks/<name>/`.

### CLI Usage

Execute tasks from a PRD directory:

```bash
compozy start \
  --name multi-repo \
  --tasks-dir tasks/multi-repo \
  --ide claude
```

Preview generated prompts without executing (dry run):

```bash
compozy start \
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

Files prefixed with `_` are meta documents. Task files (`task_*.md`) are the executable units Compozy processes.

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

The PR review workflow automates the remediation of code review feedback. Review comments are fetched into versioned rounds under the PRD directory, then batched for parallel execution. Compozy resolves provider threads automatically after a batch succeeds.

### How It Works

1. **Review** (optional) — `/review-round` performs a manual code review and creates `tasks/<name>/reviews-NNN/` with issue files
2. **Fetch** (alternative) — `compozy fetch-reviews --provider <provider> --pr <PR> --name <name>` creates `tasks/<name>/reviews-NNN/` from a provider
3. **Batch** — Compozy groups and batches `issue_NNN.md` files based on `--batch-size` and `--grouped`
4. **Execute** — AI agents process each batch concurrently, triaging issues, implementing fixes, and updating issue statuses
5. **Resolve** — after a successful batch, Compozy resolves provider threads for issue files that changed to `## Status: resolved`
6. **Verify** — each batch is validated before completion or auto-commit

### Prerequisites

- `gh` CLI installed and authenticated for the target repository
- access to the target repository through `gh` (for example via `gh auth login`)

### CLI Usage

Fetch a new review round:

```bash
compozy fetch-reviews \
  --provider coderabbit \
  --pr 259 \
  --name my-feature
```

Process batched review issues from the latest round:

```bash
compozy fix-reviews \
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
prep, err := compozy.Prepare(context.Background(), compozy.Config{
    Name:     "multi-repo",
    TasksDir: "tasks/multi-repo",
    Mode:     compozy.ModePRDTasks,
    DryRun:   true,
})
```

### PR Review

Fetch a review round, then execute the remediation loop:

```go
_, _ = compozy.FetchReviews(context.Background(), compozy.Config{
    Name:     "my-feature",
    Provider: "coderabbit",
    PR:       "259",
})

_ = compozy.Run(context.Background(), compozy.Config{
    Name:            "my-feature",
    Mode:            compozy.ModePRReview,
    IDE:             compozy.IDECodex,
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

### `compozy setup`

Install Compozy bundled public skills for supported agents/editors.

```bash
compozy setup [flags]
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

### `compozy fetch-reviews`

Fetch provider review comments into a PRD review round.

```bash
compozy fetch-reviews [flags]
```

| Flag         | Type     | Default | Description                                                                  |
| ------------ | -------- | ------- | ---------------------------------------------------------------------------- |
| `--provider` | `string` |         | Review provider name (for example: `coderabbit`)                             |
| `--pr`       | `string` |         | Pull request number                                                          |
| `--name`     | `string` |         | PRD workflow name (used for `tasks/<name>`)                                  |
| `--round`    | `int`    | `0`     | Review round number. When omitted, Compozy creates the next available round  |
| `--form`     | `bool`   | `false` | Use interactive form to collect parameters                                   |

### `compozy start`

Execute PRD task files from a PRD workflow directory.

```bash
compozy start [flags]
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

### `compozy fix-reviews`

Process review issue markdown files from a PRD review round and dispatch AI agents to remediate feedback.

```bash
compozy fix-reviews [flags]
```

| Flag                         | Type      | Default     | Description                                                                                   |
| ---------------------------- | --------- | ----------- | --------------------------------------------------------------------------------------------- |
| `--name`                     | `string`  |             | PRD workflow name (used for `tasks/<name>`)                                               |
| `--round`                    | `int`     | `0`         | Review round number. When omitted, Compozy uses the latest existing round                     |
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
cmd/compozy/             CLI entry point
command/                 Public Cobra wrapper for embedding
internal/cli/            Cobra flags, interactive form, CLI glue
internal/core/           Internal facade for preparation and execution
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

Compozy is licensed under the [MIT License](LICENSE).
