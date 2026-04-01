# Bubble Tea v2 Migration Guide

Complete reference for migrating from Bubble Tea / Lip Gloss / Bubbles v1 to v2. Sources: official upgrade guides, discussion announcements, and the Charm blog.

## Quick Start

```bash
go get charm.land/bubbletea/v2@latest
go get charm.land/bubbles/v2@latest
go get charm.land/lipgloss/v2@latest
```

All three libraries must be upgraded together.

---

## 1. Import Paths

Replace all `github.com/charmbracelet` imports with `charm.land` vanity domains:

```
github.com/charmbracelet/bubbletea      →  charm.land/bubbletea/v2
github.com/charmbracelet/lipgloss       →  charm.land/lipgloss/v2
github.com/charmbracelet/bubbles/...    →  charm.land/bubbles/v2/...
```

**Search-and-replace patterns:**
```
github.com/charmbracelet/bubbletea   →  charm.land/bubbletea/v2
github.com/charmbracelet/lipgloss    →  charm.land/lipgloss/v2
github.com/charmbracelet/bubbles/    →  charm.land/bubbles/v2/
github.com/charmbracelet/bubbles     ���  charm.land/bubbles/v2
```

> **Note:** The `runeutil` and `memoization` packages are now internal and no longer importable.

---

## 2. Bubble Tea Core Changes

### 2a. View() Returns tea.View (Not string)

The biggest change. Only the **top-level model** returns `tea.View`. Child components continue returning `string`.

```go
// v1
func (m model) View() string {
    return m.renderContent()
}

// v2
func (m model) View() tea.View {
    v := tea.NewView(m.renderContent())
    v.AltScreen = true
    v.MouseMode = tea.MouseModeCellMotion
    v.ReportFocus = true
    v.WindowTitle = "My App"
    return v
}
```

The `tea.View` struct fields:
```go
type View struct {
    Content                   string
    OnMouse                   func(msg MouseMsg) Cmd
    Cursor                    *Cursor
    BackgroundColor           color.Color
    ForegroundColor           color.Color
    WindowTitle               string
    ProgressBar               *ProgressBar
    AltScreen                 bool
    ReportFocus               bool
    DisableBracketedPasteMode bool
    MouseMode                 MouseMode
    KeyboardEnhancements      KeyboardEnhancements
}
```

### 2b. Program Options Moved to Declarative View

These v1 program options are **removed** — set them in `View()` instead:

| v1 Program Option | v2 View Field |
|---|---|
| `tea.WithAltScreen()` | `v.AltScreen = true` |
| `tea.WithMouseCellMotion()` | `v.MouseMode = tea.MouseModeCellMotion` |
| `tea.WithMouseAllMotion()` | `v.MouseMode = tea.MouseModeAllMotion` |
| `tea.EnableMouseCellMotion` (cmd) | `v.MouseMode = tea.MouseModeCellMotion` |
| `tea.EnableMouseAllMotion` (cmd) | `v.MouseMode = tea.MouseModeAllMotion` |
| `tea.EnterAltScreen` (cmd) | `v.AltScreen = true` |
| `tea.ExitAltScreen` (cmd) | `v.AltScreen = false` |
| `tea.EnableReportFocus` (cmd) | `v.ReportFocus = true` |

Remaining valid program options:
- `tea.WithColorProfile(p)` — manually set color profile
- `tea.WithWindowSize(w, h)` — useful for testing
- `tea.WithInput(r)` / `tea.WithOutput(w)` — custom I/O

### 2c. Key Messages

`tea.KeyMsg` splits into `tea.KeyPressMsg` and `tea.KeyReleaseMsg`. Use `tea.KeyMsg` to match both.

```go
// v1
case tea.KeyMsg:
    switch msg.String() { ... }

// v2
case tea.KeyPressMsg:
    switch msg.String() { ... }
```

