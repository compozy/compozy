package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/services"
	"github.com/compozy/compozy/cli/tui/styles"
)

// Sort direction constants
const (
	SortAsc  = "asc"
	SortDesc = "desc"
)

// WorkflowTableComponent provides an interactive workflow table
type WorkflowTableComponent struct {
	table        table.Model
	workflows    []services.Workflow
	filteredRows []table.Row
	width        int
	height       int
	focused      bool

	// Filtering and sorting
	filterTerm    string
	sortColumn    string
	sortDirection string // "asc" or "desc"

	// Pagination
	currentPage  int
	itemsPerPage int
	totalItems   int

	// Key bindings
	keyMap WorkflowTableKeyMap
}

// WorkflowTableKeyMap defines key bindings for the workflow table
type WorkflowTableKeyMap struct {
	SortByName    key.Binding
	SortByStatus  key.Binding
	SortByCreated key.Binding
	SortByUpdated key.Binding
	Filter        key.Binding
	ClearFilter   key.Binding
	NextPage      key.Binding
	PrevPage      key.Binding
	FirstPage     key.Binding
	LastPage      key.Binding
	Refresh       key.Binding
	Select        key.Binding
}

// DefaultWorkflowTableKeyMap returns the default key bindings
func DefaultWorkflowTableKeyMap() WorkflowTableKeyMap {
	return WorkflowTableKeyMap{
		SortByName: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "sort by name"),
		),
		SortByStatus: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "sort by status"),
		),
		SortByCreated: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "sort by created"),
		),
		SortByUpdated: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "sort by updated"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear filter"),
		),
		NextPage: key.NewBinding(
			key.WithKeys("n", "right"),
			key.WithHelp("n/→", "next page"),
		),
		PrevPage: key.NewBinding(
			key.WithKeys("p", "left"),
			key.WithHelp("p/←", "prev page"),
		),
		FirstPage: key.NewBinding(
			key.WithKeys("home"),
			key.WithHelp("home", "first page"),
		),
		LastPage: key.NewBinding(
			key.WithKeys("end"),
			key.WithHelp("end", "last page"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
	}
}

// NewWorkflowTableComponent creates a new workflow table component
func NewWorkflowTableComponent(workflows []services.Workflow) WorkflowTableComponent {
	// Create table columns
	columns := []table.Column{
		{Title: "ID", Width: 15},
		{Title: "Name", Width: 25},
		{Title: "Status", Width: 12},
		{Title: "Version", Width: 10},
		{Title: "Created", Width: 12},
		{Title: "Updated", Width: 12},
		{Title: "Tags", Width: 20},
	}

	// Create table model
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Apply custom styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(styles.Border).
		BorderBottom(true).
		Bold(true).
		Foreground(styles.Primary)
	s.Selected = s.Selected.
		Foreground(styles.Highlight).
		Background(styles.Surface).
		Bold(true)

	t.SetStyles(s)

	component := WorkflowTableComponent{
		table:         t,
		workflows:     workflows,
		sortColumn:    "name",
		sortDirection: SortAsc,
		currentPage:   0,
		itemsPerPage:  20,
		totalItems:    len(workflows),
		keyMap:        DefaultWorkflowTableKeyMap(),
	}

	// Initial data setup
	component.updateFilteredRows()
	component.updateTableRows()

	return component
}

// SetSize sets the table size
func (wt *WorkflowTableComponent) SetSize(width, height int) *WorkflowTableComponent {
	wt.width = width
	wt.height = height

	// Update table size
	wt.table.SetHeight(height - 4) // Reserve space for header and pagination

	// Adjust column widths based on available space
	availableWidth := width - 10 // Reserve space for borders and padding
	columns := []table.Column{
		{Title: "ID", Width: minInt(15, availableWidth/7)},
		{Title: "Name", Width: minInt(25, availableWidth/4)},
		{Title: "Status", Width: minInt(12, availableWidth/8)},
		{Title: "Version", Width: minInt(10, availableWidth/10)},
		{Title: "Created", Width: minInt(12, availableWidth/8)},
		{Title: "Updated", Width: minInt(12, availableWidth/8)},
		{Title: "Tags", Width: minInt(20, availableWidth/6)},
	}
	wt.table.SetColumns(columns)

	return wt
}

// SetFocused sets the focus state
func (wt *WorkflowTableComponent) SetFocused(focused bool) *WorkflowTableComponent {
	wt.focused = focused
	wt.table.Focus()
	if !focused {
		wt.table.Blur()
	}
	return wt
}

// SetWorkflows updates the workflows data
func (wt *WorkflowTableComponent) SetWorkflows(workflows []services.Workflow) *WorkflowTableComponent {
	wt.workflows = workflows
	wt.totalItems = len(workflows)
	wt.updateFilteredRows()
	wt.updateTableRows()
	return wt
}

