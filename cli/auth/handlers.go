package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/auth/tui/models"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

const (
	keyCtrlC    = "ctrl+c"
	keyEnter    = "enter"
	keyDown     = "down"
	roleUser    = "user"
	roleAdmin   = "admin"
	sortName    = "name"
	sortEmail   = "email"
	sortRole    = "role"
	sortCreated = "created"
)

// runGenerateJSON handles JSON mode for key generation
func runGenerateJSON(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)

	// Parse flags
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

	// Validate expiration date if provided
	var expires *time.Time
	if expiresStr != "" {
		t, err := time.Parse("2006-01-02", expiresStr)
		if err != nil {
			return outputJSONError("invalid expiration date format, use YYYY-MM-DD")
		}
		expires = &t
	}

	log.Debug("generating API key in JSON mode",
		"name", name,
		"description", description,
		"expires", expiresStr)

	// Generate the key
	req := &GenerateKeyRequest{
		Name:        name,
		Description: description,
	}
	if expires != nil {
		req.Expires = expires.Format("2006-01-02")
	}

	apiKey, err := client.GenerateKey(ctx, req)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to generate API key: %v", err))
	}

	// Prepare response
	response := map[string]any{
		"api_key": apiKey,
		"created": time.Now().Format(time.RFC3339),
	}

	if name != "" {
		response["name"] = name
	}
	if description != "" {
		response["description"] = description
	}
	if expires != nil {
		response["expires"] = expires.Format(time.RFC3339)
	}

	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}

// outputJSONError outputs an error in JSON format
func outputJSONError(message string) error {
	response := map[string]any{
		"error": message,
	}

	encoder := json.NewEncoder(os.Stderr)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		// If we can't encode the error response, just return the original error
		return fmt.Errorf("%s", message)
	}

	return fmt.Errorf("%s", message)
}

// runGenerateTUI handles TUI mode for key generation
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
	ctx         context.Context
	client      *Client
	spinner     spinner.Model
	generating  bool
	generated   bool
	apiKey      string
	name        string
	description string
	expires     string
	err         error
	width       int
	height      int
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
		content += "Your new API key:\n"
		content += keyStyle.Render(m.apiKey) + "\n\n"

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

// Message types for the TUI
type keyGeneratedMsg string
type errMsg struct{ err error }

// runListJSON handles JSON mode for key listing
func runListJSON(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)
	// Parse flags
	sortBy, err := cmd.Flags().GetString("sort")
	if err != nil {
		return fmt.Errorf("failed to get sort flag: %w", err)
	}
	filter, err := cmd.Flags().GetString("filter")
	if err != nil {
		return fmt.Errorf("failed to get filter flag: %w", err)
	}
	page, err := cmd.Flags().GetInt("page")
	if err != nil {
		return fmt.Errorf("failed to get page flag: %w", err)
	}
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return fmt.Errorf("failed to get limit flag: %w", err)
	}
	log.Debug("listing API keys in JSON mode",
		"sort", sortBy,
		"filter", filter,
		"page", page,
		"limit", limit)
	// Get the keys from the API
	keys, err := client.ListKeys(ctx)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to list API keys: %v", err))
	}
	// Apply client-side filtering if needed
	if filter != "" {
		filtered := make([]models.KeyInfo, 0)
		for _, key := range keys {
			// Filter by prefix or ID
			if contains(key.Prefix, filter) || contains(key.ID, filter) {
				filtered = append(filtered, key)
			}
		}
		keys = filtered
	}
	// Apply client-side sorting
	sortKeys(keys, sortBy)
	// Apply client-side pagination
	totalKeys := len(keys)
	startIdx := (page - 1) * limit
	endIdx := startIdx + limit
	if startIdx >= totalKeys {
		// Return empty result if page is out of bounds
		keys = []models.KeyInfo{}
	} else {
		if endIdx > totalKeys {
			endIdx = totalKeys
		}
		keys = keys[startIdx:endIdx]
	}
	// Prepare response with pagination metadata
	response := map[string]any{
		"keys":  keys,
		"total": totalKeys,
		"page":  page,
		"limit": limit,
		"pages": (totalKeys + limit - 1) / limit,
	}
	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}
	return nil
}

// runListTUI handles TUI mode for key listing
func runListTUI(ctx context.Context, _ *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)
	log.Debug("listing API keys in TUI mode")
	// Create and run the TUI model
	m := models.NewListModel(ctx, client)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	// Check if there was an error
	if model, ok := finalModel.(*models.ListModel); ok {
		if model.Error() != nil {
			return model.Error()
		}
	}
	return nil
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// sortKeys sorts a slice of KeyInfo based on the specified field
func sortKeys(keys []models.KeyInfo, sortBy string) {
	switch sortBy {
	case sortName:
		// Sort by prefix (which contains the name prefix)
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].Prefix < keys[j].Prefix
		})
	case "last_used":
		// Sort by last used timestamp (most recent first)
		sort.Slice(keys, func(i, j int) bool {
			// Handle nil LastUsed values
			if keys[i].LastUsed == nil && keys[j].LastUsed == nil {
				return false
			}
			if keys[i].LastUsed == nil {
				return false
			}
			if keys[j].LastUsed == nil {
				return true
			}
			return *keys[i].LastUsed > *keys[j].LastUsed
		})
	case sortCreated, "":
		// Sort by created timestamp (most recent first)
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].CreatedAt > keys[j].CreatedAt
		})
	default:
		// Sort by created timestamp (most recent first) for any unrecognized sort field
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].CreatedAt > keys[j].CreatedAt
		})
	}
}

