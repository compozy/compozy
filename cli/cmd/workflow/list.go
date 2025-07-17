package workflow

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	cliutils "github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/cli/tui/components"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	// Terminal and table constants
	defaultTerminalWidth     = 80
	terminalPaddingSpaces    = 8 // 4 spaces between columns + 4 for padding
	minTerminalWidthForTable = 40
	headerSeparatorPadding   = 4 // 4 spaces between columns

	// Default pagination constants
	defaultWorkflowLimit  = 50
	defaultWorkflowOffset = 0

	// Minimum column widths for readability
	minIDColumnWidth      = 8
	minStatusColumnWidth  = 8
	minNameColumnWidth    = 12
	minCreatedColumnWidth = 10
	minUpdatedColumnWidth = 10

	// Maximum column widths to prevent excessive space usage
	maxIDColumnWidth      = 20
	maxStatusColumnWidth  = 12
	maxNameColumnWidth    = 30
	maxCreatedColumnWidth = 20
	maxUpdatedColumnWidth = 20

	// Column width ratios (as divisors for proportional calculation)
	idColumnRatio      = 6 // ~16% of width (availableWidth/6)
	statusColumnRatio  = 8 // ~12% of width (availableWidth/8)
	nameColumnRatio    = 3 // ~33% of width (availableWidth/3)
	createdColumnRatio = 5 // ~20% of width (availableWidth/5)
	updatedColumnRatio = 5 // ~20% of width (availableWidth/5)

	// HTTP and timeout constants
	httpRequestTimeoutSeconds = 30

	// TUI layout constants
	tuiHeaderReservedSpace = 2

	// API and validation constants
	maxWorkflowLimit      = 1000
	errorMessageMaxLength = 200

	// Date format constants
	dateTimeFormat = "2006-01-02 15:04"
)

// columnWidths represents the width of each column in the table
type columnWidths struct {
	id      int
	status  int
	name    int
	created int
	updated int
}

// calculateColumnWidths calculates dynamic column widths based on terminal width
func calculateColumnWidths() columnWidths {
	// Get terminal width, default to defaultTerminalWidth if not available
	termWidth := defaultTerminalWidth
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		termWidth = width
	}

	// Reserve space for separators and padding
	availableWidth := termWidth - terminalPaddingSpaces

	// Handle very narrow terminals
	if availableWidth < minTerminalWidthForTable {
		return columnWidths{
			id:      minIDColumnWidth,
			status:  minStatusColumnWidth,
			name:    minNameColumnWidth,
			created: minCreatedColumnWidth,
			updated: minUpdatedColumnWidth,
		}
	}

	// Calculate proportional widths with minimum guarantees
	return columnWidths{
		id:      maxInt(minIDColumnWidth, minInt(maxIDColumnWidth, availableWidth/idColumnRatio)),
		status:  maxInt(minStatusColumnWidth, minInt(maxStatusColumnWidth, availableWidth/statusColumnRatio)),
		name:    maxInt(minNameColumnWidth, minInt(maxNameColumnWidth, availableWidth/nameColumnRatio)),
		created: maxInt(minCreatedColumnWidth, minInt(maxCreatedColumnWidth, availableWidth/createdColumnRatio)),
		updated: maxInt(minUpdatedColumnWidth, minInt(maxUpdatedColumnWidth, availableWidth/updatedColumnRatio)),
	}
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

// ListCmd creates the workflow list command
func ListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workflows",
		Long:  "List all workflows with optional filtering and sorting.",
		RunE:  runList,
	}

	// Global flags are now centralized in cli/flags/global.go

	// Add filtering flags
	cmd.Flags().String("status", "", "Filter by workflow status")
	cmd.Flags().StringSlice("tags", []string{}, "Filter by workflow tags")
	cmd.Flags().Int("limit", defaultWorkflowLimit, "Maximum number of workflows to return")
	cmd.Flags().Int("offset", defaultWorkflowOffset, "Number of workflows to skip")
	cmd.Flags().String("sort", "name", "Sort by field (name, created_at, updated_at, status)")
	cmd.Flags().String("order", "asc", "Sort order (asc, desc)")

	return cmd
}