// GetSelectedWorkflow returns the currently selected workflow
func (wt *WorkflowTableComponent) GetSelectedWorkflow() *services.Workflow {
	if len(wt.filteredRows) == 0 {
		return nil
	}

	selectedIndex := wt.table.Cursor()
	if selectedIndex < 0 || selectedIndex >= len(wt.filteredRows) {
		return nil
	}

	// Find the workflow by ID from the first column
	workflowID := wt.filteredRows[selectedIndex][0]
	for i := range wt.workflows {
		if string(wt.workflows[i].ID) == workflowID {
			return &wt.workflows[i]
		}
	}

	return nil
}

// Update handles component updates
func (wt *WorkflowTableComponent) Update(msg tea.Msg) (WorkflowTableComponent, tea.Cmd) {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, wt.keyMap.SortByName):
			wt.setSortColumn("name")
		case key.Matches(keyMsg, wt.keyMap.SortByStatus):
			wt.setSortColumn("status")
		case key.Matches(keyMsg, wt.keyMap.SortByCreated):
			wt.setSortColumn("created")
		case key.Matches(keyMsg, wt.keyMap.SortByUpdated):
			wt.setSortColumn("updated")
		case key.Matches(keyMsg, wt.keyMap.NextPage):
			wt.nextPage()
		case key.Matches(keyMsg, wt.keyMap.PrevPage):
			wt.prevPage()
		case key.Matches(keyMsg, wt.keyMap.FirstPage):
			wt.firstPage()
		case key.Matches(keyMsg, wt.keyMap.LastPage):
			wt.lastPage()
		case key.Matches(keyMsg, wt.keyMap.ClearFilter):
			wt.clearFilter()
		case key.Matches(keyMsg, wt.keyMap.Refresh):
			// Refresh command can be handled by parent
			return *wt, tea.Cmd(func() tea.Msg {
				return WorkflowRefreshMsg{}
			})
		case key.Matches(keyMsg, wt.keyMap.Select):
			// Select command can be handled by parent
			selected := wt.GetSelectedWorkflow()
			if selected != nil {
				return *wt, tea.Cmd(func() tea.Msg {
					return WorkflowSelectedMsg{Workflow: *selected}
				})
			}
		}
	}

	// Update the table
	wt.table, cmd = wt.table.Update(msg)
	return *wt, cmd
}

