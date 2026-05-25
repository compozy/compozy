## 0.2.5 - 2026-05-25

### 🎉 Features

- Add zsh task completion plugin docs and script (#149)- Add kiro-cli as supported ACP execution runtime (#160)- Discover task files recursively in nested subdirectories (#153)
### 🐛 Bug Fixes

- Homebrew formula- Emit one task slug per compozy completion candidate (#159)- Run managed upgrade commands (#158)
### 📚 Documentation

- Add star history on readme- Release notes
### 🧪 Testing

- Internal test fix

### Release Notes

#### Features

##### Kiro CLI as a supported ACP execution runtime
Compozy now ships first-class support for the Kiro CLI as an ACP execution runtime, alongside Claude, Codex, Cursor, and Droid. Selecting `--ide kiro` (or persisting it in workspace config) is enough — Compozy locates `kiro-cli`, probes the ACP adapter, and wires the correct bootstrap arguments per session.

### What's new

- New IDE constant `kiro` and default model `anthropic/claude-opus-4-6` (`internal/core/model/constants.go`).
- New registry entry under `internal/core/agent/registry_specs.go` registering the Kiro runtime with:
  - Command: `kiro-cli`
  - Fixed args: `acp`
  - Probe: `kiro-cli acp --help`
  - Setup agent name: `kiro-cli`
  - Docs: <https://kiro.dev/docs/cli/acp>
- Bootstrap forwards `--model <name>` when set and appends `-a` when the run uses `--access-mode full`.

### Usage

```bash
# One-off run
compozy tasks run my-task --ide kiro --model anthropic/claude-opus-4-6

# Per-workspace default
[tasks.run]
ide = "kiro"
model = "anthropic/claude-opus-4-6"
```

### Requirements

Install the Kiro CLI and ensure `kiro-cli acp` is reachable on `PATH`. Compozy's preflight probes the adapter on first run and surfaces a clear `InstallHint` if the binary is missing.

| IDE      | CLI flag     | Default model               | ACP command    |
| -------- | ------------ | --------------------------- | -------------- |
| Kiro CLI | `--ide kiro` | `anthropic/claude-opus-4-6` | `kiro-cli acp` |

##### Recursive task discovery for nested workflow directories
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

##### Zsh task-completion plugin for `compozy tasks run`
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

#### Fixes

##### Homebrew distribution switched from cask to formula
Compozy's Homebrew distribution moves from a cask to a proper formula. This simplifies installation (no separate `brew tap` step), enables `brew test`-driven smoke checks, and aligns the upgrade flow with how CLI tools are normally distributed on Homebrew.

### Install command

```bash
# Before (cask)
brew tap compozy/compozy
brew install --cask compozy

# After (formula)
brew install compozy/compozy/compozy
```

The shorthand auto-taps `compozy/compozy` and installs the `compozy` formula in a single command.

### Upgrade command

`compozy upgrade` (and the `compozy upgrade` flow inside `internal/update/install.go`) now targets the formula instead of the cask:

```bash
brew upgrade compozy/compozy/compozy
```

Existing users on the cask should reinstall via the formula path; both can't coexist on the same prefix.

### Release pipeline

- `.goreleaser.yml` replaces the `homebrew_casks:` block with a `brews:` block:
  - `directory: Formula`
  - `commit_msg_template: "Brew formula update for {{ .ProjectName }} version {{ .Tag }}"`
  - `license: "BSL-1.1"`
  - `test: system "#{bin}/compozy", "--version"` — every published formula now smoke-tests `compozy --version`.
- The release artifact for Homebrew is keyed via `ids: [compozy-archive]` so the formula picks the right archive.
- The archive comment now reads: `Keep the binary at the archive root so Homebrew formulas can install it directly.`

### README

`README.md` is updated with the new one-liner install command.