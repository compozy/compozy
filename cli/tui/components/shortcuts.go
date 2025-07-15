package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/tui/styles"
)

// ShortcutCategory represents a category of keyboard shortcuts
type ShortcutCategory struct {
	Name      string
	Shortcuts [][2]string
}

// KeyboardShortcuts displays a reference card for keyboard shortcuts
type KeyboardShortcuts struct {
	Width      int
	Height     int
	Visible    bool
	Categories []ShortcutCategory
}

// NewKeyboardShortcuts creates a new keyboard shortcuts component
func NewKeyboardShortcuts() KeyboardShortcuts {
	return KeyboardShortcuts{
		Visible:    false,
		Categories: defaultShortcutCategories(),
	}
}

// defaultShortcutCategories returns the default shortcut categories
func defaultShortcutCategories() []ShortcutCategory {
	return []ShortcutCategory{
		{
			Name: "General",
			Shortcuts: [][2]string{
				{"q", "quit application"},
				{"ctrl+c", "force quit"},
				{"?", "toggle help"},
				{"ctrl+k", "command palette"},
				{escKey, "cancel/back"},
			},
		},
		{
			Name: "Navigation",
			Shortcuts: [][2]string{
				{"↑/k", "move up"},
				{"↓/j", "move down"},
				{"←/h", "move left"},
				{"→/l", "move right"},
				{"enter", "select/confirm"},
				{"tab", "next field"},
				{"shift+tab", "previous field"},
			},
		},
		{
			Name: "Lists & Tables",
			Shortcuts: [][2]string{
				{"home", "go to first item"},
				{"end", "go to last item"},
				{"page up", "scroll up one page"},
				{"page down", "scroll down one page"},
				{"/", "search/filter"},
				{"ctrl+r", "refresh"},
			},
		},
		{
			Name: "Forms",
			Shortcuts: [][2]string{
				{"ctrl+a", "select all text"},
				{"ctrl+e", "end of line"},
				{"ctrl+u", "clear line"},
				{"ctrl+w", "delete word"},
				{"ctrl+d", "delete character"},
			},
		},
		{
			Name: "Key Management",
			Shortcuts: [][2]string{
				{"g", "generate new key"},
				{"r", "revoke selected key"},
				{"c", "copy key to clipboard"},
				{"d", "show key details"},
			},
		},
		{
			Name: "User Management",
			Shortcuts: [][2]string{
				{"n", "create new user"},
				{"e", "edit selected user"},
				{"d", "delete selected user"},
				{"v", "view user details"},
			},
		},
	}
}

// SetSize sets the shortcuts size
func (k *KeyboardShortcuts) SetSize(width, height int) *KeyboardShortcuts {
	k.Width = width
	k.Height = height
	return k
}

// Show shows the keyboard shortcuts
func (k *KeyboardShortcuts) Show() {
	k.Visible = true
}

// Hide hides the keyboard shortcuts
func (k *KeyboardShortcuts) Hide() {
	k.Visible = false
}

// Toggle toggles the shortcuts visibility
func (k *KeyboardShortcuts) Toggle() {
	k.Visible = !k.Visible
}

// Update handles shortcuts updates
func (k *KeyboardShortcuts) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		k.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		if !k.Visible {
			return nil
		}

		switch msg.String() {
		case escKey, "q":
			k.Hide()
			return nil
		}

	case ShowShortcutsMsg:
		k.Show()

	case HideShortcutsMsg:
		k.Hide()
	}

	return nil
}

// View renders the keyboard shortcuts
func (k *KeyboardShortcuts) View() string {
	if !k.Visible {
		return ""
	}

	content := styles.RenderTitle("Keyboard Shortcuts") + "\n\n"
	content += k.renderCategories()
	content += "\n" + styles.HelpStyle.Render("Press ESC or q to close")

	// Wrap in dialog
	dialog := styles.DialogStyle.
		Width(k.Width - 4).
		Render(content)

	// Center the dialog
	return lipgloss.Place(k.Width, k.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// renderCategories renders all shortcut categories
func (k *KeyboardShortcuts) renderCategories() string {
	var content string

	// Calculate number of columns based on width
	cols := 1
	if k.Width > 100 {
		cols = 3
	} else if k.Width > 60 {
		cols = 2
	}

	if cols == 1 {
		// Single column layout
		for i, category := range k.Categories {
			if i > 0 {
				content += "\n"
			}
			content += k.renderCategory(category)
		}
	} else {
		// Multi-column layout
		content += k.renderMultiColumn(cols)
	}

	return content
}

// renderCategory renders a single category
func (k *KeyboardShortcuts) renderCategory(category ShortcutCategory) string {
	var content string

	// Category header
	content += styles.HelpDescStyle.Render(category.Name) + "\n"

	// Shortcuts
	for _, shortcut := range category.Shortcuts {
		key := styles.HelpKeyStyle.Render(shortcut[0])
		desc := styles.HelpDescStyle.Render(shortcut[1])
		content += "  " + key + " " + desc + "\n"
	}

	return content
}

// renderMultiColumn renders categories in multiple columns
func (k *KeyboardShortcuts) renderMultiColumn(cols int) string {
	// Simple multi-column implementation
	// In a real implementation, you might want more sophisticated column balancing
	var columns []string
	itemsPerCol := (len(k.Categories) + cols - 1) / cols

	for col := 0; col < cols; col++ {
		var colContent string
		start := col * itemsPerCol
		end := start + itemsPerCol
		if end > len(k.Categories) {
			end = len(k.Categories)
		}

		for i := start; i < end; i++ {
			if i > start {
				colContent += "\n"
			}
			colContent += k.renderCategory(k.Categories[i])
		}
		columns = append(columns, colContent)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, columns...)
}

// Shortcuts Messages

// ShowShortcutsMsg shows the keyboard shortcuts
type ShowShortcutsMsg struct{}

// HideShortcutsMsg hides the keyboard shortcuts
type HideShortcutsMsg struct{}
