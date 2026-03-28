---
name: bubbletea
description: Build terminal user interfaces with Go and Bubbletea framework. Use for creating TUI apps with the Elm architecture, dual-pane layouts, accordion modes, mouse/keyboard handling, Lipgloss styling, and reusable components. Includes production-ready templates, effects library, and battle-tested layout patterns from real projects.
license: MIT
---

# Bubbletea TUI Development

Production-ready skill for building beautiful terminal user interfaces with Go, Bubbletea, and Lipgloss.

## When to Use This Skill

Use this skill when:
- Creating new TUI applications with Go
- Adding Bubbletea components to existing apps
- Fixing layout/rendering issues (borders, alignment, overflow)
- Implementing mouse/keyboard interactions
- Building dual-pane or multi-panel layouts
- Adding visual effects (metaballs, waves, rainbow text)
- Troubleshooting TUI rendering problems

## Core Principles

**CRITICAL**: Before implementing ANY layout, consult `references/golden-rules.md` for the 4 Golden Rules. These rules prevent the most common and frustrating TUI layout bugs.

### The 4 Golden Rules (Summary)

1. **Always Account for Borders** - Subtract 2 from height calculations BEFORE rendering panels
2. **Never Auto-Wrap in Bordered Panels** - Always truncate text explicitly
3. **Match Mouse Detection to Layout** - Use X coords for horizontal, Y coords for vertical
4. **Use Weights, Not Pixels** - Proportional layouts scale perfectly

Full details and examples in `references/golden-rules.md`.

## Creating New Projects

This project includes a production-ready template system. When this skill is bundled with a new project (via `new_project.sh`), use the existing template structure as the starting point.

### Project Structure

All new projects follow this architecture:
```
your-app/
├── main.go              # Entry point (minimal, ~21 lines)
├── types.go             # Type definitions, structs, enums
├── model.go             # Model initialization & layout calculation
├── update.go            # Message dispatcher
├── update_keyboard.go   # Keyboard handling
├── update_mouse.go      # Mouse handling
├── view.go              # View rendering & layouts
├── styles.go            # Lipgloss style definitions
├── config.go            # Configuration management
└── .claude/skills/bubbletea/  # This skill (bundled)
```

### Architecture Guidelines

- Keep `main.go` minimal (entry point only, ~21 lines)
- All types in `types.go` (structs, enums, constants)
- Separate keyboard and mouse handling into dedicated files
- One file, one responsibility
- Maximum file size: 800 lines (ideally <500)
- Configuration via YAML with hot-reload support

## Available Components

See `references/components.md` for the complete catalog of reusable components:

- **Panel System**: Single, dual-pane, multi-panel, tabbed layouts
- **Lists**: Simple list, filtered list, tree view
- **Input**: Text input, multiline, forms, autocomplete
- **Dialogs**: Confirm, input, progress, modal
- **Menus**: Context menu, command palette, menu bar
- **Status**: Status bar, title bar, breadcrumbs
- **Preview**: Text, markdown, syntax highlighting, images, hex
- **Tables**: Simple and interactive tables

## Effects Library

Beautiful physics-based animations available in the template:

- 🔮 **Metaballs** - Lava lamp-style floating blobs
- 🌊 **Wave Effects** - Sine wave distortions
- 🌈 **Rainbow Cycling** - Animated color gradients
- 🎭 **Layer Compositor** - ANSI-aware multi-layer rendering

See `references/effects.md` for usage examples and integration patterns.

## Layout Implementation Pattern

When implementing layouts, follow this sequence:

### 1. Calculate Available Space
```go
func (m model) calculateLayout() (int, int) {
    contentWidth := m.width
    contentHeight := m.height

    // Subtract UI elements
    if m.config.UI.ShowTitle {
        contentHeight -= 3  // title bar (3 lines)
    }
    if m.config.UI.ShowStatus {
        contentHeight -= 1  // status bar
    }

    // CRITICAL: Account for panel borders
    contentHeight -= 2  // top + bottom borders

    return contentWidth, contentHeight
}
```

### 2. Use Weight-Based Panel Sizing
```go
// Calculate weights based on focus/accordion mode
leftWeight, rightWeight := 1, 1
if m.accordionMode && m.focusedPanel == "left" {
    leftWeight = 2  // Focused panel gets 2x weight
}

// Calculate actual widths from weights
totalWeight := leftWeight + rightWeight
leftWidth := (availableWidth * leftWeight) / totalWeight
rightWidth := availableWidth - leftWidth
```

