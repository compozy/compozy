# CLI Command Reference

Complete reference for all Compozy CLI commands, flags, and usage examples.

## Common Flags

These flags are shared by `tasks run`, `exec`, and `reviews fix`:

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--ide` | string | `codex` | ACP runtime: claude, codex, copilot, cursor-agent, droid, gemini, opencode, pi |
| `--model` | string | per-IDE | Model override (codex/droid=gpt-5.5, claude=opus, copilot=claude-sonnet-4.6, cursor-agent=composer-1, opencode/pi=anthropic/claude-opus-4-6, gemini=gemini-2.5-pro) |
| `--reasoning-effort` | string | | Reasoning effort: low, medium, high, xhigh |
| `--add-dir` | string[] | | Additional directories for ACP runtimes (claude and codex only; repeatable or comma-separated) |
| `--auto-commit` | bool | false | Include automatic commit instructions at task/batch completion |
| `--dry-run` | bool | false | Generate prompts without running IDE tool |

---

## Setup & Config

### `compozy setup`

Install core workflow skills into target agents plus any setup assets shipped by enabled extensions.

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--agent`, `-a` | string[] | | Target agent/editor name (repeatable) |
| `--skill`, `-s` | string[] | | Setup skill name to install (repeatable) |
| `--global`, `-g` | bool | false | Install to user directory instead of project |
| `--copy` | bool | false | Copy files instead of symlinking |
| `--list`, `-l` | bool | false | List core skills and enabled extension assets without installing |
| `--yes`, `-y` | bool | false | Skip confirmation prompts |
| `--all` | bool | false | Install all skills to all agents without prompts |

```
compozy setup
compozy setup --list
compozy setup --agent codex --agent claude --skill cy-create-prd --yes
compozy setup --all
compozy setup --agent cursor --global --copy --yes
```

### `compozy upgrade`

Update the Compozy CLI to the latest release. No flags.

---

## Workflow Execution

### `compozy tasks run`

Execute PRD task files sequentially from a workflow directory through the shared daemon.

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--name` | string | | Task workflow name (resolves to `.compozy/tasks/<name>`) |
| `--include-completed` | bool | false | Include tasks already marked as completed |
| `--skip-validation` | bool | false | Skip task metadata preflight check |
| `--force` | bool | false | Continue after validation fails in non-interactive mode |
| `--attach` | string | auto | Attach mode: auto, ui, stream, detach |
| `--ui` | bool | false | Force interactive TUI attach mode |
| `--stream` | bool | false | Force textual stream attach mode |
| `--detach` | bool | false | Start the run without attaching a client |
| `--task-runtime` | string[] | | Per-task runtime override rules |
| + common flags | | | `--ide`, `--model`, `--reasoning-effort`, `--add-dir`, `--auto-commit`, `--dry-run` |

```
compozy tasks run multi-repo --ide claude
compozy tasks run --name multi-repo --ide codex --auto-commit
compozy tasks run multi-repo --stream
```

### `compozy exec [prompt]`

Execute a single ad hoc prompt through the ACP runtime. Provide prompt as argument, via `--prompt-file`, or stdin.

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--agent` | string | | Reusable agent from `.compozy/agents` or `~/.compozy/agents` |
| `--prompt-file` | string | | Path to a file containing the prompt text |
| `--format` | string | text | Output format: text, json, raw-json |
| `--verbose` | bool | false | Emit operational runtime logs to stderr |
| `--tui` | bool | false | Open the interactive TUI |
| `--persist` | bool | false | Save artifacts under `~/.compozy/runs/<run-id>/` |
| `--extensions` | bool | false | Enable executable extensions for this run |
| `--run-id` | string | | Resume a previously persisted session |
| + common flags | | | `--ide`, `--model`, `--reasoning-effort`, `--add-dir`, `--auto-commit`, `--dry-run` |

```
compozy exec "Summarize the current repository changes"
compozy exec --agent council "Decide between two designs"
compozy exec --prompt-file prompt.md --format json
cat prompt.md | compozy exec
compozy exec --persist "Review the latest changes"
compozy exec --run-id exec-20260405-120000-000000000 "Continue"
```

---

## Review

### `compozy reviews fetch`

