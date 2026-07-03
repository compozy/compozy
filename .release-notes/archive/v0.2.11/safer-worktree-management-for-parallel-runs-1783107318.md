---
title: Safer worktree management for parallel runs
type: fix
---

This release hardens the git-worktree machinery that backs parallel runs (`--parallel-tasks` and `--multiple --parallel`), closing several ways a run could start in an unsafe state or leave worktrees behind.

### Preflight guards

- **Detached HEAD is rejected** before the daemon is contacted, so a parallel run can't start from a checkout with no branch to base worktrees on.
- **Parallel runs inside a managed worktree are refused** up front, preventing nested parallel execution from colliding with the worktree it's already running in.

### Purge and cleanup

- `compozy` purge now reports **both** purged runs and purged worktrees, with safer containment checks so it only removes paths it actually owns.
- Worktree purge uses deferred handling that behaves correctly for nested active runs and missing workspace roots, and tracks produced-vs-pre-existing task worktree changes accurately.
- Terminal shutdown now also stops spawned child processes, so quitting a run doesn't leave orphaned agent processes running.

### Git environment isolation

Git subprocesses now **ignore inherited repository-scoped Git environment variables** (e.g. `GIT_DIR`, `GIT_WORK_TREE`). Previously an ambient Git env from the parent shell or a wrapping tool could redirect worktree operations at the wrong repository; each worktree command now runs against the repository Compozy intends.

### Also

- Bumped the Go toolchain patch version (1.26.3 → 1.26.4).
