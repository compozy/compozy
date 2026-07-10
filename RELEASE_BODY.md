## 0.2.12 - 2026-07-10

### 🐛 Bug Fixes

- Parallel tasks (#231)

### Release Notes

#### Features

##### GPT-5.6, Claude Fable 5, and ACP session model negotiation
Compozy now negotiates model, reasoning, and permission mode from the options advertised by ACP `session/new` / `session/load`, and applies the resolved configuration **before the first prompt** on both new and resumed sessions.

### New models and defaults

- **Codex / Droid** default to `gpt-5.6-sol`. Supported GPT-5.6 IDs include `gpt-5.6-sol`, `gpt-5.6-terra`, and `gpt-5.6-luna` when the installed adapter advertises them.
- **Claude Fable 5** accepts `--model fable`, `fable-5`, or `claude-fable-5`. Fable always uses Claude's `auto` permission mode (never `bypassPermissions`), even when `--access-mode full` was requested.
- **Cursor** model names resolve against its ACP catalog (for example `--model grok-4.5` → the advertised catalog ID). Cursor does not get a separate reasoning option when the session does not advertise one.

### Reasoning effort

`max` and `ultra` are now first-class reasoning levels in the CLI forms, setup wizard, and recovery flags. Claude's `max` is an advertised ACP effort value; if a requested effort is not advertised, Compozy stops before the prompt and lists the valid choices.

### Codex ACP adapter

The preferred Codex ACP package is now `@agentclientprotocol/codex-acp` (GPT-5.6 and `max`/`ultra` need `>= 1.1.2`). The legacy `@zed-industries/codex-acp` path remains only for older combinations such as GPT-5.5 with reasoning through `xhigh`.

#### Fixes

##### Safer parallel task and multi-run worktree lifecycle
Parallel execution no longer surprises you with worktrees or silent merges. Activation is explicit, results are retained on named branches, and cleanup refuses to delete uncommitted or unretained output.

### Explicit activation for `--parallel-tasks`

`[tasks.run.parallel] enabled` is now a compatibility field only — it does **not** authorize worktree-backed execution by itself. A single-workflow run uses per-task worktrees and an integration branch only after an explicit per-run choice:

- `--parallel-tasks=true` (or the wizard)
- `runtime_overrides.parallel_tasks.enabled=true`
- `--parallel-tasks=false` keeps the standard serial runner

### Named result branches and safe cleanup

Multi-spec (`--multiple --parallel`) children get deterministic `compozy/multi-*` result branches. Output is **never auto-merged** into the user's branch. After settlement:

- a clean worktree can be removed while committed output stays on the result branch
- an empty result branch is deleted when it still points at the base
- dirty trees or output without a proven retention point stay `preserved` with an explicit reason

`compozy runs purge` applies the same ownership, dirty-tree, and commit-retention checks.

### Single-workflow waves

Within one workflow, successful dependency-wave output is squash-merged into a temporary integration branch and fast-forwarded only if every required task succeeds. On failure the user's branch is unchanged; partial output and unsafe trees are preserved for inspection.

### Clearer handoff and wizard

The CLI prints the resolved execution kind, whether worktrees are used, and the source of that choice before starting. Stream/TUI handoff now includes `result_branch`, `worktree_status`, and `worktree_reason` so every retained or preserved tree is locatable.