Fetch review comments from a provider and write them into `.compozy/tasks/<name>/reviews-NNN/`.

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--provider` | string | | Review provider name (e.g., coderabbit) |
| `--pr` | string | | Pull request number |
| `[slug]` | string | | Workflow name (positional) |
| `--name` | string | | Workflow name |
| `--round` | int | next | Review round number (default: next available) |

```
compozy reviews fetch my-feature --provider coderabbit --pr 259
compozy reviews fetch --name my-feature --provider coderabbit --pr 259 --round 2
compozy reviews fetch --name my-feature
```

By default, `reviews fetch` imports CodeRabbit review-body comments for `nitpick`, `minor`, and `major`.
Use `[fetch_reviews].nitpicks = false` in `.compozy/config.toml` to disable that import.

### `compozy reviews fix`

Process review issue files and dispatch agents to remediate feedback.

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `[slug]` | string | | Workflow name (positional) |
| `--name` | string | | Workflow name |
| `--round` | int | latest | Review round number (default: latest existing) |
| `--reviews-dir` | string | | Path to a review round directory |
| `--batch-size` | int | 1 | Number of file groups per batch |
| `--include-resolved` | bool | false | Include already-resolved issues |
| `--concurrent` | int | 1 | Number of batches to process in parallel |
| `--attach` | string | auto | Attach mode: auto, ui, stream, detach |
| `--ui` | bool | false | Force interactive TUI attach mode |
| `--stream` | bool | false | Force textual stream attach mode |
| `--detach` | bool | false | Start the run without attaching a client |
| + common flags | | | `--ide`, `--model`, `--reasoning-effort`, `--add-dir`, `--auto-commit`, `--dry-run` |

```
compozy reviews fix my-feature --ide codex --concurrent 2 --batch-size 3
compozy reviews fix --name my-feature --round 2
compozy reviews fix --reviews-dir .compozy/tasks/my-feature/reviews-001
compozy reviews fix --name my-feature
```

---

## Utilities

### `compozy tasks validate`

Validate task file metadata before execution.

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--name` | string | | Workflow name |
| `--tasks-dir` | string | | Path to tasks directory |
| `--format` | string | | Output format |

```
compozy tasks validate --name my-feature
```

### `compozy sync`

Reconcile authored workflow artifacts under `.compozy/tasks/` into the daemon `global.db` catalog.

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--root-dir` | string | `.compozy/tasks` | Workflow root to scan |
| `--name` | string | | Restrict to one workflow |
| `--tasks-dir` | string | | Restrict to one directory |

```
compozy sync
compozy sync --name my-feature
```

### `compozy archive`

Move workflows that synced daemon state marks as complete to `.compozy/tasks/_archived/<timestamp>-<name>`.

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--root-dir` | string | `.compozy/tasks` | Workflow root to scan |
| `--name` | string | | Restrict to one workflow |
| `--tasks-dir` | string | | Restrict to one directory |

```
compozy archive
compozy archive --name my-feature
```

### `compozy migrate`

Convert legacy XML-tagged artifacts to YAML frontmatter format.

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--root-dir` | string | `.compozy/tasks` | Workflow root to scan |
| `--name` | string | | Restrict to one workflow |
| `--tasks-dir` | string | | Restrict to one directory |
| `--reviews-dir` | string | | Restrict to one review round |
| `--dry-run` | bool | false | Plan without writing |

```
compozy migrate
compozy migrate --dry-run
compozy migrate --name my-feature
```

---

## Agent Management

### `compozy agents list`

List all resolved reusable agents (workspace and global). No flags.

### `compozy agents inspect <name>`

View detailed agent definition, runtime defaults, and MCP servers for a named agent.

---

## Extensions

### `compozy ext list`

List all extensions across all scopes. No flags.

### `compozy ext inspect <name>`

View extension details including capabilities and status.

### `compozy ext install <source>`

Install an extension from a local path or GitHub repo archive.

```bash
compozy ext install ./my-extension
compozy ext install --yes compozy/compozy --remote github --ref v1.2.3 --subdir extensions/cy-idea-factory
```

### `compozy ext uninstall <name>`

Remove an installed extension.

### `compozy ext enable <name>` / `compozy ext disable <name>`

Toggle extension enablement.

### `compozy ext doctor`

Diagnose extension issues and validate configurations.
