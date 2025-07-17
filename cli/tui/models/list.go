package models

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/pkg/logger"
)

// Sort field constants
const (
	sortByCreated  = "created"
	sortByName     = "name"
	sortByLastUsed = "last_used"
)

// SortOrder represents the sort direction
type SortOrder int

const (
	SortAscending SortOrder = iota
	SortDescending
)

// ListModel represents a generic TUI model for listing items
type ListModel[T ListableItem] struct {
	BaseModel
	// UI components
	table     table.Model
	spinner   spinner.Model
	searchBox textinput.Model
	// Data
	client   DataClient[T]
	items    []T
	filtered []T
	// State
	loading     bool
	searching   bool
	sortBy      string
	sortOrder   SortOrder
	currentPage int
	pageSize    int
	// Configuration
	columns []table.Column
	// Styling
	styles listStyles
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

// NewListModel creates a new generic list model
func NewListModel[T ListableItem](ctx context.Context, client DataClient[T], columns []table.Column) *ListModel[T] {
	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	// Create search box
	searchBox := textinput.New()
	searchBox.Placeholder = "Search items..."
	searchBox.Prompt = "🔍 "
	searchBox.CharLimit = 50
	searchBox.Width = 30
	// Create table
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
	return &ListModel[T]{
		BaseModel: NewBaseModel(ctx, ModeTUI),
		table:     t,
		spinner:   s,
		searchBox: searchBox,
		client:    client,
		loading:   true,
		sortBy:    sortByCreated,
		sortOrder: SortDescending, // Default to descending for created date
		pageSize:  50,
		columns:   columns,
		styles:    newListStyles(),
	}
}

// Init initializes the model
func (m *ListModel[T]) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadItems(),
	)
}

// Update handles messages
func (m *ListModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case itemsLoadedMsg[T]:
		m.handleItemsLoaded(msg)
	case errMsg:
		m.handleError(msg)
		// Allow user to see the error and potentially retry
		m.loading = false
		return m, nil
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
func (m *ListModel[T]) handleWindowSize(msg tea.WindowSizeMsg) {
	m.table.SetHeight(msg.Height - 10) // Leave room for header and status
}

// handleKeyMsg handles keyboard input
func (m *ListModel[T]) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
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
func (m *ListModel[T]) startSearch() tea.Cmd {
	m.searching = true
	m.searchBox.Focus()
	return textinput.Blink
}

// handleEscape handles the escape key
func (m *ListModel[T]) handleEscape() {
	if m.searching {
		m.searching = false
		m.searchBox.Blur()
		m.searchBox.SetValue("")
		m.applyFilter()
	}
}

// handleEnter handles the enter key
func (m *ListModel[T]) handleEnter() {
	if m.searching {
		m.searching = false
		m.searchBox.Blur()
		m.applyFilter()
	}
}

// handleSort handles sort key
func (m *ListModel[T]) handleSort() {
	if !m.searching {
		m.cycleSortBy()
		m.updateTable()
	}
}

// handleNextPage handles next page navigation
func (m *ListModel[T]) handleNextPage() {
	if !m.searching && m.hasNextPage() {
		m.currentPage++
		m.updateTable()
	}
}

// handlePrevPage handles previous page navigation
func (m *ListModel[T]) handlePrevPage() {
	if !m.searching && m.hasPrevPage() {
		m.currentPage--
		m.updateTable()
	}
}

// handleRefresh handles refresh key
func (m *ListModel[T]) handleRefresh() tea.Cmd {
	if !m.searching {
		m.loading = true
		m.SetError(nil) // Clear any previous error when retrying
		return m.loadItems()
	}
	return nil
}

// handleItemsLoaded handles successful item loading
func (m *ListModel[T]) handleItemsLoaded(msg itemsLoadedMsg[T]) {
	m.loading = false
	m.items = msg.items
	m.filtered = m.items
	m.applyFilter()
	m.updateTable()
}

// handleError handles error messages
func (m *ListModel[T]) handleError(msg errMsg) {
	m.loading = false
	m.SetError(msg.err)
}

// handleSpinnerTick handles spinner animation
func (m *ListModel[T]) handleSpinnerTick(msg spinner.TickMsg) tea.Cmd {
	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return cmd
	}
	return nil
}