Key struct changes:
- `key.Type` + `key.Runes` replaced by `key.Code` + `key.Text`
- Modifiers in `key.Mod` (not separate booleans)
- Space bar returns `"space"` (not `" "`)
- New: `key.BaseCode` (US PC-101 layout), `key.IsRepeat`, `key.Keystroke()`

### 2d. Mouse Messages

Single `tea.MouseMsg` splits into four types:

```go
// v1
case tea.MouseMsg:
    switch msg.Type {
    case tea.MouseLeft: ...
    case tea.MouseWheelUp: ...
    }

// v2
case tea.MouseClickMsg:
    if msg.Button == tea.MouseLeft { ... }
case tea.MouseReleaseMsg:
    // ...
case tea.MouseWheelMsg:
    // ...
case tea.MouseMotionMsg:
    // ...
```

### 2e. Paste Messages

Paste events are now separate from key events:

```go
// v1: paste arrived as tea.KeyMsg with msg.Paste flag

// v2
case tea.PasteMsg:
    m.text += msg.Content
case tea.PasteStartMsg:
    // ...
case tea.PasteEndMsg:
    // ...
```

### 2f. Cursor Control

Control cursor position, color, and shape from View:

```go
func (m model) View() tea.View {
    var v tea.View
    v.SetContent("Hello, world!")
    if m.showCursor {
        v.Cursor = &tea.Cursor{
            Position: tea.Position{X: 14, Y: 0},
            Shape:    tea.CursorBlock,
            Blink:    true,
            Color:    lipgloss.Green,
        }
    }
    return v
}
```

### 2g. New v2 Features

**Keyboard enhancements** — detect and use progressive keyboard features:
```go
func (m model) View() tea.View {
    var v tea.View
    v.KeyboardEnhancements.ReportEventTypes = true  // key release events
    return v
}

// Detect support
case tea.KeyboardEnhancementsMsg:
    if msg.SupportsKeyDisambiguation() { ... }
```

**Clipboard** (OSC52, works over SSH):
```go
return m, tea.SetClipboard("text")
return m, tea.ReadClipboard()

case tea.ClipboardMsg:
    content := msg.String()
```

**Terminal colors** — query and set:
```go
func (m model) Init() tea.Cmd {
    return tea.Batch(
        tea.RequestBackgroundColor,
        tea.RequestForegroundColor,
    )
}

case tea.BackgroundColorMsg:
    m.isDark = msg.IsDark()
case tea.ForegroundColorMsg:
    // ...
```

**Environment variables** (critical for SSH apps):
```go
case tea.EnvMsg:
    m.term = msg.Getenv("TERM")  // client's, not server's
```

**Color profile detection:**
```go
case tea.ColorProfileMsg:
    m.colorProfile = msg.Profile
```

**Progress bar** (native):
```go
v.ProgressBar = tea.NewProgressBar(tea.ProgressBarDefault, 0.75)
```

**Raw escape sequences:**
```go
return m, tea.Raw(ansi.RequestPrimaryDeviceAttributes)
```

---

## 3. Lip Gloss v2 Changes

### 3a. AdaptiveColor Removed

`lipgloss.AdaptiveColor` no longer exists. Choose one of:

**Option 1: Bubble Tea integration (recommended)**
```go
case tea.BackgroundColorMsg:
    m.isDark = msg.IsDark()
    m.styles = newStyles(m.isDark)

func newStyles(isDark bool) styles {
    lightDark := lipgloss.LightDark(isDark)
    return styles{
        accent: lipgloss.NewStyle().Foreground(lightDark(
            lipgloss.Color("#874BFD"),
            lipgloss.Color("#7D56F4"),
        )),
    }
}
```

**Option 2: compat package (quick migration, not SSH-safe)**
```go
import "charm.land/lipgloss/v2/compat"

color := compat.AdaptiveColor{
    Light: lipgloss.Color("#f1f1f1"),
    Dark:  lipgloss.Color("#cccccc"),
}
```

**Option 3: Manual**
```go
h.Styles = help.DefaultDarkStyles()   // force dark
h.Styles = help.DefaultLightStyles()  // force light
```

