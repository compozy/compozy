# Bubbles Component Catalog

Quick reference for all Bubbles components with key APIs and patterns.

---

## Table of Contents

- [Text Input](#text-input)
- [Text Area](#text-area)
- [List](#list)
- [Table](#table)
- [Viewport](#viewport)
- [Spinner](#spinner)
- [Progress](#progress)
- [File Picker](#file-picker)
- [Paginator](#paginator)
- [Help](#help)
- [Timer](#timer)
- [Stopwatch](#stopwatch)
- [Key Bindings](#key-bindings)

---

## Text Input

**Package:** `github.com/charmbracelet/bubbles/textinput`

```go
import "github.com/charmbracelet/bubbles/textinput"

ti := textinput.New()
ti.Placeholder = "Type here..."
ti.Focus()
ti.CharLimit = 156
ti.Width = 40

// Styling
ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))

// Password mode
ti.EchoMode = textinput.EchoPassword
ti.EchoCharacter = '•'

// In Update
ti, cmd = ti.Update(msg)

// Get value
value := ti.Value()

// Reset
ti.SetValue("")
ti.Reset()
```

**Key Methods:**
- `Focus()` / `Blur()` - Control focus state
- `Value()` / `SetValue(string)` - Get/set text
- `CharLimit` - Max characters
- `Width` - Display width
- `EchoMode` - Normal, Password, or None

---

## Text Area

**Package:** `github.com/charmbracelet/bubbles/textarea`

```go
import "github.com/charmbracelet/bubbles/textarea"

ta := textarea.New()
ta.Placeholder = "Write something..."
ta.Focus()
ta.SetWidth(80)
ta.SetHeight(10)
ta.CharLimit = 1000

// Line numbers
ta.ShowLineNumbers = true

// Styling
ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color("236"))

// In Update
ta, cmd = ta.Update(msg)

// Get value
value := ta.Value()

// Set value
ta.SetValue("Initial content\nLine 2")
```

**Key Methods:**
- `SetWidth(int)` / `SetHeight(int)` - Dimensions
- `Value()` / `SetValue(string)` - Content
- `Line()` / `LineCount()` - Current line info
- `CursorDown()` / `CursorUp()` - Programmatic cursor movement

---

## List

**Package:** `github.com/charmbracelet/bubbles/list`

```go
import "github.com/charmbracelet/bubbles/list"

// Define item type implementing list.Item interface
type item struct {
    title, desc string
}
func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

// Create list
items := []list.Item{
    item{"First", "Description 1"},
    item{"Second", "Description 2"},
}

l := list.New(items, list.NewDefaultDelegate(), 40, 20)
l.Title = "My List"
l.SetShowStatusBar(true)
l.SetFilteringEnabled(true)

// Styling
l.Styles.Title = lipgloss.NewStyle().
    Foreground(lipgloss.Color("205")).
    Bold(true)

// In Update
l, cmd = l.Update(msg)

// Get selected item
if i, ok := l.SelectedItem().(item); ok {
    // Use i
}

// Update items
l.SetItems(newItems)
```

**Key Methods:**
- `SelectedItem()` - Get current selection
- `Index()` - Current index
- `SetItems([]list.Item)` - Replace items
- `InsertItem(index, item)` - Add item
- `RemoveItem(index)` - Remove item
- `SetSize(w, h)` - Dimensions
- `SetFilteringEnabled(bool)` - Toggle fuzzy filter

**Delegate Customization:**
```go
d := list.NewDefaultDelegate()
d.Styles.SelectedTitle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("205")).
    Bold(true)
d.Styles.SelectedDesc = lipgloss.NewStyle().
    Foreground(lipgloss.Color("240"))

l := list.New(items, d, width, height)
```

---

## Table

**Package:** `github.com/charmbracelet/bubbles/table`

```go
import "github.com/charmbracelet/bubbles/table"

columns := []table.Column{
    {Title: "ID", Width: 4},
    {Title: "Name", Width: 20},
    {Title: "Status", Width: 10},
}

rows := []table.Row{
    {"1", "Alice", "Active"},
    {"2", "Bob", "Inactive"},
}

t := table.New(
    table.WithColumns(columns),
    table.WithRows(rows),
    table.WithFocused(true),
    table.WithHeight(10),
)

// Styling
s := table.DefaultStyles()
s.Header = s.Header.
    BorderStyle(lipgloss.NormalBorder()).
    BorderForeground(lipgloss.Color("240")).
    BorderBottom(true).
    Bold(true)
s.Selected = s.Selected.
    Foreground(lipgloss.Color("229")).
    Background(lipgloss.Color("57")).
    Bold(false)
t.SetStyles(s)

// In Update
t, cmd = t.Update(msg)

// Get selected row
row := t.SelectedRow()

// Update data
t.SetRows(newRows)
t.SetColumns(newColumns)
```

**Key Methods:**
- `SelectedRow()` - Get selected row data
- `Cursor()` - Current row index
- `SetRows([]table.Row)` - Replace rows
- `SetWidth(int)` / `SetHeight(int)` - Dimensions
- `Focus()` / `Blur()` - Focus control
- `GotoTop()` / `GotoBottom()` - Navigation

---

## Viewport

**Package:** `github.com/charmbracelet/bubbles/viewport`

```go
import "github.com/charmbracelet/bubbles/viewport"

vp := viewport.New(80, 20)
vp.SetContent(longContent)

// Mouse wheel scrolling
vp.MouseWheelEnabled = true
vp.MouseWheelDelta = 3

// Styling
vp.Style = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("240"))

// In Update
vp, cmd = vp.Update(msg)

// Programmatic scroll
vp.GotoTop()
vp.GotoBottom()
vp.LineDown(5)
vp.LineUp(5)
vp.HalfViewDown()
vp.HalfViewUp()

// Scroll info
percent := vp.ScrollPercent()
atTop := vp.AtTop()
atBottom := vp.AtBottom()
```

**Key Methods:**
- `SetContent(string)` - Set scrollable content
- `Width` / `Height` - Dimensions (set directly)
- `YOffset` - Current scroll position
- `ScrollPercent()` - 0.0-1.0 scroll progress
- `AtTop()` / `AtBottom()` - Boundary checks

**Pattern: Glamour + Viewport**
```go
md, _ := glamour.Render(markdownContent, "dark")
vp.SetContent(md)
```

---

## Spinner

**Package:** `github.com/charmbracelet/bubbles/spinner`

```go
import "github.com/charmbracelet/bubbles/spinner"

s := spinner.New()
s.Spinner = spinner.Dot  // See presets below
s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

// In Init (start spinning)
func (m model) Init() tea.Cmd {
    return m.spinner.Tick
}

// In Update
case spinner.TickMsg:
    var cmd tea.Cmd
    m.spinner, cmd = m.spinner.Update(msg)
    return m, cmd
```

**Spinner Presets:**
- `spinner.Line`
- `spinner.Dot`
- `spinner.MiniDot`
- `spinner.Jump`
- `spinner.Pulse`
- `spinner.Points`
- `spinner.Globe`
- `spinner.Moon`
- `spinner.Monkey`
- `spinner.Meter`
- `spinner.Hamburger`

**Custom Spinner:**
```go
s.Spinner = spinner.Spinner{
    Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
    FPS:    time.Second / 10,
}
```

---

## Progress

**Package:** `github.com/charmbracelet/bubbles/progress`

```go
import "github.com/charmbracelet/bubbles/progress"

// Gradient style
p := progress.New(progress.WithDefaultGradient())

// Solid color
p := progress.New(progress.WithSolidFill("#7D56F4"))

// Custom gradient
p := progress.New(progress.WithGradient("#7D56F4", "#FF5F87"))

// Width
p.Width = 40

// Without percentage display
p.ShowPercentage = false

// Render at percentage (0.0-1.0)
view := p.ViewAs(0.75)

// Or set percent and use View()
p.SetPercent(0.75)
view := p.View()
```

**Animation Pattern:**
```go
type model struct {
    progress progress.Model
    percent  float64
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case progress.FrameMsg:
        progressModel, cmd := m.progress.Update(msg)
        m.progress = progressModel.(progress.Model)
        return m, cmd
    case downloadProgressMsg:
        m.percent = msg.percent
        return m, m.progress.SetPercent(m.percent)
    }
    return m, nil
}
```

---

## File Picker

**Package:** `github.com/charmbracelet/bubbles/filepicker`

```go
import "github.com/charmbracelet/bubbles/filepicker"

fp := filepicker.New()
fp.AllowedTypes = []string{".txt", ".md", ".go"}
fp.CurrentDirectory, _ = os.UserHomeDir()
fp.ShowHidden = false
fp.ShowSize = true
fp.ShowPermissions = false

// Styling
fp.Styles.Selected = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

// In Init
func (m model) Init() tea.Cmd {
    return m.filepicker.Init()
}

// In Update
fp, cmd = fp.Update(msg)

// Check for selection
if didSelect, path := fp.DidSelectFile(msg); didSelect {
    // User selected path
}

if didSelect, path := fp.DidSelectDisabledFile(msg); didSelect {
    // User tried to select disallowed file type
}
```

**Key Properties:**
- `AllowedTypes` - Whitelist extensions
- `CurrentDirectory` - Starting directory
- `ShowHidden` - Show dotfiles
- `DirAllowed` - Allow directory selection
- `FileAllowed` - Allow file selection

---

## Paginator

**Package:** `github.com/charmbracelet/bubbles/paginator`

```go
import "github.com/charmbracelet/bubbles/paginator"

p := paginator.New()
p.Type = paginator.Dots    // or paginator.Arabic
p.PerPage = 10
p.SetTotalPages(len(items) / p.PerPage)

// Styling
p.ActiveDot = lipgloss.NewStyle().
    Foreground(lipgloss.Color("205")).
    Render("●")
p.InactiveDot = lipgloss.NewStyle().
    Foreground(lipgloss.Color("240")).
    Render("○")

// In Update
p, cmd = p.Update(msg)

// Navigation
p.NextPage()
p.PrevPage()
p.Page = 3  // Jump to page

// Get items for current page
start, end := p.GetSliceBounds(len(items))
pageItems := items[start:end]
```

**Render:**
```go
func (m model) View() string {
    start, end := m.paginator.GetSliceBounds(len(m.items))

    var b strings.Builder
    for _, item := range m.items[start:end] {
        b.WriteString(item.Render())
        b.WriteRune('\n')
    }

    b.WriteString("\n")
    b.WriteString(m.paginator.View())

    return b.String()
}
```

---

## Help

**Package:** `github.com/charmbracelet/bubbles/help`

```go
import (
    "github.com/charmbracelet/bubbles/help"
    "github.com/charmbracelet/bubbles/key"
)

// Define key bindings
type keyMap struct {
    Up    key.Binding
    Down  key.Binding
    Help  key.Binding
    Quit  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
    return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
    return [][]key.Binding{
        {k.Up, k.Down},
        {k.Help, k.Quit},
    }
}

var keys = keyMap{
    Up: key.NewBinding(
        key.WithKeys("up", "k"),
        key.WithHelp("↑/k", "up"),
    ),
    Down: key.NewBinding(
        key.WithKeys("down", "j"),
        key.WithHelp("↓/j", "down"),
    ),
    Help: key.NewBinding(
        key.WithKeys("?"),
        key.WithHelp("?", "toggle help"),
    ),
    Quit: key.NewBinding(
        key.WithKeys("q", "ctrl+c"),
        key.WithHelp("q", "quit"),
    ),
}

// Create help model
h := help.New()
h.Width = 80  // Wrap at width

// Toggle full/short
h.ShowAll = true  // Full help
h.ShowAll = false // Short help

// Render
helpView := h.View(keys)
```

**Styling:**
```go
h.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
h.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
h.Styles.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
h.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
```

---

## Timer

**Package:** `github.com/charmbracelet/bubbles/timer`

```go
import "github.com/charmbracelet/bubbles/timer"

t := timer.NewWithInterval(5*time.Minute, time.Second)

// In Init (start timer)
func (m model) Init() tea.Cmd {
    return m.timer.Init()
}

// In Update
case timer.TickMsg:
    var cmd tea.Cmd
    m.timer, cmd = m.timer.Update(msg)
    return m, cmd

case timer.StartStopMsg:
    var cmd tea.Cmd
    m.timer, cmd = m.timer.Update(msg)
    return m, cmd

case timer.TimeoutMsg:
    // Timer finished
    m.timerDone = true

// Controls
cmd := m.timer.Toggle()  // Start/stop
cmd := m.timer.Start()
cmd := m.timer.Stop()

// Display
remaining := m.timer.Timeout.String()
```

---

## Stopwatch

**Package:** `github.com/charmbracelet/bubbles/stopwatch`

```go
import "github.com/charmbracelet/bubbles/stopwatch"

sw := stopwatch.NewWithInterval(time.Millisecond * 100)

// In Init
func (m model) Init() tea.Cmd {
    return m.stopwatch.Init()
}

// In Update
case stopwatch.TickMsg:
    var cmd tea.Cmd
    m.stopwatch, cmd = m.stopwatch.Update(msg)
    return m, cmd

case stopwatch.StartStopMsg:
    var cmd tea.Cmd
    m.stopwatch, cmd = m.stopwatch.Update(msg)
    return m, cmd

// Controls
cmd := m.stopwatch.Toggle()
cmd := m.stopwatch.Start()
cmd := m.stopwatch.Stop()
cmd := m.stopwatch.Reset()

// Display
elapsed := m.stopwatch.Elapsed().String()
```

---

## Key Bindings

**Package:** `github.com/charmbracelet/bubbles/key`

```go
import "github.com/charmbracelet/bubbles/key"

// Define binding
quit := key.NewBinding(
    key.WithKeys("q", "ctrl+c"),
    key.WithHelp("q", "quit"),
)

// Check if pressed
case tea.KeyMsg:
    if key.Matches(msg, quit) {
        return m, tea.Quit
    }

// Enable/disable
quit.SetEnabled(false)  // Disable binding
if quit.Enabled() { ... }
```

**Common Key Names:**
- `"enter"`, `"space"`, `"tab"`
- `"up"`, `"down"`, `"left"`, `"right"`
- `"home"`, `"end"`, `"pgup"`, `"pgdown"`
- `"backspace"`, `"delete"`
- `"esc"`, `"ctrl+c"`, `"ctrl+z"`
- `"f1"` through `"f12"`
- Single characters: `"a"`, `"A"`, `"1"`, `"@"`
