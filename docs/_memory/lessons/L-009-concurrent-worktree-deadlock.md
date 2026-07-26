# L-009 — Concurrent worktree commits deadlock; isolate `AGH_HOME` + ports

**Class:** Workflow
**Date discovered:** 2026-04-07 (`tasks-skills-v2-d2eb18` task_05); reinforced repeatedly
**Evidence sources:** local_runs + multiple Codex sessions

## Context

`tasks-skills-v2-d2eb18-20260407-222116-000000000_1/jobs/task_05-f8e6c1.err.log` showed `fatal: Unable to create '.git/index.lock': File exists.` Concurrent task workers tried to commit in the same worktree simultaneously. This is a Compozy-orchestration-level bug: multiple subagents must not share a worktree for `git add`/`commit`.

Symmetrically, parallel QA runs that grab the default `~/.agh/` home and the default daemon port deadlock each other — both daemons race to claim the same SQLite locks, port, and tmux-bridge socket.

## Root cause

Two failure modes share a common cause: shared mutable state across parallel agents. Git index lock, SQLite advisory lock, daemon listening port, tmux socket, `~/.agh/` home — all become contention points when concurrent agents assume defaults.

## Rule

> When the user runs multiple AGH/Compozy agents in parallel worktrees, every test or QA run uses a unique `AGH_HOME` and a unique daemon port. Default home and default port use is forbidden in QA flows when concurrency is signaled.
>
> Compozy must serialize commits per worktree (or each subagent must own its own worktree).

## Operationalization for parallel QA

- Set `AGH_HOME` per scenario:
  ```bash
  export AGH_HOME=$(mktemp -d -t agh-XXXX)
  # OR use worktree-scoped: AGH_HOME=Compozy/_worktrees/<slug>/.agh
  ```
- Pick a unique daemon port via `free-port` or set `[http] port = ...` in scenario `config.toml`.
- Pick a unique `tmux-bridge` socket path if tmux automation is used.
- Block any operation writing to `~/.agh/` or default ports when concurrency is signaled.

## Operationalization for parallel commits

- Each subagent owns its own worktree (Compozy already does this — see `Compozy/_worktrees/<slug>/`).
- Within a worktree, commits are serial.
- Never run two `git commit` invocations from different processes against the same `.git/index`.

## Detection signals

- `fatal: Unable to create '.git/index.lock'` — concurrent git operations.
- `database is locked` — SQLite from concurrent daemons.
- Two daemon processes listening on the same port — port collision.
- `cannot bind to socket` for tmux-bridge — socket collision.

## Source

- `.compozy/runs/tasks-skills-v2-d2eb18-20260407-222116-000000000_1/jobs/task_05-f8e6c1.err.log`
- Multiple Codex sessions ("$qa-execution ... você deve isolar o AGH_HOME aqui e a porta")
- `../analysis/analysis_local_runs.md` lesson LL-4, `../analysis/analysis_codex_sessions.md` (worktree-isolation candidate)
- `feedback_worktree_isolation.md` user memory