// runRevokeJSON handles JSON mode for key revocation
func runRevokeJSON(ctx context.Context, cmd *cobra.Command, client *Client, keyID string) error {
	log := logger.FromContext(ctx)

	// Get force flag
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}

	log.Debug("revoking API key in JSON mode",
		"key_id", keyID,
		"force", force)

	// If not forced, we should show a warning (in a real implementation,
	// we'd show affected resources)
	if !force {
		// For now, just show a confirmation prompt via stderr
		fmt.Fprintf(os.Stderr, "Warning: This will permanently revoke the API key.\n")
		fmt.Fprintf(os.Stderr, "Use --force to skip this confirmation.\n")
		fmt.Fprintf(os.Stderr, "Continue? (y/N): ")

		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			return outputJSONError("revocation canceled")
		}
		if response != "y" && response != "Y" {
			return outputJSONError("revocation canceled")
		}
	}

	// Revoke the key
	err = client.RevokeKey(ctx, keyID)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to revoke API key: %v", err))
	}

	// Prepare response
	response := map[string]any{
		"message": "API key revoked successfully",
		"key_id":  keyID,
		"revoked": time.Now().Format(time.RFC3339),
	}

	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}

// runRevokeTUI handles TUI mode for key revocation
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
	keys     []models.KeyInfo
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
	keys []models.KeyInfo
}

type revokeKeyRevokedMsg struct{}

// runCreateUserJSON handles JSON mode for user creation
func runCreateUserJSON(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)

	// Parse flags
	email, err := cmd.Flags().GetString("email")
	if err != nil {
		return fmt.Errorf("failed to get email flag: %w", err)
	}
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("failed to get name flag: %w", err)
	}
	role, err := cmd.Flags().GetString("role")
	if err != nil {
		return fmt.Errorf("failed to get role flag: %w", err)
	}

	// Validate role
	if role != roleAdmin && role != roleUser {
		return outputJSONError("invalid role: must be 'admin' or 'user'")
	}

	log.Debug("creating user in JSON mode",
		"email", email,
		"name", name,
		"role", role)

	// Create the user
	req := CreateUserRequest{
		Email: email,
		Name:  name,
		Role:  role,
	}

	user, err := client.CreateUser(ctx, req)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to create user: %v", err))
	}

	// Prepare response
	response := map[string]any{
		"user":    user,
		"message": "User created successfully",
	}

	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}

// runCreateUserTUI handles TUI mode for user creation
func runCreateUserTUI(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)

	// Parse flags for initial values
	email, err := cmd.Flags().GetString("email")
	if err != nil {
		return fmt.Errorf("failed to get email flag: %w", err)
	}
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("failed to get name flag: %w", err)
	}
	role, err := cmd.Flags().GetString("role")
	if err != nil {
		return fmt.Errorf("failed to get role flag: %w", err)
	}

	log.Debug("creating user in TUI mode")

	// Create and run the TUI model
	m := newCreateUserModel(ctx, client, email, name, role)
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
	ctx         context.Context
	client      *Client
	state       createUserState
	email       string
	name        string
	role        string
	created     bool
	createdUser *models.UserInfo
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

// newCreateUserModel creates a new TUI model for user creation
func newCreateUserModel(ctx context.Context, client *Client, email, name, role string) *createUserModel {
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
		email:   email,
		name:    name,
		role:    role,
		spinner: s,
		inputs:  inputs,
	}
}

// Init initializes the model
func (m *createUserModel) Init() tea.Cmd {
	return m.spinner.Tick
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
		return m.handleInputState(msg)
	case stateUserConfirming:
		return m.handleConfirmState(msg)
	case stateUserCompleted:
		return m, tea.Quit
	}
	return m, nil
}

// handleInputState handles keyboard input in the input state
func (m *createUserModel) handleInputState(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		return m, tea.Quit
	case "tab", "down":
		m.cursorPos = (m.cursorPos + 1) % 3
	case "shift+tab", "up":
		m.cursorPos = (m.cursorPos + 2) % 3 // Go backwards
	case keyEnter:
		if err := m.validateInputs(); err != nil {
			m.err = err
			return m, nil
		}
		m.email = m.inputs[0]
		m.name = m.inputs[1]
		m.role = m.inputs[2]
		m.state = stateUserConfirming
		m.err = nil
	case "backspace":
		if m.inputs[m.cursorPos] != "" {
			m.inputs[m.cursorPos] = m.inputs[m.cursorPos][:len(m.inputs[m.cursorPos])-1]
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.inputs[m.cursorPos] += string(msg.Runes)
		}
	}
	return m, nil
}

