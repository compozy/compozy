<div align="center">
  <h1>Compozy</h1>
  <p><strong>Orchestrate AI coding agents from idea to shipped code — in a single pipeline.</strong></p>
  <p>
    <a href="https://github.com/compozy/compozy/actions/workflows/ci.yml">
      <img src="https://github.com/compozy/compozy/actions/workflows/ci.yml/badge.svg" alt="CI">
    </a>
    <a href="https://pkg.go.dev/github.com/compozy/compozy">
      <img src="https://pkg.go.dev/badge/github.com/compozy/compozy.svg" alt="Go Reference">
    </a>
    <a href="https://goreportcard.com/report/github.com/compozy/compozy">
      <img src="https://goreportcard.com/badge/github.com/compozy/compozy" alt="Go Report Card">
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT">
    </a>
    <a href="https://github.com/compozy/compozy/releases">
      <img src="https://img.shields.io/github/v/release/compozy/compozy?include_prereleases" alt="Release">
    </a>
  </p>
</div>

One CLI to replace scattered prompts, manual task tracking, and copy-paste review cycles. Compozy drives the full lifecycle of AI-assisted development: product ideation, technical specification, task breakdown with codebase-informed enrichment, concurrent execution across agents, and automated PR review remediation.

## Highlights

- **One command, 40+ agents.** Install bundled skills into Claude Code, Codex, Cursor, Droid, and 40+ other agents and editors with `compozy setup` — no npm, pipx, or external tools required.
- **Idea to code in 5 steps.** Structured pipeline: PRD → TechSpec → Tasks → Execution → Review. Each phase produces plain markdown artifacts that feed into the next.
- **Codebase-aware enrichment.** Tasks aren't generic prompts. Compozy spawns parallel agents to explore your codebase, discover patterns, and ground every task in real project context.
- **Multi-agent execution.** Run tasks through Claude Code, Codex, Cursor, or Droid — just change `--ide`. Concurrent batch processing with configurable timeouts, retries, and exponential backoff, all with a live terminal UI.
- **Provider-agnostic reviews.** Fetch review comments from CodeRabbit, GitHub, or run AI-powered reviews internally. All normalize to the same format. Provider threads resolve automatically after fixes.
- **Markdown everywhere.** PRDs, specs, tasks, reviews, and ADRs are human-readable markdown files. Version-controlled, diffable, editable between steps. No vendor lock-in.
- **Frontmatter for machine-readable metadata.** Tasks and review issues keep parseable metadata in standard YAML frontmatter instead of custom XML tags.
- **Single binary, local-first.** Compiles to one Go binary with zero runtime dependencies. Your code and data stay on your machine.
- **Embeddable.** Use as a standalone CLI or import as a Go package into your own tools.

## Installation

#### Homebrew

```bash
brew tap compozy/compozy
brew install --cask compozy
```

#### NPM

```bash
npm install -g @compozy/cli
```

#### Go

```bash
go install github.com/compozy/compozy/cmd/compozy@latest
```

#### From Source

```bash
git clone git@github.com:compozy/compozy.git
cd compozy && make verify && go build ./cmd/compozy
```

Then install bundled skills into your AI agents:

```bash
compozy setup          # interactive — pick agents and skills
compozy setup --all    # install everything to every detected agent
```

## How It Works

```
compozy setup                           Install skills (once per project)
   │
   ▼
/cy-create-prd user-auth               .compozy/tasks/user-auth/_prd.md
   │                                    + Architecture Decision Records
   ▼
/cy-create-techspec user-auth          .compozy/tasks/user-auth/_techspec.md
   │
   ▼
/cy-create-tasks user-auth             .compozy/tasks/user-auth/task_01.md … task_N.md
   │
   ▼
compozy sync --name user-auth          Refresh task workflow _meta.md
   │
   ▼
compozy start --name user-auth         AI agents execute each task
   │
   ▼
compozy fetch-reviews / /cy-review-round  .compozy/tasks/user-auth/reviews-001/
   │
   ▼
compozy fix-reviews --name user-auth   Issues triaged, fixed, resolved
   │
   ▼
Repeat until clean → Ship
```

Every artifact is a plain markdown file in `.compozy/tasks/<name>/`. You can read, edit, or version-control any of them between steps.

Task and review issue files use YAML frontmatter for parseable metadata such as `status`, `domain`, `severity`, and `provider_ref`. Task workflow `_meta.md` files can be refreshed explicitly with `compozy sync`. If you have an older project with XML-tagged artifacts, run `compozy migrate` once before using `start` or `fix-reviews`.

## Quick Start

This walkthrough builds a feature called **user-auth** from idea to shipped code.

### 1. Install skills

