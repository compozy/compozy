# TUI App Design Patterns Gallery

Real-world design analysis of exceptional TUI applications, organized by the pattern they exemplify. Use for inspiration and precedent when designing your own TUI.

---

## Persistent Multi-Panel Pattern

### lazygit — The Gold Standard
**Framework:** Go (gocui fork) | **Layout:** 5 left panels + right detail

The defining multi-panel TUI. All views (status, files, branches, commits, stash) remain visible simultaneously. The left column acts as a selector; the right column shows context for the selected item.

**Key innovations:**
- **Contextual keybinding footer** — available actions update as focus changes. Goal: zero memorization.
- **Popup layering** — confirmation dialogs and commit editors appear as overlays without losing spatial context.
- **Command transparency** — shows the actual git commands being executed under the hood.
- **Guided multi-step workflows** — interactive rebase, conflict resolution use progressive disclosure through confirmations.

**Why it works:** Users build spatial memory. Branches are *always* top-left, commits are *always* middle-left. No navigation required to see the full picture.

### lazydocker — Real-Time Dashboard Variant
**Framework:** Go (gocui) | **Layout:** Master list (left) + detail tabs (right)

Two-pane horizontal split. Left switches between Docker objects; right shows live detail with tabs for logs, stats, config. Live ASCII resource graphs render directly in the detail pane.

**Key innovation:** Docker Compose awareness — auto-groups containers into "Services" and "Standalone." The layout adapts to project structure.

### oxker — Single-Screen Everything
**Framework:** Rust (Ratatui) | **Layout:** All panels always visible

Containers list, logs, CPU charts, memory charts, and port mappings visible simultaneously. No tabs, no drill-down. Click-sortable column headers bring spreadsheet interaction to the terminal.

**Key innovation:** Mixed input — full keyboard AND mouse support. Neither forced; both first-class.

---

## Drill-Down Stack Pattern

### k9s — Command-Mode Navigation
**Framework:** Go (tcell/tview) | **Navigation:** Enter descends, Esc ascends, `:resource` jumps

The Kubernetes TUI. An infinite drill-down through resources: cluster → namespace → deployment → pod → container → logs.

**Key innovations:**
- **`:resource` command mode** — type `:pods`, `:deployments` to jump directly. The TUI equivalent of a URL bar.
- **XRay mode** — unique tree visualization showing a resource and all its related resources across types.
- **Pulse view** — heads-up display of cluster health metrics.
- **Context-aware skins** — different color schemes per Kubernetes cluster. Production is visually distinct from staging. Safety through color.

**Why it works:** The command-mode (`:`) plus drill-down (Enter/Esc) creates two navigation dimensions — direct jumps for known targets, exploration for discovery.

### diskonaut — Spatial Treemap
**Framework:** Rust (tui-rs) | **Navigation:** Arrow keys select blocks, Enter drills in

The entire terminal fills with rectangles proportional to file/directory size. A genuine treemap visualization in text mode.

**Key innovations:**
- **Progressive scanning** — treemap builds in real-time as filesystem scan progresses. Explore already-scanned regions while scanning continues.
- **Zoom levels** — `+`/`-` reveal smaller files that appear as `x` at default zoom.
- **Deletion tracking** — inline delete with cumulative freed-space counter.

**Why it works:** Spatial reasoning. You literally *see* which files are largest by their visual area. No need to compare numbers.

---

## Miller Columns Pattern

### yazi — Async-First File Management
**Framework:** Rust (Ratatui + Tokio) | **Layout:** 3 columns: parent / current / preview

The modern file manager. Miller columns with async I/O for never-blocking navigation.

**Key innovations:**
- **Inline image previews** — renders images directly in terminal via auto-detected protocols (Kitty, iTerm2, Sixel).
- **Async architecture** — dual-priority task queue (micro: metadata reads, macro: file transfers). UI never freezes.
- **Smart preview preloading** — predictive loading based on cursor position. Code gets syntax highlighting, images get decoded, archives get listed.
- **Concurrent Lua plugins** — `ya.sync()` and `ya.async()` modes for parallel plugin execution.

**Why it works:** The "never freeze" principle. Large directories, slow network mounts, big file previews — the interface stays responsive. The three-column layout provides past/present/future spatial context at every level.

### ranger — Vim-Native Miller Columns
**Framework:** Python (curses) | **Navigation:** hjkl maps to column movement

The original vim-keybinding file manager. `h` goes up a directory (left column), `l` enters (right column).

**Key innovation:** **Bookmarks** (`m<key>` to set, `'<key>` to jump) provide teleportation — bypass hierarchical navigation entirely.

---

## Tab-Based Workspace Pattern

### gitui — Performance as UX
**Framework:** Rust (Ratatui) | **Layout:** 5 top tabs, split panes within each

Five tabs (Status, Log, Files, Stashing, Stashes) provide persistent navigation landmarks. Each tab is a focused workspace.

**Key innovations:**
- **Line-level staging** — stage individual hunks or lines in the diff view.
- **Single-key mnemonics** — interface shows `[c]ommit [a]mend [p]ush` directly inline.
- **Performance** — 2× faster than lazygit with 1/15th memory on the Linux kernel repo (900k+ commits). Speed changes interaction patterns — users browse and explore rather than search-and-jump.

**Why it works:** When navigating 900k commits feels instant, speed becomes a design feature.

### harlequin — Terminal IDE
**Framework:** Python (Textual) | **Layout:** 3 panels: catalog / editor / results

Full SQL IDE in the terminal. Data catalog tree (left), tabbed query editor (center), virtualized results table (bottom).

