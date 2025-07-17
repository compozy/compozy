package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/tui/styles"
)

// CommandPalette provides quick access to actions via Ctrl+K
type CommandPalette struct {
	Width    int
	Height   int
	Visible  bool
	Input    textinput.Model
	Commands []Command
	Filtered []Command
	Selected int
}

// Command represents a command in the palette
type Command struct {
	ID          string
	Name        string
	Description string
	Category    string
	Shortcut    string
	Action      tea.Cmd
}

// NewCommandPalette creates a new command palette
func NewCommandPalette() CommandPalette {
	input := textinput.New()
	input.Placeholder = "Type to search commands..."
	input.Focus()

	return CommandPalette{
		Input:    input,
		Visible:  false,
		Selected: 0,
		Commands: defaultCommands(),
	}
}

// defaultCommands returns the default command set
func defaultCommands() []Command {
	return []Command{
		{
			ID:          "help",
			Name:        "Show Help",
			Description: "Display context-sensitive help",
			Category:    "Navigation",
			Shortcut:    "?",
		},
		{
			ID:          "quit",
			Name:        "Quit Application",
			Description: "Exit the application",
			Category:    "Navigation",
			Shortcut:    "q",
		},
		{
			ID:          "generate-key",
			Name:        "Generate API Key",
			Description: "Create a new API key",
			Category:    "Keys",
			Shortcut:    "",
		},
		{
			ID:          "list-keys",
			Name:        "List API Keys",
			Description: "View all API keys",
			Category:    "Keys",
			Shortcut:    "",
		},
		{
			ID:          "revoke-key",
			Name:        "Revoke API Key",
			Description: "Revoke an existing API key",
			Category:    "Keys",
			Shortcut:    "",
		},
		{
			ID:          "create-user",
			Name:        "Create User",
			Description: "Create a new user account",
			Category:    "Users",
			Shortcut:    "",
		},
		{
			ID:          "list-users",
			Name:        "List Users",
			Description: "View all user accounts",
			Category:    "Users",
			Shortcut:    "",
		},
		{
			ID:          "update-user",
			Name:        "Update User",
			Description: "Modify user account details",
			Category:    "Users",
			Shortcut:    "",
		},
		{
			ID:          "delete-user",
			Name:        "Delete User",
			Description: "Remove a user account",
			Category:    "Users",
			Shortcut:    "",
		},
		{
			ID:          "tutorial",
			Name:        "Start Tutorial",
			Description: "Interactive tutorial for first-time users",
			Category:    "Help",
			Shortcut:    "",
		},
		{
			ID:          "shortcuts",
			Name:        "Keyboard Shortcuts",
			Description: "Display keyboard shortcut reference",
			Category:    "Help",
			Shortcut:    "",
		},
	}
}

// SetSize sets the palette size
func (p *CommandPalette) SetSize(width, height int) {
	p.Width = width
	p.Height = height
	p.Input.Width = width - 6
}

// Show shows the command palette
func (p *CommandPalette) Show() {
	p.Visible = true
	p.Input.Focus()
	p.Input.SetValue("")
	p.Selected = 0
	p.filterCommands("")
}

// Hide hides the command palette
func (p *CommandPalette) Hide() {
	p.Visible = false
	p.Input.Blur()
}

// Toggle toggles the palette visibility
func (p *CommandPalette) Toggle() {
	if p.Visible {
		p.Hide()
	} else {
		p.Show()
	}
}

// Update handles palette updates
func (p *CommandPalette) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		return p.handleKeyMsg(msg)
	case ShowPaletteMsg:
		p.Show()
	case HidePaletteMsg:
		p.Hide()
	}
	return nil
}

// handleKeyMsg handles keyboard input
func (p *CommandPalette) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	if !p.Visible {
		if msg.String() == "ctrl+k" {
			p.Show()
		}
		return nil
	}
	return p.handleVisibleKeyMsg(msg)
}

// handleVisibleKeyMsg handles keyboard input when palette is visible
func (p *CommandPalette) handleVisibleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		p.Hide()
		return nil
	case "enter":
		return p.handleEnterKey()
	case "up", "ctrl+p":
		if p.Selected > 0 {
			p.Selected--
		}
	case "down", "ctrl+n":
		if p.Selected < len(p.Filtered)-1 {
			p.Selected++
		}
	default:
		return p.handleInputUpdate(msg)
	}
	return nil
}

// handleEnterKey handles enter key press
func (p *CommandPalette) handleEnterKey() tea.Cmd {
	if len(p.Filtered) > 0 && p.Selected < len(p.Filtered) {
		cmd := p.executeCommand(&p.Filtered[p.Selected])
		p.Hide()
		return cmd
	}
	return nil
}

