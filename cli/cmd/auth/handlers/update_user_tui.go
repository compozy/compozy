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

// UpdateUserTUI handles user update in TUI mode using the unified executor pattern
func UpdateUserTUI(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, args []string) error {
	log := logger.FromContext(ctx)
	if len(args) == 0 {
		return fmt.Errorf("user ID is required")
	}
	userID := args[0]
	email, err := cobraCmd.Flags().GetString("email")
	if err != nil {
		return fmt.Errorf("failed to get email flag: %w", err)
	}
	name, err := cobraCmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("failed to get name flag: %w", err)
	}
	role, err := cobraCmd.Flags().GetString("role")
	if err != nil {
		return fmt.Errorf("failed to get role flag: %w", err)
	}
	log.Debug("updating user in TUI mode", "user_id", userID)
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return fmt.Errorf("auth client not available")
	}
	m := newUpdateUserModel(ctx, authClient, userID, email, name, role)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	if model, ok := finalModel.(*updateUserModel); ok {
		if model.err != nil {
			return model.err
		}
		if !model.updated {
			return fmt.Errorf("user update canceled")
		}
	}
	return nil
}

// updateUserModel represents the TUI model for user update
type updateUserModel struct {
	ctx         context.Context
	client      api.UserUpdater
	state       updateUserState
	userID      string
	email       string
	name        string
	role        string
	updated     bool
	updatedUser *api.UserInfo
	err         error
	width       int
	height      int
	spinner     spinner.Model
	loading     bool
	cursorPos   int
	inputs      []string
}

type updateUserState int

const (
	stateUpdateInputting updateUserState = iota
	stateUpdateConfirming
	stateUpdateUpdating
	stateUpdateCompleted
)

// Message types
type userUpdatedMsg struct{ user *api.UserInfo }

// newUpdateUserModel creates a new TUI model for user update
func newUpdateUserModel(
	ctx context.Context,
	client api.UserUpdater,
	userID, email, name, role string,
) *updateUserModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	inputs := []string{email, name, role}
	return &updateUserModel{
		ctx:     ctx,
		client:  client,
		userID:  userID,
		state:   stateUpdateInputting,
		spinner: s,
		inputs:  inputs,
		email:   email,
		name:    name,
		role:    role,
	}
}

// Init initializes the model
func (m *updateUserModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// updateUser updates the user
func (m *updateUserModel) updateUser() tea.Cmd {
	return func() tea.Msg {
		req := api.UpdateUserRequest{}
		changed := false
		if m.inputs[0] != "" {
			email := m.inputs[0]
			req.Email = &email
			changed = true
		}
		if m.inputs[1] != "" {
			name := m.inputs[1]
			req.Name = &name
			changed = true
		}
		if m.inputs[2] != "" {
			role := m.inputs[2]
			req.Role = &role
			changed = true
		}
		if !changed {
			return errMsg{fmt.Errorf("no changes specified")}
		}
		user, err := m.client.UpdateUser(m.ctx, m.userID, req)
		if err != nil {
			return errMsg{err}
		}
		return userUpdatedMsg{user}
	}
}

// Update handles messages
func (m *updateUserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case userUpdatedMsg:
		m.loading = false
		m.updated = true
		m.updatedUser = msg.user
		m.state = stateUpdateCompleted
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
func (m *updateUserModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateUpdateInputting:
		return m.handleInputting(msg)
	case stateUpdateConfirming:
		return m.handleConfirming(msg)
	case stateUpdateCompleted:
		return m, tea.Quit
	}
	return m, nil
}

// handleInputting handles keyboard input during the inputting state
func (m *updateUserModel) handleInputting(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		return m, tea.Quit
	case keyEnter:
		if err := m.validateInputs(); err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.state = stateUpdateConfirming
		return m, nil
	case "tab", keyDown:
		m.cursorPos = (m.cursorPos + 1) % len(m.inputs)
	case "shift+tab", "up":
		m.cursorPos--
		if m.cursorPos < 0 {
			m.cursorPos = len(m.inputs) - 1
		}
	case "backspace":
		if m.inputs[m.cursorPos] != "" {
			m.inputs[m.cursorPos] = m.inputs[m.cursorPos][:len(m.inputs[m.cursorPos])-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.inputs[m.cursorPos] += msg.String()
		}
	}
	return m, nil
}

// handleConfirming handles keyboard input during the confirming state
func (m *updateUserModel) handleConfirming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.state = stateUpdateUpdating
		m.loading = true
		cmd := m.updateUser()
		return m, cmd
	case "n", "N", "q", keyCtrlC:
		m.state = stateUpdateInputting
		return m, nil
	}
	return m, nil
}

