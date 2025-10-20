package handlers

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// DeleteUserTUI handles user deletion in TUI mode using the unified executor pattern
func DeleteUserTUI(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, args []string) error {
	log := logger.FromContext(ctx)
	// Get user ID from arguments
	if len(args) == 0 {
		return fmt.Errorf("user ID required")
	}
	userID := args[0]
	// Parse flags
	force, err := cobraCmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}
	cascade, err := cobraCmd.Flags().GetBool("cascade")
	if err != nil {
		return fmt.Errorf("failed to get cascade flag: %w", err)
	}
	log.Debug("deleting user in TUI mode",
		"user_id", userID,
		"force", force,
		"cascade", cascade)
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return fmt.Errorf("auth client not available")
	}
	// Create and run the TUI model
	m := newDeleteUserModel(ctx, authClient, userID, force, cascade)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	// Check if deletion was successful
	if model, ok := finalModel.(*deleteUserModel); ok {
		if model.err != nil {
			return model.err
		}
		if !model.deleted {
			return fmt.Errorf("user deletion canceled")
		}
	}
	return nil
}

// deleteUserModel represents the TUI model for user deletion
type deleteUserModel struct {
	ctx     context.Context
	client  api.UserDeleter
	state   deleteUserState
	userID  string
	force   bool
	cascade bool
	deleted bool
	err     error
	width   int
	height  int
	spinner spinner.Model
	loading bool
}

type deleteUserState int

const (
	stateDeleteConfirming deleteUserState = iota
	stateDeleteDeleting
	stateDeleteCompleted
)

// Message types
type userDeletedMsg struct{}

// newDeleteUserModel creates a new TUI model for user deletion
func newDeleteUserModel(
	ctx context.Context,
	client api.UserDeleter,
	userID string,
	force, cascade bool,
) *deleteUserModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	// If force is already set, skip confirmation
	initialState := stateDeleteConfirming
	if force {
		initialState = stateDeleteDeleting
	}
	return &deleteUserModel{
		ctx:     ctx,
		client:  client,
		state:   initialState,
		userID:  userID,
		force:   force,
		cascade: cascade,
		spinner: s,
	}
}

// Init initializes the model
func (m *deleteUserModel) Init() tea.Cmd {
	if m.force {
		// If force is set, start deletion immediately
		m.loading = true
		return tea.Batch(m.spinner.Tick, m.deleteUser())
	}
	return m.spinner.Tick
}

// deleteUser deletes the user
func (m *deleteUserModel) deleteUser() tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteUser(m.ctx, m.userID)
		if err != nil {
			return errMsg{err}
		}
		return userDeletedMsg{}
	}
}

// Update handles messages
func (m *deleteUserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case userDeletedMsg:
		m.loading = false
		m.deleted = true
		m.state = stateDeleteCompleted
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, tea.Quit

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// handleKeyMsg handles keyboard input
func (m *deleteUserModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateDeleteConfirming:
		return m.handleConfirming(msg)
	case stateDeleteCompleted:
		return m, tea.Quit
	}
	return m, nil
}

// handleConfirming handles keyboard input during the confirming state
func (m *deleteUserModel) handleConfirming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.state = stateDeleteDeleting
		m.loading = true
		cmd := m.deleteUser()
		return m, cmd
	case "n", "N", "q", keyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// View renders the UI
func (m *deleteUserModel) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("❌ Error: %v", m.err))
	}
	switch m.state {
	case stateDeleteConfirming:
		return m.viewConfirming()
	case stateDeleteDeleting:
		return fmt.Sprintf("%s Deleting user...", m.spinner.View())
	case stateDeleteCompleted:
		return m.viewCompleted()
	}
	return ""
}

// viewConfirming renders the confirmation view
func (m *deleteUserModel) viewConfirming() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 2).
		Width(60)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196"))
	content := titleStyle.Render("⚠️  Delete User Confirmation") + "\n\n"
	content += fmt.Sprintf("You are about to delete user: %s\n", m.userID)
	if m.cascade {
		content += "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Render("⚠️  CASCADE MODE: This will also delete all related data!")
	}
	content += "\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true).
		Render("This action cannot be undone!")
	content += "\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("y/Y to delete • n/N/q to cancel")
	return style.Render(content)
}

// viewCompleted renders the completion view
func (m *deleteUserModel) viewCompleted() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("10")).
		Padding(1, 2).
		Width(50)
	content := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("10")).
		Render("✅ User Deleted Successfully")
	content += fmt.Sprintf("\n\nUser ID: %s", m.userID)
	if m.cascade {
		content += "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Render("Related data was also deleted (cascade mode)")
	}
	content += "\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Press any key to exit")
	return style.Render(content)
}