// handleInputUpdate handles text input updates
func (p *CommandPalette) handleInputUpdate(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	p.Input, cmd = p.Input.Update(msg)
	query := p.Input.Value()
	p.filterCommands(query)
	if p.Selected >= len(p.Filtered) {
		p.Selected = 0
	}
	return cmd
}

// filterCommands filters commands based on the query
func (p *CommandPalette) filterCommands(query string) {
	if query == "" {
		p.Filtered = p.Commands
		return
	}

	query = strings.ToLower(query)
	p.Filtered = nil

	for _, cmd := range p.Commands {
		if strings.Contains(strings.ToLower(cmd.Name), query) ||
			strings.Contains(strings.ToLower(cmd.Description), query) ||
			strings.Contains(strings.ToLower(cmd.Category), query) {
			p.Filtered = append(p.Filtered, cmd)
		}
	}
}

// executeCommand executes the selected command
func (p *CommandPalette) executeCommand(cmd *Command) tea.Cmd {
	// Return a message that other components can handle
	return func() tea.Msg {
		return ExecuteCommandMsg{Command: *cmd}
	}
}

// View renders the command palette
func (p *CommandPalette) View() string {
	if !p.Visible {
		return ""
	}
	var content strings.Builder
	content.WriteString(styles.RenderTitle("Command Palette"))
	content.WriteString("\n\n")
	content.WriteString(p.Input.View())
	content.WriteString("\n\n")
	p.renderCommands(&content)
	content.WriteString("\n\n")
	content.WriteString(styles.HelpStyle.Render("↑/↓ navigate • enter select • esc close"))
	dialog := styles.DialogStyle.Width(p.Width - 4).Render(content.String())
	return lipgloss.Place(p.Width, p.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// renderCommands renders the command list
func (p *CommandPalette) renderCommands(content *strings.Builder) {
	if len(p.Filtered) == 0 {
		content.WriteString(styles.HelpStyle.Render("No commands found"))
		return
	}
	maxItems := 8
	start, end := p.calculateVisibleRange(maxItems)
	currentCategory := ""
	for i := start; i < end; i++ {
		cmd := p.Filtered[i]
		if cmd.Category != currentCategory {
			if currentCategory != "" {
				content.WriteString("\n")
			}
			content.WriteString(styles.HelpDescStyle.Render(cmd.Category))
			content.WriteString("\n")
			currentCategory = cmd.Category
		}
		p.renderCommandItem(content, &cmd, i)
	}
	if len(p.Filtered) > maxItems {
		p.renderPaginationHint(content, start, end)
	}
}

// calculateVisibleRange calculates the visible range for commands
func (p *CommandPalette) calculateVisibleRange(maxItems int) (start, end int) {
	start = 0
	end = len(p.Filtered)
	if end > maxItems {
		if p.Selected >= maxItems/2 {
			start = p.Selected - maxItems/2
			end = start + maxItems
			if end > len(p.Filtered) {
				end = len(p.Filtered)
				start = end - maxItems
			}
		} else {
			end = maxItems
		}
	}
	return start, end
}

// renderCommandItem renders a single command item
func (p *CommandPalette) renderCommandItem(content *strings.Builder, cmd *Command, index int) {
	prefix := "  "
	if index == p.Selected {
		prefix = "▶ "
	}
	name := cmd.Name
	if cmd.Shortcut != "" {
		name += " (" + cmd.Shortcut + ")"
	}
	if index == p.Selected {
		line := styles.SelectedRowStyle.Render(prefix + name)
		content.WriteString(line)
		content.WriteString("\n")
		if cmd.Description != "" {
			desc := styles.HelpStyle.Render("    " + cmd.Description)
			content.WriteString(desc)
		}
	} else {
		line := prefix + name
		content.WriteString(line)
	}
	content.WriteString("\n")
}

// renderPaginationHint renders pagination information
func (p *CommandPalette) renderPaginationHint(content *strings.Builder, start, end int) {
	showing := end - start
	total := len(p.Filtered)
	hint := styles.HelpStyle.Render(fmt.Sprintf("Showing %d/%d", showing, total))
	content.WriteString("\n")
	content.WriteString(hint)
}

// Command Palette Messages

// ShowPaletteMsg shows the command palette
type ShowPaletteMsg struct{}

// HidePaletteMsg hides the command palette
type HidePaletteMsg struct{}

// ExecuteCommandMsg executes a command
type ExecuteCommandMsg struct {
	Command Command
}

// AddCommand adds a command to the palette
func (p *CommandPalette) AddCommand(cmd *Command) {
	p.Commands = append(p.Commands, *cmd)
	p.filterCommands(p.Input.Value())
}

// SetCommands sets the command list
func (p *CommandPalette) SetCommands(commands []Command) {
	p.Commands = commands
	p.filterCommands(p.Input.Value())
}