// updateComponents updates UI components based on state
func (m *ListModel[T]) updateComponents(msg tea.Msg) tea.Cmd {
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
func (m *ListModel[T]) View() string {
	if m.Error() != nil {
		return m.styles.error.Render(fmt.Sprintf("❌ Error: %v", m.Error()))
	}
	if m.loading {
		return fmt.Sprintf("%s Loading items...", m.spinner.View())
	}
	var s strings.Builder
	// Title
	s.WriteString(m.styles.title.Render("📋 Items"))
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
func (m *ListModel[T]) buildStatusBar() string {
	totalFiltered := len(m.filtered)
	totalItems := len(m.items)
	startIdx := m.currentPage * m.pageSize
	endIdx := startIdx + m.pageSize
	if endIdx > totalFiltered {
		endIdx = totalFiltered
	}
	status := fmt.Sprintf("Showing %d-%d of %d", startIdx+1, endIdx, totalFiltered)
	if totalFiltered < totalItems {
		status += fmt.Sprintf(" (filtered from %d)", totalItems)
	}
	sortDirection := "↑"
	if m.sortOrder == SortDescending {
		sortDirection = "↓"
	}
	status += fmt.Sprintf(" | Sort: %s %s", m.sortBy, sortDirection)
	if m.currentPage > 0 || m.hasNextPage() {
		status += fmt.Sprintf(" | Page %d/%d", m.currentPage+1, m.totalPages())
	}
	return status
}

// buildHelp creates the help text
func (m *ListModel[T]) buildHelp() string {
	if m.searching {
		return "esc: cancel • enter: apply filter"
	}
	// Show retry option when there's an error
	if m.Error() != nil {
		return "r: retry • q: quit"
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

// applyFilter filters items based on search term
func (m *ListModel[T]) applyFilter() {
	searchTerm := strings.ToLower(m.searchBox.Value())
	if searchTerm == "" {
		m.filtered = m.items
	} else {
		m.filtered = make([]T, 0)
		for _, item := range m.items {
			if item.MatchesSearch(searchTerm) {
				m.filtered = append(m.filtered, item)
			}
		}
	}
	// Reset to first page when filtering
	m.currentPage = 0
	m.updateTable()
}

// updateTable updates the table with current data
func (m *ListModel[T]) updateTable() {
	// Sort the filtered items
	m.sortItems(m.filtered, m.sortBy, m.sortOrder)
	// Get current page of items
	startIdx := m.currentPage * m.pageSize
	endIdx := startIdx + m.pageSize
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}
	// Convert to table rows
	rows := make([]table.Row, 0)
	for i := startIdx; i < endIdx; i++ {
		item := m.filtered[i]
		row := make(table.Row, len(m.columns))
		for j, col := range m.columns {
			row[j] = item.GetDisplayValue(col.Title)
		}
		rows = append(rows, row)
	}
	m.table.SetRows(rows)
}

// sortItems sorts items based on the specified field and order
func (m *ListModel[T]) sortItems(items []T, sortBy string, order SortOrder) {
	sort.Slice(items, func(i, j int) bool {
		keyI := items[i].GetSortKey(sortBy)
		keyJ := items[j].GetSortKey(sortBy)

		// Compare based on type using type switch
		var ascending bool

		switch valI := keyI.(type) {
		case time.Time:
			if valJ, ok := keyJ.(time.Time); ok {
				ascending = valI.Before(valJ)
			}
		case string:
			if valJ, ok := keyJ.(string); ok {
				ascending = valI < valJ
			}
		case int:
			if valJ, ok := keyJ.(int); ok {
				ascending = valI < valJ
			}
		case int64:
			if valJ, ok := keyJ.(int64); ok {
				ascending = valI < valJ
			}
		case float64:
			if valJ, ok := keyJ.(float64); ok {
				ascending = valI < valJ
			}
		default:
			return false
		}

		// Apply sort order
		if order == SortDescending {
			return !ascending
		}
		return ascending
	})
}

// cycleSortBy cycles through sort options and toggles order
func (m *ListModel[T]) cycleSortBy() {
	switch m.sortBy {
	case sortByCreated:
		m.sortBy = sortByName
		m.sortOrder = SortAscending // Names usually sorted ascending
	case sortByName:
		m.sortBy = sortByLastUsed
		m.sortOrder = SortDescending // Recent usage first
	case sortByLastUsed:
		m.sortBy = sortByCreated
		m.sortOrder = SortDescending // Recent creation first
	}
}

// hasNextPage returns true if there's a next page
func (m *ListModel[T]) hasNextPage() bool {
	return (m.currentPage+1)*m.pageSize < len(m.filtered)
}

// hasPrevPage returns true if there's a previous page
func (m *ListModel[T]) hasPrevPage() bool {
	return m.currentPage > 0
}

// totalPages returns the total number of pages
func (m *ListModel[T]) totalPages() int {
	if len(m.filtered) == 0 {
		return 1
	}
	return (len(m.filtered) + m.pageSize - 1) / m.pageSize
}

// loadItems loads the items using the client
func (m *ListModel[T]) loadItems() tea.Cmd {
	return func() tea.Msg {
		log := logger.FromContext(m.Context())
		log.Debug("loading items")
		items, err := m.client.ListKeys(m.Context())
		if err != nil {
			return errMsg{err}
		}
		return itemsLoadedMsg[T]{items: items}
	}
}

// Message types
type itemsLoadedMsg[T any] struct {
	items []T
}

type errMsg struct {
	err error
}
