# Compozy Completion Plugin

This plugin adds shell completion for `compozy tasks run` so task slugs are completed from the
nearest `.compozy/tasks` directory relative to your current working directory.

## What it does

- Completes `tasks` after `compozy`
- Completes `run` after `compozy tasks`
- Completes all directories under `.compozy/tasks` after `compozy tasks run`
- Works in worktrees and repository copies by scanning upward from `$PWD` until it finds `.compozy/tasks`

## Installation

1. Copy the plugin file into your shell folder (already placed by default at:
   `~/.zsh/compozy-completion/compozy-completion.plugin.zsh`).

   ```zsh
   # if needed
   cp /path/to/compozy/zsh/compozy-completion/compozy-completion.plugin.zsh \
     "$HOME/.zsh/compozy-completion/compozy-completion.plugin.zsh"
   ```

2. Source it from your `~/.zshrc`:

   ```zsh
   if [[ -f "$HOME/.zsh/compozy-completion/compozy-completion.plugin.zsh" ]]; then
     source "$HOME/.zsh/compozy-completion/compozy-completion.plugin.zsh"
   fi
   ```

3. Reload your shell:

   ```zsh
   source ~/.zshrc
   ```

## Quick usage

From any Compozy workspace:

```zsh
cd /path/to/repo/.compozy-task-root
compozy tasks run <TAB>
```

The command will suggest task directory names found in `.compozy/tasks`.

## Notes

- Keep `.compozy/tasks` present in the workspace root or an ancestor directory.
- If there are no tasks, completion will fall back to default zsh behavior for that command position.
