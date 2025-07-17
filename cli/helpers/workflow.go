package helpers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/robfig/cron/v3"
	"github.com/tidwall/gjson"
)

// WorkflowFilterOptions represents advanced filtering options for workflows
type WorkflowFilterOptions struct {
	api.WorkflowFilters
	JSONPath     string        `json:"json_path,omitempty"     validate:"omitempty,jsonpath"`
	SortBy       string        `json:"sort_by,omitempty"       validate:"omitempty,oneof=name created_at updated_at status"`
	SortOrder    string        `json:"sort_order,omitempty"    validate:"omitempty,oneof=asc desc"`
	ServerOnly   bool          `json:"server_only,omitempty"`
	EnableCache  bool          `json:"enable_cache,omitempty"`
	CacheTimeout time.Duration `json:"cache_timeout,omitempty"`
}

// WorkflowValidator provides validation utilities for workflow operations
type WorkflowValidator struct {
	validator *validator.Validate
	once      sync.Once
}

// NewWorkflowValidator creates a new workflow validator instance
func NewWorkflowValidator() *WorkflowValidator {
	return &WorkflowValidator{
		validator: validator.New(),
	}
}

// initValidator initializes the validator with custom rules
func (v *WorkflowValidator) initValidator() {
	v.once.Do(func() {
		// Register custom validation rules
		if err := v.validator.RegisterValidation("cron", v.validateCron); err != nil {
			return // Skip validation registration on error
		}
		if err := v.validator.RegisterValidation("jsonpath", v.validateJSONPath); err != nil {
			return // Skip validation registration on error
		}
	})
}

// validateCron validates cron expressions
func (v *WorkflowValidator) validateCron(fl validator.FieldLevel) bool {
	cronExpr := fl.Field().String()
	if cronExpr == "" {
		return true // Allow empty for optional fields
	}

	// Parse the cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		return false
	}

	// Ensure the expression yields at least one next time within 1 year
	now := time.Now()
	next := schedule.Next(now)
	return next.Before(now.AddDate(1, 0, 0))
}

// validateJSONPath validates JSON path expressions
func (v *WorkflowValidator) validateJSONPath(fl validator.FieldLevel) bool {
	jsonPath := fl.Field().String()
	if jsonPath == "" {
		return true // Allow empty for optional fields
	}

	// Test with a simple JSON object to validate the path
	testJSON := `{"test": "value"}`
	result := gjson.Get(testJSON, jsonPath)
	// If the path is valid, result.Type will not be Null unless the path doesn't exist
	return result.Type != gjson.Null || jsonPath == "test"
}

// ValidateWorkflowFilters validates workflow filter options
func (v *WorkflowValidator) ValidateWorkflowFilters(filters *WorkflowFilterOptions) error {
	v.initValidator()
	return v.validator.Struct(filters)
}

// ValidateScheduleRequest validates schedule update requests
func (v *WorkflowValidator) ValidateScheduleRequest(req *api.UpdateScheduleRequest) error {
	v.initValidator()
	return v.validator.Struct(req)
}

// ValidateWorkflowExecution validates workflow execution input
func (v *WorkflowValidator) ValidateWorkflowExecution(input *api.ExecutionInput) error {
	v.initValidator()
	return v.validator.Struct(input)
}

// WorkflowFilterer provides advanced filtering capabilities
type WorkflowFilterer struct {
	cache     *sync.Map
	validator *WorkflowValidator
}

// NewWorkflowFilterer creates a new workflow filterer
func NewWorkflowFilterer() *WorkflowFilterer {
	return &WorkflowFilterer{
		cache:     &sync.Map{},
		validator: NewWorkflowValidator(),
	}
}