**Key innovations:**
- **1M+ row virtual tables** — scrollable results that don't load everything into memory.
- **Full-screen toggle** — `F10` expands any panel to fill the terminal (IDE "zen mode").
- **12+ community themes** — Catppuccin, Dracula, Nord, Monokai out of the box.
- **Adapter plugins** — database backends installed as pip packages.

**Why it works:** Proves that "terminal = limited" is a myth. DBeaver-level functionality with htop-level responsiveness.

---

## Overlay / Popup Pattern

### atuin — Augmented Shell Primitive
**Framework:** Rust (Ratatui) | **Trigger:** Replaces Ctrl+R

Replaces the 40-year-old Ctrl+R shell history with a full TUI overlay that appears on demand and disappears after selection.

**Key innovations:**
- **Multi-dimensional filtering** — toggle filters for host, session, directory, and global scope.
- **Rich metadata** — each entry shows command, duration, exit code, host, timestamp.
- **Configurable density** — from single-line fzf-style to full-screen explorer.
- **Cross-device sync** — encrypted history sync across machines.

**Why it works:** The "popup TUI" pattern — summoned, used, dismissed. Invisible when not needed. Transforms a basic shell feature with structure and intelligence.

### posting — IDE Patterns in Terminal
**Framework:** Python (Textual) | **Layout:** IDE three-panel with innovations

HTTP client for terminals. The Postman-for-terminal that introduced several novel TUI interaction patterns.

**Key innovations:**
- **Jump mode** — Vimium-style: press a key, letter overlays appear on every interactive element, press the letter to jump directly. Eliminates Tab cycling entirely.
- **Command palette** — `Ctrl+P` opens VS Code-style fuzzy command search.
- **YAML-based request storage** — git-friendly, version-controllable, team-shareable.
- **Environment-aware styling** — production URLs get blinking red backgrounds as visual safety rails.

**Why it works:** Jump mode is genuinely novel for TUIs. Instead of navigating *through* the interface to reach a target, users navigate *to* the target directly.

---

## Widget Dashboard Pattern

### btop — Polished System Monitor
**Framework:** C++ (custom) | **Layout:** T-shaped grid of bordered widget boxes

Per-core CPU graphs, memory breakdown, network I/O, disk activity, and process table in a T-shaped dashboard.

**Key innovations:**
- **Bordered box zones** — each widget is a self-contained panel with title, creating a dashboard-of-dashboards.
- **Braille sparklines** — high-resolution graphs using Unicode braille characters.
- **Theme ecosystem** — rich theming with 24-bit truecolor and 256-color fallback.

### bottom (btm) — Configurable Widget Composition
**Framework:** Rust (Ratatui) | **Layout:** Configurable widget grid

Similar to btop but the dashboard layout is configurable via TOML. Users define their own widget arrangement.

**Key innovations:**
- **Braille + dot sparklines** — Unicode braille default with dot marker fallback.
- **Basic mode** — `--basic` flag strips graphs, shows htop-style tables only. Progressive complexity.
- **Battery and temperature** — first-class hardware widgets beyond CPU/memory.

---

## Terminal Multiplexer Pattern

### zellij — Reimagined Workspace
**Framework:** Rust (custom) | **Layout:** Tiled + floating panes

Modern terminal multiplexer that treats the terminal as a workspace, not just a multiplexer.

**Key innovations:**
- **Floating panes** — first-class floating windows that overlay tiled panes. Toggle with `Alt+f`.
- **Stacked panes** — when resize shrinks a pane too small, it stacks with neighbors showing only title bars. The active one expands. Novel responsive behavior.
- **Session resurrection** — closed sessions preserve full state. Resurrect any previous session exactly.
- **Modal keybinding** — enter Pane mode, then use simple keys. Status bar shows current mode and available keys. Solves tmux's discoverability problem.
- **KDL layout files** — declarative, version-controllable workspace definitions.
- **WASM plugins** — sandboxed, crash-proof, language-agnostic extensions.

---

## Classic Patterns Still Relevant

### vim/neovim — Composable Grammar
The interaction model where keystrokes compose as a language: `d2w` = "delete 2 words." Grammar: `{operator}{count}{motion}`. The key insight: **composable grammar over memorized shortcuts**.

Modern extensions: Telescope (fuzzy finder popup), Which-Key (shows continuations after prefix), floating windows for previews.

### tmux — Prefix Key Namespace
All commands sit behind a prefix key (`Ctrl+b`), preventing collisions with inner programs. The **status bar as wayfinding strip** pattern — a single persistent line showing session/window/pane context.

### htop — Semantic Color Encoding
CPU meter colors encode meaning: green = user processes, red = kernel, blue = low-priority, cyan = virtualization overhead. Expert users diagnose system state at a glance from color ratios alone. **Color as data channel, not decoration.**

### Midnight Commander — Orthodox File Manager
The dual-pane paradigm: source and destination simultaneously visible. `Tab` switches active panel. File operations default to using the opposite panel as destination. 40 years old and still unbeaten for power-user file manipulation.

### tig — View Stack Navigation
Views push onto a navigation stack. Close a view → return to previous. Browser-like back-navigation through git data. Master-detail split: list view above, detail below. **The view stack pattern makes complex data navigable.**

---

## Cross-Cutting Insights

1. **The best TUIs feel alive** — real-time updates, responsive to every keypress, async operations never freeze.
2. **Spatial consistency builds mastery** — users remember *where* things are, not *how* to find them.
3. **The modern TUI trinity** — command palette + vim motions + contextual footer covers every skill level.
4. **Speed is a feature** — sub-millisecond response to keypresses creates a fundamentally different interaction quality.
5. **Configuration as code** — YAML/TOML/KDL config files enable version control, sharing, and reproducibility.
6. **Every great TUI has an escape hatch** — `q` to quit, `Esc` to go back, `?` for help. Always.
