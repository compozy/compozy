# Bubble Tea v2 Components Catalog

Reusable components for building TUI applications. All components follow the Elm architecture pattern (Init, Update, View).

> **v2 Note:** All imports use `charm.land/*/v2` paths. Many components have updated APIs — see per-component notes below.

## Panel System

Pre-built panel layouts for different UI arrangements.

### Single Panel

Full-screen single view with optional title and status bars.

**Use for:**
- Simple focused interfaces
- Full-screen text editors
- Single-purpose tools

**Implementation:**
```go
func (m model) renderSinglePanel() string {
    contentWidth, contentHeight := m.calculateLayout()

    // Create panel with full available space
    panel := m.styles.Panel.
        Width(contentWidth).
        Render(content)

    return panel
}
```

### Dual Pane

Side-by-side panels with configurable split ratio and accordion mode.

**Use for:**
- File browsers with preview
- Split editors
- Source/destination views

**Features:**
- Dynamic split ratio (50/50, 66/33, 75/25)
- Accordion mode (focused panel expands)
- Responsive (stacks vertically on narrow terminals)
- Weight-based sizing for smooth resizing

**Implementation:**
```go
func (m model) renderDualPane() string {
    contentWidth, contentHeight := m.calculateLayout()

    // Calculate weights based on focus/accordion
    leftWeight, rightWeight := 1, 1
    if m.accordionMode && m.focusedPanel == "left" {
        leftWeight = 2
    }

    // Calculate actual widths from weights
    totalWeight := leftWeight + rightWeight
    leftWidth := (contentWidth * leftWeight) / totalWeight
    rightWidth := contentWidth - leftWidth

    // Render panels
    leftPanel := m.renderPanel("left", leftWidth, contentHeight)
    rightPanel := m.renderPanel("right", rightWidth, contentHeight)

    return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}
```

**Keyboard shortcuts:**
- `Tab` - Switch focus between panels
- `a` - Toggle accordion mode
- Arrow keys - Focus panel in direction

**Mouse support:**
- Click panel to focus (via `tea.MouseClickMsg`)
- Works in both horizontal and vertical stack modes

### Multi-Panel

3+ panels with configurable sizes and arrangements.

**Use for:**
- IDEs (file tree, editor, terminal, output)
- Dashboard views
- Complex workflows

**Common layouts:**
- Three-column (25/50/25)
- Three-row
- Grid (2x2, 3x3)
- Sidebar + main + inspector

**Implementation:**
```go
// Three-column example
mainWeight, leftWeight, rightWeight := 2, 1, 1  // 50/25/25
totalWeight := mainWeight + leftWeight + rightWeight

leftWidth := (contentWidth * leftWeight) / totalWeight
mainWidth := (contentWidth * mainWeight) / totalWeight
rightWidth := contentWidth - leftWidth - mainWidth
```

### Tabbed

Multiple views with tab switching.

**Use for:**
- Multiple documents
- Settings pages
- Different data views

**Features:**
- Tab bar with active indicator
- Keyboard shortcuts (`1-9`, `Ctrl+Tab`)
- Mouse click to switch tabs (via `tea.MouseClickMsg`)
- Close tab support

## Lists

### Simple List

Basic scrollable list of items.

**Use for:**
- File listings
- Menu options
- Search results

**Features:**
- Keyboard navigation (Up/Down, Home/End, PgUp/PgDn)
- Mouse scrolling and selection
- Visual selection indicator
- Viewport scrolling (only visible items rendered)

**Integration (v2):**
```go
import "charm.land/bubbles/v2/list"

type model struct {
    list   list.Model
    isDark bool
}

func newModel(isDark bool) model {
    items := []list.Item{
        item{title: "Item 1", desc: "Description 1"},
        item{title: "Item 2", desc: "Description 2"},
    }
    l := list.New(items, list.NewDefaultDelegate(), 0, 0)
    l.Styles = list.DefaultStyles(isDark)
    return model{list: l, isDark: isDark}
}
```

