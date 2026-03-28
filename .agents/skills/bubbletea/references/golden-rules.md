# The 4 Golden Rules for TUI Layout

These rules prevent the most common and frustrating TUI layout bugs. They were discovered through trial-and-error on real projects and will save you hours of debugging.

## Rule #1: Always Account for Borders

**Subtract 2 from height calculations BEFORE rendering panels.**

### The Problem

Lipgloss borders add height to your content. If you calculate content height without accounting for borders, panels will overflow and cover other UI elements.

### The Math

```
WRONG:
contentHeight = totalHeight - 3 (title) - 1 (status) = totalHeight - 4
Panel renders with borders = contentHeight + 2 (borders)
Actual height used = totalHeight - 4 + 2 = totalHeight - 2 (TOO TALL!)

CORRECT:
contentHeight = totalHeight - 3 (title) - 1 (status) - 2 (borders) = totalHeight - 6
Panel renders with borders = contentHeight + 2
Actual height used = totalHeight - 6 + 2 = totalHeight - 4 ✓
```

### Visual Layout

```
┌─────────────────────────────────┐  ← Title Bar (3 lines)
│  App Title                      │
│  Subtitle/Info                  │
├─────────────────────────────────┤  ─┐
│ ┌─────────────┬───────────────┐ │   │
│ │             │               │ │   │
│ │   Left      │     Right     │ │   │ Content Height
│ │   Panel     │     Panel     │ │   │ (minus borders)
│ │             │               │ │   │
│ └─────────────┴───────────────┘ │   │
├─────────────────────────────────┤  ─┘
│ Status Bar: Help text here      │  ← Status Bar (1 line)
└─────────────────────────────────┘

Panel borders (┌─┐ └─┘) = 2 lines total (top + bottom)
```

### Correct Implementation

```go
func (m model) calculateLayout() (int, int) {
    contentWidth := m.width
    contentHeight := m.height

    if m.config.UI.ShowTitle {
        contentHeight -= 3 // title bar (3 lines)
    }
    if m.config.UI.ShowStatus {
        contentHeight -= 1 // status bar
    }

    // CRITICAL: Account for panel borders
    contentHeight -= 2 // top + bottom borders

    return contentWidth, contentHeight
}
```

### Height Calculation Example

```
Total Terminal Height: 25
- Title Bar:           -3
- Status Bar:          -1
- Panel Borders:       -2
─────────────────────────
Content Height:        19 ✓
```

## Rule #2: Never Auto-Wrap in Bordered Panels

**Always truncate text explicitly to prevent wrapping.**

### The Problem

When text wraps inside a bordered panel, it can cause:
- Panels to become different heights (misalignment)
- Content to overflow panel boundaries
- Inconsistent rendering across different terminal widths

### Why This Happens

Lipgloss auto-wraps text that exceeds the panel width. In bordered panels, this creates extra lines you didn't account for in your height calculations.

### The Solution

Calculate the maximum text width and truncate ALL strings before rendering:

```go
// Calculate max text width to prevent wrapping
maxTextWidth := panelWidth - 4 // -2 for borders, -2 for padding

// Truncate ALL text before rendering
title = truncateString(title, maxTextWidth)
subtitle = truncateString(subtitle, maxTextWidth)

// Truncate content lines too
for i := 0; i < availableContentLines && i < len(content); i++ {
    line := truncateString(content[i], maxTextWidth)
    lines = append(lines, line)
}

// Helper function
func truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-1] + "…"
}
```

### Real-World Example

Without truncation, this subtitle wraps:
```
┌─────────────┐
│Weight: 2 | │
│Size: 80x25 │  ← Wrapped to 2 lines!
└─────────────┘
```

With truncation:
```
┌─────────────┐
│Weight: 2 | …│  ← Truncated, stays 1 line
└─────────────┘
```

## Rule #3: Match Mouse Detection to Layout

**Use X coordinates for horizontal layouts, Y coordinates for vertical layouts.**

### The Problem

If your layout orientation changes (side-by-side vs stacked), but your mouse detection logic doesn't, clicks won't work correctly.

### The Solution

Check layout mode before processing mouse events:

```go
func (m model) handleLeftClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
    // ... boundary checks ...

    if m.shouldUseVerticalStack() {
        // Vertical stack mode: use Y coordinates
        topHeight, _ := m.calculateVerticalStackLayout()
        relY := msg.Y - contentStartY

        if relY < topHeight {
            m.focusedPanel = "left"  // Top panel
        } else if relY > topHeight {
            m.focusedPanel = "right" // Bottom panel
        }
    } else {
        // Side-by-side mode: use X coordinates
        leftWidth, _ := m.calculateDualPaneLayout()

        if msg.X < leftWidth {
            m.focusedPanel = "left"
        } else if msg.X > leftWidth {
            m.focusedPanel = "right"
        }
    }

    return m, nil
}
```

### Visual Guide

**Horizontal Layout (use X coordinates):**
```
┌────────┬────────┐
│ Left   │ Right  │
│        │        │
└────────┴────────┘
    ↑ msg.X determines which panel
```

**Vertical Layout (use Y coordinates):**
```
┌────────────────┐
│ Top            │  ↑
├────────────────┤  msg.Y determines
│ Bottom         │  which panel
└────────────────┘
```

## Rule #4: Use Weights, Not Pixels