### 3. Truncate Text to Prevent Wrapping
```go
// Calculate max text width to prevent wrapping
maxTextWidth := panelWidth - 4  // -2 borders, -2 padding

// Truncate ALL text before rendering
title = truncateString(title, maxTextWidth)
subtitle = truncateString(subtitle, maxTextWidth)

func truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-1] + "…"
}
```

## Mouse Interaction Pattern

Always check layout mode before processing mouse events:

```go
func (m model) handleLeftClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
    if m.shouldUseVerticalStack() {
        // Vertical stack mode: use Y coordinates
        topHeight, _ := m.calculateVerticalStackLayout()
        relY := msg.Y - contentStartY

        if relY < topHeight {
            m.focusedPanel = "left"  // Top panel
        } else {
            m.focusedPanel = "right" // Bottom panel
        }
    } else {
        // Side-by-side mode: use X coordinates
        leftWidth, _ := m.calculateDualPaneLayout()

        if msg.X < leftWidth {
            m.focusedPanel = "left"
        } else {
            m.focusedPanel = "right"
        }
    }

    return m, nil
}
```

## Common Pitfalls to Avoid

See `references/troubleshooting.md` for detailed solutions to common issues:

### ❌ DON'T: Set explicit Height() on bordered panels
```go
// BAD: Can cause misalignment
panelStyle := lipgloss.NewStyle().
    Border(border).
    Height(height)  // Don't do this!
```

### ✅ DO: Fill content to exact height
```go
// GOOD: Fill content lines to exact height
for len(lines) < innerHeight {
    lines = append(lines, "")
}
panelStyle := lipgloss.NewStyle().Border(border)
```

## Testing and Debugging

When panels don't align or render incorrectly:

1. **Check height accounting** - Verify contentHeight calculation subtracts all UI elements + borders
2. **Check text wrapping** - Ensure all strings are truncated to maxTextWidth
3. **Check mouse detection** - Verify X/Y coordinate usage matches layout orientation
4. **Check border consistency** - Use same border style for all panels

See `references/troubleshooting.md` for the complete debugging decision tree.

## Configuration System

All projects support YAML configuration with hot-reload:

```yaml
theme: "dark"
keybindings: "default"

layout:
  type: "dual_pane"
  split_ratio: 0.5
  accordion_mode: true

ui:
  show_title: true
  show_status: true
  mouse_enabled: true
  show_icons: true
```

Configuration files are loaded from:
1. `~/.config/your-app/config.yaml` (user config)
2. `./config.yaml` (local override)

## Dependencies

**Required:**
```
github.com/charmbracelet/bubbletea
github.com/charmbracelet/lipgloss
github.com/charmbracelet/bubbles
gopkg.in/yaml.v3
```

**Optional** (uncomment in go.mod as needed):
```
github.com/charmbracelet/glamour       # Markdown rendering
github.com/charmbracelet/huh           # Forms
github.com/alecthomas/chroma/v2        # Syntax highlighting
github.com/evertras/bubble-table       # Interactive tables
github.com/koki-develop/go-fzf         # Fuzzy finder
```

## Reference Documentation

All reference files are loaded progressively as needed:

- **golden-rules.md** - Critical layout patterns and anti-patterns
- **components.md** - Complete catalog of reusable components
- **troubleshooting.md** - Common issues and debugging decision tree
- **emoji-width-fix.md** - Battle-tested solution for emoji alignment across terminals (xterm, WezTerm, Termux, Windows Terminal)

## External Resources

- [Bubbletea Documentation](https://github.com/charmbracelet/bubbletea)
- [Lipgloss Documentation](https://github.com/charmbracelet/lipgloss)
- [Bubbles Components](https://github.com/charmbracelet/bubbles)
- [Charm Ecosystem](https://charm.sh/)

## Best Practices Summary

1. **Always** consult golden-rules.md before implementing layouts
2. **Always** use weight-based sizing for flexible layouts
3. **Always** truncate text explicitly (never rely on auto-wrap)
4. **Always** match mouse detection to layout orientation
5. **Always** account for borders in height calculations
6. **Never** set explicit Height() on bordered Lipgloss styles
7. **Never** assume layout orientation in mouse handlers

Follow these patterns and you'll avoid 90% of TUI layout bugs.
