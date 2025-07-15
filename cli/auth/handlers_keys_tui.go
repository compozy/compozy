package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

func runGenerateTUI(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)

	// Parse flags for initial values
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("failed to get name flag: %w", err)
	}
	description, err := cmd.Flags().GetString("description")
	if err != nil {
		return fmt.Errorf("failed to get description flag: %w", err)
	}
	expiresStr, err := cmd.Flags().GetString("expires")
	if err != nil {
		return fmt.Errorf("failed to get expires flag: %w", err)
	}

	log.Debug("generating API key in TUI mode")

	// Create and run the TUI model
	m := newGenerateModel(ctx, client, name, description, expiresStr)
	p := tea.NewProgram(&m)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Check if generation was successful
	if model, ok := finalModel.(*generateModel); ok {
		if model.err != nil {
			return model.err
		}
		if !model.generated {
			return fmt.Errorf("key generation canceled")
		}
	}

	return nil
}

// generateModel represents the TUI model for key generation
type generateModel struct {
	ctx             context.Context
	client          *Client
	spinner         spinner.Model
	generating      bool
	generated       bool
	apiKey          string
	name            string
	description     string
	expires         string
	err             error
	width           int
	height          int
	clipboardCopied bool
}

// newGenerateModel creates a new TUI model for key generation
func newGenerateModel(ctx context.Context, client *Client, name, description, expires string) generateModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	return generateModel{
		ctx:         ctx,
		client:      client,
		spinner:     s,
		name:        name,
		description: description,
		expires:     expires,
	}
}

// Init initializes the model
func (m *generateModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.generateKey(),
	)
}

// Update handles messages
func (m *generateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", keyCtrlC:
			return m, tea.Quit
		case keyEnter:
			if m.generated {
				return m, tea.Quit
			}
		case "c":
			if m.generated && m.apiKey != "" {
				cmd := m.copyToClipboard()
				return m, cmd
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case keyGeneratedMsg:
		m.generating = false
		m.generated = true
		m.apiKey = string(msg)
		return m, nil

	case errMsg:
		m.generating = false
		m.err = msg.err
		return m, tea.Quit

	case clipboardCopiedMsg:
		m.clipboardCopied = true
		return m, nil
	}

	return m, nil
}

// View renders the UI
func (m *generateModel) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("❌ Error: %v", m.err))
	}

	if m.generating {
		return fmt.Sprintf("%s Generating API key...", m.spinner.View())
	}

	if m.generated {
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("69")).
			Padding(1, 2).
			Width(60)

		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("69"))

		keyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

		content := titleStyle.Render("✅ API Key Generated Successfully!") + "\n\n"

		// Show only partial key for security
		partialKey := m.apiKey
		if len(m.apiKey) > 16 {
			partialKey = m.apiKey[:16] + "..."
		}

		content += "Your new API key (showing first 16 chars):\n"
		content += keyStyle.Render(partialKey) + "\n\n"

		if m.clipboardCopied {
			content += "✅ Full key copied to clipboard!\n"
		} else {
			content += "Press 'c' to copy the full key to clipboard\n"
		}
		content += "Press 'q' to quit\n\n"

		if m.name != "" {
			content += fmt.Sprintf("Name: %s\n", m.name)
		}
		if m.description != "" {
			content += fmt.Sprintf("Description: %s\n", m.description)
		}
		if m.expires != "" {
			content += fmt.Sprintf("Expires: %s\n", m.expires)
		}

		content += "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("⚠️  Save this key securely. You won't be able to see it again!")

		content += "\n\nPress Enter or 'q' to exit"

		return style.Render(content)
	}

	return ""
}

// generateKey generates the API key
func (m *generateModel) generateKey() tea.Cmd {
	return func() tea.Msg {
		req := &GenerateKeyRequest{
			Name:        m.name,
			Description: m.description,
			Expires:     m.expires,
		}
		apiKey, err := m.client.GenerateKey(m.ctx, req)
		if err != nil {
			return errMsg{err}
		}
		return keyGeneratedMsg(apiKey)
	}
}

// copyToClipboard copies the API key to clipboard
func (m *generateModel) copyToClipboard() tea.Cmd {
	return func() tea.Msg {
		if err := clipboard.WriteAll(m.apiKey); err != nil {
			return errMsg{fmt.Errorf("failed to copy to clipboard: %w", err)}
		}
		return clipboardCopiedMsg{}
	}
}

// Message types for the TUI

// Message types for the TUI
type keyGeneratedMsg string

func runListTUI(ctx context.Context, _ *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)
	log.Debug("listing API keys in TUI mode")
	// Create table columns
	columns := []table.Column{
		{Title: "Prefix", Width: 20},
		{Title: "Created", Width: 20},
		{Title: "Last Used", Width: 20},
		{Title: "Usage Count", Width: 12},
	}
	// Create and run the TUI model
	m := models.NewListModel[KeyInfo](ctx, client, columns)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	// Check if there was an error
	if model, ok := finalModel.(*models.ListModel[KeyInfo]); ok {
		if model.Error() != nil {
			return model.Error()
		}
	}
	return nil
}

// contains checks if a string contains a substring (case-insensitive)

func runRevokeTUI(ctx context.Context, _ *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)
	log.Debug("revoking API key in TUI mode")

	// Create and run the TUI model
	m := newRevokeModel(ctx, client)
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
	ctx      context.Context
	client   *Client
	keys     []KeyInfo
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

// newRevokeModel creates a new TUI model for key revocation
func newRevokeModel(ctx context.Context, client *Client) *revokeModel {
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
		if key.LastUsed != nil {
			if t, err := time.Parse(time.RFC3339, *key.LastUsed); err == nil {
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

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

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

	if selectedKey.LastUsed != nil {
		if t, err := time.Parse(time.RFC3339, *selectedKey.LastUsed); err == nil {
			content += fmt.Sprintf("  Last used: %s\n", t.Format("2006-01-02 15:04"))
		}
	}

	content += "\n" + warningStyle.Render("This action cannot be undone!") + "\n\n"
	content += "Are you sure you want to revoke this key? (y/N)"

	return style.Render(content)
}

// viewDone renders the completion view
func (m *revokeModel) viewDone() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1, 2).
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("69"))

	content := titleStyle.Render("✅ API Key Revoked Successfully!") + "\n\n"
	content += fmt.Sprintf("The key %s has been revoked.\n", m.keys[m.selected].Prefix)
	content += "\nPress any key to exit"

	return style.Render(content)
}

// loadKeys loads the API keys
func (m *revokeModel) loadKeys() tea.Cmd {
	return func() tea.Msg {
		keys, err := m.client.ListKeys(m.ctx)
		if err != nil {
			return errMsg{err}
		}
		return revokeKeysLoadedMsg{keys: keys}
	}
}

// revokeKey revokes the selected key
func (m *revokeModel) revokeKey() tea.Cmd {
	return func() tea.Msg {
		selectedKey := m.keys[m.selected]
		err := m.client.RevokeKey(m.ctx, selectedKey.ID)
		if err != nil {
			return errMsg{err}
		}
		return revokeKeyRevokedMsg{}
	}
}

// Message types for the revoke TUI
type revokeKeysLoadedMsg struct {
	keys []KeyInfo
}

type revokeKeyRevokedMsg struct{}