### 3b. Colors are color.Color

`lipgloss.Color()` now returns `color.Color` (from `image/color`), not `lipgloss.TerminalColor`:

```go
// v1
type TerminalColor interface{ ... }
type Color string

// v2
func Color(string) color.Color
type RGBColor struct{ R, G, B uint8 }
```

### 3c. Color Downsampling

Built into Bubble Tea v2 automatically. For standalone Lip Gloss (no Bubble Tea):

```go
s := someStyle.Render("text")
lipgloss.Println(s)    // auto-downsample to stdout
lipgloss.Fprint(os.Stderr, s)  // auto-downsample to stderr
```

### 3d. Styles are Pure

Lip Gloss v2 styles no longer query I/O. Bubble Tea manages I/O and gives orders to Lip Gloss. No more lock-ups from I/O contention.

---

## 4. Bubbles v2 Component Changes

### Global Patterns

These patterns repeat across multiple components:

**a. `tea.KeyMsg` → `tea.KeyPressMsg`**
All Bubbles handle `tea.KeyPressMsg` in v2.

**b. Width/Height fields → getter/setter methods**
Affected: `filepicker`, `help`, `progress`, `table`, `textinput`, `viewport`
```go
// v1: m.Width = 40
// v2: m.SetWidth(40) / m.Width()
```

**c. `DefaultKeyMap` vars → functions**
Affected: `paginator`, `textarea`, `textinput`
```go
// v1: km := textinput.DefaultKeyMap
// v2: km := textinput.DefaultKeyMap()
```

**d. `NewModel` aliases removed**
Affected: `help`, `list`, `paginator`, `spinner`, `textinput`
```go
// v1: m := help.NewModel()
// v2: m := help.New()
```

**e. `DefaultStyles()` requires `isDark bool`**
Affected: `help`, `list`, `textarea`, `textinput`

### Per-Component Quick Reference

| Component | Key Changes |
|---|---|
| **cursor** | `Blink` → `IsBlinked`, `BlinkCmd()` → `Blink()` |
| **filepicker** | `DefaultStylesWithRenderer(r)` → `DefaultStyles()`, Height getter/setter |
| **help** | `NewModel` → `New()`, Width getter/setter, `DefaultStyles(isDark)` |
| **list** | `NewModel` → `New()`, `DefaultStyles(isDark)`, `NewDefaultItemStyles(isDark)`, FilterPrompt/FilterCursor consolidated into `Styles.Filter` |
| **paginator** | `NewModel` → `New()`, `DefaultKeyMap()` func, `UsePgUpPgDownKeys` etc. removed |
| **progress** | Width getter/setter, colors are `color.Color`, `WithGradient` → `WithColors`, `WithDefaultGradient` → `WithDefaultBlend` |
| **spinner** | `NewModel` → `New()`, `spinner.Tick()` → `model.Tick()` |
| **stopwatch** | `NewWithInterval(d)` → `New(WithInterval(d))` |
| **table** | Width/Height via getter/setter |
| **textarea** | `DefaultKeyMap()` func, `Style` → `StyleState`, `FocusedStyle`/`BlurredStyle` → `Styles.Focused`/`Styles.Blurred`, `SetCursor` → `SetCursorColumn`, `DefaultStyles(isDark)` |
| **textinput** | `NewModel` → `New()`, `DefaultKeyMap()` func, Width getter/setter, style fields → `Styles` struct, `CompletionStyle` → `Suggestion`, `Cursor` field → `Cursor()` method |
| **timer** | `NewWithInterval(t, i)` → `New(t, WithInterval(i))` |
| **viewport** | `New(w, h)` → `New(...Option)`, Width/Height/YOffset getter/setter, `HighPerformanceRendering` removed. New: `SoftWrap`, `LeftGutterFunc`, highlights, `SetContentLines`, `GetContent`, `FillHeight`, `StyleLineFunc`, horizontal scrolling |

---

## 5. Migration Checklist