// handleConfirmState handles keyboard input in the confirm state
func (m *createUserModel) handleConfirmState(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.state = stateUserCreating
		m.loading = true
		cmd := m.createUser()
		return m, cmd
	case "n", "N", "q", keyCtrlC:
		m.state = stateUserInputting
	}
	return m, nil
}

// validateInputs validates the user inputs
func (m *createUserModel) validateInputs() error {
	if m.inputs[0] == "" {
		return fmt.Errorf("email is required")
	}
	if m.inputs[2] != roleAdmin && m.inputs[2] != roleUser {
		return fmt.Errorf("role must be 'admin' or 'user'")
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
		return m.viewInput()
	case stateUserConfirming:
		return m.viewConfirmation()
	case stateUserCreating:
		return fmt.Sprintf("%s Creating user...", m.spinner.View())
	case stateUserCompleted:
		return m.viewDone()
	}

	return ""
}

// viewInput renders the input form
func (m *createUserModel) viewInput() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1, 2).
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("69"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57"))

	content := titleStyle.Render("👤 Create New User") + "\n\n"

	fields := []struct {
		label string
		value string
		hint  string
	}{
		{"Email", m.inputs[0], "(required)"},
		{"Name", m.inputs[1], "(optional)"},
		{"Role", m.inputs[2], "(admin or user)"},
	}

	for i, field := range fields {
		label := labelStyle.Render(field.label + ": ")
		value := field.value
		if value == "" {
			value = field.hint
		}

		if i == m.cursorPos {
			content += "> " + label + selectedStyle.Render(value+"█") + "\n"
		} else {
			content += "  " + label + inputStyle.Render(value) + "\n"
		}
	}

	content += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Tab/↑↓ to navigate • Enter to confirm • q to quit")

	return style.Render(content)
}

// viewConfirmation renders the confirmation view
func (m *createUserModel) viewConfirmation() string {
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

	content := titleStyle.Render("⚠️  Confirm User Creation") + "\n\n"
	content += "You are about to create the following user:\n\n"
	content += keyStyle.Render(fmt.Sprintf("  Email: %s", m.email)) + "\n"
	if m.name != "" {
		content += fmt.Sprintf("  Name: %s\n", m.name)
	}
	content += fmt.Sprintf("  Role: %s\n", m.role)
	content += "\nCreate this user? (y/N)"

	return style.Render(content)
}

// viewDone renders the completion view
func (m *createUserModel) viewDone() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1, 2).
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("69"))

	content := titleStyle.Render("✅ User Created Successfully!") + "\n\n"
	if m.createdUser != nil {
		content += fmt.Sprintf("User ID: %s\n", m.createdUser.ID)
		content += fmt.Sprintf("Email: %s\n", m.createdUser.Email)
		if m.createdUser.Name != "" {
			content += fmt.Sprintf("Name: %s\n", m.createdUser.Name)
		}
		content += fmt.Sprintf("Role: %s\n", m.createdUser.Role)
		content += fmt.Sprintf("Created: %s\n", m.createdUser.CreatedAt)
	}
	content += "\nPress any key to exit"

	return style.Render(content)
}

// createUser creates the user
func (m *createUserModel) createUser() tea.Cmd {
	return func() tea.Msg {
		req := CreateUserRequest{
			Email: m.email,
			Name:  m.name,
			Role:  m.role,
		}
		user, err := m.client.CreateUser(m.ctx, req)
		if err != nil {
			return errMsg{err}
		}
		return userCreatedMsg{user: user}
	}
}

// Message types for the create user TUI
type userCreatedMsg struct {
	user *models.UserInfo
}

// userFilters holds the parsed command line flags for user filtering
type userFilters struct {
	roleFilter string
	sortBy     string
	filterStr  string
	activeOnly bool
}

// parseListUsersFlags extracts and validates flags for user listing
func parseListUsersFlags(cmd *cobra.Command) (*userFilters, error) {
	roleFilter, err := cmd.Flags().GetString("role")
	if err != nil {
		return nil, fmt.Errorf("failed to get role flag: %w", err)
	}
	sortBy, err := cmd.Flags().GetString("sort")
	if err != nil {
		return nil, fmt.Errorf("failed to get sort flag: %w", err)
	}
	filterStr, err := cmd.Flags().GetString("filter")
	if err != nil {
		return nil, fmt.Errorf("failed to get filter flag: %w", err)
	}
	activeOnly, err := cmd.Flags().GetBool("active")
	if err != nil {
		return nil, fmt.Errorf("failed to get active flag: %w", err)
	}

	// Validate role filter
	if roleFilter != "" && roleFilter != roleAdmin && roleFilter != roleUser {
		return nil, fmt.Errorf("invalid role filter: %s (must be 'admin' or 'user')", roleFilter)
	}

	// Validate sort field
	validSorts := []string{sortCreated, sortName, sortEmail, sortRole}
	validSort := false
	for _, valid := range validSorts {
		if sortBy == valid {
			validSort = true
			break
		}
	}
	if !validSort {
		return nil, fmt.Errorf("invalid sort field: %s (must be one of: %v)", sortBy, validSorts)
	}

	return &userFilters{
		roleFilter: roleFilter,
		sortBy:     sortBy,
		filterStr:  filterStr,
		activeOnly: activeOnly,
	}, nil
}