> **v2 Change:** `list.DefaultStyles()` now requires `isDark bool`. `list.NewDefaultItemStyles()` also requires `isDark bool`. `NewModel` removed — use `New()`.

### Filtered List

List with fuzzy search/filter.

**Use for:**
- Quick file finder
- Command palette
- Searchable settings

**Features:**
- Real-time filtering as you type
- Fuzzy matching
- Highlighted matches

**Dependencies:**
```go
github.com/koki-develop/go-fzf
```

### Tree View

Hierarchical list with expand/collapse.

**Use for:**
- Directory trees
- Nested data structures
- Outline views

**Features:**
- Expand/collapse nodes
- Indentation levels
- Parent/child relationships
- Recursive rendering

## Input Components

### Text Input

Single-line text field.

**Use for:**
- Forms
- Search boxes
- Prompts

**Integration (v2):**
```go
import "charm.land/bubbles/v2/textinput"

type model struct {
    input  textinput.Model
    isDark bool
}

func newModel(isDark bool) model {
    ti := textinput.New()
    ti.Placeholder = "Enter text..."
    ti.SetStyles(textinput.DefaultStyles(isDark))
    ti.Focus()

    return model{input: ti, isDark: isDark}
}
```

> **v2 Changes:**
> - `NewModel()` removed — use `New()`
> - `Model.Width` field removed — use `Model.SetWidth()` / `Model.Width()`
> - `DefaultKeyMap` var removed — use `DefaultKeyMap()` func
> - Style fields (`PromptStyle`, `TextStyle`, `PlaceholderStyle`, `CompletionStyle`, `CursorStyle`) replaced by `Styles` struct with `Focused`/`Blurred` states
> - `Model.Cursor` (cursor.Model) replaced by `Model.Cursor()` (func returning *tea.Cursor)
> - Use `Model.SetStyles()` / `Model.Styles()` to get/set styles
> - `CompletionStyle` renamed to `StyleState.Suggestion`

### Multiline Input

Text area for longer content.

**Use for:**
- Commit messages
- Notes
- Configuration editing

**Integration (v2):**
```go
import "charm.land/bubbles/v2/textarea"

type model struct {
    textarea textarea.Model
    isDark   bool
}

func newModel(isDark bool) model {
    ta := textarea.New()
    ta.Styles = textarea.DefaultStyles(isDark)
    return model{textarea: ta, isDark: isDark}
}
```

> **v2 Changes:**
> - `DefaultKeyMap` var removed — use `DefaultKeyMap()` func
> - `textarea.Style` type renamed to `textarea.StyleState`
> - `Model.FocusedStyle` / `Model.BlurredStyle` moved to `Model.Styles.Focused` / `Model.Styles.Blurred`
> - `DefaultStyles()` now returns single `Styles` struct (not two values) and requires `isDark bool`
> - `Model.SetCursor(col)` renamed to `Model.SetCursorColumn(col)`
> - New: `Column()`, `ScrollYOffset()`, `ScrollPosition()`, `MoveToBeginning()`, `MoveToEnd()`

### Forms

Structured input with multiple fields.

**Use for:**
- Settings dialogs
- User registration
- Multi-field input

**Integration:**
```go
import "github.com/charmbracelet/huh"

form := huh.NewForm(
    huh.NewGroup(
        huh.NewInput().
            Title("Name").
            Value(&name),
        huh.NewInput().
            Title("Email").
            Value(&email),
    ),
)
```

### Autocomplete

Text input with suggestions.

**Use for:**
- Command entry
- File paths
- Tag selection

**Features:**
- Real-time suggestions
- Keyboard navigation of suggestions
- Tab completion

## Dialogs

### Confirm Dialog

Yes/No confirmation.

**Use for:**
- Delete confirmations
- Save prompts
- Destructive actions

**Example:**
```
+-----------------------------+
| Delete this file?           |
|                             |
|  [Yes]  [No]               |
+-----------------------------+
```

### Input Dialog

Prompt for single value.

**Use for:**
- Quick input
- Rename operations
- New file creation

### Progress Dialog

Show long-running operations.

