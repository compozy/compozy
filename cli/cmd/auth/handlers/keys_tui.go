package handlers

import (
	"context"
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// GenerateTUI handles the key generation in TUI mode
func GenerateTUI(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	name, err := cobraCmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("failed to get name flag: %w", err)
	}
	description, err := cobraCmd.Flags().GetString("description")
	if err != nil {
		return fmt.Errorf("failed to get description flag: %w", err)
	}
	expiresStr, err := cobraCmd.Flags().GetString("expires")
	if err != nil {
		return fmt.Errorf("failed to get expires flag: %w", err)
	}
	log.Debug("generating API key in TUI mode")
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return fmt.Errorf("auth client not available")
	}
	m := newGenerateModel(ctx, authClient, name, description, expiresStr)
	p := tea.NewProgram(&m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
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
	client          api.AuthClient
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
func newGenerateModel(ctx context.Context, client api.AuthClient, name, description, expires string) generateModel {
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
	m.generating = true
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
	switch {
	case m.err != nil:
		return renderKeyGenerationError(m.err)
	case m.generating:
		return renderGeneratingMessage(m.spinner.View())
	case m.generated:
		return renderGeneratedSummary(m)
	default:
		return ""
	}
}

func renderKeyGenerationError(err error) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Render(fmt.Sprintf("❌ Error: %v", err))
}

func renderGeneratingMessage(spinnerView string) string {
	return fmt.Sprintf("%s Generating API key...", spinnerView)
}

func renderGeneratedSummary(m *generateModel) string {
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
	partialKey := renderPartialKey(m.apiKey)
	content += "Your new API key (showing first 16 chars):\n"
	content += keyStyle.Render(partialKey) + "\n\n"
	content += renderClipboardStatus(m.clipboardCopied)
	content += renderKeyMetadata(m)
	content += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("⚠️  Save this key securely. You won't be able to see it again!")
	content += "\n\nPress Enter or 'q' to exit"
	return style.Render(content)
}

func renderPartialKey(apiKey string) string {
	if len(apiKey) > 16 {
		return apiKey[:16] + "..."
	}
	return apiKey
}

func renderClipboardStatus(copied bool) string {
	if copied {
		return "✅ Full key copied to clipboard!\n"
	}
	return "Press 'c' to copy the full key to clipboard\n"
}

func renderKeyMetadata(m *generateModel) string {
	var details string
	details += "Press 'q' to quit\n\n"
	if m.name != "" {
		details += fmt.Sprintf("Name: %s\n", m.name)
	}
	if m.description != "" {
		details += fmt.Sprintf("Description: %s\n", m.description)
	}
	if m.expires != "" {
		details += fmt.Sprintf("Expires: %s\n", m.expires)
	}
	return details
}

// generateKey generates the API key
func (m *generateModel) generateKey() tea.Cmd {
	return func() tea.Msg {
		req := &api.GenerateKeyRequest{
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
type keyGeneratedMsg string

type clipboardCopiedMsg struct{}