// runList handles the workflow list command execution using the unified executor pattern
func runList(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
	}, cmd.ModeHandlers{
		JSON: listJSONHandler,
		TUI:  listTUIHandler,
	}, args)
}

// listJSONHandler handles JSON mode for workflow list
func listJSONHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	return handleListJSON(ctx, cobraCmd, executor.GetAuthClient(), nil)
}

// listTUIHandler handles TUI mode for workflow list
func listTUIHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	return handleListTUI(ctx, cobraCmd, executor.GetAuthClient(), nil)
}

// handleListJSON handles workflow list command in JSON mode
func handleListJSON(ctx context.Context, cmd *cobra.Command, client api.AuthClient, _ []string) error {
	log := logger.FromContext(ctx)

	// Validate sort flags
	if err := validateSortFlags(cmd); err != nil {
		return err
	}

	// Parse filters from flags
	filters, err := parseFiltersFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("invalid filters: %w", err)
	}

	// Get workflows from API
	workflows, err := getWorkflows(ctx, client, &filters)
	if err != nil {
		return fmt.Errorf("failed to fetch workflows: %w", err)
	}

	// Create JSON formatter with pretty printing enabled
	formatter := cliutils.NewJSONFormatter(true)

	// Format workflow list response
	jsonOutput, err := formatter.FormatWorkflowList(workflows, len(workflows), filters.Limit, filters.Offset)
	if err != nil {
		return fmt.Errorf("failed to format JSON response: %w", err)
	}

	// Output the formatted JSON
	fmt.Println(jsonOutput)

	log.Debug("workflows listed successfully", "count", len(workflows), "mode", "json")
	return nil
}

// handleListTUI handles workflow list command in TUI mode
func handleListTUI(ctx context.Context, cmd *cobra.Command, client api.AuthClient, _ []string) error {
	log := logger.FromContext(ctx)

	// Validate sort flags
	if err := validateSortFlags(cmd); err != nil {
		return err
	}

	// Parse filters from flags
	filters, err := parseFiltersFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("invalid filters: %w", err)
	}

	// Get workflows from API
	workflows, err := getWorkflows(ctx, client, &filters)
	if err != nil {
		return fmt.Errorf("failed to fetch workflows: %w", err)
	}

	// Use interactive TUI with WorkflowTableComponent
	if err := runWorkflowTUI(ctx, workflows); err != nil {
		log.Error("failed to run workflow TUI", "error", err)
		// Fall back to simple table display if TUI fails
		displayWorkflowTable(workflows)
	}

	log.Debug("workflows listed successfully", "count", len(workflows), "mode", "tui")
	return nil
}

// runWorkflowTUI runs the interactive TUI for workflow listing
func runWorkflowTUI(ctx context.Context, workflows []api.Workflow) error {
	log := logger.FromContext(ctx)

	// Create workflow table component
	tableComponent := components.NewWorkflowTableComponent(workflows)

	// Create a simple TUI program model
	model := &workflowTUIModel{
		table: tableComponent,
	}

	// Create TUI program
	program := tea.NewProgram(model, tea.WithAltScreen())

	log.Debug("starting workflow TUI", "workflow_count", len(workflows))

	// Run the program
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("failed to run TUI program: %w", err)
	}

	return nil
}

// workflowTUIModel implements the bubbletea.Model interface for workflow listing
type workflowTUIModel struct {
	table components.WorkflowTableComponent
}

// Init initializes the TUI model
func (m *workflowTUIModel) Init() tea.Cmd {
	return nil
}

