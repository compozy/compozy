package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/cli/auth"
	"github.com/compozy/compozy/cli/services"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/tidwall/pretty"
)

// ListCmd creates the workflow list command
func ListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workflows",
		Long:  "List all workflows with optional filtering and sorting.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return auth.ExecuteCommand(cmd, auth.ModeHandlers{
				JSON: handleListJSON,
				TUI:  handleListTUI,
			}, args)
		},
	}

	// Add mode flags
	auth.AddModeFlags(cmd)

	// Add filtering flags
	cmd.Flags().String("status", "", "Filter by workflow status")
	cmd.Flags().StringSlice("tags", []string{}, "Filter by workflow tags")
	cmd.Flags().Int("limit", 50, "Maximum number of workflows to return")
	cmd.Flags().Int("offset", 0, "Number of workflows to skip")
	cmd.Flags().String("sort", "name", "Sort by field (name, created_at, updated_at, status)")
	cmd.Flags().String("order", "asc", "Sort order (asc, desc)")

	return cmd
}

// handleListJSON handles workflow list command in JSON mode
func handleListJSON(ctx context.Context, cmd *cobra.Command, client *auth.Client, _ []string) error {
	log := logger.FromContext(ctx)

	// Parse filters from flags
	filters, err := parseFiltersFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("invalid filters: %w", err)
	}

	// Get workflows from API
	workflows, err := getWorkflows(ctx, client, filters)
	if err != nil {
		return fmt.Errorf("failed to fetch workflows: %w", err)
	}

	// Create JSON response
	response := models.APIResponse{
		Data: map[string]any{
			"workflows": workflows,
			"total":     len(workflows),
		},
		Meta: map[string]any{
			"limit":  filters.Limit,
			"offset": filters.Offset,
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Pretty print JSON
	prettyJSON := pretty.Pretty(jsonData)
	fmt.Println(string(prettyJSON))

	log.Debug("workflows listed successfully", "count", len(workflows), "mode", "json")
	return nil
}

// handleListTUI handles workflow list command in TUI mode
func handleListTUI(ctx context.Context, cmd *cobra.Command, client *auth.Client, _ []string) error {
	log := logger.FromContext(ctx)

	// Parse filters from flags
	filters, err := parseFiltersFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("invalid filters: %w", err)
	}

	// Get workflows from API
	workflows, err := getWorkflows(ctx, client, filters)
	if err != nil {
		return fmt.Errorf("failed to fetch workflows: %w", err)
	}

	// TODO: Implement full TUI program with interactive table component
	// tableComponent := components.NewWorkflowTableComponent(workflows)
	// For now, display a simple table format as fallback
	displayWorkflowTable(workflows)

	log.Debug("workflows listed successfully", "count", len(workflows), "mode", "tui")
	return nil
}

// parseFiltersFromFlags parses workflow filters from command flags
func parseFiltersFromFlags(cmd *cobra.Command) (services.WorkflowFilters, error) {
	var filters services.WorkflowFilters

	// Get status filter
	status, err := cmd.Flags().GetString("status")
	if err != nil {
		return filters, fmt.Errorf("failed to get status flag: %w", err)
	}
	if status != "" {
		filters.Status = status
	}

	// Get tags filter
	tags, err := cmd.Flags().GetStringSlice("tags")
	if err != nil {
		return filters, fmt.Errorf("failed to get tags flag: %w", err)
	}
	if len(tags) > 0 {
		filters.Tags = tags
	}

	// Get limit
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return filters, fmt.Errorf("failed to get limit flag: %w", err)
	}
	if limit > 0 {
		filters.Limit = limit
	}

	// Get offset
	offset, err := cmd.Flags().GetInt("offset")
	if err != nil {
		return filters, fmt.Errorf("failed to get offset flag: %w", err)
	}
	if offset >= 0 {
		filters.Offset = offset
	}

	return filters, nil
}

// getWorkflows fetches workflows from the API
func getWorkflows(
	_ context.Context,
	_ *auth.Client,
	filters services.WorkflowFilters,
) ([]services.Workflow, error) {
	// TODO: Replace with actual API client call once unified API client is available
	// For now, return mock data to allow testing

	// Mock error case for testing error handling
	if filters.Limit < 0 {
		return nil, fmt.Errorf("invalid limit: %d", filters.Limit)
	}

	return mockWorkflows(), nil
}

// displayWorkflowTable displays workflows in a simple table format
func displayWorkflowTable(workflows []services.Workflow) {
	if len(workflows) == 0 {
		fmt.Println("No workflows found.")
		return
	}

	// Print header
	fmt.Printf("%-20s %-10s %-30s %-20s %-20s\n", "ID", "STATUS", "NAME", "CREATED", "UPDATED")
	fmt.Println(strings.Repeat("-", 100))

	// Print workflows
	for i := range workflows {
		workflow := &workflows[i]
		fmt.Printf("%-20s %-10s %-30s %-20s %-20s\n",
			truncateString(string(workflow.ID), 20),
			string(workflow.Status),
			truncateString(workflow.Name, 30),
			workflow.CreatedAt.Format("2006-01-02 15:04"),
			workflow.UpdatedAt.Format("2006-01-02 15:04"),
		)
	}

	fmt.Printf("\nTotal: %d workflows\n", len(workflows))
}

// truncateString truncates a string to the specified length
func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}

// mockWorkflows returns mock workflow data for testing
func mockWorkflows() []services.Workflow {
	return []services.Workflow{
		{
			ID:          "wf-001",
			Name:        "Data Processing Pipeline",
			Description: "Processes incoming data and generates reports",
			Version:     "1.0.0",
			Status:      services.WorkflowStatusActive,
			CreatedAt:   time.Now().Add(-24 * time.Hour),
			UpdatedAt:   time.Now().Add(-1 * time.Hour),
			Tags:        []string{"data", "pipeline", "reports"},
			Metadata: map[string]string{
				"owner": "data-team",
				"env":   "production",
			},
		},
		{
			ID:          "wf-002",
			Name:        "User Onboarding Flow",
			Description: "Automated user onboarding and setup",
			Version:     "2.1.0",
			Status:      services.WorkflowStatusActive,
			CreatedAt:   time.Now().Add(-48 * time.Hour),
			UpdatedAt:   time.Now().Add(-2 * time.Hour),
			Tags:        []string{"user", "onboarding", "automation"},
			Metadata: map[string]string{
				"owner": "user-team",
				"env":   "production",
			},
		},
		{
			ID:          "wf-003",
			Name:        "Backup and Cleanup",
			Description: "Daily backup and cleanup tasks",
			Version:     "1.2.0",
			Status:      services.WorkflowStatusInactive,
			CreatedAt:   time.Now().Add(-72 * time.Hour),
			UpdatedAt:   time.Now().Add(-3 * time.Hour),
			Tags:        []string{"backup", "cleanup", "maintenance"},
			Metadata: map[string]string{
				"owner": "ops-team",
				"env":   "production",
			},
		},
	}
}