```bash
compozy setup --all --yes
```

Auto-detects installed agents and copies (or symlinks) skills into their configuration directories.

### 2. Create a PRD

Inside your AI agent (Claude Code, Codex, Cursor, etc.):

```
/cy-create-prd user-auth
```

Interactive brainstorming session — asks clarifying questions, spawns parallel agents to research your codebase and the web, produces a business-focused PRD with ADRs.

### 3. Create a TechSpec

```
/cy-create-techspec user-auth
```

Reads your PRD, explores the codebase architecture, asks technical clarification questions. Produces architecture specs, API designs, and data models.

### 4. Break down into tasks

```
/cy-create-tasks user-auth
```

Analyzes both documents, explores your codebase for relevant files and patterns, produces individually executable task files with status tracking, context, and acceptance criteria.

### 5. Execute tasks

```bash
compozy start --name user-auth --ide claude
```

Each pending task is processed sequentially — the agent reads the spec, implements the code, validates it, and updates the task status. Use `--dry-run` to preview prompts without executing.

### 6. Review

**Option A** — AI-powered review inside your agent:

```
/cy-review-round user-auth
```

**Option B** — Fetch from an external provider:

```bash
compozy fetch-reviews --provider coderabbit --pr 42 --name user-auth
```

Both produce the same output: `.compozy/tasks/user-auth/reviews-001/issue_*.md`

### 7. Fix review issues

```bash
compozy fix-reviews --name user-auth --ide claude --concurrent 2 --batch-size 3
```

Agents triage each issue as valid or invalid, implement fixes for valid issues, and update statuses. Provider threads are resolved automatically.

### 8. Iterate and ship

Repeat steps 6–7. Each cycle creates a new review round (`reviews-002/`, `reviews-003/`), preserving full history. When clean — merge and ship.

## Skills

Compozy bundles 7 skills that its workflows depend on. They run inside your AI agent — no context switching to external tools.

| Skill | Purpose |
| --- | --- |
| `cy-create-prd` | Interactive brainstorming → Product Requirements Document with ADRs |
| `cy-create-techspec` | PRD → Technical Specification with architecture exploration |
| `cy-create-tasks` | PRD + TechSpec → Independently implementable task files |
| `cy-execute-task` | Executes one task end-to-end: implement, validate, track, commit |
| `cy-review-round` | Comprehensive code review → structured issue files |
| `cy-fix-reviews` | Triage, fix, verify, and resolve review issues |
| `cy-final-verify` | Enforces verification evidence before any completion claim |

### Supported Agents

**Execution** (`compozy start`, `compozy fix-reviews`) — 4 agents that can run tasks:

| Agent | `--ide` flag |
| --- | --- |
| Claude Code | `claude` |
| Codex | `codex` |
| Cursor | `cursor` |
| Droid | `droid` |

**Skill installation** (`compozy setup`) — 40+ agents and editors, including Claude Code, Codex, Cursor, Droid, Gemini CLI, GitHub Copilot, Windsurf, Amp, Continue, Goose, Roo Code, Augment, Kiro CLI, Cline, and many more. Run `compozy setup` to see all detected agents on your system.

When installing to multiple agents, Compozy offers two modes:

- **Symlink** *(default)* — One canonical copy with symlinks from each agent directory. All agents stay in sync.
- **Copy** — Independent copies per agent. Use `--copy` when symlinks are not supported.

## CLI Reference

<details>
<summary><code>compozy setup</code> — Install bundled skills for supported agents</summary>

```bash
compozy setup [flags]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--agent`, `-a` | | Target agent name (repeatable) |
| `--skill`, `-s` | | Skill name to install (repeatable) |
| `--global`, `-g` | `false` | Install to user directory instead of project |
| `--copy` | `false` | Copy files instead of symlinking |
| `--list`, `-l` | `false` | List bundled skills without installing |
| `--yes`, `-y` | `false` | Skip confirmation prompts |
| `--all` | `false` | Install all skills to all agents |

</details>

<details>
<summary><code>compozy migrate</code> — Convert legacy XML-tagged artifacts to frontmatter</summary>

```bash
compozy migrate [flags]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--root-dir` | `.compozy/tasks` | Workflow root to scan recursively |
| `--name` | | Restrict migration to one workflow name |
| `--tasks-dir` | | Restrict migration to one task workflow directory |
| `--reviews-dir` | | Restrict migration to one review round directory |
| `--dry-run` | `false` | Preview migrations without writing files |

</details>

<details>
<summary><code>compozy sync</code> — Refresh task workflow metadata files</summary>

