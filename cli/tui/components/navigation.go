package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const ctrlKKey = "ctrl+k"

// NavigationManager manages all navigation components
type NavigationManager struct {
	Width      int
	Height     int
	Help       HelpComponent
	Palette    CommandPalette
	Tutorial   Tutorial
	Shortcuts  KeyboardShortcuts
	Breadcrumb Breadcrumb
}

// NewNavigationManager creates a new navigation manager
func NewNavigationManager() NavigationManager {
	return NavigationManager{
		Help:       NewHelpComponent(),
		Palette:    NewCommandPalette(),
		Tutorial:   NewTutorial(),
		Shortcuts:  NewKeyboardShortcuts(),
		Breadcrumb: NewBreadcrumb(),
	}
}

// SetSize sets the size for all navigation components
func (n *NavigationManager) SetSize(width, height int) {
	n.Width = width
	n.Height = height
	n.Help = n.Help.SetSize(width, height)
	n.Palette.SetSize(width, height)
	n.Tutorial.SetSize(width, height)
	n.Shortcuts.SetSize(width, height)
	n.Breadcrumb.SetWidth(width)
}

// Update handles updates for all navigation components
func (n *NavigationManager) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Update all components
	help, helpCmd := n.Help.Update(msg)
	n.Help = help
	if helpCmd != nil {
		cmds = append(cmds, helpCmd)
	}

	paletteCmd := n.Palette.Update(msg)
	if paletteCmd != nil {
		cmds = append(cmds, paletteCmd)
	}

	tutorialCmd := n.Tutorial.Update(msg)
	if tutorialCmd != nil {
		cmds = append(cmds, tutorialCmd)
	}

	shortcutsCmd := n.Shortcuts.Update(msg)
	if shortcutsCmd != nil {
		cmds = append(cmds, shortcutsCmd)
	}

	// Handle special navigation keys
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case ctrlKKey:
			if !n.AnyVisible() {
				n.Palette.Show()
			}
		case "?":
			if !n.AnyVisible() {
				n.Help = n.Help.Toggle()
			}
		}
	}

	// Handle palette commands
	if executeMsg, ok := msg.(ExecuteCommandMsg); ok {
		return n.handlePaletteCommand(&executeMsg.Command)
	}

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

// handlePaletteCommand handles commands from the command palette
func (n *NavigationManager) handlePaletteCommand(cmd *Command) tea.Cmd {
	switch cmd.ID {
	case "help":
		n.Help = n.Help.Show()
	case "tutorial":
		n.Tutorial.Start()
	case "shortcuts":
		n.Shortcuts.Show()
	default:
		// Return the command as a message for the parent to handle
		return func() tea.Msg {
			return ExecuteCommandMsg{Command: *cmd}
		}
	}
	return nil
}

// View renders all visible navigation components
func (n *NavigationManager) View() string {
	var overlays []string

	// Breadcrumb is always rendered if it has items
	breadcrumb := n.Breadcrumb.View()

	// Add overlays in order of priority (last one on top)
	if n.Help.Visible {
		overlays = append(overlays, n.Help.View())
	}

	if n.Shortcuts.Visible {
		overlays = append(overlays, n.Shortcuts.View())
	}

	if n.Tutorial.Visible {
		overlays = append(overlays, n.Tutorial.View())
	}

	if n.Palette.Visible {
		overlays = append(overlays, n.Palette.View())
	}

	// Combine breadcrumb and overlays
	if len(overlays) == 0 {
		return breadcrumb
	}

	// If we have overlays, render the top one over the breadcrumb
	topOverlay := overlays[len(overlays)-1]
	if breadcrumb != "" {
		// Place breadcrumb at top, overlay in center
		breadcrumbView := lipgloss.PlaceHorizontal(n.Width, lipgloss.Left, breadcrumb)
		return breadcrumbView + "\n" + topOverlay
	}

	return topOverlay
}

// AnyVisible returns true if any navigation component is visible
func (n *NavigationManager) AnyVisible() bool {
	return n.Help.Visible || n.Palette.Visible || n.Tutorial.Visible || n.Shortcuts.Visible
}

// SetContextHelp sets context-sensitive help for the current screen
func (n *NavigationManager) SetContextHelp(title string, bindings [][2]string) {
	n.Help = n.Help.SetTitle(title).SetBindings(bindings)
}

// SetBreadcrumb sets the current breadcrumb path
func (n *NavigationManager) SetBreadcrumb(items []BreadcrumbItem) {
	n.Breadcrumb.SetItems(items)
}

// GetBreadcrumbHeight returns the height used by breadcrumb
func (n *NavigationManager) GetBreadcrumbHeight() int {
	return n.Breadcrumb.GetHeight()
}

// IsNavigationKey returns true if the key is handled by navigation
func (n *NavigationManager) IsNavigationKey(key string) bool {
	switch key {
	case ctrlKKey, "?":
		return true
	case escKey:
		return n.AnyVisible()
	default:
		return false
	}
}

// GetCommands returns available commands for the command palette
func (n *NavigationManager) GetCommands() []Command {
	return n.Palette.Commands
}

// AddCommand adds a command to the palette
func (n *NavigationManager) AddCommand(cmd *Command) {
	n.Palette.AddCommand(cmd)
}

// SetCommands sets the command list for the palette
func (n *NavigationManager) SetCommands(commands []Command) {
	n.Palette.SetCommands(commands)
}