// View renders the table
func (wt *WorkflowTableComponent) View() string {
	if wt.width <= 0 || wt.height <= 0 {
		return ""
	}

	var sections []string

	// Header with sort information
	header := wt.renderHeader()
	sections = append(sections, header)

	// Table view
	tableView := wt.table.View()
	sections = append(sections, tableView)

	// Pagination info
	pagination := wt.renderPagination()
	sections = append(sections, pagination)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the table header with sort and filter info
func (wt *WorkflowTableComponent) renderHeader() string {
	var parts []string

	// Sort indicator
	sortIndicator := fmt.Sprintf("Sort: %s %s", wt.sortColumn, wt.sortDirection)
	parts = append(parts, styles.InfoStyle.Render(sortIndicator))

	// Filter indicator
	if wt.filterTerm != "" {
		filterIndicator := fmt.Sprintf("Filter: %s", wt.filterTerm)
		parts = append(parts, styles.WarningStyle.Render(filterIndicator))
	}

	// Total count
	totalIndicator := fmt.Sprintf("Total: %d", len(wt.filteredRows))
	parts = append(parts, styles.HelpStyle.Render(totalIndicator))

	return strings.Join(parts, " • ")
}

// renderPagination renders pagination information
func (wt *WorkflowTableComponent) renderPagination() string {
	if wt.totalItems == 0 {
		return styles.PaginationStyle.Render("No workflows found")
	}

	startItem := wt.currentPage*wt.itemsPerPage + 1
	endItem := minInt(startItem+wt.itemsPerPage-1, len(wt.filteredRows))
	totalPages := (len(wt.filteredRows) + wt.itemsPerPage - 1) / wt.itemsPerPage

	if totalPages == 0 {
		totalPages = 1
	}

	pagination := fmt.Sprintf(
		"Page %d of %d • Items %d-%d of %d",
		wt.currentPage+1,
		totalPages,
		startItem,
		endItem,
		len(wt.filteredRows),
	)

	return styles.PaginationStyle.Render(pagination)
}

// setSortColumn sets the sort column and direction
func (wt *WorkflowTableComponent) setSortColumn(column string) {
	if wt.sortColumn == column {
		// Toggle direction
		if wt.sortDirection == SortAsc {
			wt.sortDirection = SortDesc
		} else {
			wt.sortDirection = SortAsc
		}
	} else {
		wt.sortColumn = column
		wt.sortDirection = SortAsc
	}

	wt.updateFilteredRows()
	wt.updateTableRows()
}

// clearFilter clears the current filter
func (wt *WorkflowTableComponent) clearFilter() {
	wt.filterTerm = ""
	wt.currentPage = 0
	wt.updateFilteredRows()
	wt.updateTableRows()
}

// nextPage goes to the next page
func (wt *WorkflowTableComponent) nextPage() {
	totalPages := (len(wt.filteredRows) + wt.itemsPerPage - 1) / wt.itemsPerPage
	if wt.currentPage < totalPages-1 {
		wt.currentPage++
		wt.updateTableRows()
	}
}

// prevPage goes to the previous page
func (wt *WorkflowTableComponent) prevPage() {
	if wt.currentPage > 0 {
		wt.currentPage--
		wt.updateTableRows()
	}
}

// firstPage goes to the first page
func (wt *WorkflowTableComponent) firstPage() {
	wt.currentPage = 0
	wt.updateTableRows()
}

// lastPage goes to the last page
func (wt *WorkflowTableComponent) lastPage() {
	totalPages := (len(wt.filteredRows) + wt.itemsPerPage - 1) / wt.itemsPerPage
	if totalPages > 0 {
		wt.currentPage = totalPages - 1
		wt.updateTableRows()
	}
}

// updateFilteredRows updates the filtered and sorted rows
func (wt *WorkflowTableComponent) updateFilteredRows() {
	// Convert workflows to rows
	rows := make([]table.Row, 0, len(wt.workflows))

	for i := range wt.workflows {
		workflow := &wt.workflows[i]
		// Apply filter
		if wt.filterTerm != "" && !wt.matchesFilter(workflow) {
			continue
		}

		// Format tags
		tags := strings.Join(workflow.Tags, ", ")
		if len(tags) > 18 {
			tags = tags[:15] + "..."
		}

		row := table.Row{
			string(workflow.ID),
			truncateString(workflow.Name, 23),
			string(workflow.Status),
			workflow.Version,
			workflow.CreatedAt.Format("2006-01-02"),
			workflow.UpdatedAt.Format("2006-01-02"),
			tags,
		}
		rows = append(rows, row)
	}

	// Sort rows
	wt.sortRows(rows)

	wt.filteredRows = rows
}

// matchesFilter checks if a workflow matches the current filter
func (wt *WorkflowTableComponent) matchesFilter(workflow *services.Workflow) bool {
	filterLower := strings.ToLower(wt.filterTerm)

	// Check name
	if strings.Contains(strings.ToLower(workflow.Name), filterLower) {
		return true
	}

	// Check status
	if strings.Contains(strings.ToLower(string(workflow.Status)), filterLower) {
		return true
	}

	// Check tags
	for _, tag := range workflow.Tags {
		if strings.Contains(strings.ToLower(tag), filterLower) {
			return true
		}
	}

	// Check description
	if strings.Contains(strings.ToLower(workflow.Description), filterLower) {
		return true
	}

	return false
}

// sortRows sorts the rows based on current sort settings
func (wt *WorkflowTableComponent) sortRows(rows []table.Row) {
	sort.Slice(rows, func(i, j int) bool {
		var less bool

		switch wt.sortColumn {
		case "name":
			less = strings.ToLower(rows[i][1]) < strings.ToLower(rows[j][1])
		case "status":
			less = rows[i][2] < rows[j][2]
		case "created":
			less = rows[i][4] < rows[j][4]
		case "updated":
			less = rows[i][5] < rows[j][5]
		default:
			less = strings.ToLower(rows[i][1]) < strings.ToLower(rows[j][1])
		}

		if wt.sortDirection == SortDesc {
			less = !less
		}

		return less
	})
}

// updateTableRows updates the table with the current page of rows
func (wt *WorkflowTableComponent) updateTableRows() {
	if len(wt.filteredRows) == 0 {
		wt.table.SetRows([]table.Row{})
		return
	}

	// Calculate page bounds
	start := wt.currentPage * wt.itemsPerPage
	end := minInt(start+wt.itemsPerPage, len(wt.filteredRows))

	// Ensure bounds are valid
	if start >= len(wt.filteredRows) {
		start = 0
		wt.currentPage = 0
		end = minInt(wt.itemsPerPage, len(wt.filteredRows))
	}

	pageRows := wt.filteredRows[start:end]
	wt.table.SetRows(pageRows)
}

// truncateString truncates a string to the specified length
func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Message types for parent component communication
type WorkflowRefreshMsg struct{}
type WorkflowSelectedMsg struct {
	Workflow services.Workflow
}
