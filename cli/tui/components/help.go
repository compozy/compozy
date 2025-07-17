package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/tui/styles"
)

// HelpComponent displays context-sensitive help
type HelpComponent struct {
	Width    int
	Height   int
	Bindings [][2]string
	Title    string
	Visible  bool
}

// NewHelpComponent creates a new help component
func NewHelpComponent() HelpComponent {
	return HelpComponent{
		Title:   "Help",
		Visible: false,
	}
}

// SetSize sets the component size
func (h HelpComponent) SetSize(width, height int) HelpComponent {
	h.Width = width
	h.Height = height
	return h
}

// SetBindings sets the key bindings to display
func (h HelpComponent) SetBindings(bindings [][2]string) HelpComponent {
	h.Bindings = bindings
	return h
}

// SetTitle sets the help title
func (h HelpComponent) SetTitle(title string) HelpComponent {
	h.Title = title
	return h
}

// Show shows the help component
func (h HelpComponent) Show() HelpComponent {
	h.Visible = true
	return h
}

// Hide hides the help component
func (h HelpComponent) Hide() HelpComponent {
	h.Visible = false
	return h
}

// Toggle toggles the help visibility
func (h HelpComponent) Toggle() HelpComponent {
	h.Visible = !h.Visible
	return h
}

// Update handles help component updates
func (h HelpComponent) Update(msg tea.Msg) (HelpComponent, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.Width = msg.Width
		h.Height = msg.Height
	case tea.KeyMsg:
		if msg.String() == "?" {
			h = h.Toggle()
		}
		if msg.String() == "esc" && h.Visible {
			h = h.Hide()
		}
	case HelpBindingsMsg:
		h.Bindings = msg.Bindings
		h.Title = msg.Title
	case ShowHelpMsg:
		h = h.Show()
	case HideHelpMsg:
		h = h.Hide()
	}
	return h, nil
}

// View renders the help component
func (h HelpComponent) View() string {
	if !h.Visible {
		return ""
	}

	content := styles.RenderTitle(h.Title) + "\n\n"

	// Render key bindings in two columns if we have enough width
	if h.Width > 60 && len(h.Bindings) > 6 {
		content += h.renderTwoColumns()
	} else {
		content += h.renderSingleColumn()
	}

	// Add close instruction
	content += "\n" + styles.HelpStyle.Render("Press ESC or ? to close")

	// Wrap in dialog box
	dialog := styles.DialogStyle.
		Width(h.Width - 4).
		Height(h.Height - 4).
		Render(content)

	// Center the dialog
	return lipgloss.Place(h.Width, h.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// renderSingleColumn renders bindings in a single column
func (h HelpComponent) renderSingleColumn() string {
	var content string
	for _, binding := range h.Bindings {
		key := styles.HelpKeyStyle.Render(binding[0])
		desc := styles.HelpDescStyle.Render(binding[1])
		content += key + " " + desc + "\n"
	}
	return content
}

// renderTwoColumns renders bindings in two columns
func (h HelpComponent) renderTwoColumns() string {
	mid := (len(h.Bindings) + 1) / 2

	var leftCol, rightCol string

	// Left column
	for i := 0; i < mid; i++ {
		binding := h.Bindings[i]
		key := styles.HelpKeyStyle.Render(binding[0])
		desc := styles.HelpDescStyle.Render(binding[1])
		leftCol += key + " " + desc + "\n"
	}

	// Right column
	for i := mid; i < len(h.Bindings); i++ {
		binding := h.Bindings[i]
		key := styles.HelpKeyStyle.Render(binding[0])
		desc := styles.HelpDescStyle.Render(binding[1])
		rightCol += key + " " + desc + "\n"
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
}

// GetHeight returns the component height when visible
func (h HelpComponent) GetHeight() int {
	if !h.Visible {
		return 0
	}
	return h.Height
}

// HelpBindingsMsg updates help bindings
type HelpBindingsMsg struct {
	Title    string
	Bindings [][2]string
}

// ShowHelpMsg shows the help component
type ShowHelpMsg struct{}

// HideHelpMsg hides the help component
type HideHelpMsg struct{}

// NewHelpBindingsMsg creates a help bindings message
func NewHelpBindingsMsg(title string, bindings [][2]string) HelpBindingsMsg {
	return HelpBindingsMsg{
		Title:    title,
		Bindings: bindings,
	}
}

// DefaultKeyBindings returns default key bindings for auth commands
func DefaultKeyBindings() [][2]string {
	return [][2]string{
		{"q", "quit"},
		{"?", "toggle help"},
		{"↑/k", "move up"},
		{"↓/j", "move down"},
		{"enter", "select"},
		{"esc", "cancel/back"},
		{"tab", "next field"},
		{"shift+tab", "previous field"},
		{"ctrl+c", "force quit"},
	}
}