**Use for:**
- File uploads
- Build processes
- Data processing

**Integration (v2):**
```go
import "charm.land/bubbles/v2/progress"

type model struct {
    progress progress.Model
}

func newModel() model {
    // v2: Colors use lipgloss.Color (not string), gradient options renamed
    p := progress.New(
        progress.WithColors(lipgloss.Color("#5A56E0"), lipgloss.Color("#EE6FF8")),
    )
    return model{progress: p}
}
```

> **v2 Changes:**
> - `Model.Width` field removed — use `Model.SetWidth()` / `Model.Width()`
> - Color types changed from `string` to `color.Color`
> - `WithGradient(a, b)` replaced by `WithColors(colors...)`
> - `WithDefaultGradient()` replaced by `WithDefaultBlend()`
> - `WithScaledGradient(a, b)` replaced by `WithColors(...) + WithScaled(true)`
> - `WithSolidFill(string)` replaced by `WithColors(singleColor)`
> - `WithColorProfile()` removed (automatic)
> - New: `WithColorFunc()` for dynamic coloring, `WithScaled(bool)`

### Modal

Full overlay dialog.

**Use for:**
- Settings
- Help screens
- Complex forms

## Menus

### Context Menu

Right-click or keyboard-triggered menu.

**Use for:**
- File operations
- Quick actions
- Tool integration

### Command Palette

Fuzzy searchable command list.

**Use for:**
- Command discovery
- Keyboard-first workflows
- Power user features

**Keyboard:**
- `Ctrl+P` or `Ctrl+Shift+P` to open
- Type to filter
- Enter to execute

### Menu Bar

Top-level menu system.

**Use for:**
- Traditional application menus
- Organized commands
- Discoverability

## Status Components

### Status Bar

Bottom bar showing state and help.

**Use for:**
- Current mode/state
- Keyboard hints
- File info

**Pattern:**
```go
func (m model) renderStatusBar() string {
    left := fmt.Sprintf("%s | %s", m.mode, m.filename)
    right := fmt.Sprintf("Line %d/%d", m.cursor, m.lineCount)

    width := m.width
    gap := width - lipgloss.Width(left) - lipgloss.Width(right)

    return left + strings.Repeat(" ", gap) + right
}
```

### Title Bar

Top bar with app title and context.

**Use for:**
- Application name
- Current path/document
- Action buttons

### Breadcrumbs

Path navigation component.

**Use for:**
- Directory navigation
- Nested views
- History trail

## Preview Components

### Text Preview

Rendered text with syntax highlighting.

**Use for:**
- File preview
- Code display
- Log viewing

**Integration:**
```go
import "github.com/alecthomas/chroma/v2/quick"

func renderCode(code, language string) string {
    var buf bytes.Buffer
    quick.Highlight(&buf, code, language, "terminal256", "monokai")
    return buf.String()
}
```

### Markdown Preview

Rendered markdown.

**Integration:**
```go
import "github.com/charmbracelet/glamour"

func renderMarkdown(md string) (string, error) {
    renderer, _ := glamour.NewTermRenderer(
        glamour.WithAutoStyle(),
        glamour.WithWordWrap(80),
    )
    return renderer.Render(md)
}
```

### Image Preview

ASCII/Unicode art from images.

**Use for:**
- Image thumbnails
- Visual file preview
- Logos/artwork

**External tools:**
- `catimg` - Convert images to 256-color ASCII
- `viu` - View images in terminal with full color

### Hex Preview

Binary file viewer.

**Use for:**
- Binary file inspection
- Debugging
- Data analysis

## Tables

### Simple Table

Static data display using Bubbles v2 table.

**Integration (v2):**
```go
import "charm.land/bubbles/v2/table"

t := table.New(
    table.WithColumns(columns),
    table.WithRows(rows),
)
t.SetWidth(80)
t.SetHeight(20)
```

> **v2 Change:** `Model.Width`/`Model.Height` fields removed — use `SetWidth()`/`SetHeight()`/`Width()`/`Height()`.

### Interactive Table

Navigable table with selection.

