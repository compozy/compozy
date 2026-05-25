---
title: Zsh task-completion plugin for `compozy tasks run`
type: feature
---

Compozy now ships a self-contained zsh completion plugin that completes task slugs for `compozy tasks run` from the nearest `.compozy/tasks` directory relative to your shell's `$PWD`. It works across worktrees and repository copies — no global config needed.

### What it completes

- `tasks` after `compozy`
- `run` after `compozy tasks`
- Task slugs after `compozy tasks run` — one suggestion per task directory under the discovered `.compozy/tasks`

If no `.compozy/tasks` is found in any ancestor of `$PWD`, completion falls back to the default zsh behavior at that position.

### Installation

```zsh
# 1. Copy the plugin into your shell folder
cp /path/to/compozy/zsh/compozy-completion/compozy-completion.plugin.zsh \
  "$HOME/.zsh/compozy-completion/compozy-completion.plugin.zsh"

# 2. Source it from ~/.zshrc
if [[ -f "$HOME/.zsh/compozy-completion/compozy-completion.plugin.zsh" ]]; then
  source "$HOME/.zsh/compozy-completion/compozy-completion.plugin.zsh"
fi

# 3. Reload
source ~/.zshrc
```

```zsh
cd /path/to/repo
compozy tasks run <TAB>     # → suggests task directory names from .compozy/tasks
```

Full docs live at `zsh/compozy-completion/README.md` in the repo.

### Discovery model

`_compozy_tasks_workspace` walks upward from `$PWD` until it finds a directory containing `.compozy/tasks`, then enumerates that directory for slugs. This keeps completion fast and avoids spawning Compozy on every TAB.

### Companion fix

A bug in the initial plugin was also fixed in this release: when multiple subdirectories existed under `.compozy/tasks`, the same slug could be emitted more than once on TAB. Slug discovery now goes through a dedicated `_compozy_task_slugs` helper that:

- Returns one slug per task directory, deduplicated.
- Uses zsh `(N/)` glob qualifiers so it gracefully no-ops when the directory is empty.
- Splits results with `(@f)` to keep slugs containing spaces or special characters intact.

End result: `compozy tasks run <TAB>` shows each available task slug exactly once.