// Update handles TUI updates
func (m *workflowTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			// Handle selection
			selected := m.table.GetSelectedWorkflow()
			if selected != nil {
				// For now, just quit - in a full implementation, this would show details
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		// Update table size
		m.table.SetSize(msg.Width, msg.Height-tuiHeaderReservedSpace) // Reserve space for header
	}

	// Update table component
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the TUI
func (m *workflowTUIModel) View() string {
	help := "Press 'q' to quit, 'enter' to select workflow"
	return fmt.Sprintf("%s\n\n%s", m.table.View(), help)
}

// parseFiltersFromFlags parses workflow filters from command flags
func parseFiltersFromFlags(cmd *cobra.Command) (api.WorkflowFilters, error) {
	var filters api.WorkflowFilters
	if err := parseBasicFilters(cmd, &filters); err != nil {
		return filters, err
	}
	if err := parsePaginationFilters(cmd, &filters); err != nil {
		return filters, err
	}
	if err := parseSortFilters(cmd, &filters); err != nil {
		return filters, err
	}
	return filters, nil
}

// parseBasicFilters parses status and tags filters
func parseBasicFilters(cmd *cobra.Command, filters *api.WorkflowFilters) error {
	status, err := cmd.Flags().GetString("status")
	if err != nil {
		return fmt.Errorf("failed to get status flag: %w", err)
	}
	if status != "" {
		filters.Status = status
	}
	tags, err := cmd.Flags().GetStringSlice("tags")
	if err != nil {
		return fmt.Errorf("failed to get tags flag: %w", err)
	}
	if len(tags) > 0 {
		filters.Tags = tags
	}
	return nil
}

// parsePaginationFilters parses limit and offset filters
func parsePaginationFilters(cmd *cobra.Command, filters *api.WorkflowFilters) error {
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return fmt.Errorf("failed to get limit flag: %w", err)
	}
	if limit < 0 {
		return fmt.Errorf("limit must be non-negative, got: %d", limit)
	}
	if limit > maxWorkflowLimit {
		return fmt.Errorf("limit too large (max %d), got: %d", maxWorkflowLimit, limit)
	}
	if limit > 0 {
		filters.Limit = limit
	}
	offset, err := cmd.Flags().GetInt("offset")
	if err != nil {
		return fmt.Errorf("failed to get offset flag: %w", err)
	}
	if offset < 0 {
		return fmt.Errorf("offset must be non-negative, got: %d", offset)
	}
	if offset >= 0 {
		filters.Offset = offset
	}
	return nil
}

// parseSortFilters parses sort and order filters
func parseSortFilters(cmd *cobra.Command, filters *api.WorkflowFilters) error {
	sort, err := cmd.Flags().GetString("sort")
	if err != nil {
		return fmt.Errorf("failed to get sort flag: %w", err)
	}
	if sort != "" {
		filters.SortBy = sort
	}
	order, err := cmd.Flags().GetString("order")
	if err != nil {
		return fmt.Errorf("failed to get order flag: %w", err)
	}
	if order != "" {
		filters.SortOrder = order
	}
	return nil
}

// validateSortFlags validates sort and order flags
func validateSortFlags(cmd *cobra.Command) error {
	sort, err := cmd.Flags().GetString("sort")
	if err != nil {
		return fmt.Errorf("failed to get sort flag: %w", err)
	}
	if err := cliutils.ValidateEnum(sort, []string{"name", "created_at", "updated_at", "status"}, "sort"); err != nil {
		return err
	}
	order, err := cmd.Flags().GetString("order")
	if err != nil {
		return fmt.Errorf("failed to get order flag: %w", err)
	}
	if err := cliutils.ValidateEnum(order, []string{"asc", "desc"}, "order"); err != nil {
		return err
	}
	return nil
}

// getWorkflows fetches workflows from the API using the real API client
func getWorkflows(
	ctx context.Context,
	authClient api.AuthClient,
	filters *api.WorkflowFilters,
) ([]api.Workflow, error) {
	log := logger.FromContext(ctx)

	// Create API client from auth client configuration
	workflowService := createAPIClient(authClient)

	// Use the workflow service to list workflows
	log.Debug("fetching workflows from API", "filters", filters)
	workflows, err := workflowService.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	// Handle edge case: empty workflow list
	if len(workflows) == 0 {
		log.Info("no workflows found matching the specified filters")
	}

	log.Debug("workflows fetched successfully", "count", len(workflows))
	return workflows, nil
}