**Proportional layouts scale perfectly across all terminal sizes.**

### The Problem

Fixed pixel widths break when:
- Terminal is resized
- Different monitors have different dimensions
- Users have portrait vs landscape terminals

### The Solution: Weight-Based Layout (LazyGit Pattern)

Instead of calculating pixel widths, assign **weights** to panels:

```go
// Calculate weights based on focus
leftWeight, rightWeight := 1, 1

if m.accordionMode && m.focusedPanel == "left" {
    leftWeight = 2  // Focused panel gets 2x weight
}

// Calculate actual widths from weights
totalWeight := leftWeight + rightWeight
leftWidth := (availableWidth * leftWeight) / totalWeight
rightWidth := availableWidth - leftWidth
```

### Weight Examples

**Equal weights (1:1) = 50/50 split:**
```
Total width: 80
leftWeight: 1, rightWeight: 1
totalWeight: 2

leftWidth = (80 * 1) / 2 = 40
rightWidth = 80 - 40 = 40

┌──────────────────────┬──────────────────────┐
│                      │                      │
│       50%            │       50%            │
│                      │                      │
└──────────────────────┴──────────────────────┘
```

**Focused weight (2:1) = 66/33 split:**
```
Total width: 80
leftWeight: 2, rightWeight: 1
totalWeight: 3

leftWidth = (80 * 2) / 3 = 53
rightWidth = 80 - 53 = 27

┌────────────────────────────────┬─────────────┐
│                                │             │
│           66%                  │    33%      │
│                                │             │
└────────────────────────────────┴─────────────┘
```

### Why This Works

1. **Proportional** - Always maintains exact ratios
2. **Simple** - No complex formulas, just division
3. **Immediate** - No animations needed, instant resize
4. **Flexible** - Change weight = instant layout change
5. **Scalable** - Works at any terminal size

### Common Weight Patterns

```go
// Equal split
leftWeight, rightWeight := 1, 1  // 50/50

// Accordion mode (focused panel larger)
if focusedPanel == "left" {
    leftWeight, rightWeight = 2, 1  // 66/33
} else {
    leftWeight, rightWeight = 1, 2  // 33/66
}

// Three panels
mainWeight, leftWeight, rightWeight := 2, 1, 1  // 50/25/25

// Preview mode (main content larger)
contentWeight, previewWeight := 3, 1  // 75/25
```

## Common Pitfalls

### ❌ DON'T: Set explicit Height() on bordered styles

```go
// BAD: Can cause misalignment
panelStyle := lipgloss.NewStyle().
    Border(border).
    Height(height)  // Don't do this!
```

**Why it's bad:** The height includes borders, making calculations confusing and error-prone.

### ✅ DO: Fill content to exact height, let borders add naturally

```go
// GOOD: Fill content lines to exact height
for len(lines) < innerHeight {
    lines = append(lines, "")
}
panelStyle := lipgloss.NewStyle().Border(border)
// No Height() - let content determine it
```

### ❌ DON'T: Assume layout orientation in mouse handlers

```go
// BAD: Always using X coordinate
if msg.X < leftWidth {
    // This breaks in vertical stack!
}
```

### ✅ DO: Check layout mode first

```go
// GOOD: Different logic per orientation
if m.shouldUseVerticalStack() {
    // Use Y coordinates
} else {
    // Use X coordinates
}
```

## Debugging Checklist

When panels don't align or render incorrectly, check in this order:

1. **Height accounting (Rule #1)**
   - Did you subtract 2 for borders?
   - Formula: `totalHeight - titleLines - statusLines - 2`

2. **Text wrapping (Rule #2)**
   - Is text wrapping to multiple lines?
   - `maxWidth = panelWidth - 4`
   - Truncate ALL strings explicitly

3. **Mouse detection (Rule #3)**
   - Vertical stack? → Use `msg.Y`
   - Horizontal? → Use `msg.X`
   - Match detection to layout mode

4. **Weight calculations (Rule #4)**
   - Using weights instead of pixels?
   - Formula: `(totalWidth * weight) / totalWeights`

## Decision Tree

```
Panel Layout Problem?
│
├─ Panels covering title/status bar?
│  └─> Check height accounting (Rule #1)
│      - Did you subtract 2 for borders?
│      - Formula: totalHeight - titleLines - statusLines - 2
│
├─ Panels misaligned (different heights)?
│  └─> Check text wrapping (Rule #2)
│      - Is text wrapping to multiple lines?
│      - maxWidth = panelWidth - 4
│      - Truncate ALL strings explicitly
│
├─ Mouse clicks not working?
│  └─> Check mouse detection (Rule #3)
│      - Vertical stack? → Use msg.Y
│      - Horizontal? → Use msg.X
│      - Match detection to layout mode
│
└─ Accordion/resize janky?
   └─> Check weight calculations (Rule #4)
       - Using weights instead of pixels?
       - Formula: (totalWidth * weight) / totalWeights
```

## Summary

Follow these 4 rules and you'll avoid 90% of TUI layout bugs:

1. ✅ **Always account for borders** - Subtract 2 before rendering
2. ✅ **Never auto-wrap** - Truncate explicitly
3. ✅ **Match mouse to layout** - X for horizontal, Y for vertical
4. ✅ **Use weights** - Proportional scaling

These patterns are battle-tested and will save you hours of debugging frustration.
