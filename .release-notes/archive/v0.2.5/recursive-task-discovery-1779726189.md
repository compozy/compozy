---
title: Recursive task discovery for nested workflow directories
type: feature
---

`compozy tasks run` can now discover `task_NNN.md` files in nested subdirectories under `.compozy/tasks/<slug>/`, so multi-feature workflows can be organized by area instead of flattened into a single root. The behavior is opt-in via a new `--recursive` / `-r` flag — existing flat workflows are byte-identical to before.

### Enabling recursion

```bash
# One-off
compozy tasks run my-workflow --recursive
compozy tasks run my-workflow -r

# Per-workspace default
[tasks.run]
recursive = true
```

The option is also exposed in the interactive task-runtime form and threaded through the daemon runtime overrides (`internal/cli/daemon_commands.go`).

### Discovery rules

When `--recursive` is set:

- `task_NNN.md` files are walked across nested subdirectories.
- The following directories are skipped during the walk:
  - Any directory starting with `.` or `_`
  - `reviews-*` review rounds
  - `adrs/`
  - `memory/`
- Tasks are grouped by directory: root tasks first, then each subdirectory in alphabetical order, sorted numerically within each group.
- Path traversal is hardened via `os.OpenRoot` and validated forward-slash relative paths in `internal/core/tasks/store.go`.

Example layout:

```
.compozy/tasks/my-feature/
├── task_001.md                      # root group (first)
├── features/auth/
│   ├── task_001.md
│   └── task_002.md
└── features/billing/
    └── task_001.md
```

Run order: root → `features/auth/task_001` → `features/auth/task_002` → `features/billing/task_001`.

### `_meta.md` and bulk completion are recursion-aware

`_meta.md` totals and bulk completion now always traverse recursively, so a flat run that happens to have stray nested files no longer under-reports counts. Recursion-mode mismatches between metadata and execution are eliminated.

### Per-subdir workflow memory

Workflow memory is scoped per subdirectory to keep any single `MEMORY.md` well below its 12 KB / 150-line soft cap when recursing across many sub-features:

- A task at `features/auth/task_001.md` writes workflow notes to `memory/features/auth/MEMORY.md`.
- Reads walk up to the closest-ancestor `MEMORY.md`, so shared context still propagates.
- Writes stay isolated to the task's immediate scope.

Path sanitization (`sanitizeTaskMemoryRelpath` / `validateTaskMemoryRelpath` in `internal/core/memory/store.go`) rejects leading slashes, empty segments, and `..` to prevent escape.

### Caveats

- DB sync and the extension Host API still operate on the slug root only.
- Skip-list directories cannot currently be customized per workspace.
