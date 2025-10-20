package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/api"
	cliutils "github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/cli/tui/styles"
)

type SortOrder string

// Sort direction constants
const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

// WorkflowTableComponent provides an interactive workflow table
type WorkflowTableComponent struct {
	table        table.Model
	workflows    []api.Workflow
	filteredRows []table.Row
	width        int
	height       int
	focused      bool

	// Filtering and sorting
	filterTerm    string
	sortColumn    string
	sortDirection SortOrder

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
		SortByName:    newWorkflowBinding([]string{"1"}, "sort by name", "1"),
		SortByStatus:  newWorkflowBinding([]string{"2"}, "sort by status", "2"),
		SortByCreated: newWorkflowBinding([]string{"3"}, "sort by created", "3"),
		SortByUpdated: newWorkflowBinding([]string{"4"}, "sort by updated", "4"),
		Filter:        newWorkflowBinding([]string{"/"}, "filter", "/"),
		ClearFilter:   newWorkflowBinding([]string{"esc"}, "clear filter", "esc"),
		NextPage:      newWorkflowBinding([]string{"n", "right"}, "next page", "n/→"),
		PrevPage:      newWorkflowBinding([]string{"p", "left"}, "prev page", "p/←"),
		FirstPage:     newWorkflowBinding([]string{"home"}, "first page", "home"),
		LastPage:      newWorkflowBinding([]string{"end"}, "last page", "end"),
		Refresh:       newWorkflowBinding([]string{"r"}, "refresh", "r"),
		Select:        newWorkflowBinding([]string{"enter"}, "select", "enter"),
	}
}

// NewWorkflowTableComponent creates a new workflow table component
func NewWorkflowTableComponent(workflows []api.Workflow) WorkflowTableComponent {
	columns := buildWorkflowTableColumns()
	tableModel := newWorkflowTableModel(columns)
	component := WorkflowTableComponent{
		table:         tableModel,
		workflows:     workflows,
		sortColumn:    "name",
		sortDirection: SortOrderAsc,
		currentPage:   0,
		itemsPerPage:  20,
		totalItems:    len(workflows),
		keyMap:        DefaultWorkflowTableKeyMap(),
	}
	component.updateFilteredRows()
	component.updateTableRows()
	return component
}

func newWorkflowBinding(keys []string, help, display string) key.Binding {
	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(display, help),
	)
}

func buildWorkflowTableColumns() []table.Column {
	return []table.Column{
		{Title: "ID", Width: 15},
		{Title: "Name", Width: 25},
		{Title: "Status", Width: 12},
		{Title: "Version", Width: 10},
		{Title: "Created", Width: 12},
		{Title: "Updated", Width: 12},
		{Title: "Tags", Width: 20},
	}
}

func newWorkflowTableModel(columns []table.Column) table.Model {
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(defaultWorkflowTableStyles())
	return t
}

func defaultWorkflowTableStyles() table.Styles {
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
	return s
}

// SetSize sets the table size
func (wt *WorkflowTableComponent) SetSize(width, height int) *WorkflowTableComponent {
	wt.width = width
	wt.height = height
	tableHeight := max(1, height-4) // Reserve space for header and pagination
	wt.table.SetHeight(tableHeight)
	if width < 40 {
		columns := []table.Column{
			{Title: "Name", Width: max(8, width/2)},
			{Title: "Status", Width: max(6, width/3)},
		}
		wt.table.SetColumns(columns)
		return wt
	}
	availableWidth := width - 10 // Reserve space for borders and padding
	columns := []table.Column{
		{Title: "ID", Width: max(8, min(15, availableWidth/7))},
		{Title: "Name", Width: max(10, min(25, availableWidth/4))},
		{Title: "Status", Width: max(6, min(12, availableWidth/8))},
		{Title: "Version", Width: max(5, min(10, availableWidth/10))},
		{Title: "Created", Width: max(8, min(12, availableWidth/8))},
		{Title: "Updated", Width: max(8, min(12, availableWidth/8))},
		{Title: "Tags", Width: max(8, min(20, availableWidth/6))},
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
func (wt *WorkflowTableComponent) SetWorkflows(workflows []api.Workflow) *WorkflowTableComponent {
	wt.workflows = workflows
	wt.totalItems = len(workflows)
	wt.updateFilteredRows()
	wt.updateTableRows()
	return wt
}

// GetSelectedWorkflow returns the currently selected workflow
func (wt *WorkflowTableComponent) GetSelectedWorkflow() *api.Workflow {
	if len(wt.filteredRows) == 0 {
		return nil
	}
	selectedIndex := wt.table.Cursor()
	if selectedIndex < 0 || selectedIndex >= len(wt.filteredRows) {
		return nil
	}
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
			return *wt, tea.Cmd(func() tea.Msg {
				return WorkflowRefreshMsg{}
			})
		case key.Matches(keyMsg, wt.keyMap.Select):
			selected := wt.GetSelectedWorkflow()
			if selected != nil {
				return *wt, tea.Cmd(func() tea.Msg {
					return WorkflowSelectedMsg{Workflow: *selected}
				})
			}
		}
	}
	wt.table, cmd = wt.table.Update(msg)
	return *wt, cmd
}