// FilterWorkflows applies advanced filtering to workflow collections
func (f *WorkflowFilterer) FilterWorkflows(
	ctx context.Context,
	workflows []api.Workflow,
	options *WorkflowFilterOptions,
) ([]api.Workflow, error) {
	log := logger.FromContext(ctx)

	// Validate filter options
	if err := f.validator.ValidateWorkflowFilters(options); err != nil {
		return nil, fmt.Errorf("invalid filter options: %w", err)
	}

	// If no JSON path filtering is needed, return original workflows
	if options.JSONPath == "" {
		return f.sortWorkflows(workflows, options), nil
	}

	// Apply JSON path filtering
	filtered := make([]api.Workflow, 0, len(workflows))

	for i := range workflows {
		if f.matchesJSONPath(&workflows[i], options.JSONPath) {
			filtered = append(filtered, workflows[i])
		}
	}

	log.Debug(
		"filtered workflows",
		"original_count",
		len(workflows),
		"filtered_count",
		len(filtered),
		"json_path",
		options.JSONPath,
	)

	return f.sortWorkflows(filtered, options), nil
}

// matchesJSONPath checks if a workflow matches the JSON path criteria
func (f *WorkflowFilterer) matchesJSONPath(workflow *api.Workflow, jsonPath string) bool {
	// Convert workflow to JSON for path matching
	workflowJSON := fmt.Sprintf(`{
		"id": "%s",
		"name": "%s",
		"description": "%s",
		"version": "%s",
		"status": "%s",
		"created_at": "%s",
		"updated_at": "%s",
		"tags": [%s],
		"metadata": %s
	}`,
		workflow.ID,
		workflow.Name,
		workflow.Description,
		workflow.Version,
		workflow.Status,
		workflow.CreatedAt.Format(time.RFC3339),
		workflow.UpdatedAt.Format(time.RFC3339),
		formatTags(workflow.Tags),
		formatMetadata(workflow.Metadata),
	)

	result := gjson.Get(workflowJSON, jsonPath)
	return result.Exists() && result.Type != gjson.Null
}

// formatTags formats tags for JSON
func formatTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	quoted := make([]string, len(tags))
	for i, tag := range tags {
		quoted[i] = fmt.Sprintf(`%q`, tag)
	}
	return strings.Join(quoted, ",")
}

