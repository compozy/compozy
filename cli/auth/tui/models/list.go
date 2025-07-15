package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/auth/sorting"
	"github.com/compozy/compozy/pkg/logger"
)

// Sort field constants
const (
	sortByCreated  = "created"
	sortByName     = "name"
	sortByLastUsed = "last_used"
)

// ListModel represents the TUI model for listing API keys
type ListModel struct {
	BaseModel
	// UI components
	table     table.Model
	spinner   spinner.Model
	searchBox textinput.Model
	// Data
	client   AuthClient
	keys     []KeyInfo
	filtered []KeyInfo
	// State
	loading     bool
	searching   bool
	sortBy      string
	currentPage int
	pageSize    int
	// Styling
	styles listStyles
}

// AuthClient interface for auth operations
type AuthClient interface {
	ListKeys(ctx context.Context) ([]KeyInfo, error)
}

// listStyles holds all styling for the list view
type listStyles struct {
	title     lipgloss.Style
	help      lipgloss.Style
	searchBox lipgloss.Style
	statusBar lipgloss.Style
	error     lipgloss.Style
}

// newListStyles creates the default styles
func newListStyles() listStyles {
	return listStyles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("69")).
			MarginBottom(1),
		help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		searchBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("69")).
			Padding(0, 1),
		statusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1),
		error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")),
	}
}

// NewListModel creates a new list model
func NewListModel(ctx context.Context, client AuthClient) *ListModel {
	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	// Create search box
	searchBox := textinput.New()
	searchBox.Placeholder = "Search keys..."
	searchBox.Prompt = "🔍 "
	searchBox.CharLimit = 50
	searchBox.Width = 30
	// Create table
	columns := []table.Column{
		{Title: "Prefix", Width: 20},
		{Title: "Created", Width: 20},
		{Title: "Last Used", Width: 20},
		{Title: "Usage Count", Width: 12},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	// Set table styles
	tableStyle := table.DefaultStyles()
	tableStyle.Header = tableStyle.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	tableStyle.Selected = tableStyle.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(tableStyle)
	return &ListModel{
		BaseModel: NewBaseModel(ctx, ModeTUI),
		table:     t,
		spinner:   s,
		searchBox: searchBox,
		client:    client,
		loading:   true,
		sortBy:    sortByCreated,
		pageSize:  50,
		styles:    newListStyles(),
	}
}

// Init initializes the model
func (m *ListModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadKeys(),
	)
}

// Update handles messages
func (m *ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	// Handle base model updates
	if cmd := m.BaseModel.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)
	case tea.KeyMsg:
		if m.loading {
			return m, tea.Batch(cmds...)
		}
		cmd := m.handleKeyMsg(msg)
		if cmd != nil {
			return m, cmd
		}
	case keysLoadedMsg:
		m.handleKeysLoaded(msg)
	case errMsg:
		m.handleError(msg)
		return m, tea.Quit
	case spinner.TickMsg:
		if cmd := m.handleSpinnerTick(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	// Update components
	if cmd := m.updateComponents(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// handleWindowSize handles window resize events
func (m *ListModel) handleWindowSize(msg tea.WindowSizeMsg) {
	m.table.SetHeight(msg.Height - 10) // Leave room for header and status
}

// handleKeyMsg handles keyboard input
func (m *ListModel) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c", "q":
		return tea.Quit
	case "/":
		return m.startSearch()
	case "esc":
		m.handleEscape()
	case "enter":
		m.handleEnter()
	case "s":
		m.handleSort()
	case "n":
		m.handleNextPage()
	case "p":
		m.handlePrevPage()
	case "r":
		return m.handleRefresh()
	}
	return nil
}

// startSearch starts the search mode
func (m *ListModel) startSearch() tea.Cmd {
	m.searching = true
	m.searchBox.Focus()
	return textinput.Blink
}

// handleEscape handles the escape key
func (m *ListModel) handleEscape() {
	if m.searching {
		m.searching = false
		m.searchBox.Blur()
		m.searchBox.SetValue("")
		m.applyFilter()
	}
}

// handleEnter handles the enter key
func (m *ListModel) handleEnter() {
	if m.searching {
		m.searching = false
		m.searchBox.Blur()
		m.applyFilter()
	}
}

// handleSort handles sort key
func (m *ListModel) handleSort() {
	if !m.searching {
		m.cycleSortBy()
		m.updateTable()
	}
}

// handleNextPage handles next page navigation
func (m *ListModel) handleNextPage() {
	if !m.searching && m.hasNextPage() {
		m.currentPage++
		m.updateTable()
	}
}

// handlePrevPage handles previous page navigation
func (m *ListModel) handlePrevPage() {
	if !m.searching && m.hasPrevPage() {
		m.currentPage--
		m.updateTable()
	}
}

// handleRefresh handles refresh key
func (m *ListModel) handleRefresh() tea.Cmd {
	if !m.searching {
		m.loading = true
		return m.loadKeys()
	}
	return nil
}

// handleKeysLoaded handles successful key loading
func (m *ListModel) handleKeysLoaded(msg keysLoadedMsg) {
	m.loading = false
	m.keys = msg.keys
	m.filtered = m.keys
	m.applyFilter()
	m.updateTable()
}

// handleError handles error messages
func (m *ListModel) handleError(msg errMsg) {
	m.loading = false
	m.SetError(msg.err)
}

// handleSpinnerTick handles spinner animation
func (m *ListModel) handleSpinnerTick(msg spinner.TickMsg) tea.Cmd {
	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return cmd
	}
	return nil
}