**Use for:**
- Database browsers
- CSV viewers
- Process lists

**Integration (third-party):**
```go
import "github.com/evertras/bubble-table/table"

type model struct {
    table table.Model
}

func (m model) Init() tea.Cmd {
    m.table = table.New([]table.Column{
        table.NewColumn("id", "ID", 10),
        table.NewColumn("name", "Name", 20),
    })
    return nil
}
```

**Features:**
- Sort by column
- Row selection
- Keyboard navigation
- Column resize

## Viewport (v2)

The viewport component has significant new features in v2.

**Integration (v2):**
```go
import "charm.land/bubbles/v2/viewport"

// v2: Constructor uses functional options
vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(24))
vp.SetContent("Long content here...")
```

> **v2 Changes:**
> - `New(w, h int)` replaced by `New(...Option)` with functional options
> - `Model.Width`/`Model.Height`/`Model.YOffset` fields removed — use setter/getter methods
> - `HighPerformanceRendering` removed entirely
> - New: `SoftWrap`, `LeftGutterFunc`, `SetHighlights()`, `SetContentLines()`, `GetContent()`, `FillHeight`, `StyleLineFunc`, horizontal scrolling

## Spinner (v2)

```go
import "charm.land/bubbles/v2/spinner"

s := spinner.New()  // NewModel() removed, use New()
s.Spinner = spinner.Dot
```

> **v2 Change:** `spinner.Tick()` package function replaced by `model.Tick()` method.

## Help (v2)

```go
import "charm.land/bubbles/v2/help"

h := help.New()  // NewModel() removed, use New()
h.SetWidth(80)   // Width field removed, use SetWidth()
h.Styles = help.DefaultStyles(isDark)  // Requires isDark parameter
```

## Component Integration Patterns

### Composing Components

```go
type model struct {
    // Multiple components in one view
    list     list.Model
    preview  string
    input    textinput.Model
    focused  string  // which component has focus
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        // Route to focused component (v2: KeyPressMsg not KeyMsg)
        switch m.focused {
        case "list":
            var cmd tea.Cmd
            m.list, cmd = m.list.Update(msg)
            return m, cmd
        case "input":
            var cmd tea.Cmd
            m.input, cmd = m.input.Update(msg)
            return m, cmd
        }
    }
    return m, nil
}
```

### Lazy Loading Components

Only initialize components when needed:

```go
type model struct {
    preview     *PreviewComponent  // nil until needed
    previewPath string
}

func (m *model) showPreview(path string) {
    if m.preview == nil {
        m.preview = NewPreviewComponent()
    }
    m.preview.Load(path)
}
```

### Component Communication

Use Bubble Tea commands to communicate between components:

```go
type fileSelectedMsg struct {
    path string
}

// In list component Update
case tea.KeyPressMsg:
    if key.Matches(msg, m.keymap.Enter) {
        selectedFile := m.list.SelectedItem()
        return m, func() tea.Msg {
            return fileSelectedMsg{path: selectedFile.Path()}
        }
    }

// In main model Update
case fileSelectedMsg:
    m.preview.Load(msg.path)
    return m, nil
```

## Best Practices

1. **Keep components focused** - Each component should have one responsibility
2. **Use Bubbles v2 package** - Don't reinvent standard components
3. **Lazy initialization** - Create components when needed, not upfront
4. **Proper sizing** - Always pass explicit width/height to components via setter methods
5. **Clean interfaces** - Components should expose minimal, clear APIs
6. **Pass isDark** - Always pass the `isDark` flag when creating component styles

## Dependencies

**Core Charm libraries (v2):**
```
charm.land/bubbletea/v2    # Framework
charm.land/lipgloss/v2     # Styling
charm.land/bubbles/v2      # Standard components
```

**Extended functionality:**
```
github.com/charmbracelet/glamour      # Markdown rendering
github.com/charmbracelet/huh          # Forms
github.com/alecthomas/chroma/v2       # Syntax highlighting
github.com/evertras/bubble-table      # Interactive tables
github.com/koki-develop/go-fzf        # Fuzzy finder
```