// filterAndSortUsers applies filtering and sorting to the user list
func filterAndSortUsers(users []models.UserInfo, filters *userFilters) []models.UserInfo {
	filtered := make([]models.UserInfo, 0, len(users))

	for _, user := range users {
		// Apply role filter
		if filters.roleFilter != "" && user.Role != filters.roleFilter {
			continue
		}

		// Apply text filter (name or email)
		if filters.filterStr != "" {
			if !contains(user.Name, filters.filterStr) && !contains(user.Email, filters.filterStr) {
				continue
			}
		}

		// TODO: Apply active filter when KeyCount field is available
		// For now, we skip the active filter
		if filters.activeOnly {
			// Skip for now - would need API key count information
			continue
		}

		filtered = append(filtered, user)
	}

	// Sort users
	sort.Slice(filtered, func(i, j int) bool {
		switch filters.sortBy {
		case sortName:
			return filtered[i].Name < filtered[j].Name
		case sortEmail:
			return filtered[i].Email < filtered[j].Email
		case sortRole:
			return filtered[i].Role < filtered[j].Role
		case sortCreated:
			return filtered[i].CreatedAt < filtered[j].CreatedAt
		default:
			return filtered[i].CreatedAt < filtered[j].CreatedAt
		}
	})

	return filtered
}

// runListUsersJSON handles JSON mode for user listing
func runListUsersJSON(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)

	// Parse flags
	filters, err := parseListUsersFlags(cmd)
	if err != nil {
		return err
	}

	log.Debug("listing users in JSON mode",
		"role", filters.roleFilter,
		"sort", filters.sortBy,
		"filter", filters.filterStr,
		"activeOnly", filters.activeOnly)

	// Get users from API
	users, err := client.ListUsers(ctx)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to list users: %v", err))
	}

	// Apply filters and sorting
	filteredUsers := filterAndSortUsers(users, filters)

	// Prepare response
	response := map[string]any{
		"users": filteredUsers,
		"total": len(filteredUsers),
	}

	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}

// runListUsersTUI handles TUI mode for user listing
func runListUsersTUI(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)

	// Parse flags for initial values
	roleFilter, err := cmd.Flags().GetString("role")
	if err != nil {
		return fmt.Errorf("failed to get role flag: %w", err)
	}
	sortBy, err := cmd.Flags().GetString("sort")
	if err != nil {
		return fmt.Errorf("failed to get sort flag: %w", err)
	}
	filterStr, err := cmd.Flags().GetString("filter")
	if err != nil {
		return fmt.Errorf("failed to get filter flag: %w", err)
	}

	log.Debug("listing users in TUI mode")

	// Create and run the TUI model
	m := newListUsersModel(ctx, client, roleFilter, sortBy, filterStr)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Check for errors
	if model, ok := finalModel.(*listUsersModel); ok && model.err != nil {
		return model.err
	}

	return nil
}

// listUsersModel represents the TUI model for user listing
type listUsersModel struct {
	ctx        context.Context
	client     *Client
	users      []models.UserInfo
	filtered   []models.UserInfo
	selected   int
	offset     int
	pageSize   int
	filter     string
	roleFilter string
	sortBy     string
	width      int
	height     int
	loading    bool
	err        error
	spinner    spinner.Model
	inputMode  bool
}

// newListUsersModel creates a new TUI model for user listing
func newListUsersModel(ctx context.Context, client *Client, roleFilter, sortBy, filter string) *listUsersModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	return &listUsersModel{
		ctx:        ctx,
		client:     client,
		pageSize:   10,
		filter:     filter,
		roleFilter: roleFilter,
		sortBy:     sortBy,
		loading:    true,
		spinner:    s,
	}
}

// Init initializes the model
func (m *listUsersModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadUsers(),
		m.spinner.Tick,
	)
}

// Update handles messages
func (m *listUsersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Adjust page size based on window height
		m.pageSize = maxInt(5, m.height-10)
		return m, nil

	case tea.KeyMsg:
		if m.inputMode {
			return m.handleInputMode(msg)
		}
		return m.handleNormalMode(msg)

	case usersListLoadedMsg:
		m.loading = false
		m.users = msg.users
		m.applyFilters()
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

// handleNavigation handles navigation key presses
func (m *listUsersModel) handleNavigation(key string) {
	switch key {
	case "up", "k":
		if m.selected > 0 {
			m.selected--
			if m.selected < m.offset {
				m.offset = m.selected
			}
		}
	case keyDown, "j":
		if m.selected < len(m.filtered)-1 {
			m.selected++
			if m.selected >= m.offset+m.pageSize {
				m.offset = m.selected - m.pageSize + 1
			}
		}
	case "pgup":
		m.selected = maxInt(0, m.selected-m.pageSize)
		m.offset = maxInt(0, m.offset-m.pageSize)
	case "pgdn":
		maxSelected := len(m.filtered) - 1
		m.selected = minInt(maxSelected, m.selected+m.pageSize)
		if m.selected >= m.offset+m.pageSize {
			m.offset = minInt(maxSelected-m.pageSize+1, m.selected-m.pageSize+1)
		}
	case "home":
		m.selected = 0
		m.offset = 0
	case "end":
		m.selected = len(m.filtered) - 1
		m.offset = maxInt(0, len(m.filtered)-m.pageSize)
	}
}

// cycleRoleFilter cycles through role filter options
func (m *listUsersModel) cycleRoleFilter() {
	switch m.roleFilter {
	case "":
		m.roleFilter = "admin"
	case "admin":
		m.roleFilter = "user"
	case "user":
		m.roleFilter = ""
	}
	m.applyFilters()
}

// cycleSortBy cycles through sort field options
func (m *listUsersModel) cycleSortBy() {
	switch m.sortBy {
	case "created":
		m.sortBy = "name"
	case sortName:
		m.sortBy = "email"
	case sortEmail:
		m.sortBy = "role"
	case sortRole:
		m.sortBy = "created"
	}
	m.applyFilters()
}

// handleNormalMode handles key presses in normal mode
func (m *listUsersModel) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "/":
		m.inputMode = true
	case "r":
		m.cycleRoleFilter()
	case "s":
		m.cycleSortBy()
	case keyEnter:
		// TODO: Quick actions for user edit/delete
	default:
		m.handleNavigation(key)
	}

	return m, nil
}