// validateInputs validates the user inputs
func (m *updateUserModel) validateInputs() error {
	if m.inputs[2] != "" && m.inputs[2] != roleUser && m.inputs[2] != roleAdmin {
		return fmt.Errorf("role must be '%s' or '%s'", roleUser, roleAdmin)
	}
	if m.inputs[0] == "" && m.inputs[1] == "" && m.inputs[2] == "" {
		return fmt.Errorf("at least one field must be specified for update")
	}
	return nil
}

// View renders the UI
func (m *updateUserModel) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("❌ Error: %v", m.err))
	}
	switch m.state {
	case stateUpdateInputting:
		return m.viewInputting()
	case stateUpdateConfirming:
		return m.viewConfirming()
	case stateUpdateUpdating:
		return fmt.Sprintf("%s Updating user...", m.spinner.View())
	case stateUpdateCompleted:
		return m.viewCompleted()
	}
	return ""
}

// viewInputting renders the input view
func (m *updateUserModel) viewInputting() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1, 2).
		Width(60)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("69"))
	labels := []string{"Email:", "Name:", "Role:"}
	content := titleStyle.Render(fmt.Sprintf("✏️ Update User %s", m.userID)) + "\n\n"
	for i, label := range labels {
		inputStyle := lipgloss.NewStyle()
		if i == m.cursorPos {
			inputStyle = inputStyle.Background(lipgloss.Color("57"))
		}
		value := m.inputs[i]
		if i == m.cursorPos {
			value += "_"
		}
		content += fmt.Sprintf("%s %s\n", label, inputStyle.Render(value))
	}
	content += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Tab/↓ to navigate • Enter to confirm • q to quit")
	return style.Render(content)
}

// viewConfirming renders the confirmation view
func (m *updateUserModel) viewConfirming() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(1, 2).
		Width(50)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))
	content := titleStyle.Render("📝 Confirm User Update") + "\n\n"
	content += fmt.Sprintf("User ID: %s\n", m.userID)
	if m.inputs[0] != "" {
		content += fmt.Sprintf("New Email: %s\n", m.inputs[0])
	}
	if m.inputs[1] != "" {
		content += fmt.Sprintf("New Name: %s\n", m.inputs[1])
	}
	if m.inputs[2] != "" {
		content += fmt.Sprintf("New Role: %s\n", m.inputs[2])
	}
	content += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("y/Y to update • n/N to edit • q to quit")
	return style.Render(content)
}

// viewCompleted renders the completion view
func (m *updateUserModel) viewCompleted() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("10")).
		Padding(1, 2).
		Width(50)
	content := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("10")).
		Render("✅ User Updated Successfully")
	if m.updatedUser != nil {
		content += fmt.Sprintf("\n\nUser ID: %s", m.updatedUser.ID)
		content += fmt.Sprintf("\nEmail: %s", m.updatedUser.Email)
		content += fmt.Sprintf("\nName: %s", m.updatedUser.Name)
		content += fmt.Sprintf("\nRole: %s", m.updatedUser.Role)
	}
	content += "\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Press any key to exit")
	return style.Render(content)
}
