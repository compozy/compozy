package handlers

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// ListUsersTUI handles user listing in TUI mode using the unified executor pattern
func ListUsersTUI(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	log := logger.FromContext(ctx)
	log.Debug("listing users in TUI mode")
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return fmt.Errorf("auth client not available")
	}
	roleFilter, err := cobraCmd.Flags().GetString("role")
	if err != nil {
		return fmt.Errorf("failed to get role flag: %w", err)
	}
	sortBy, err := cobraCmd.Flags().GetString("sort")
	if err != nil {
		return fmt.Errorf("failed to get sort flag: %w", err)
	}
	filterStr, err := cobraCmd.Flags().GetString("filter")
	if err != nil {
		return fmt.Errorf("failed to get filter flag: %w", err)
	}
	activeOnly, err := cobraCmd.Flags().GetBool("active")
	if err != nil {
		return fmt.Errorf("failed to get active flag: %w", err)
	}
	m := newListUsersModel(ctx, authClient, roleFilter, sortBy, filterStr, activeOnly)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	if model, ok := finalModel.(*listUsersModel); ok && model.err != nil {
		return model.err
	}
	return nil
}

// listUsersModel represents the TUI model for user listing
type listUsersModel struct {
	ctx        context.Context
	client     api.AuthClient
	table      table.Model
	spinner    spinner.Model
	users      []api.UserInfo
	loading    bool
	err        error
	quitting   bool
	roleFilter string
	sortBy     string
	filter     string
	activeOnly bool
}

// newListUsersModel creates a new TUI model for user listing
func newListUsersModel(
	ctx context.Context,
	client api.AuthClient,
	roleFilter, sortBy, filter string,
	activeOnly bool,
) *listUsersModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	columns := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Email", Width: 40},
		{Title: "Role", Width: 10},
		{Title: "Created", Width: 16},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)
	return &listUsersModel{
		ctx:        ctx,
		client:     client,
		table:      t,
		spinner:    s,
		loading:    true,
		roleFilter: roleFilter,
		sortBy:     sortBy,
		filter:     filter,
		activeOnly: activeOnly,
	}
}

type usersLoadedMsg struct {
	users []api.UserInfo
}

func (m *listUsersModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadUsers,
	)
}

func (m *listUsersModel) loadUsers() tea.Msg {
	users, err := m.client.ListUsers(m.ctx)
	if err != nil {
		return errMsg{err}
	}
	return usersLoadedMsg{users: users}
}

func (m *listUsersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}

	case usersLoadedMsg:
		m.loading = false
		m.users = msg.users
		m.updateTable()
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, tea.Quit

	case spinner.TickMsg:
		if m.loading {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	if !m.loading {
		m.table, cmd = m.table.Update(msg)
	}
	return m, cmd
}

func (m *listUsersModel) View() string {
	if m.quitting {
		return ""
	}
	if m.loading {
		return fmt.Sprintf("\n\n   %s Loading users...\n\n", m.spinner.View())
	}
	if m.err != nil {
		return fmt.Sprintf("\n\nError: %v\n\n", m.err)
	}
	return fmt.Sprintf("\n%s\n\nUsers: %d\n\nPress q to quit\n", m.table.View(), len(m.users))
}

func (m *listUsersModel) updateTable() {
	filteredUsers := m.applyFilters(m.users)
	m.applySorting(filteredUsers)
	rows := make([]table.Row, len(filteredUsers))
	for i, user := range filteredUsers {
		createdAt := formatDate(user.CreatedAt)
		rows[i] = table.Row{
			user.Name,
			user.Email,
			user.Role,
			createdAt,
		}
	}
	m.table.SetRows(rows)
}

func (m *listUsersModel) applyFilters(users []api.UserInfo) []api.UserInfo {
	filtered := make([]api.UserInfo, 0, len(users))
	var activeWindow time.Duration
	for _, user := range users {
		if m.roleFilter != "" && user.Role != m.roleFilter {
			continue
		}

		if m.filter != "" && !userMatchesTextFilter(&user, m.filter) {
			continue
		}

		if m.activeOnly {
			if activeWindow == 0 {
				activeWindow = activeUserWindowDuration(m.ctx)
			}
			if !isUserActive(activeWindow, &user) {
				continue
			}
		}

		filtered = append(filtered, user)
	}
	return filtered
}

func (m *listUsersModel) applySorting(users []api.UserInfo) {
	sort.Slice(users, func(i, j int) bool {
		switch m.sortBy {
		case "name":
			return users[i].Name < users[j].Name
		case "email":
			return users[i].Email < users[j].Email
		case "role":
			return users[i].Role < users[j].Role
		case "created":
			return users[i].CreatedAt < users[j].CreatedAt
		default:
			return users[i].CreatedAt < users[j].CreatedAt
		}
	})
}

// formatDate formats a date string for display
func formatDate(dateStr string) string {
	if dateStr == "" {
		return "N/A"
	}
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t.Format("2006-01-02 15:04")
	}
	return dateStr
}