// handleInputMode handles key presses in filter input mode
func (m *listUsersModel) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "escape":
		m.inputMode = false

	case keyEnter:
		m.inputMode = false
		m.applyFilters()

	case "backspace":
		if m.filter != "" {
			m.filter = m.filter[:len(m.filter)-1]
			m.applyFilters()
		}

	default:
		if len(msg.String()) == 1 {
			m.filter += msg.String()
			m.applyFilters()
		}
	}

	return m, nil
}

// renderHeader renders the UI header section
func (m *listUsersModel) renderHeader() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		MarginBottom(1).
		Render("User Management")

	filterStr := m.renderFilterInfo()
	sortStr := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(fmt.Sprintf("Sort: %s", m.sortBy))

	var result strings.Builder
	result.WriteString(header + "\n")
	if filterStr != "" {
		result.WriteString(filterStr + "  ")
	}
	result.WriteString(sortStr + "\n\n")
	return result.String()
}

// renderFilterInfo renders the filter information line
func (m *listUsersModel) renderFilterInfo() string {
	filterInfo := []string{}
	if m.roleFilter != "" {
		filterInfo = append(filterInfo, fmt.Sprintf("Role: %s", m.roleFilter))
	}
	if m.filter != "" {
		filterInfo = append(filterInfo, fmt.Sprintf("Search: %s", m.filter))
	}
	if len(filterInfo) > 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(fmt.Sprintf("Filters: %s", strings.Join(filterInfo, ", ")))
	}
	return ""
}

// renderTable renders the user table
func (m *listUsersModel) renderTable() string {
	tableHeader := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("69")).
		Render(fmt.Sprintf("%-30s %-20s %-10s %-10s %s",
			"Email", "Name", "Role", "Keys", "Created"))

	var userList []string
	end := minInt(m.offset+m.pageSize, len(m.filtered))
	for i := m.offset; i < end; i++ {
		user := m.filtered[i]
		style := lipgloss.NewStyle()
		prefix := "  "

		if i == m.selected {
			style = style.Background(lipgloss.Color("238"))
			prefix = "> "
		}

		row := fmt.Sprintf("%-30s %-20s %-10s %-10s %s",
			truncate(user.Email, 30),
			truncate(user.Name, 20),
			user.Role,
			"-",            // KeyCount not available yet
			user.CreatedAt) // CreatedAt is already a string

		userList = append(userList, style.Render(prefix+row))
	}

	var result strings.Builder
	result.WriteString(tableHeader + "\n")
	result.WriteString(strings.Repeat("─", 90) + "\n")
	result.WriteString(strings.Join(userList, "\n"))
	return result.String()
}

// renderFooter renders the status bar and help text
func (m *listUsersModel) renderFooter() string {
	end := minInt(m.offset+m.pageSize, len(m.filtered))
	status := fmt.Sprintf("Showing %d-%d of %d users",
		m.offset+1,
		end,
		len(m.filtered))
	if len(m.filtered) != len(m.users) {
		status += fmt.Sprintf(" (filtered from %d total)", len(m.users))
	}

	statusBar := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1).
		Render(status)

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1).
		Render("↑/↓: Navigate  /: Search  r: Filter role  s: Change sort  q: Quit")

	var result strings.Builder
	result.WriteString("\n" + statusBar)
	result.WriteString("\n" + help)

	if m.inputMode {
		result.WriteString("\n\nSearch: " + m.filter + "█")
	}

	return result.String()
}

// View renders the UI
func (m *listUsersModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s Loading users...\n\n  Press Ctrl+C to cancel", m.spinner.View())
	}

	var view strings.Builder
	view.WriteString(m.renderHeader())
	view.WriteString(m.renderTable())
	view.WriteString(m.renderFooter())

	return view.String()
}

