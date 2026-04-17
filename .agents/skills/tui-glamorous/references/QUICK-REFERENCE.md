# Quick Reference: Charm Copy-Paste Patterns

Fast-access patterns for common Charm tasks. Copy, paste, ship.

---

## Table of Contents

- [Shell: Gum One-Liners](#shell-gum-one-liners)
- [Go: Bubble Tea Starter](#go-bubble-tea-starter)
- [Go: Lip Gloss Styles](#go-lip-gloss-styles)
- [Go: Huh Forms](#go-huh-forms)
- [Go: Common Components](#go-common-components)
- [Install Commands](#install-commands)
- [Production Patterns](#production-patterns)

---

## Shell: Gum One-Liners

```bash
# Input with placeholder
NAME=$(gum input --placeholder "Enter your name")

# Password input
PASS=$(gum input --password --placeholder "Password")

# Single selection
CHOICE=$(gum choose "option1" "option2" "option3")

# Multi-select
SELECTED=$(gum choose --no-limit "a" "b" "c" "d")

# Fuzzy filter from stdin
BRANCH=$(git branch | gum filter)
FILE=$(find . -name "*.go" | gum filter)

# Confirmation (returns exit code)
gum confirm "Delete?" && rm -rf ./tmp

# Spinner during command
gum spin --title "Installing..." -- npm install

# Styled box
gum style --border rounded --padding "1 2" --foreground 212 "Done!"

# Multi-line input
BODY=$(gum write --placeholder "Enter message...")

# File picker
FILE=$(gum file .)

# Horizontal layout
gum join --horizontal "$(gum style --border rounded 'Left')" "$(gum style --border rounded 'Right')"
```

---

## Go: Bubble Tea Starter

```go
package main

import (
    "fmt"
    "os"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type model struct {
    cursor int
    items  []string
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
            fmt.Println("Selected:", m.items[m.cursor])
            return m, tea.Quit
        }
    }
    return m, nil
}

var selected = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)

func (m model) View() string {
    s := ""
    for i, item := range m.items {
        cursor := "  "
        if i == m.cursor {
            cursor = "▸ "
            s += selected.Render(cursor+item) + "\n"
        } else {
            s += cursor + item + "\n"
        }
    }
    return s + "\n↑/↓: navigate • enter: select • q: quit"
}

func main() {
    m := model{items: []string{"Option A", "Option B", "Option C"}}
    if _, err := tea.NewProgram(m).Run(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

**Run:** `go run .`

---

## Go: Lip Gloss Styles

```go
import "github.com/charmbracelet/lipgloss"

// Basic styles
title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
subtle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
error := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
success := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))

// Box with border
box := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("62")).
    Padding(1, 2)

// Adaptive colors (light/dark aware)
adaptive := lipgloss.AdaptiveColor{Light: "#000", Dark: "#fff"}

// Layout: join horizontal
left := lipgloss.NewStyle().Width(30).Render("Left")
right := lipgloss.NewStyle().Width(50).Render("Right")
lipgloss.JoinHorizontal(lipgloss.Top, left, right)

// Layout: join vertical
header := "Header"
body := "Body"
footer := "Footer"
lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

// Center in container
lipgloss.Place(80, 24, lipgloss.Center, lipgloss.Center, "Centered content")
```

---

## Go: Huh Forms

```go
import "github.com/charmbracelet/huh"

// Simple input
var name string
huh.NewInput().Title("Name").Value(&name).Run()

// Password
var password string
huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&password).Run()

// Select
var choice string
huh.NewSelect[string]().
    Title("Pick one").
    Options(
        huh.NewOption("First", "first"),
        huh.NewOption("Second", "second"),
    ).
    Value(&choice).
    Run()

// Multi-select
var selected []string
huh.NewMultiSelect[string]().
    Title("Pick many").
    Options(huh.NewOptions("A", "B", "C", "D")...).
    Value(&selected).
    Run()

// Confirm
var confirmed bool
huh.NewConfirm().Title("Continue?").Value(&confirmed).Run()

// Full form with groups
var (
    name    string
    email   string
    confirm bool
)
huh.NewForm(
    huh.NewGroup(
        huh.NewInput().Title("Name").Value(&name),
        huh.NewInput().Title("Email").Value(&email),
    ),
    huh.NewGroup(
        huh.NewConfirm().Title("Submit?").Value(&confirm),
    ),
).Run()
```

---

## Go: Common Components

```go
import (
    "github.com/charmbracelet/bubbles/list"
    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/bubbles/textinput"
    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/bubbles/progress"
)

// Spinner
s := spinner.New()
s.Spinner = spinner.Dot
// In Init: return s.Tick
// In Update: s, cmd = s.Update(msg)

// Text input
ti := textinput.New()
ti.Placeholder = "Type here..."
ti.Focus()
// In Update: ti, cmd = ti.Update(msg)
// Get value: ti.Value()

// Progress bar
p := progress.New(progress.WithDefaultGradient())
// Render: p.ViewAs(0.75)  // 75%

// Viewport (scrollable content)
vp := viewport.New(80, 20)
vp.SetContent(longText)
// In Update: vp, cmd = vp.Update(msg)

// List (requires item type implementing list.Item interface)
type item struct{ title, desc string }
func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

items := []list.Item{item{"One", "First"}, item{"Two", "Second"}}
l := list.New(items, list.NewDefaultDelegate(), 40, 20)
l.Title = "My List"
```

---

## Install Commands

```bash
# Shell tools (all at once)
brew install gum glow vhs freeze mods

# Go libraries
go get github.com/charmbracelet/bubbletea@latest \
       github.com/charmbracelet/bubbles@latest \
       github.com/charmbracelet/lipgloss@latest \
       github.com/charmbracelet/huh@latest \
       github.com/charmbracelet/glamour@latest \
       github.com/charmbracelet/wish@latest

# v2 track (bleeding edge)
go get charm.land/bubbletea/v2@latest
go get charm.land/lipgloss/v2@latest
```

---

## Production Patterns

### Terminal Detection

```go
import "github.com/charmbracelet/x/term"

func main() {
    if !term.IsTerminal(int(os.Stdin.Fd())) || os.Getenv("NO_TUI") != "" {
        runPlainMode()
        return
    }
    runTUI()
}
```

### Window Size Handling

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        // Resize components
        m.list.SetSize(m.width, m.height-4)
        m.viewport.Width = m.width
        m.viewport.Height = m.height - 6
    }
    return m, nil
}
```

### Alt Screen + Mouse

```go
tea.NewProgram(
    model,
    tea.WithAltScreen(),        // Full-screen mode
    tea.WithMouseCellMotion(),  // Mouse support
)
```

### Debug Logging

```go
if os.Getenv("DEBUG") != "" {
    f, _ := tea.LogToFile("debug.log", "debug")
    defer f.Close()
}
```

---

## VHS Recording Template

```tape
Output demo.gif
Set FontSize 16
Set Width 1200
Set Height 600
Set Theme "Catppuccin Mocha"

Type "my-command --flag"
Sleep 500ms
Enter
Sleep 2s

Type "q"
Sleep 500ms
```

Run: `vhs demo.tape`