```bash
compozy sync [flags]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--root-dir` | `.compozy/tasks` | Workflow root to scan |
| `--name` | | Restrict sync to one workflow name |
| `--tasks-dir` | | Restrict sync to one task workflow directory |

</details>

<details>
<summary><code>compozy start</code> — Execute PRD task files</summary>

```bash
compozy start [flags]
```

Running `compozy start` with no flags opens the interactive form automatically.

| Flag | Default | Description |
| --- | --- | --- |
| `--name` | | Workflow name (`.compozy/tasks/<name>`) |
| `--tasks-dir` | | Path to tasks directory |
| `--ide` | `codex` | Agent: `claude`, `codex`, `cursor`, `droid` |
| `--model` | *(per IDE)* | Model override |
| `--reasoning-effort` | `medium` | `low`, `medium`, `high`, `xhigh` |
| `--timeout` | `10m` | Activity timeout per job |
| `--max-retries` | `0` | Retry failed jobs N times |
| `--retry-backoff-multiplier` | `1.5` | Timeout multiplier per retry |
| `--tail-lines` | `30` | Log lines shown per job in UI |
| `--add-dir` | | Additional directories to allow (repeatable) |
| `--auto-commit` | `false` | Auto-commit after each task |
| `--include-completed` | `false` | Re-run completed tasks |
| `--dry-run` | `false` | Preview prompts without executing |

</details>

<details>
<summary><code>compozy fetch-reviews</code> — Fetch review comments into a review round</summary>

```bash
compozy fetch-reviews [flags]
```

Running `compozy fetch-reviews` with no flags opens the interactive form automatically.

| Flag | Default | Description |
| --- | --- | --- |
| `--provider` | | Review provider (`coderabbit`, etc.) |
| `--pr` | | Pull request number |
| `--name` | | Workflow name |
| `--round` | `0` | Round number (auto-increments if omitted) |

</details>

<details>
<summary><code>compozy fix-reviews</code> — Dispatch AI agents to remediate review issues</summary>

```bash
compozy fix-reviews [flags]
```

Running `compozy fix-reviews` with no flags opens the interactive form automatically.

| Flag | Default | Description |
| --- | --- | --- |
| `--name` | | Workflow name |
| `--round` | `0` | Round number (latest if omitted) |
| `--reviews-dir` | | Override review directory path |
| `--ide` | `codex` | Agent: `claude`, `codex`, `cursor`, `droid` |
| `--model` | *(per IDE)* | Model override |
| `--batch-size` | `1` | Issues per batch |
| `--concurrent` | `1` | Parallel batches |
| `--grouped` | `false` | Generate grouped issue summaries |
| `--include-resolved` | `false` | Re-process resolved issues |
| `--reasoning-effort` | `medium` | `low`, `medium`, `high`, `xhigh` |
| `--timeout` | `10m` | Activity timeout per job |
| `--max-retries` | `0` | Retry failed jobs N times |
| `--retry-backoff-multiplier` | `1.5` | Timeout multiplier per retry |
| `--tail-lines` | `30` | Log lines shown per job in UI |
| `--add-dir` | | Additional directories to allow (repeatable) |
| `--auto-commit` | `false` | Auto-commit after each batch |
| `--dry-run` | `false` | Preview prompts without executing |

</details>

<details>
<summary><strong>Go Package Usage</strong> — Use Compozy as a library in your own tools</summary>

```go
// Prepare work without executing
prep, err := compozy.Prepare(context.Background(), compozy.Config{
    Name:     "multi-repo",
    TasksDir: ".compozy/tasks/multi-repo",
    Mode:     compozy.ModePRDTasks,
    DryRun:   true,
})

// Fetch reviews and run remediation
_, _ = compozy.FetchReviews(context.Background(), compozy.Config{
    Name:     "my-feature",
    Provider: "coderabbit",
    PR:       "259",
})

// Preview a legacy artifact migration
_, _ = compozy.Migrate(context.Background(), compozy.MigrationConfig{
    DryRun: true,
})

_ = compozy.Run(context.Background(), compozy.Config{
    Name:            "my-feature",
    Mode:            compozy.ModePRReview,
    IDE:             compozy.IDECodex,
    ReasoningEffort: "medium",
})

// Embed the Cobra command in another CLI
root := command.New()
_ = root.Execute()
```

</details>

<details>
<summary><strong>Project Layout</strong></summary>

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
.compozy/tasks/          Default workflow artifact root (PRDs, TechSpecs, tasks, ADRs, reviews)
```

</details>

## Development

```bash
make verify    # Full pipeline: fmt → lint → test → build
make fmt       # Format code
make lint      # Lint (zero tolerance)
make test      # Tests with race detector
make build     # Compile binary
make deps      # Tidy and verify modules
```

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE)