// applyFilters applies the current filters and sorting to the user list
func (m *listUsersModel) applyFilters() {
	m.filtered = []models.UserInfo{}

	// Apply filters
	for _, user := range m.users {
		// Filter by role
		if m.roleFilter != "" && user.Role != m.roleFilter {
			continue
		}

		// Filter by search string
		if m.filter != "" {
			lowerFilter := strings.ToLower(m.filter)
			if !strings.Contains(strings.ToLower(user.Email), lowerFilter) &&
				!strings.Contains(strings.ToLower(user.Name), lowerFilter) {
				continue
			}
		}

		m.filtered = append(m.filtered, user)
	}

	// Apply sorting
	switch m.sortBy {
	case sortName:
		sort.Slice(m.filtered, func(i, j int) bool {
			return m.filtered[i].Name < m.filtered[j].Name
		})
	case sortEmail:
		sort.Slice(m.filtered, func(i, j int) bool {
			return m.filtered[i].Email < m.filtered[j].Email
		})
	case sortRole:
		sort.Slice(m.filtered, func(i, j int) bool {
			return m.filtered[i].Role < m.filtered[j].Role
		})
	default:
		sort.Slice(m.filtered, func(i, j int) bool {
			return m.filtered[i].CreatedAt < m.filtered[j].CreatedAt
		})
	}

	// Reset selection if out of bounds
	if m.selected >= len(m.filtered) {
		m.selected = maxInt(0, len(m.filtered)-1)
	}
	if m.offset >= len(m.filtered) {
		m.offset = maxInt(0, len(m.filtered)-m.pageSize)
	}
}

// loadUsers loads the user list from the API
func (m *listUsersModel) loadUsers() tea.Cmd {
	return func() tea.Msg {
		users, err := m.client.ListUsers(m.ctx)
		if err != nil {
			return errMsg{err}
		}
		return usersListLoadedMsg{users: users}
	}
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Message types for the list users TUI
type usersListLoadedMsg struct {
	users []models.UserInfo
}

// runUpdateUserJSON handles JSON mode for user updates
func runUpdateUserJSON(ctx context.Context, cmd *cobra.Command, client *Client, userID string) error {
	log := logger.FromContext(ctx)

	// Parse flags
	email, err := cmd.Flags().GetString("email")
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to get email flag: %v", err))
	}
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to get name flag: %v", err))
	}
	role, err := cmd.Flags().GetString("role")
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to get role flag: %v", err))
	}

	// Validate role if provided
	if role != "" && role != roleUser && role != roleAdmin {
		return outputJSONError("role must be 'user' or 'admin'")
	}

	// Create update request with only specified fields
	req := UpdateUserRequest{}
	if email != "" {
		req.Email = &email
	}
	if name != "" {
		req.Name = &name
	}
	if role != "" {
		req.Role = &role
	}

	// Check if any fields were provided
	if req.Email == nil && req.Name == nil && req.Role == nil {
		return outputJSONError("at least one field (email, name, role) must be provided")
	}

	log.Debug("updating user in JSON mode", "user_id", userID)

	// Update the user
	user, err := client.UpdateUser(ctx, userID, req)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to update user: %v", err))
	}

	// Output JSON response
	response := map[string]any{
		"user":    user,
		"message": "User updated successfully",
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}

// runUpdateUserTUI handles TUI mode for user updates
func runUpdateUserTUI(ctx context.Context, _ *cobra.Command, client *Client, userID string) error {
	log := logger.FromContext(ctx)

	log.Debug("updating user in TUI mode", "user_id", userID)

	// Create and run the TUI model
	m := newUpdateUserModel(ctx, client, userID)
	p := tea.NewProgram(&m)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Check if the operation completed successfully
	model, ok := finalModel.(*updateUserModel)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}
	if model.err != nil {
		return model.err
	}

	if model.IsQuitting() && !model.success {
		fmt.Println("Operation canceled")
	}

	return nil
}

// runDeleteUserJSON handles JSON mode for user deletion
func runDeleteUserJSON(ctx context.Context, cmd *cobra.Command, client *Client, userID string) error {
	log := logger.FromContext(ctx)

	// Parse flags
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}
	cascade, err := cmd.Flags().GetBool("cascade")
	if err != nil {
		return fmt.Errorf("failed to get cascade flag: %w", err)
	}

	log.Debug("deleting user in JSON mode", "user_id", userID, "force", force, "cascade", cascade)

	// If not forced, show warning and require confirmation
	if !force {
		return outputJSONError("user deletion requires --force flag in JSON mode for safety")
	}

	// TODO: If cascade is enabled, also delete user's API keys
	// This would require additional API endpoint or client method
	if cascade {
		log.Debug("cascade deletion requested but not yet implemented")
	}

	// Delete the user
	err = client.DeleteUser(ctx, userID)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to delete user: %v", err))
	}

	// Output JSON response
	response := map[string]any{
		"message": "User deleted successfully",
		"user_id": userID,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}

// runDeleteUserTUI handles TUI mode for user deletion
func runDeleteUserTUI(ctx context.Context, _ *cobra.Command, client *Client, userID string) error {
	log := logger.FromContext(ctx)

	log.Debug("deleting user in TUI mode", "user_id", userID)

	// Create and run the TUI model
	m := newDeleteUserModel(ctx, client, userID)
	p := tea.NewProgram(&m)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Check if the operation completed successfully
	model, ok := finalModel.(*deleteUserModel)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}
	if model.err != nil {
		return model.err
	}

	if model.IsQuitting() && !model.success {
		fmt.Println("Operation canceled")
	}

	return nil
}