// createAPIClient creates an API client from the auth client configuration
func createAPIClient(authClient api.AuthClient) api.WorkflowService {
	// Create HTTP client using shared utility with custom timeout
	config := &cliutils.HTTPClientConfig{
		Timeout: time.Duration(httpRequestTimeoutSeconds) * time.Second,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
	}
	client := cliutils.CreateHTTPClient(authClient, config)

	return &workflowAPIService{
		authClient: authClient,
		httpClient: client,
	}
}

// workflowAPIService implements the WorkflowService interface using the auth client
type workflowAPIService struct {
	authClient api.AuthClient
	httpClient *resty.Client
}

// List implements the WorkflowService.List method using the auth client
func (s *workflowAPIService) List(ctx context.Context, filters *api.WorkflowFilters) ([]api.Workflow, error) {
	log := logger.FromContext(ctx)
	req := s.prepareWorkflowListRequest(ctx, filters)
	result, err := s.executeWorkflowListRequest(req)
	if err != nil {
		return nil, err
	}
	log.Debug("workflows fetched successfully", "count", len(result.Data.Workflows))
	return result.Data.Workflows, nil
}

// prepareWorkflowListRequest prepares the HTTP request with query parameters
func (s *workflowAPIService) prepareWorkflowListRequest(
	ctx context.Context,
	filters *api.WorkflowFilters,
) *resty.Request {
	req := s.httpClient.R().SetContext(ctx)
	if filters.Status != "" {
		req.SetQueryParam("status", filters.Status)
	}
	if len(filters.Tags) > 0 {
		// URL encode each tag individually to handle special characters
		encodedTags := make([]string, len(filters.Tags))
		for i, tag := range filters.Tags {
			encodedTags[i] = url.QueryEscape(tag)
		}
		req.SetQueryParam("tags", strings.Join(encodedTags, ","))
	}
	if filters.Limit > 0 {
		req.SetQueryParam("limit", fmt.Sprintf("%d", filters.Limit))
	}
	if filters.Offset > 0 {
		req.SetQueryParam("offset", fmt.Sprintf("%d", filters.Offset))
	}
	if filters.SortBy != "" {
		req.SetQueryParam("sort", filters.SortBy)
	}
	if filters.SortOrder != "" {
		req.SetQueryParam("order", filters.SortOrder)
	}
	return req
}

// executeWorkflowListRequest executes the HTTP request and handles errors
func (s *workflowAPIService) executeWorkflowListRequest(req *resty.Request) (*struct {
	Data struct {
		Workflows []api.Workflow `json:"workflows"`
	} `json:"data"`
}, error) {
	var result struct {
		Data struct {
			Workflows []api.Workflow `json:"workflows"`
		} `json:"data"`
	}
	resp, err := req.SetResult(&result).Get("/workflows")
	if err != nil {
		return nil, s.handleRequestError(err)
	}
	if resp.StatusCode() >= http.StatusBadRequest {
		return nil, s.handleHTTPError(resp)
	}
	return &result, nil
}

// handleRequestError handles network and request errors
func (s *workflowAPIService) handleRequestError(err error) error {
	if cliutils.IsNetworkError(err) {
		return fmt.Errorf(
			"network error: unable to connect to Compozy server - check your connection and server status: %w",
			err,
		)
	}
	if cliutils.IsTimeoutError(err) {
		return fmt.Errorf("request timed out: server may be busy, try again later: %w", err)
	}
	return fmt.Errorf("failed to list workflows: %w", err)
}

