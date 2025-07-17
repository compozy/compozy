package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// RevokeTUI handles key revocation in TUI mode using the unified executor pattern
func RevokeTUI(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	log := logger.FromContext(ctx)
	log.Debug("revoking API key in TUI mode")

	authClient := executor.GetAuthClient()
	if authClient == nil {
		return fmt.Errorf("auth client not available")
	}

	// Create and run the TUI model
	m := newRevokeModel(ctx, authClient)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Check if revocation was successful
	if model, ok := finalModel.(*revokeModel); ok {
		if model.err != nil {
			return model.err
		}
		if !model.revoked {
			return fmt.Errorf("key revocation canceled")
		}
	}

	return nil
}

// revokeModel represents the TUI model for key revocation
type revokeModel struct {
	ctx    context.Context
	client interface {
		ListKeys(ctx context.Context) ([]api.KeyInfo, error)
		RevokeKey(ctx context.Context, keyID string) error
	}
	keys     []api.KeyInfo
	selected int
	state    revokeState
	revoked  bool
	err      error
	width    int
	height   int
	spinner  spinner.Model
	loading  bool
}

type revokeState int

const (
	stateLoadingKeys revokeState = iota
	stateSelectingKey
	stateConfirming
	stateRevoking
	stateDone
)

// Message types
type revokeKeysLoadedMsg struct{ keys []api.KeyInfo }
type revokeKeyRevokedMsg struct{}

// newRevokeModel creates a new TUI model for key revocation
func newRevokeModel(ctx context.Context, client interface {
	ListKeys(ctx context.Context) ([]api.KeyInfo, error)
	RevokeKey(ctx context.Context, keyID string) error
}) *revokeModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	return &revokeModel{
		ctx:     ctx,
		client:  client,
		state:   stateLoadingKeys,
		spinner: s,
		loading: true,
	}
}

// Init initializes the model
func (m *revokeModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadKeys(),
	)
}

// loadKeys loads the API keys
func (m *revokeModel) loadKeys() tea.Cmd {
	return func() tea.Msg {
		keys, err := m.client.ListKeys(m.ctx)
		if err != nil {
			return errMsg{err}
		}
		return revokeKeysLoadedMsg{keys}
	}
}

// revokeKey revokes the selected key
func (m *revokeModel) revokeKey() tea.Cmd {
	return func() tea.Msg {
		if m.selected >= len(m.keys) {
			return errMsg{fmt.Errorf("invalid key selection")}
		}
		selectedKey := m.keys[m.selected]
		err := m.client.RevokeKey(m.ctx, selectedKey.ID)
		if err != nil {
			return errMsg{err}
		}
		return revokeKeyRevokedMsg{}
	}
}

// Update handles messages
func (m *revokeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case revokeKeysLoadedMsg:
		m.loading = false
		m.keys = msg.keys
		if len(m.keys) == 0 {
			m.err = fmt.Errorf("no API keys found")
			return m, tea.Quit
		}
		m.state = stateSelectingKey
		return m, nil

	case revokeKeyRevokedMsg:
		m.loading = false
		m.revoked = true
		m.state = stateDone
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
func (m *revokeModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateSelectingKey:
		return m.handleSelectingKeyInput(msg)
	case stateConfirming:
		return m.handleConfirmingInput(msg)
	case stateDone:
		return m, tea.Quit
	}
	return m, nil
}

// handleSelectingKeyInput handles keyboard input in the selecting key state
func (m *revokeModel) handleSelectingKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		return m, tea.Quit
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case keyDown, "j":
		if m.selected < len(m.keys)-1 {
			m.selected++
		}
	case keyEnter:
		m.state = stateConfirming
		return m, nil
	}
	return m, nil
}

// handleConfirmingInput handles keyboard input in the confirming state
func (m *revokeModel) handleConfirmingInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.state = stateRevoking
		m.loading = true
		cmd := m.revokeKey()
		return m, cmd
	case "n", "N", "q", keyCtrlC:
		m.state = stateSelectingKey
		return m, nil
	}
	return m, nil
}

// View renders the UI
func (m *revokeModel) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("❌ Error: %v", m.err))
	}

	switch m.state {
	case stateLoadingKeys:
		return fmt.Sprintf("%s Loading API keys...", m.spinner.View())
	case stateSelectingKey:
		return m.viewKeySelection()
	case stateConfirming:
		return m.viewConfirmation()
	case stateRevoking:
		return fmt.Sprintf("%s Revoking API key...", m.spinner.View())
	case stateDone:
		return m.viewDone()
	}

	return ""
}

// viewKeySelection renders the key selection view
func (m *revokeModel) viewKeySelection() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1, 2).
		Width(80)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("69"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	content := titleStyle.Render("🔑 Select API Key to Revoke") + "\n\n"

	for i, key := range m.keys {
		created := key.CreatedAt
		if t, err := time.Parse(time.RFC3339, key.CreatedAt); err == nil {
			created = t.Format("2006-01-02 15:04")
		}

		line := fmt.Sprintf("  %s - Created: %s", key.Prefix, created)
		if key.LastUsedAt != "" {
			if t, err := time.Parse(time.RFC3339, key.LastUsedAt); err == nil {
				line += fmt.Sprintf(" - Last used: %s", t.Format("2006-01-02 15:04"))
			}
		}

		if i == m.selected {
			content += selectedStyle.Render("> "+line[2:]) + "\n"
		} else {
			content += line + "\n"
		}
	}

	content += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("↑/↓ to navigate • Enter to select • q to quit")

	return style.Render(content)
}

// viewConfirmation renders the confirmation view
func (m *revokeModel) viewConfirmation() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(1, 2).
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("69"))

	selectedKey := m.keys[m.selected]
	content := titleStyle.Render("⚠️  Confirm Key Revocation") + "\n\n"
	content += "You are about to revoke the following API key:\n\n"
	content += keyStyle.Render(fmt.Sprintf("  Key: %s", selectedKey.Prefix)) + "\n"
	content += fmt.Sprintf("  ID: %s\n", selectedKey.ID)

	created := selectedKey.CreatedAt
	if t, err := time.Parse(time.RFC3339, selectedKey.CreatedAt); err == nil {
		created = t.Format("2006-01-02 15:04")
	}
	content += fmt.Sprintf("  Created: %s\n", created)

	if selectedKey.LastUsedAt != "" {
		if t, err := time.Parse(time.RFC3339, selectedKey.LastUsedAt); err == nil {
			content += fmt.Sprintf("  Last used: %s\n", t.Format("2006-01-02 15:04"))
		}
	}

	content += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true).
		Render("⚠️  This action cannot be undone!")

	content += "\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("y/Y to confirm • n/N to cancel • q to quit")

	return style.Render(content)
}

// viewDone renders the completion view
func (m *revokeModel) viewDone() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("10")).
		Padding(1, 2).
		Width(50)

	content := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("10")).
		Render("✅ API Key Revoked Successfully")

	content += "\n\nThe API key has been permanently revoked."
	content += "\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Press any key to exit")

	return style.Render(content)
}