// updateUserModel represents the TUI model for updating users
type updateUserModel struct {
	models.BaseModel
	client    *Client
	userID    string
	user      *models.UserInfo
	fields    map[string]string
	selected  int
	editing   bool
	editField string
	editValue string
	spinner   spinner.Model
	loading   bool
	success   bool
	err       error
}

// newUpdateUserModel creates a new update user model
func newUpdateUserModel(ctx context.Context, client *Client, userID string) updateUserModel {
	s := spinner.New()
	s.Spinner = spinner.Dot

	return updateUserModel{
		BaseModel: models.NewBaseModel(ctx, models.ModeTUI),
		client:    client,
		userID:    userID,
		fields: map[string]string{
			"email": "",
			"name":  "",
			"role":  "",
		},
		spinner: s,
		loading: true,
	}
}

// Init implements tea.Model
func (m *updateUserModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadUser(),
	)
}

// Update implements tea.Model
func (m *updateUserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle base model updates
	cmd := m.BaseModel.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if m.IsQuitting() {
		return m, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editing {
			return m.handleEditingKeys(msg, cmds)
		}
		return m.handleNormalKeys(msg, cmds)

	case userLoadedMsg:
		m.user = &msg.user
		m.fields["email"] = m.user.Email
		m.fields["name"] = m.user.Name
		m.fields["role"] = m.user.Role
		m.loading = false

	case userUpdatedMsg:
		m.success = true
		m.loading = false
		m.Quit()

	case errMsg:
		m.err = msg.err
		m.loading = false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.loading {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// handleEditingKeys handles key presses while editing a field
func (m *updateUserModel) handleEditingKeys(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEnter:
		m.fields[m.editField] = m.editValue
		m.editing = false
		m.editField = ""
		m.editValue = ""

	case "esc":
		m.editing = false
		m.editField = ""
		m.editValue = ""

	default:
		switch msg.Type {
		case tea.KeyBackspace:
			if m.editValue != "" {
				m.editValue = m.editValue[:len(m.editValue)-1]
			}
		case tea.KeyRunes:
			m.editValue += string(msg.Runes)
		}
	}

	return m, tea.Batch(cmds...)
}

// handleNormalKeys handles key presses in normal mode
func (m *updateUserModel) handleNormalKeys(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, tea.Batch(cmds...)
	}

	fieldNames := []string{"email", "name", "role"}

	switch msg.String() {
	case keyDown, "j":
		m.selected = (m.selected + 1) % (len(fieldNames) + 1) // +1 for save option

	case "up", "k":
		m.selected = (m.selected - 1 + len(fieldNames) + 1) % (len(fieldNames) + 1)

	case keyEnter:
		if m.selected < len(fieldNames) {
			// Edit field
			fieldName := fieldNames[m.selected]
			m.editing = true
			m.editField = fieldName
			m.editValue = m.fields[fieldName]
		} else {
			// Save changes
			m.loading = true
			cmds = append(cmds, m.spinner.Tick, m.saveUser())
		}
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m *updateUserModel) View() string {
	if m.IsQuitting() {
		if m.success {
			return "✅ User updated successfully!\n"
		}
		return ""
	}

	if m.loading {
		return fmt.Sprintf("\n%s Loading user information...\n", m.spinner.View())
	}

	if m.err != nil {
		return fmt.Sprintf("❌ Error: %v\n", m.err)
	}

	var b strings.Builder
	b.WriteString("\n📝 Update User\n\n")

	if m.user != nil {
		b.WriteString(fmt.Sprintf("User ID: %s\n\n", m.user.ID))

		fieldNames := []string{"email", "name", "role"}
		labels := []string{"Email", "Name", "Role"}

		for i, fieldName := range fieldNames {
			label := labels[i]
			value := m.fields[fieldName]

			if m.editing && m.editField == fieldName {
				b.WriteString(fmt.Sprintf("  %s: %s█\n", label, m.editValue))
			} else {
				marker := "  "
				if m.selected == i {
					marker = "▶ "
				}
				b.WriteString(fmt.Sprintf("%s%s: %s\n", marker, label, value))
			}
		}

		b.WriteString("\n")

		// Save option
		marker := "  "
		if m.selected == len(fieldNames) {
			marker = "▶ "
		}
		b.WriteString(fmt.Sprintf("%s💾 Save Changes\n\n", marker))

		if m.editing {
			b.WriteString("Press Enter to save, Esc to cancel\n")
		} else {
			b.WriteString("Use ↑/↓ to navigate, Enter to edit/save, Ctrl+C to quit\n")
		}
	}

	return b.String()
}

// loadUser loads the user information
func (m *updateUserModel) loadUser() tea.Cmd {
	return func() tea.Msg {
		// Get current user list to find the user
		users, err := m.client.ListUsers(m.Context())
		if err != nil {
			return errMsg{err}
		}

		for _, user := range users {
			if user.ID == m.userID {
				return userLoadedMsg{user: user}
			}
		}

		return errMsg{fmt.Errorf("user with ID %s not found", m.userID)}
	}
}

// saveUser saves the user changes
func (m *updateUserModel) saveUser() tea.Cmd {
	return func() tea.Msg {
		req := UpdateUserRequest{}
		changed := false

		if m.fields["email"] != m.user.Email {
			email := m.fields["email"]
			req.Email = &email
			changed = true
		}
		if m.fields["name"] != m.user.Name {
			name := m.fields["name"]
			req.Name = &name
			changed = true
		}
		if m.fields["role"] != m.user.Role {
			role := m.fields["role"]
			req.Role = &role
			changed = true
		}

		if !changed {
			return errMsg{fmt.Errorf("no changes detected")}
		}

		updatedUser, err := m.client.UpdateUser(m.Context(), m.userID, req)
		if err != nil {
			return errMsg{err}
		}

		return userUpdatedMsg{user: *updatedUser}
	}
}

// deleteUserModel represents the TUI model for deleting users
type deleteUserModel struct {
	models.BaseModel
	client      *Client
	userID      string
	user        *models.UserInfo
	showConfirm bool
	selected    int
	spinner     spinner.Model
	loading     bool
	success     bool
	err         error
}

// newDeleteUserModel creates a new delete user model
func newDeleteUserModel(ctx context.Context, client *Client, userID string) deleteUserModel {
	s := spinner.New()
	s.Spinner = spinner.Dot

	return deleteUserModel{
		BaseModel: models.NewBaseModel(ctx, models.ModeTUI),
		client:    client,
		userID:    userID,
		spinner:   s,
		loading:   true,
	}
}

// Init implements tea.Model
func (m *deleteUserModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadUser(),
	)
}

// Update implements tea.Model
func (m *deleteUserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle base model updates
	cmd := m.BaseModel.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if m.IsQuitting() {
		return m, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeys(msg, cmds)

	case userLoadedMsg:
		m.user = &msg.user
		m.loading = false
		m.showConfirm = true

	case userDeletedMsg:
		m.success = true
		m.loading = false
		m.Quit()

	case errMsg:
		m.err = msg.err
		m.loading = false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.loading {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// handleKeys handles key presses
func (m *deleteUserModel) handleKeys(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if m.loading || !m.showConfirm {
		return m, tea.Batch(cmds...)
	}

	switch msg.String() {
	case "left", "h":
		m.selected = 0 // No

	case "right", "l":
		m.selected = 1 // Yes

	case keyEnter:
		if m.selected == 1 { // Yes
			m.loading = true
			m.showConfirm = false
			cmds = append(cmds, m.spinner.Tick, m.deleteUser())
		} else { // No
			m.Quit()
		}
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m *deleteUserModel) View() string {
	if m.IsQuitting() {
		if m.success {
			return "✅ User deleted successfully!\n"
		}
		return ""
	}

	if m.loading {
		if m.user == nil {
			return fmt.Sprintf("\n%s Loading user information...\n", m.spinner.View())
		}
		return fmt.Sprintf("\n%s Deleting user...\n", m.spinner.View())
	}

	if m.err != nil {
		return fmt.Sprintf("❌ Error: %v\n", m.err)
	}

	if m.showConfirm && m.user != nil {
		var b strings.Builder
		b.WriteString("\n🗑️  Delete User\n\n")
		b.WriteString(fmt.Sprintf("User: %s (%s)\n", m.user.Name, m.user.Email))
		b.WriteString(fmt.Sprintf("Role: %s\n", m.user.Role))
		b.WriteString(fmt.Sprintf("ID: %s\n\n", m.user.ID))

		b.WriteString("⚠️  This action cannot be undone!\n\n")
		b.WriteString("Are you sure you want to delete this user?\n\n")

		// Yes/No buttons
		noStyle := "  No  "
		yesStyle := "  Yes "
		if m.selected == 0 {
			noStyle = "▶ No ◀"
		} else {
			yesStyle = "▶ Yes ◀"
		}

		b.WriteString(fmt.Sprintf("%s    %s\n\n", noStyle, yesStyle))
		b.WriteString("Use ←/→ to select, Enter to confirm, Ctrl+C to quit\n")

		return b.String()
	}

	return ""
}

// loadUser loads the user information for deletion
func (m *deleteUserModel) loadUser() tea.Cmd {
	return func() tea.Msg {
		// Get current user list to find the user
		users, err := m.client.ListUsers(m.Context())
		if err != nil {
			return errMsg{err}
		}

		for _, user := range users {
			if user.ID == m.userID {
				return userLoadedMsg{user: user}
			}
		}

		return errMsg{fmt.Errorf("user with ID %s not found", m.userID)}
	}
}

// deleteUser deletes the user
func (m *deleteUserModel) deleteUser() tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteUser(m.Context(), m.userID)
		if err != nil {
			return errMsg{err}
		}

		return userDeletedMsg{userID: m.userID}
	}
}

// Message types for user operations
type userLoadedMsg struct {
	user models.UserInfo
}

type userUpdatedMsg struct {
	user models.UserInfo
}

type userDeletedMsg struct {
	userID string
}
