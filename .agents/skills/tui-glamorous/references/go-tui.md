# Go TUI Development with Charm

Building terminal user interfaces in Go with Bubble Tea ecosystem.

---

## Table of Contents

- [The 5-Minute TUI](#the-5-minute-tui)
- [Core Architecture: The Elm Pattern](#core-architecture-the-elm-pattern)
- [UI Pattern Recipes](#ui-pattern-recipes)
  - [Command Palette](#command-palette-fuzzy-search)
  - [Confirmation Dialog](#confirmation-dialog)
  - [Split Pane Layout](#split-pane-layout)
  - [Toast/Notification](#toastnotification)
  - [Progress with Details](#progress-with-details)
  - [Tab Navigation](#tab-navigation)
  - [Error Display](#error-display)
- [Library Quick Reference](#library-quick-reference)
- [Progressive Enhancement Path](#progressive-enhancement-path)
- [Production Hardening](#production-hardening)
- [Debugging TUIs](#debugging-tuis)
- [Anti-Patterns](#anti-patterns)
- [When NOT to Use Full TUI](#when-not-to-use-full-tui)
- [THE EXACT PROMPTS](#the-exact-prompts)

---

## The 5-Minute TUI

Copy this, modify the items, ship it:

```go
package main

import (
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

var (
    selected = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
    normal   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
    title    = lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(lipgloss.Color("62"))
)

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
            if m.cursor > 0 {
                m.cursor--
            }
        case "down", "j":
            if m.cursor < len(m.items)-1 {
                m.cursor++
            }
        case "enter":
            fmt.Printf("\nYou chose: %s\n", m.items[m.cursor])
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m model) View() string {
    s := title.Render("Select an item") + "\n\n"
    for i, item := range m.items {
        cursor := "  "
        style := normal
        if m.cursor == i {
            cursor = "▸ "
            style = selected
        }
        s += cursor + style.Render(item) + "\n"
    }
    s += "\n" + normal.Render("↑/↓: move • enter: select • q: quit")
    return s
}

func main() {
    m := model{items: []string{"Option A", "Option B", "Option C"}}
    if _, err := tea.NewProgram(m).Run(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

**Run:** `go mod init example && go get github.com/charmbracelet/bubbletea github.com/charmbracelet/lipgloss && go run .`

---

## Core Architecture: The Elm Pattern

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│    Model    │───▸│   Update    │───▸│    View     │
│  (state)    │    │  (logic)    │    │  (render)   │
└─────────────┘    └─────────────┘    └─────────────┘
       ▲                  │
       │                  │
       └──────────────────┘
              Msg (events)
```

**Model:** All state in one struct. Width, height, cursor, data, error, loading...

**Update:** Pure function. `(model, msg) → (model, cmd)`. Never blocks. Never mutates.

**View:** Pure function. `model → string`. No side effects. Just render.

**Cmd:** Async work. Returns a Msg when done. HTTP calls, file I/O, timers...

```go
type model struct {
    width, height int      // Terminal size
    state         screen   // Current screen
    err           error    // Last error
    loading       bool     // Loading state
    // ... your data
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        loadInitialData,     // Async data fetch
        m.spinner.Tick,      // Start spinner
    )
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // ALWAYS handle these first
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        // Resize all components here
    case tea.KeyMsg:
        if msg.String() == "ctrl+c" {
            return m, tea.Quit
        }
    case errMsg:
        m.err = msg.err
        m.loading = false
    }
    // Then delegate to current screen/components
    return m, nil
}

func (m model) View() string {
    if m.err != nil {
        return renderError(m.err)
    }
    if m.loading {
        return m.spinner.View() + " Loading..."
    }
    return m.renderCurrentScreen()
}
```

---

## UI Pattern Recipes

### Command Palette (Fuzzy Search)

```go
items := []list.Item{
    item{title: "New File", key: "ctrl+n"},
    item{title: "Open File", key: "ctrl+o"},
    item{title: "Save", key: "ctrl+s"},
}
l := list.New(items, list.NewDefaultDelegate(), 40, 14)
l.Title = "Commands"
l.SetShowStatusBar(false)
l.SetFilteringEnabled(true)  // Built-in fuzzy search!
l.Styles.Title = titleStyle
```

### Confirmation Dialog

```go
// With Huh (simplest)
var confirm bool
huh.NewConfirm().
    Title("Delete all files?").
    Description("This cannot be undone.").
    Affirmative("Yes, delete").
    Negative("Cancel").
    Value(&confirm).
    Run()

// Or styled with Lip Gloss
dialogStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("205")).
    Padding(1, 2).
    Width(40)

dialog := dialogStyle.Render(
    titleStyle.Render("⚠️  Confirm Delete") + "\n\n" +
    "This will delete 42 files.\n\n" +
    "[Y]es  [N]o",
)
```

### Split Pane Layout

```go
func (m model) View() string {
    sideW := 30
    mainW := m.width - sideW - 3  // -3 for border

    sideStyle := lipgloss.NewStyle().
        Width(sideW).
        Height(m.height - 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("240"))

    mainStyle := lipgloss.NewStyle().
        Width(mainW).
        Height(m.height - 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("62"))

    side := sideStyle.Render(m.sidebar.View())
    main := mainStyle.Render(m.content.View())

    return lipgloss.JoinHorizontal(lipgloss.Top, side, main)
}
```

### Toast/Notification

```go
type model struct {
    toast       string
    toastTimer  int
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case successMsg:
        m.toast = "✓ " + string(msg)
        m.toastTimer = 30  // frames
        return m, tick()
    case tickMsg:
        if m.toastTimer > 0 {
            m.toastTimer--
            return m, tick()
        }
        m.toast = ""
    }
    return m, nil
}

func (m model) View() string {
    view := m.mainContent()
    if m.toast != "" {
        toast := lipgloss.NewStyle().
            Background(lipgloss.Color("35")).
            Foreground(lipgloss.Color("255")).
            Padding(0, 2).
            Render(m.toast)
        view = lipgloss.Place(m.width, m.height, lipgloss.Right, lipgloss.Top, toast)
    }
    return view
}
```

### Progress with Details

```go
type model struct {
    progress progress.Model
    current  string
    done     int
    total    int
}

func (m model) View() string {
    pct := float64(m.done) / float64(m.total)

    return lipgloss.JoinVertical(lipgloss.Left,
        titleStyle.Render("Installing dependencies"),
        "",
        m.progress.ViewAs(pct),
        "",
        subtle.Render(fmt.Sprintf("(%d/%d) %s", m.done, m.total, m.current)),
    )
}
```

### Tab Navigation

```go
type model struct {
    tabs      []string
    activeTab int
}

func (m model) View() string {
    var renderedTabs []string
    for i, t := range m.tabs {
        style := inactiveTab
        if i == m.activeTab {
            style = activeTab
        }
        renderedTabs = append(renderedTabs, style.Render(t))
    }

    tabRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
    content := m.tabContent[m.activeTab].View()

    return lipgloss.JoinVertical(lipgloss.Left, tabRow, content)
}

var (
    activeTab = lipgloss.NewStyle().
        Bold(true).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("62")).
        Padding(0, 2)
    inactiveTab = lipgloss.NewStyle().
        Border(lipgloss.HiddenBorder()).
        Padding(0, 2)
)
```

### Error Display

```go
func renderError(err error) string {
    errStyle := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("196")).
        Padding(1, 2).
        Width(60)

    titleStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color("196")).
        Bold(true)

    return errStyle.Render(
        titleStyle.Render("✗ Error") + "\n\n" +
        wordwrap.String(err.Error(), 56) + "\n\n" +
        subtle.Render("Press any key to continue"),
    )
}
```

---

## Library Quick Reference

| Library | Purpose | Key Types |
|---------|---------|-----------|
| **Bubble Tea** | TUI framework | `tea.Model`, `tea.Cmd`, `tea.Msg` |
| **Bubbles** | Components | `list.Model`, `textinput.Model`, `viewport.Model`, `table.Model`, `spinner.Model`, `progress.Model` |
| **Lip Gloss** | Styling | `lipgloss.Style`, `lipgloss.Color`, `lipgloss.Border` |
| **Huh** | Forms | `huh.Form`, `huh.Input`, `huh.Select`, `huh.Confirm` |
| **Glamour** | Markdown | `glamour.Render()`, `glamour.NewTermRenderer()` |
| **Harmonica** | Animation | `harmonica.Spring`, `harmonica.FPS()` |
| **Log** | Logging | `log.Info()`, `log.Error()` |

**Install:**
```bash
go get github.com/charmbracelet/bubbletea@latest \
       github.com/charmbracelet/bubbles@latest \
       github.com/charmbracelet/lipgloss@latest \
       github.com/charmbracelet/huh@latest \
       github.com/charmbracelet/glamour@latest \
       github.com/charmbracelet/harmonica@latest \
       github.com/charmbracelet/log@latest
```

**v2 Track (bleeding edge):**
```bash
go get charm.land/bubbletea/v2@latest
go get charm.land/bubbles/v2@latest
go get charm.land/lipgloss/v2@latest
```

---

## Progressive Enhancement Path

### Level 1: Styled Output

Replace `fmt.Println` with Lip Gloss:

```go
// Before
fmt.Println("Error: file not found")

// After
errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
fmt.Println(errStyle.Render("Error: file not found"))
```

### Level 2: Interactive Prompts

Replace `fmt.Scanf` with Huh:

```go
// Before
fmt.Print("Enter name: ")
fmt.Scanf("%s", &name)

// After
huh.NewInput().Title("Enter name").Value(&name).Run()
```

### Level 3: Full TUI

Convert to Bubble Tea with components.

### Level 4: Polish

Add animation, mouse support, themes, help system...

---

## Production Hardening

### Must-Have Checklist

```
□ Handle tea.WindowSizeMsg (responsive layout)
□ Handle ctrl+c gracefully (cleanup, restore terminal)
□ Log to file, not stdout (use tea.LogToFile)
□ Test with small terminals (80x24 minimum)
□ Test with no color (TERM=dumb, NO_COLOR=1)
□ Test with light AND dark backgrounds
□ Add --no-tui or --plain flag for scripting
□ Handle errors visually (don't just crash)
□ Show loading states for async operations
□ Include keyboard hints (help component)
```

### Optional but Impressive

```
□ Mouse support (WithMouseCellMotion)
□ Focus reporting (pause when backgrounded)
□ Alt screen (full-window mode)
□ Smooth animations (Harmonica springs)
□ Accessible mode (screen reader support)
□ Custom themes
□ Config file for preferences
□ VHS tape for README demo
```

---

## Debugging TUIs

### 1. File Logging

```go
if os.Getenv("DEBUG") != "" {
    f, _ := tea.LogToFile("debug.log", "debug")
    defer f.Close()
}

log.Printf("cursor moved to %d", m.cursor)
```

Run: `DEBUG=1 go run . 2>&1 | tee debug.log`
Watch: `tail -f debug.log`

### 2. Debug View Mode

```go
func (m model) View() string {
    view := m.normalView()

    if m.debug {
        debug := fmt.Sprintf(
            "w=%d h=%d cursor=%d state=%v",
            m.width, m.height, m.cursor, m.state,
        )
        view += "\n" + lipgloss.NewStyle().
            Foreground(lipgloss.Color("240")).
            Render(debug)
    }
    return view
}
```

### 3. Message Tracing

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    log.Printf("msg: %T %+v", msg, msg)
    // ... rest of update
}
```

### 4. Panic Recovery

```go
func main() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Fprintf(os.Stderr, "panic: %v\n%s", r, debug.Stack())
        }
    }()
    // ...
}
```

---

## Anti-Patterns

| Anti-Pattern | Why Bad | Fix |
|--------------|---------|-----|
| Blocking in Update | Freezes entire UI | Use commands for I/O |
| Ignoring WindowSizeMsg | Broken layout on resize | Always handle, resize components |
| Logging to stdout | Corrupts TUI display | Log to file |
| Hardcoded dimensions | Breaks on different terminals | Calculate from WindowSizeMsg |
| Mutating model directly | Unpredictable state | Return new model from Update |
| Deeply nested Views | Hard to maintain | Extract render functions |
| One giant Update switch | Unmaintainable | Delegate to screen/component handlers |
| Raw ANSI codes | Won't adapt to terminal | Use Lip Gloss |
| Manual prompt loops | Reinventing Huh poorly | Use Huh forms |

---

## When NOT to Use Full TUI

Charm adds complexity. Skip full Bubble Tea when:

- **Output is piped:** `mytool | grep foo` — use plain text
- **No interaction needed:** Pure data transformation — just print
- **CI/CD scripts:** Headless environments — use flags/env vars
- **Very simple prompts:** One yes/no — use Huh standalone

**Escape hatch pattern:**

```go
func main() {
    if !term.IsTerminal(int(os.Stdin.Fd())) || os.Getenv("NO_TUI") != "" {
        runPlainMode()
        return
    }
    runTUI()
}
```

---

## THE EXACT PROMPTS

### "Make My CLI Glamorous"

```
I have a Go CLI tool that currently uses fmt.Println and flag parsing.
Transform it into a polished TUI using Charmbracelet libraries:

1. Replace all fmt.Println output with Lip Gloss styled text
2. Replace any user prompts with Huh forms or Bubbles inputs
3. Add a proper help screen using Glamour for markdown rendering
4. Add keyboard navigation with clear visual feedback
5. Handle terminal resize gracefully
6. Add a loading spinner for any async operations
7. Use the alt screen for full-window mode

Preserve all existing functionality while dramatically improving UX.
```

### "Build a TUI Dashboard"

```
Create a terminal dashboard using Charmbracelet that displays:
- A header with app name and status
- A sidebar with navigation (list component)
- A main content area (viewport for scrolling)
- A footer with keyboard hints (help component)

Requirements:
- Responsive to terminal resize
- Mouse support for clicking items
- Smooth transitions when switching views
- Proper focus management between panes
- Clean exit behavior (restore terminal state)

Use Bubble Tea for state, Bubbles for components, Lip Gloss for layout.
```

### "Add Charm to Existing CLI"

```
I have an existing CLI using [cobra/urfave/flag]. Add Charm polish:

1. Keep the existing command structure
2. Add interactive mode when run without args
3. Style all output with Lip Gloss
4. Add progress bars for long operations
5. Add confirmation prompts for destructive actions
6. Show errors in styled error boxes
7. Add --no-tui flag to disable for scripting

Show me how to integrate without breaking existing behavior.
```