// handleHTTPError handles HTTP status errors
func (s *workflowAPIService) handleHTTPError(resp *resty.Response) error {
	switch resp.StatusCode() {
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed: please check your API key or login credentials")
	case http.StatusForbidden:
		return fmt.Errorf("permission denied: you don't have access to list workflows")
	case http.StatusNotFound:
		return fmt.Errorf("workflow endpoint not found: server may be misconfigured")
	default:
		if resp.StatusCode() >= http.StatusInternalServerError {
			return fmt.Errorf("server error (status %d): try again later or contact support", resp.StatusCode())
		}
		errorMsg := cliutils.SanitizeForJSON(resp.String())
		if len(errorMsg) > errorMessageMaxLength {
			errorMsg = errorMsg[:errorMessageMaxLength] + "..."
		}
		return fmt.Errorf("API error: %s (status %d)", errorMsg, resp.StatusCode())
	}
}

// Get implements the WorkflowService.Get method using the auth client
func (s *workflowAPIService) Get(ctx context.Context, id core.ID) (*api.WorkflowDetail, error) {
	log := logger.FromContext(ctx)

	// Make the API call
	var result struct {
		Data api.WorkflowDetail `json:"data"`
	}

	resp, err := s.httpClient.R().SetContext(ctx).SetResult(&result).Get(fmt.Sprintf("/workflows/%s", id))
	if err != nil {
		// Handle network errors
		if cliutils.IsNetworkError(err) {
			return nil, fmt.Errorf("network error: unable to connect to Compozy server: %w", err)
		}
		if cliutils.IsTimeoutError(err) {
			return nil, fmt.Errorf("request timed out: server may be busy: %w", err)
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Handle HTTP errors
	if resp.StatusCode() >= http.StatusBadRequest {
		if resp.StatusCode() == http.StatusUnauthorized {
			return nil, fmt.Errorf("authentication failed: please check your API key or login credentials")
		}
		if resp.StatusCode() == http.StatusForbidden {
			return nil, fmt.Errorf("permission denied: you don't have access to this workflow")
		}
		if resp.StatusCode() == http.StatusNotFound {
			return nil, fmt.Errorf("workflow not found: workflow with ID %s does not exist", id)
		}
		if resp.StatusCode() >= http.StatusInternalServerError {
			return nil, fmt.Errorf("server error (status %d): try again later", resp.StatusCode())
		}
		return nil, fmt.Errorf("API error: %s (status %d)", resp.String(), resp.StatusCode())
	}

	log.Debug("workflow retrieved successfully", "workflow_id", id)
	return &result.Data, nil
}

// displayWorkflowTable displays workflows in a simple table format
func displayWorkflowTable(workflows []api.Workflow) {
	if len(workflows) == 0 {
		fmt.Println("\n📋 No workflows found.")
		fmt.Println("\n💡 Try:")
		fmt.Println("  • Creating a new workflow")
		fmt.Println("  • Adjusting your filters (--status, --tags)")
		fmt.Println("  • Checking your permissions")
		return
	}

	// Calculate dynamic column widths based on terminal width
	widths := calculateColumnWidths()

	// Print header
	fmt.Printf("%-*s %-*s %-*s %-*s %-*s\n",
		widths.id, "ID",
		widths.status, "STATUS",
		widths.name, "NAME",
		widths.created, "CREATED",
		widths.updated, "UPDATED")

	totalWidth := widths.id + widths.status + widths.name + widths.created + widths.updated +
		headerSeparatorPadding // 4 spaces between columns
	fmt.Println(strings.Repeat("-", totalWidth))

	// Print workflows
	for i := range workflows {
		fmt.Printf("%-*s %-*s %-*s %-*s %-*s\n",
			widths.id, cliutils.Truncate(string(workflows[i].ID), widths.id),
			widths.status, string(workflows[i].Status),
			widths.name, cliutils.Truncate(workflows[i].Name, widths.name),
			widths.created, workflows[i].CreatedAt.Format(dateTimeFormat),
			widths.updated, workflows[i].UpdatedAt.Format(dateTimeFormat),
		)
	}

	fmt.Printf("\nTotal: %d workflows\n", len(workflows))
}
