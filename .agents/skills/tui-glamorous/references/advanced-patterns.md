# Advanced Charm Patterns

Deep-dive reference for production Charm applications.

---

## Table of Contents

- [Complete Bubble Tea App Template](#complete-bubble-tea-app-template)
- [Lip Gloss Layout Patterns](#lip-gloss-layout-patterns)
  - [Three-Column Dashboard](#three-column-dashboard)
  - [Modal Overlay](#modal-overlay)
  - [Status Bar](#status-bar)
- [Huh Advanced Forms](#huh-advanced-forms)
  - [Dynamic Options](#dynamic-options)
  - [Validation Chains](#validation-chains)
  - [Custom Themes](#custom-themes)
- [Harmonica Animation Recipes](#harmonica-animation-recipes)
- [Wish SSH App Patterns](#wish-ssh-app-patterns)
- [Testing Patterns](#testing-patterns)
- [Performance Tips](#performance-tips)

---

## Complete Bubble Tea App Template

```go
package main

import (
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/list"
    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────
// Theme (define once, use everywhere)
// ─────────────────────────────────────────────────────────────

var (
    subtle    = lipgloss.AdaptiveColor{Light: "236", Dark: "248"}
    highlight = lipgloss.AdaptiveColor{Light: "205", Dark: "212"}
    special   = lipgloss.AdaptiveColor{Light: "39", Dark: "86"}

    titleStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FFFDF5")).
        Background(lipgloss.Color("#7D56F4")).
        Padding(0, 1)

    infoStyle = lipgloss.NewStyle().
        BorderStyle(lipgloss.NormalBorder()).
        BorderTop(true).
        BorderForeground(subtle)

    docStyle = lipgloss.NewStyle().Padding(1, 2)
)

// ─────────────────────────────────────────────────────────────
// Model
// ─────────────────────────────────────────────────────────────

type screen int

const (
    screenLoading screen = iota
    screenMain
    screenDetail
)

type model struct {
    screen   screen
    width    int
    height   int

    // Components
    spinner  spinner.Model
    list     list.Model
    viewport viewport.Model

    // State
    loading  bool
    err      error
}

func initialModel() model {
    s := spinner.New()
    s.Spinner = spinner.Dot
    s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

    return model{
        screen:  screenLoading,
        spinner: s,
        loading: true,
    }
}

// ─────────────────────────────────────────────────────────────
// Messages
// ─────────────────────────────────────────────────────────────

type dataLoadedMsg struct {
    items []list.Item
}

type errMsg struct {
    err error
}

// ─────────────────────────────────────────────────────────────
// Commands
// ─────────────────────────────────────────────────────────────

func loadData() tea.Msg {
    // Simulate async data fetch
    // In reality: HTTP call, DB query, etc.
    items := []list.Item{
        item{title: "Item 1", desc: "Description 1"},
        item{title: "Item 2", desc: "Description 2"},
    }
    return dataLoadedMsg{items: items}
}

// ─────────────────────────────────────────────────────────────
// Lifecycle
// ─────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        loadData,
    )
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd
    var cmd tea.Cmd

    switch msg := msg.(type) {

    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        m.list.SetSize(m.width, m.height-4)
        m.viewport.Width = m.width
        m.viewport.Height = m.height - 6

    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "esc":
            if m.screen == screenDetail {
                m.screen = screenMain
            }
        case "enter":
            if m.screen == screenMain {
                m.screen = screenDetail
                // Load detail content into viewport
            }
        }

    case spinner.TickMsg:
        if m.loading {
            m.spinner, cmd = m.spinner.Update(msg)
            cmds = append(cmds, cmd)
        }

    case dataLoadedMsg:
        m.loading = false
        m.screen = screenMain
        m.list = list.New(msg.items, list.NewDefaultDelegate(), m.width, m.height-4)
        m.list.Title = "My Items"

    case errMsg:
        m.loading = false
        m.err = msg.err
    }

    // Delegate to active component
    switch m.screen {
    case screenMain:
        m.list, cmd = m.list.Update(msg)
        cmds = append(cmds, cmd)
    case screenDetail:
        m.viewport, cmd = m.viewport.Update(msg)
        cmds = append(cmds, cmd)
    }

    return m, tea.Batch(cmds...)
}

func (m model) View() string {
    if m.err != nil {
        return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
    }

    switch m.screen {
    case screenLoading:
        return fmt.Sprintf("\n\n   %s Loading...\n\n", m.spinner.View())
    case screenMain:
        return docStyle.Render(m.list.View())
    case screenDetail:
        return docStyle.Render(
            titleStyle.Render("Detail View") + "\n\n" +
            m.viewport.View() + "\n\n" +
            infoStyle.Render("↑/↓: scroll • esc: back • q: quit"),
        )
    }
    return ""
}

// ─────────────────────────────────────────────────────────────
// List item implementation
// ─────────────────────────────────────────────────────────────

type item struct {
    title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

// ─────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────

func main() {
    // Enable debug logging to file
    if os.Getenv("DEBUG") != "" {
        f, _ := tea.LogToFile("debug.log", "debug")
        defer f.Close()
    }

    p := tea.NewProgram(
        initialModel(),
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )

    if _, err := p.Run(); err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        os.Exit(1)
    }
}
```

---

## Lip Gloss Layout Patterns

### Three-Column Dashboard

```go
func (m model) View() string {
    // Calculate column widths
    sidebarWidth := 25
    mainWidth := m.width - sidebarWidth*2 - 4  // -4 for borders

    // Style definitions
    sidebarStyle := lipgloss.NewStyle().
        Width(sidebarWidth).
        Height(m.height - 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(subtle)

    mainStyle := lipgloss.NewStyle().
        Width(mainWidth).
        Height(m.height - 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(highlight)

    // Render columns
    leftSidebar := sidebarStyle.Render(m.nav.View())
    mainContent := mainStyle.Render(m.content.View())
    rightSidebar := sidebarStyle.Render(m.details.View())

    // Join horizontally
    return lipgloss.JoinHorizontal(lipgloss.Top,
        leftSidebar,
        mainContent,
        rightSidebar,
    )
}
```

### Modal Overlay

```go
func (m model) View() string {
    // Base content
    base := m.mainContent.View()

    if !m.showModal {
        return base
    }

    // Modal dimensions
    modalWidth := 60
    modalHeight := 20

    // Modal style
    modalStyle := lipgloss.NewStyle().
        Width(modalWidth).
        Height(modalHeight).
        Border(lipgloss.DoubleBorder()).
        BorderForeground(lipgloss.Color("205")).
        Padding(1, 2)

    modal := modalStyle.Render(m.modalContent)

    // Center modal in viewport
    modalX := (m.width - modalWidth) / 2
    modalY := (m.height - modalHeight) / 2

    // Overlay (Lip Gloss v2 has native overlay; v1 uses string manipulation)
    return placeOverlay(modalX, modalY, modal, base)
}

// Simple overlay for v1 (v2 has lipgloss.Overlay)
func placeOverlay(x, y int, overlay, base string) string {
    return lipgloss.Place(
        lipgloss.Width(base),
        lipgloss.Height(base),
        lipgloss.Center, lipgloss.Center,
        overlay,
    )
}
```

### Status Bar

```go
func statusBar(width int, mode string, filename string, modified bool) string {
    modeStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FFFDF5")).
        Background(lipgloss.Color("#FF5F87")).
        Padding(0, 1)

    fileStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FFFDF5")).
        Background(lipgloss.Color("#6124DF")).
        Padding(0, 1)

    infoStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FFFDF5")).
        Background(lipgloss.Color("#A550DF")).
        Padding(0, 1)

    modeStr := modeStyle.Render(mode)

    name := filename
    if modified {
        name += " [+]"
    }
    fileStr := fileStyle.Render(name)

    info := infoStyle.Render("UTF-8 | LF")

    // Calculate gap
    gap := width - lipgloss.Width(modeStr) - lipgloss.Width(fileStr) - lipgloss.Width(info)
    if gap < 0 {
        gap = 0
    }

    return modeStr + strings.Repeat(" ", gap) + fileStr + info
}
```

---

## Huh Advanced Forms

### Dynamic Options

```go
var (
    country string
    state   string
)

form := huh.NewForm(
    huh.NewGroup(
        huh.NewSelect[string]().
            Title("Country").
            Options(
                huh.NewOption("USA", "us"),
                huh.NewOption("Canada", "ca"),
            ).
            Value(&country),

        huh.NewSelect[string]().
            Title("State/Province").
            OptionsFunc(func() []huh.Option[string] {
                switch country {
                case "us":
                    return huh.NewOptions("California", "New York", "Texas")
                case "ca":
                    return huh.NewOptions("Ontario", "Quebec", "BC")
                default:
                    return nil
                }
            }, &country). // Re-evaluate when country changes
            Value(&state),
    ),
)
```

### Validation Chains

```go
huh.NewInput().
    Title("Email").
    Validate(func(s string) error {
        if s == "" {
            return fmt.Errorf("email required")
        }
        if !strings.Contains(s, "@") {
            return fmt.Errorf("invalid email format")
        }
        if !strings.HasSuffix(s, ".com") && !strings.HasSuffix(s, ".org") {
            return fmt.Errorf("must be .com or .org")
        }
        return nil
    }).
    Value(&email)
```

### Custom Themes

```go
theme := huh.ThemeBase()
theme.Focused.Title = theme.Focused.Title.Foreground(lipgloss.Color("205"))
theme.Focused.Description = theme.Focused.Description.Foreground(lipgloss.Color("240"))

form.WithTheme(theme)
```

---

## Harmonica Animation Recipes

### Smooth Scroll Position

```go
type model struct {
    scrollY       float64
    scrollVel     float64
    targetScrollY float64
    spring        harmonica.Spring
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "j", "down":
            m.targetScrollY += 3  // Scroll down
            return m, tick()
        case "k", "up":
            m.targetScrollY -= 3  // Scroll up
            return m, tick()
        }

    case tickMsg:
        m.scrollY, m.scrollVel = m.spring.Update(m.scrollY, m.scrollVel, m.targetScrollY)

        // Stop ticking when settled
        if math.Abs(m.scrollY-m.targetScrollY) < 0.01 && math.Abs(m.scrollVel) < 0.01 {
            return m, nil
        }
        return m, tick()
    }
    return m, nil
}
```

### Progress Bar with Overshoot

```go
type model struct {
    progress    float64
    progressVel float64
    target      float64
    spring      harmonica.Spring
}

func newModel() model {
    // Under-damped spring for bounce effect
    return model{
        spring: harmonica.NewSpring(harmonica.FPS(60), 8.0, 0.3),
    }
}

func (m model) View() string {
    percent := m.progress / 100.0
    if percent > 1 {
        percent = 1  // Clamp for render (but physics can overshoot)
    }
    if percent < 0 {
        percent = 0
    }
    return progressBar.ViewAs(percent)
}
```

### Cursor Position Animation

```go
type model struct {
    cursorX, cursorXVel float64
    cursorY, cursorYVel float64
    targetX, targetY    float64
    spring              harmonica.Spring
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg.(type) {
    case tickMsg:
        m.cursorX, m.cursorXVel = m.spring.Update(m.cursorX, m.cursorXVel, m.targetX)
        m.cursorY, m.cursorYVel = m.spring.Update(m.cursorY, m.cursorYVel, m.targetY)
        return m, tick()
    }
    return m, nil
}

func (m model) View() string {
    // Round to integer for terminal position
    x := int(math.Round(m.cursorX))
    y := int(math.Round(m.cursorY))
    return placeCursor(x, y, m.content)
}
```

---

## Wish SSH App Patterns

### Per-User State

```go
func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
    pty, _, _ := s.Pty()

    // User-specific initialization
    user := s.User()
    pubKey := s.PublicKey()

    // Load user's saved state (from DB, file, etc.)
    savedState := loadUserState(user)

    return model{
        user:   user,
        pubKey: pubKey,
        width:  pty.Window.Width,
        height: pty.Window.Height,
        state:  savedState,
    }, []tea.ProgramOption{tea.WithAltScreen()}
}
```

### Multi-Room Chat

```go
type server struct {
    rooms map[string]*room
    mu    sync.RWMutex
}

type room struct {
    name    string
    clients map[string]*client
    msgs    chan message
}

func (srv *server) middleware() wish.Middleware {
    return func(next ssh.Handler) ssh.Handler {
        return func(s ssh.Session) {
            roomName := s.Command()[0] // e.g., ssh server join #general
            if roomName == "" {
                roomName = "lobby"
            }

            room := srv.getOrCreateRoom(roomName)
            client := room.addClient(s.User(), s)

            p := tea.NewProgram(
                chatModel{room: room, client: client},
                tea.WithInput(s),
                tea.WithOutput(s),
                tea.WithAltScreen(),
            )
            p.Run()

            room.removeClient(client)
        }
    }
}
```

### Rate Limiting Middleware

```go
func rateLimitMiddleware(rps float64) wish.Middleware {
    limiter := rate.NewLimiter(rate.Limit(rps), int(rps))

    return func(next ssh.Handler) ssh.Handler {
        return func(s ssh.Session) {
            if !limiter.Allow() {
                wish.Println(s, "Rate limited. Try again later.")
                return
            }
            next(s)
        }
    }
}
```

---

## Testing Patterns

### Headless Bubble Tea Tests

```go
func TestApp(t *testing.T) {
    m := initialModel()

    // Simulate window size
    m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

    // Simulate keypress
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

    // Assert state
    model := m.(model)
    if model.cursor != 1 {
        t.Errorf("expected cursor=1, got %d", model.cursor)
    }

    // Assert view contains expected content
    view := m.View()
    if !strings.Contains(view, "Expected Text") {
        t.Errorf("view missing expected content")
    }
}
```

### Golden File Tests for Views

```go
func TestView_GoldenFile(t *testing.T) {
    m := model{
        items: []string{"A", "B", "C"},
        cursor: 1,
    }

    got := m.View()

    golden := filepath.Join("testdata", "view.golden")
    if os.Getenv("UPDATE_GOLDEN") != "" {
        os.WriteFile(golden, []byte(got), 0644)
    }

    want, _ := os.ReadFile(golden)
    if got != string(want) {
        t.Errorf("view mismatch:\n%s", diff(string(want), got))
    }
}
```

---

## Performance Tips

1. **Minimize allocations in View()**: Pre-allocate strings.Builder
2. **Cache Glamour output**: Render markdown once, not every frame
3. **Batch component updates**: Single tea.Batch, not multiple returns
4. **Use viewport for long content**: Don't render off-screen lines
5. **Profile with pprof**: `go tool pprof http://localhost:6060/debug/pprof/profile`

```go
// Cache expensive renders
type model struct {
    cachedMarkdown string
    markdownDirty  bool
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case contentChangedMsg:
        m.markdownDirty = true
    }
    return m, nil
}

func (m model) View() string {
    if m.markdownDirty {
        m.cachedMarkdown, _ = glamour.Render(m.content, "dark")
        m.markdownDirty = false
    }
    return m.cachedMarkdown
}
```
