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

// CreateUserTUI handles user creation in TUI mode using the unified executor pattern
func CreateUserTUI(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	log := logger.FromContext(ctx)

	// Parse flags for initial values
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

	log.Debug("creating user in TUI mode")

	authClient := executor.GetAuthClient()
	if authClient == nil {
		return fmt.Errorf("auth client not available")
	}

	// Create and run the TUI model
	m := newCreateUserModel(ctx, authClient, email, name, role)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Check if creation was successful
	if model, ok := finalModel.(*createUserModel); ok {
		if model.err != nil {
			return model.err
		}
		if !model.created {
			return fmt.Errorf("user creation canceled")
		}
	}

	return nil
}

// createUserModel represents the TUI model for user creation
type createUserModel struct {
	ctx    context.Context
	client interface {
		CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.UserInfo, error)
	}
	state       createUserState
	email       string
	name        string
	role        string
	created     bool
	createdUser *api.UserInfo
	err         error
	width       int
	height      int
	spinner     spinner.Model
	loading     bool
	cursorPos   int
	inputs      []string
}

type createUserState int

const (
	stateUserInputting createUserState = iota
	stateUserConfirming
	stateUserCreating
	stateUserCompleted
)

// Message types
type userCreatedMsg struct{ user *api.UserInfo }

// newCreateUserModel creates a new TUI model for user creation
func newCreateUserModel(ctx context.Context, client interface {
	CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.UserInfo, error)
}, email, name, role string) *createUserModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	// Initialize inputs with provided values
	inputs := []string{email, name, role}

	// Set default role if empty
	if inputs[2] == "" {
		inputs[2] = roleUser
	}

	return &createUserModel{
		ctx:     ctx,
		client:  client,
		state:   stateUserInputting,
		spinner: s,
		inputs:  inputs,
		email:   email,
		name:    name,
		role:    role,
	}
}

// Init initializes the model
func (m *createUserModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// createUser creates the user
func (m *createUserModel) createUser() tea.Cmd {
	return func() tea.Msg {
		req := api.CreateUserRequest{
			Email: m.inputs[0],
			Name:  m.inputs[1],
			Role:  m.inputs[2],
		}

		user, err := m.client.CreateUser(m.ctx, req)
		if err != nil {
			return errMsg{err}
		}
		return userCreatedMsg{user}
	}
}

// Update handles messages
func (m *createUserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case userCreatedMsg:
		m.loading = false
		m.created = true
		m.createdUser = msg.user
		m.state = stateUserCompleted
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
func (m *createUserModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateUserInputting:
		return m.handleInputting(msg)
	case stateUserConfirming:
		return m.handleConfirming(msg)
	case stateUserCompleted:
		return m, tea.Quit
	}
	return m, nil
}

// handleInputting handles keyboard input during the inputting state
func (m *createUserModel) handleInputting(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		return m, tea.Quit
	case keyEnter:
		// Validate inputs
		if err := m.validateInputs(); err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.state = stateUserConfirming
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
		// Add character to current input
		if len(msg.String()) == 1 {
			m.inputs[m.cursorPos] += msg.String()
		}
	}
	return m, nil
}

// handleConfirming handles keyboard input during the confirming state
func (m *createUserModel) handleConfirming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.state = stateUserCreating
		m.loading = true
		cmd := m.createUser()
		return m, cmd
	case "n", "N", "q", keyCtrlC:
		m.state = stateUserInputting
		return m, nil
	}
	return m, nil
}

// validateInputs validates the user inputs
func (m *createUserModel) validateInputs() error {
	if m.inputs[0] == "" {
		return fmt.Errorf("email is required")
	}
	if m.inputs[1] == "" {
		return fmt.Errorf("name is required")
	}
	if m.inputs[2] != roleUser && m.inputs[2] != roleAdmin {
		return fmt.Errorf("role must be '%s' or '%s'", roleUser, roleAdmin)
	}
	return nil
}

// View renders the UI
func (m *createUserModel) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("❌ Error: %v", m.err))
	}

	switch m.state {
	case stateUserInputting:
		return m.viewInputting()
	case stateUserConfirming:
		return m.viewConfirming()
	case stateUserCreating:
		return fmt.Sprintf("%s Creating user...", m.spinner.View())
	case stateUserCompleted:
		return m.viewCompleted()
	}

	return ""
}

// viewInputting renders the input view
func (m *createUserModel) viewInputting() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1, 2).
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("69"))

	labels := []string{"Email:", "Name:", "Role:"}
	content := titleStyle.Render("👤 Create New User") + "\n\n"

	for i, label := range labels {
		inputStyle := lipgloss.NewStyle()
		if i == m.cursorPos {
			inputStyle = inputStyle.Background(lipgloss.Color("57"))
		}

		value := m.inputs[i]
		if i == m.cursorPos {
			value += "_" // Show cursor
		}

		content += fmt.Sprintf("%s %s\n", label, inputStyle.Render(value))
	}

	content += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Tab/↓ to navigate • Enter to confirm • q to quit")

	return style.Render(content)
}

// viewConfirming renders the confirmation view
func (m *createUserModel) viewConfirming() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(1, 2).
		Width(50)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	content := titleStyle.Render("📝 Confirm User Creation") + "\n\n"
	content += fmt.Sprintf("Email: %s\n", m.inputs[0])
	content += fmt.Sprintf("Name: %s\n", m.inputs[1])
	content += fmt.Sprintf("Role: %s\n", m.inputs[2])

	content += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("y/Y to create • n/N to edit • q to quit")

	return style.Render(content)
}

// viewCompleted renders the completion view
func (m *createUserModel) viewCompleted() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("10")).
		Padding(1, 2).
		Width(50)

	content := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("10")).
		Render("✅ User Created Successfully")

	if m.createdUser != nil {
		content += fmt.Sprintf("\n\nUser ID: %s", m.createdUser.ID)
		content += fmt.Sprintf("\nEmail: %s", m.createdUser.Email)
		content += fmt.Sprintf("\nName: %s", m.createdUser.Name)
		content += fmt.Sprintf("\nRole: %s", m.createdUser.Role)
	}

	content += "\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Press any key to exit")

	return style.Render(content)
}