### Phase 1: Imports
- [ ] Replace all `github.com/charmbracelet/bubbletea` with `charm.land/bubbletea/v2`
- [ ] Replace all `github.com/charmbracelet/lipgloss` with `charm.land/lipgloss/v2`
- [ ] Replace all `github.com/charmbracelet/bubbles/...` with `charm.land/bubbles/v2/...`
- [ ] Run `go get` for all three v2 packages
- [ ] Remove imports of `runeutil` (now internal)

### Phase 2: Core Bubble Tea
- [ ] Change top-level `View() string` to `View() tea.View`
- [ ] Move alt screen, mouse mode, focus reporting from program options to `View()` struct
- [ ] Replace `tea.KeyMsg` with `tea.KeyPressMsg` in all `Update` functions
- [ ] Replace `tea.MouseMsg` with split types (`MouseClickMsg`, `MouseWheelMsg`, etc.)
- [ ] Update paste handling if applicable

### Phase 3: Lip Gloss
- [ ] Replace `AdaptiveColor` with `isDark` pattern or `compat` package
- [ ] Add `tea.RequestBackgroundColor` to `Init()` if using light/dark styles
- [ ] Handle `tea.BackgroundColorMsg` in `Update()`
- [ ] Update any direct color type references (`TerminalColor` → `color.Color`)

### Phase 4: Bubbles Components
- [ ] Replace `NewModel()` calls with `New()`
- [ ] Replace `DefaultKeyMap` vars with `DefaultKeyMap()` func calls
- [ ] Replace direct Width/Height field access with getter/setter methods
- [ ] Update `DefaultStyles()` calls to pass `isDark bool`
- [ ] Update textarea style access (`FocusedStyle` → `Styles.Focused`)
- [ ] Update textinput style access (individual fields → `Styles` struct)
- [ ] Update viewport constructor to use functional options

### Phase 5: Verify
- [ ] Build compiles without errors
- [ ] Run tests
- [ ] Test in multiple terminals (especially keyboard enhancements)
- [ ] Verify light/dark mode works correctly

---

## 6. Removed Symbols Quick Reference

| Package | Removed | Replacement |
|---------|---------|-------------|
| bubbletea | `tea.WithAltScreen()` | `v.AltScreen = true` in View() |
| bubbletea | `tea.WithMouseCellMotion()` | `v.MouseMode = tea.MouseModeCellMotion` |
| bubbletea | `tea.KeyMsg` (for press only) | `tea.KeyPressMsg` |
| bubbletea | `tea.MouseMsg` | Split: `MouseClickMsg`, `MouseWheelMsg`, `MouseMotionMsg`, `MouseReleaseMsg` |
| lipgloss | `AdaptiveColor` | `LightDark(isDark)` or `compat.AdaptiveColor` |
| lipgloss | `TerminalColor` | `color.Color` |
| bubbles | `runeutil` package | Internalized (not importable) |
| bubbles | `*.NewModel()` | `*.New()` |
| bubbles | `DefaultKeyMap` (var) | `DefaultKeyMap()` (func) |
| bubbles | `*.Width`/`*.Height` (fields) | `SetWidth()`/`Width()`, `SetHeight()`/`Height()` |
| viewport | `New(w, h int)` | `New(...Option)` |
| viewport | `HighPerformanceRendering` | Removed entirely |
| progress | `WithGradient(a, b)` | `WithColors(colors...)` |
| progress | `WithDefaultGradient()` | `WithDefaultBlend()` |
| progress | `WithSolidFill(s)` | `WithColors(color)` |
| textarea | `Style` type | `StyleState` type |
| textarea | `FocusedStyle`/`BlurredStyle` | `Styles.Focused`/`Styles.Blurred` |
| textarea | `SetCursor(col)` | `SetCursorColumn(col)` |
| textinput | `PromptStyle` etc. | `Styles.Focused.Prompt` etc. |
| textinput | `CompletionStyle` | `StyleState.Suggestion` |
