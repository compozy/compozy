# Bubbletea Components Catalog

Reusable components for building TUI applications. All components follow the Elm architecture pattern (Init, Update, View).

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
- Click panel to focus
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
- Mouse click to switch tabs
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

**Integration:**
```go
import "github.com/charmbracelet/bubbles/list"

type model struct {
    list list.Model
}

func (m model) Init() tea.Cmd {
    items := []list.Item{
        item{title: "Item 1", desc: "Description 1"},
        item{title: "Item 2", desc: "Description 2"},
    }
    m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
    return nil
}
```

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

**Integration:**
```go
import "github.com/charmbracelet/bubbles/textinput"

type model struct {
    input textinput.Model
}

func (m model) Init() tea.Cmd {
    m.input = textinput.New()
    m.input.Placeholder = "Enter text..."
    m.input.Focus()
    return textinput.Blink
}
```

### Multiline Input

Text area for longer content.

**Use for:**
- Commit messages
- Notes
- Configuration editing

**Integration:**
```go
import "github.com/charmbracelet/bubbles/textarea"

type model struct {
    textarea textarea.Model
}
```

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
┌─────────────────────────────┐
│ Delete this file?           │
│                             │
│  [Yes]  [No]                │
└─────────────────────────────┘
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

**Integration:**
```go
import "github.com/charmbracelet/bubbles/progress"

type model struct {
    progress progress.Model
}
```

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

**Example:**
```
┌─────────────┐
│ Open        │
│ Copy        │
│ Delete      │
│ Properties  │
└─────────────┘
```

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

**Example:**
```
File  Edit  View  Help
```

## Status Components

### Status Bar

Bottom bar showing state and help.

**Use for:**
- Current mode/state
- Keyboard hints
- File info

**Example:**
```
┌────────────────────────────────────┐
│                                    │
│        Content area                │
│                                    │
├────────────────────────────────────┤
│ Normal | file.txt | Line 10/100   │
└────────────────────────────────────┘
```

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

**Example:**
```
Home > Projects > TUITemplate > components
```

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

**Example:**
```
00000000: 7f45 4c46 0201 0100 0000 0000 0000 0000  .ELF............
00000010: 0200 3e00 0100 0000 6009 4000 0000 0000  ..>.....`.@.....
```

## Tables

### Simple Table

Static data display.

**Use for:**
- Data display
- Reports
- Comparison views

### Interactive Table

Navigable table with selection.

**Use for:**
- Database browsers
- CSV viewers
- Process lists

**Integration:**
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
    case tea.KeyMsg:
        // Route to focused component
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

Use Bubbletea commands to communicate between components:

```go
type fileSelectedMsg struct {
    path string
}

// In list component Update
case tea.KeyMsg:
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
2. **Use bubbles package** - Don't reinvent standard components
3. **Lazy initialization** - Create components when needed, not upfront
4. **Proper sizing** - Always pass explicit width/height to components
5. **Clean interfaces** - Components should expose minimal, clear APIs

## External Dependencies

**Core Charm libraries:**
```
github.com/charmbracelet/bubbletea    # Framework
github.com/charmbracelet/lipgloss     # Styling
github.com/charmbracelet/bubbles      # Standard components
```

**Extended functionality:**
```
github.com/charmbracelet/glamour      # Markdown rendering
github.com/charmbracelet/huh          # Forms
github.com/alecthomas/chroma/v2       # Syntax highlighting
github.com/evertras/bubble-table      # Interactive tables
github.com/koki-develop/go-fzf        # Fuzzy finder
```

See `go.mod` in template for complete list of optional dependencies.