// updateComponents updates UI components based on state
func (m *ListModel) updateComponents(msg tea.Msg) tea.Cmd {
	if m.searching {
		var cmd tea.Cmd
		m.searchBox, cmd = m.searchBox.Update(msg)
		// Apply filter on each keystroke
		m.applyFilter()
		return cmd
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return cmd
}

// View renders the UI
func (m *ListModel) View() string {
	if m.Error() != nil {
		return m.styles.error.Render(fmt.Sprintf("❌ Error: %v", m.Error()))
	}
	if m.loading {
		return fmt.Sprintf("%s Loading API keys...", m.spinner.View())
	}
	var s strings.Builder
	// Title
	s.WriteString(m.styles.title.Render("🔐 API Keys"))
	s.WriteString("\n\n")
	// Search box (if searching)
	if m.searching {
		s.WriteString(m.styles.searchBox.Render(m.searchBox.View()))
		s.WriteString("\n\n")
	}
	// Table
	s.WriteString(m.table.View())
	s.WriteString("\n")
	// Status bar
	status := m.buildStatusBar()
	s.WriteString(m.styles.statusBar.Render(status))
	// Help
	help := m.buildHelp()
	s.WriteString("\n")
	s.WriteString(m.styles.help.Render(help))
	return s.String()
}

// buildStatusBar creates the status bar text
func (m *ListModel) buildStatusBar() string {
	totalFiltered := len(m.filtered)
	totalKeys := len(m.keys)
	startIdx := m.currentPage * m.pageSize
	endIdx := startIdx + m.pageSize
	if endIdx > totalFiltered {
		endIdx = totalFiltered
	}
	status := fmt.Sprintf("Showing %d-%d of %d", startIdx+1, endIdx, totalFiltered)
	if totalFiltered < totalKeys {
		status += fmt.Sprintf(" (filtered from %d)", totalKeys)
	}
	status += fmt.Sprintf(" | Sort: %s", m.sortBy)
	if m.currentPage > 0 || m.hasNextPage() {
		status += fmt.Sprintf(" | Page %d/%d", m.currentPage+1, m.totalPages())
	}
	return status
}

// buildHelp creates the help text
func (m *ListModel) buildHelp() string {
	if m.searching {
		return "esc: cancel • enter: apply filter"
	}
	help := []string{
		"↑/↓: navigate",
		"/: search",
		"s: sort",
		"r: refresh",
	}
	if m.hasPrevPage() {
		help = append(help, "p: prev page")
	}
	if m.hasNextPage() {
		help = append(help, "n: next page")
	}
	help = append(help, "q: quit")
	return strings.Join(help, " • ")
}

// applyFilter filters keys based on search term
func (m *ListModel) applyFilter() {
	searchTerm := strings.ToLower(m.searchBox.Value())
	if searchTerm == "" {
		m.filtered = m.keys
	} else {
		m.filtered = make([]KeyInfo, 0)
		for _, key := range m.keys {
			if strings.Contains(strings.ToLower(key.Prefix), searchTerm) ||
				strings.Contains(strings.ToLower(key.ID), searchTerm) {
				m.filtered = append(m.filtered, key)
			}
		}
	}
	// Reset to first page when filtering
	m.currentPage = 0
	m.updateTable()
}

// updateTable updates the table with current data
func (m *ListModel) updateTable() {
	// Sort the filtered keys
	sorting.SortKeys(m.filtered, m.sortBy)
	// Get current page of keys
	startIdx := m.currentPage * m.pageSize
	endIdx := startIdx + m.pageSize
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}
	// Convert to table rows
	rows := make([]table.Row, 0)
	for i := startIdx; i < endIdx; i++ {
		key := m.filtered[i]
		lastUsed := "Never"
		if key.LastUsed != nil {
			// Parse and format the timestamp
			if t, err := time.Parse(time.RFC3339, *key.LastUsed); err == nil {
				lastUsed = t.Format("2006-01-02 15:04")
			} else {
				lastUsed = *key.LastUsed
			}
		}
		// Parse and format created timestamp
		created := key.CreatedAt
		if t, err := time.Parse(time.RFC3339, key.CreatedAt); err == nil {
			created = t.Format("2006-01-02 15:04")
		}
		// TODO: Add usage count when available from API
		usageCount := "N/A"
		rows = append(rows, table.Row{
			key.Prefix,
			created,
			lastUsed,
			usageCount,
		})
	}
	m.table.SetRows(rows)
}

// cycleSortBy cycles through sort options
func (m *ListModel) cycleSortBy() {
	switch m.sortBy {
	case sortByCreated:
		m.sortBy = sortByName
	case sortByName:
		m.sortBy = sortByLastUsed
	case sortByLastUsed:
		m.sortBy = sortByCreated
	}
}

// hasNextPage returns true if there's a next page
func (m *ListModel) hasNextPage() bool {
	return (m.currentPage+1)*m.pageSize < len(m.filtered)
}

// hasPrevPage returns true if there's a previous page
func (m *ListModel) hasPrevPage() bool {
	return m.currentPage > 0
}

// totalPages returns the total number of pages
func (m *ListModel) totalPages() int {
	if len(m.filtered) == 0 {
		return 1
	}
	return (len(m.filtered) + m.pageSize - 1) / m.pageSize
}

// loadKeys loads the API keys
func (m *ListModel) loadKeys() tea.Cmd {
	return func() tea.Msg {
		log := logger.FromContext(m.Context())
		log.Debug("loading API keys")
		keys, err := m.client.ListKeys(m.Context())
		if err != nil {
			return errMsg{err}
		}
		return keysLoadedMsg{keys: keys}
	}
}

// Message types
type keysLoadedMsg struct {
	keys []KeyInfo
}

type errMsg struct {
	err error
}
