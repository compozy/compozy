---
name: tui-glamorous
description: >-
  Build terminal UIs with Charmbracelet (Bubble Tea, Lip Gloss, Gum). Use when:
  Go TUI, shell prompts/spinners, "make CLI prettier", adaptive layouts, async
  rendering, focus state machines, sparklines, heatmaps, kanban boards, SSH apps.
---

# Building Glamorous TUIs with Charmbracelet

## Quick Router — Start Here

| I need to... | Use | Reference |
|--------------|-----|-----------|
| **Add prompts/spinners to a shell script** | Gum (no Go) | [Shell Scripts](references/shell-scripts.md) |
| **Build a Go TUI** | Bubble Tea + Lip Gloss | [Go TUI](references/go-tui.md) |
| **Build a production-grade Go TUI** | Above + elite patterns | [Production Architecture](references/production-architecture.md) |
| **Serve a TUI over SSH** | Wish + Bubble Tea | [Infrastructure](references/infrastructure.md) |
| **Record a terminal demo** | VHS | [Shell Scripts](references/shell-scripts.md#vhs-terminal-recording) |
| **Find a Bubbles component** | list, table, viewport, spinner, progress... | [Component Catalog](references/component-catalog.md) |
| **Get a copy-paste pattern** | Layouts, forms, animation, testing | [Quick Reference](references/QUICK-REFERENCE.md) / [Advanced Patterns](references/advanced-patterns.md) |

---

## Decision Guide

```
Is it a shell script?
├─ Yes → Use Gum
│        Need recording? → VHS
│        Need AI? → Mods
│
└─ No (Go application)
   │
   ├─ Just styled output? → Lip Gloss only
   ├─ Simple prompts/forms? → Huh standalone
   ├─ Full interactive TUI? → Bubble Tea + Bubbles + Lip Gloss
   │  │
   │  └─ Production-grade?  → Also add elite patterns:
   │     (multi-view, data-    two-phase async, immutable snapshots,
   │      dense, must be       adaptive layout, focus state machine,
   │      fast & polished)     semantic theming, pre-computed styles
   │                           → See Production Architecture reference
   │
   └─ Need SSH access? → Wish + Bubble Tea
```

---

## Shell Scripts (No Go Required)

```bash
brew install gum  # One-time install
```

```bash
# Input
NAME=$(gum input --placeholder "Your name")

# Selection
COLOR=$(gum choose "red" "green" "blue")

# Fuzzy filter from stdin
BRANCH=$(git branch | gum filter)

# Confirmation
gum confirm "Continue?" && echo "yes"

# Spinner
gum spin --title "Working..." -- long-command

# Styled output
gum style --border rounded --padding "1 2" "Hello"
```

**[Full Gum Reference →](references/shell-scripts.md#gum-the-essential-tool)**
**[VHS Recording →](references/shell-scripts.md#vhs-terminal-recording)**
**[Mods AI →](references/shell-scripts.md#mods-ai-in-terminal)**

---

## Go Applications

```bash
go get github.com/charmbracelet/bubbletea github.com/charmbracelet/lipgloss
```

### Minimal TUI (Copy & Run)

```go
package main

import (
    "fmt"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

var highlight = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)

type model struct {
    items  []string
    cursor int
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "up", "k":
            if m.cursor > 0 { m.cursor-- }
        case "down", "j":
            if m.cursor < len(m.items)-1 { m.cursor++ }
        case "enter":
            fmt.Printf("Selected: %s\n", m.items[m.cursor])
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m model) View() string {
    s := ""
    for i, item := range m.items {
        if i == m.cursor {
            s += highlight.Render("▸ "+item) + "\n"
        } else {
            s += "  " + item + "\n"
        }
    }
    return s + "\n(↑/↓ move, enter select, q quit)"
}

func main() {
    m := model{items: []string{"Option A", "Option B", "Option C"}}
    tea.NewProgram(m).Run()
}
```

### Library Cheat Sheet

| Need | Library | Example |
|------|---------|---------|
| TUI framework | `bubbletea` | `tea.NewProgram(model).Run()` |
| Components | `bubbles` | `list.New()`, `textinput.New()` |
| Styling | `lipgloss` | `style.Foreground(lipgloss.Color("212"))` |
| Forms | `huh` | `huh.NewInput().Title("Name").Run()` |
| Markdown | `glamour` | `glamour.Render(md, "dark")` |
| Animation | `harmonica` | `harmonica.NewSpring()` |

**[Full Go TUI Guide →](references/go-tui.md)**
**[All Bubbles Components →](references/component-catalog.md)**
**[Layout & Animation Patterns →](references/advanced-patterns.md)**

---

## SSH Apps (Infrastructure)

```go
s, _ := wish.NewServer(
    wish.WithAddress(":2222"),
    wish.WithHostKeyPath(".ssh/key"),
    wish.WithMiddleware(
        bubbletea.Middleware(handler),
        logging.Middleware(),
    ),
)
s.ListenAndServe()
```

Connect: `ssh localhost -p 2222`

**[Full Infrastructure Guide →](references/infrastructure.md)**

---

## Production TUI Architecture (Elite Patterns)

Beyond basic Bubble Tea: patterns that make TUIs feel fast, polished, and professional.
Each links to a full code example in [Production Architecture](references/production-architecture.md).

### My TUI is slow or janky

| Symptom | Pattern | Fix |
|---------|---------|-----|
| UI blocks during computation | [Two-Phase Async](references/production-architecture.md#two-phase-async-architecture) | Phase 1 instant, Phase 2 background goroutine |
| Render path holds mutex | [Immutable Snapshots](references/production-architecture.md#immutable-snapshot-pattern) | Pre-build snapshot, atomic pointer swap |
| File changes cause stutter | [Background Worker](references/production-architecture.md#background-worker-with-file-watching) | Debounced watcher + coalescing |
| Thousands of allocs per frame | [Pre-Computed Styles](references/production-architecture.md#pre-computed-styles-for-performance) | Allocate delegate styles once at startup |
| O(n²) string concat in View() | [strings.Builder](references/production-architecture.md#stringsbuilder-in-view) | Pre-allocated Builder with Grow() |
| Glamour re-renders every frame | [Cached Markdown](references/production-architecture.md#cached-markdown-rendering) | Cache by content hash, invalidate on width change |
| GC pauses during interaction | [Idle-Time GC](references/production-architecture.md#idle-time-gc-management) | Trigger GC during idle periods |
| Large dataset = high memory | [Object Pooling](references/production-architecture.md#object-pooling--memory-efficiency) | sync.Pool with pre-allocated slices |
| Rendering off-screen items | [Viewport Virtualization](references/production-architecture.md#viewport-virtualization) | Only render visible rows |

### My layout breaks on different terminals

| Symptom | Pattern | Fix |
|---------|---------|-----|
| Hardcoded widths break | [Adaptive Layout](references/production-architecture.md#adaptive-layout-engine) | 3-4 responsive breakpoints (80/100/140/180 cols) |
| Colors wrong on light terminals | [Semantic Theming](references/production-architecture.md#semantic-theming-system) | `lipgloss.AdaptiveColor` + WCAG AA contrast |
| Items have equal priority → list shuffles | [Deterministic Sorting](references/production-architecture.md#deterministic-stable-sorting) | Stable sort with tie-breaking secondary key |
| Sort mode not visible | [Dynamic Status Bar](references/production-architecture.md#status-bar-with-dynamic-segments) | Left/right segments with gap-fill |

### My TUI has multiple views and it's getting messy

| Symptom | Pattern | Fix |
|---------|---------|-----|
| Key routing chaos | [Focus State Machine](references/production-architecture.md#multi-view-focus-state-machine) | Explicit focus enum + modal priority layer |
| User gets lost in nested views | [Breadcrumb Navigation](references/production-architecture.md#breadcrumb-navigation) | `Home > Board > Priority` path indicator |
| Overlay dismiss loses position | [Focus Restoration](references/production-architecture.md#focus-restoration) | Save focus before overlay, restore on dismiss |
| Old async results overwrite new data | [Stale Message Detection](references/production-architecture.md#stale-message-detection) | Compare data hash before applying results |
| Multiple component updates per frame | [tea.Batch Accumulation](references/production-architecture.md#teabatch-command-accumulation) | Collect cmds in slice, return `tea.Batch(cmds...)` |
| Background goroutine panic kills TUI | [Error Recovery](references/production-architecture.md#error-recovery-in-background-goroutines) | `defer/recover` wrapper for all goroutines |

### I want to add data-rich visualizations

| Want | Pattern | Code |
|------|---------|------|
| Bar charts in list columns | [Unicode Sparklines](references/production-architecture.md#data-visualization-sparklines--heatmaps) | `▇▅▂` using 8-level block characters |
| Color-by-intensity | [Perceptual Heatmaps](references/production-architecture.md#data-visualization-sparklines--heatmaps) | gray → blue → purple → pink gradient |
| Dependency graph in terminal | [ASCII Graph Renderer](references/production-architecture.md#custom-asciiunicode-graph-renderer) | Canvas + Manhattan routing (╭─╮│╰╯) |
| Age at a glance | [Age Color Coding](references/production-architecture.md#data-visualization-sparklines--heatmaps) | Fresh=green, aging=yellow, stale=red |
| Borders that mean something | [Semantic Borders](references/production-architecture.md#color-coded-borders-encoding-state) | Red=blocked, green=ready, yellow=high-impact |

### I want my TUI to feel polished and professional

| Want | Pattern | Key Idea |
|------|---------|----------|
| Vim-style `gg`/`G` | [Vim Key Combos](references/production-architecture.md#vim-key-combo-tracking) | Track `waitingForG` state between keystrokes |
| Search without jank | [Debounced Search](references/production-architecture.md#debounced-search) | 150ms timer, fire only when typing stops |
| Search across all fields at once | [Composite FilterValue](references/production-architecture.md#composite-filtervalue-for-zero-allocation-fuzzy-search) | Flatten all fields into one string |
| 4-line cards with metadata | [Rich Delegates](references/production-architecture.md#rich-multi-line-list-delegates) | Custom delegate with Height()=4 |
| Expand detail inline | [Inline Expansion](references/production-architecture.md#inline-expansion) | Toggle with `d`, auto-collapse on j/k |
| Copy to clipboard | [Clipboard Integration](references/production-architecture.md#clipboard-integration) | `y` for ID, `C` for markdown + toast feedback |
| `?` / `` ` `` / `;` help | [Multi-Tier Help](references/production-architecture.md#multi-tier-help-system) | Quick ref + tutorial + persistent sidebar |
| Kanban with mode switching | [Kanban Swimlanes](references/production-architecture.md#kanban-board-with-swimlane-modes) | Pre-computed board states, O(1) switch |
| Collapsible tree with h/l | [Tree Navigation](references/production-architecture.md#flattened-tree-navigation) | Flatten tree to visible list for j/k nav |
| Suspend TUI for vim edit | [Editor Dispatch](references/production-architecture.md#smart-editor-dispatch) | `tea.ExecProcess` for terminal, background for GUI |
| Remember expand/collapse | [Persistent State](references/production-architecture.md#persistent-ui-state) | Save to JSON, graceful degradation on corrupt |
| Tune via env vars | [Env Preferences](references/production-architecture.md#environment-variable-preferences) | `NO_COLOR`, theme, debounce, split ratio |
| Optional feature missing? | [Graceful Degradation](references/production-architecture.md#graceful-degradation) | Detect at startup, hide unavailable features |

**[Full Production Architecture Guide →](references/production-architecture.md)**

---

## Pre-Flight Checklist (Every TUI)

- [ ] Handle `tea.WindowSizeMsg` — resize all components
- [ ] Handle `ctrl+c` — cleanup, restore terminal state
- [ ] Detect piped stdin/stdout — fall back to plain text
- [ ] Test on 80×24 minimum terminal
- [ ] Provide `--no-tui` / `NO_TUI` escape hatch
- [ ] Test with both light AND dark backgrounds
- [ ] Test with `NO_COLOR=1` and `TERM=dumb`

For production TUIs, see the [full checklist](references/production-architecture.md#production-pre-flight-checklist) (16 must-have + 20 polish items).

---

## When NOT to Use Charm

- **Output is piped:** `mytool | grep` → plain text
- **CI/CD:** No terminal → use flags/env vars
- **One simple prompt:** Maybe `fmt.Scanf` is fine

**Escape hatch:**
```go
if !term.IsTerminal(os.Stdin.Fd()) || os.Getenv("NO_TUI") != "" {
    runPlainMode()
    return
}
```

---

## All References

| I need... | Read this |
|-----------|-----------|
| Copy-paste one-liners | [Quick Reference](references/QUICK-REFERENCE.md) |
| Prompts to give Claude for TUI tasks | [Prompts](references/PROMPTS.md) |
| Gum / VHS / Mods / Freeze / Glow | [Shell Scripts](references/shell-scripts.md) |
| Bubble Tea architecture, debugging, anti-patterns | [Go TUI](references/go-tui.md) |
| Bubbles component APIs (list, table, viewport...) | [Component Catalog](references/component-catalog.md) |
| Theming, layouts, animation, Huh forms, testing | [Advanced Patterns](references/advanced-patterns.md) |
| Elite patterns: async, snapshots, focus machines, adaptive layout, sparklines, kanban, trees, caching | [Production Architecture](references/production-architecture.md) |
| Wish SSH server, Soft Serve, teatest | [Infrastructure](references/infrastructure.md) |