// formatMetadata formats metadata for JSON
func formatMetadata(metadata map[string]string) string {
	if len(metadata) == 0 {
		return "{}"
	}

	parts := make([]string, 0, len(metadata))
	for k, v := range metadata {
		parts = append(parts, fmt.Sprintf(`%q: %q`, k, v))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// sortWorkflows sorts workflows based on the provided options
func (f *WorkflowFilterer) sortWorkflows(
	workflows []api.Workflow,
	options *WorkflowFilterOptions,
) []api.Workflow {
	if options.SortBy == "" {
		return workflows
	}

	sorted := make([]api.Workflow, len(workflows))
	copy(sorted, workflows)

	sort.Slice(sorted, func(i, j int) bool {
		result := f.compareWorkflows(&sorted[i], &sorted[j], options.SortBy)
		if options.SortOrder == "desc" {
			return !result
		}
		return result
	})

	return sorted
}

// compareWorkflows compares two workflows based on the sort criteria
func (f *WorkflowFilterer) compareWorkflows(a, b *api.Workflow, sortBy string) bool {
	switch sortBy {
	case "name":
		return a.Name < b.Name
	case "created_at":
		return a.CreatedAt.Before(b.CreatedAt)
	case "updated_at":
		return a.UpdatedAt.Before(b.UpdatedAt)
	case "status":
		return string(a.Status) < string(b.Status)
	default:
		return a.Name < b.Name // Default to name sorting
	}
}

// WorkflowExecutionFilterer provides filtering for workflow executions
type WorkflowExecutionFilterer struct {
	validator *WorkflowValidator
}

// NewWorkflowExecutionFilterer creates a new execution filterer
func NewWorkflowExecutionFilterer() *WorkflowExecutionFilterer {
	return &WorkflowExecutionFilterer{
		validator: NewWorkflowValidator(),
	}
}

// FilterExecutions filters workflow executions based on criteria
func (f *WorkflowExecutionFilterer) FilterExecutions(
	ctx context.Context,
	executions []api.Execution,
	filters *api.ExecutionFilters,
) ([]api.Execution, error) {
	log := logger.FromContext(ctx)

	if filters == nil {
		return executions, nil
	}

	filtered := make([]api.Execution, 0, len(executions))

	for i := range executions {
		if f.matchesExecutionFilters(&executions[i], filters) {
			filtered = append(filtered, executions[i])
		}
	}

	log.Debug("filtered executions", "original_count", len(executions), "filtered_count", len(filtered))

	return filtered, nil
}

// matchesExecutionFilters checks if an execution matches the filter criteria
func (f *WorkflowExecutionFilterer) matchesExecutionFilters(
	execution *api.Execution,
	filters *api.ExecutionFilters,
) bool {
	// Filter by workflow ID
	if filters.WorkflowID != "" && execution.WorkflowID != filters.WorkflowID {
		return false
	}

	// Filter by status
	if filters.Status != "" && execution.Status != filters.Status {
		return false
	}

	return true
}

// WorkflowErrorHandler provides enhanced error handling for workflow operations
type WorkflowErrorHandler struct{}

// NewWorkflowErrorHandler creates a new error handler
func NewWorkflowErrorHandler() *WorkflowErrorHandler {
	return &WorkflowErrorHandler{}
}

// HandleWorkflowError wraps workflow-related errors with context
func (h *WorkflowErrorHandler) HandleWorkflowError(
	ctx context.Context,
	err error,
	operation string,
	workflowID core.ID,
) error {
	if err == nil {
		return nil
	}

	log := logger.FromContext(ctx)
	log.Error("workflow operation failed", "operation", operation, "workflow_id", workflowID, "error", err)

	switch {
	case strings.Contains(err.Error(), "not found"):
		return fmt.Errorf(
			"workflow %s not found - check that the workflow exists and you have permission to access it: %w",
			workflowID,
			err,
		)
	case strings.Contains(err.Error(), "unauthorized"):
		return fmt.Errorf(
			"insufficient permissions to %s workflow %s - check your API key and permissions: %w",
			operation,
			workflowID,
			err,
		)
	case strings.Contains(err.Error(), "timeout"):
		return fmt.Errorf(
			"operation %s timed out for workflow %s - the server may be busy, try again later: %w",
			operation,
			workflowID,
			err,
		)
	case strings.Contains(err.Error(), "validation"):
		return fmt.Errorf("validation failed for workflow %s - check your input data: %w", workflowID, err)
	default:
		return fmt.Errorf("failed to %s workflow %s: %w", operation, workflowID, err)
	}
}

// HandleExecutionError wraps execution-related errors with context
func (h *WorkflowErrorHandler) HandleExecutionError(
	ctx context.Context,
	err error,
	operation string,
	executionID core.ID,
) error {
	if err == nil {
		return nil
	}

	log := logger.FromContext(ctx)
	log.Error("execution operation failed", "operation", operation, "execution_id", executionID, "error", err)

	switch {
	case strings.Contains(err.Error(), "not found"):
		return fmt.Errorf(
			"execution %s not found - check that the execution exists and you have permission to access it: %w",
			executionID,
			err,
		)
	case strings.Contains(err.Error(), "unauthorized"):
		return fmt.Errorf(
			"insufficient permissions to %s execution %s - check your API key and permissions: %w",
			operation,
			executionID,
			err,
		)
	case strings.Contains(err.Error(), "timeout"):
		return fmt.Errorf(
			"operation %s timed out for execution %s - the server may be busy, try again later: %w",
			operation,
			executionID,
			err,
		)
	case strings.Contains(err.Error(), "canceled"):
		return fmt.Errorf(
			"execution %s was canceled - this may be intentional or due to a timeout: %w",
			executionID,
			err,
		)
	default:
		return fmt.Errorf("failed to %s execution %s: %w", operation, executionID, err)
	}
}

// HandleScheduleError wraps schedule-related errors with context
func (h *WorkflowErrorHandler) HandleScheduleError(
	ctx context.Context,
	err error,
	operation string,
	workflowID core.ID,
) error {
	if err == nil {
		return nil
	}

	log := logger.FromContext(ctx)
	log.Error("schedule operation failed", "operation", operation, "workflow_id", workflowID, "error", err)

	switch {
	case strings.Contains(err.Error(), "not found"):
		return fmt.Errorf(
			"schedule for workflow %s not found - check that the workflow exists and has a schedule: %w",
			workflowID,
			err,
		)
	case strings.Contains(err.Error(), "invalid cron"):
		return fmt.Errorf(
			"invalid cron expression for workflow %s - check your cron syntax (e.g., '0 0 * * *' for daily): %w",
			workflowID,
			err,
		)
	case strings.Contains(err.Error(), "unauthorized"):
		return fmt.Errorf(
			"insufficient permissions to %s schedule for workflow %s - check your API key and permissions: %w",
			operation,
			workflowID,
			err,
		)
	default:
		return fmt.Errorf("failed to %s schedule for workflow %s: %w", operation, workflowID, err)
	}
}

// SuggestionProvider provides helpful suggestions for common errors
type SuggestionProvider struct{}

// NewSuggestionProvider creates a new suggestion provider
func NewSuggestionProvider() *SuggestionProvider {
	return &SuggestionProvider{}
}

// GetWorkflowSuggestions provides suggestions for workflow-related errors
func (s *SuggestionProvider) GetWorkflowSuggestions(err error) []string {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())
	var suggestions []string

	switch {
	case strings.Contains(errStr, "not found"):
		suggestions = append(suggestions,
			"List available workflows with 'compozy workflow list'",
			"Check that the workflow ID is correct",
			"Verify you have permission to access this workflow",
		)
	case strings.Contains(errStr, "unauthorized"):
		suggestions = append(suggestions,
			"Check your API key configuration",
			"Verify your user permissions",
			"Contact your administrator if you need access",
		)
	case strings.Contains(errStr, "timeout"):
		suggestions = append(suggestions,
			"Try the operation again in a few moments",
			"Check your network connection",
			"Consider increasing the timeout if this happens frequently",
		)
	case strings.Contains(errStr, "validation"):
		suggestions = append(suggestions,
			"Check your input data format",
			"Ensure all required fields are provided",
			"Verify data types match the expected format",
		)
	}

	return suggestions
}

// GetExecutionSuggestions provides suggestions for execution-related errors
func (s *SuggestionProvider) GetExecutionSuggestions(err error) []string {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())
	var suggestions []string

	switch {
	case strings.Contains(errStr, "not found"):
		suggestions = append(suggestions,
			"List available executions with 'compozy execution list'",
			"Check that the execution ID is correct",
			"The execution may have been cleaned up if it's old",
		)
	case strings.Contains(errStr, "canceled"):
		suggestions = append(suggestions,
			"Check if the execution was manually canceled",
			"Verify the execution didn't timeout",
			"Review execution logs for more details",
		)
	case strings.Contains(errStr, "failed"):
		suggestions = append(suggestions,
			"Check execution logs for error details",
			"Verify the workflow configuration is correct",
			"Ensure all required inputs were provided",
		)
	}

	return suggestions
}

// GetScheduleSuggestions provides suggestions for schedule-related errors
func (s *SuggestionProvider) GetScheduleSuggestions(err error) []string {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())
	var suggestions []string

	switch {
	case strings.Contains(errStr, "invalid cron"):
		suggestions = append(suggestions,
			"Check your cron expression syntax",
			"Use online cron validators to test your expression",
			"Common patterns: '0 0 * * *' (daily), '0 * * * *' (hourly)",
			"Ensure your expression yields future dates",
		)
	case strings.Contains(errStr, "not found"):
		suggestions = append(suggestions,
			"Check that the workflow exists",
			"Verify the workflow supports scheduling",
			"Create a schedule first with 'compozy schedule update'",
		)
	}

	return suggestions
}