// View renders the table
func (wt *WorkflowTableComponent) View() string {
	if wt.width <= 0 || wt.height <= 0 {
		return ""
	}
	var sections []string
	header := wt.renderHeader()
	sections = append(sections, header)
	tableView := wt.table.View()
	sections = append(sections, tableView)
	pagination := wt.renderPagination()
	sections = append(sections, pagination)
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the table header with sort and filter info
func (wt *WorkflowTableComponent) renderHeader() string {
	var parts []string
	sortIndicator := fmt.Sprintf("Sort: %s %s", wt.sortColumn, wt.sortDirection)
	parts = append(parts, styles.InfoStyle.Render(sortIndicator))
	if wt.filterTerm != "" {
		filterIndicator := fmt.Sprintf("Filter: %s", wt.filterTerm)
		parts = append(parts, styles.WarningStyle.Render(filterIndicator))
	}
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
	endItem := min(startItem+wt.itemsPerPage-1, len(wt.filteredRows))
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
		if wt.sortDirection == SortOrderAsc {
			wt.sortDirection = SortOrderDesc
		} else {
			wt.sortDirection = SortOrderAsc
		}
	} else {
		wt.sortColumn = column
		wt.sortDirection = SortOrderAsc
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
	rows := make([]table.Row, 0, len(wt.workflows))
	for i := range wt.workflows {
		workflow := &wt.workflows[i]
		if wt.filterTerm != "" && !wt.matchesFilter(workflow) {
			continue
		}

		tags := strings.Join(workflow.Tags, ", ")
		if len(tags) > 18 {
			tags = tags[:15] + "..."
		}

		row := table.Row{
			string(workflow.ID),
			cliutils.Truncate(workflow.Name, 23),
			string(workflow.Status),
			workflow.Version,
			workflow.CreatedAt.Format("2006-01-02"),
			workflow.UpdatedAt.Format("2006-01-02"),
			tags,
		}
		rows = append(rows, row)
	}
	wt.sortRows(rows)
	wt.filteredRows = rows
}

// matchesFilter checks if a workflow matches the current filter
func (wt *WorkflowTableComponent) matchesFilter(workflow *api.Workflow) bool {
	if cliutils.Contains(workflow.Name, wt.filterTerm) {
		return true
	}
	if cliutils.Contains(string(workflow.Status), wt.filterTerm) {
		return true
	}
	for _, tag := range workflow.Tags {
		if cliutils.Contains(tag, wt.filterTerm) {
			return true
		}
	}
	return cliutils.Contains(workflow.Description, wt.filterTerm)
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

		if wt.sortDirection == SortOrderDesc {
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
	start := wt.currentPage * wt.itemsPerPage
	end := min(start+wt.itemsPerPage, len(wt.filteredRows))
	if start >= len(wt.filteredRows) {
		start = 0
		wt.currentPage = 0
		end = min(wt.itemsPerPage, len(wt.filteredRows))
	}
	pageRows := wt.filteredRows[start:end]
	wt.table.SetRows(pageRows)
}

// Message types for parent component communication
type WorkflowRefreshMsg struct{}
type WorkflowSelectedMsg struct {
	Workflow api.Workflow
}
