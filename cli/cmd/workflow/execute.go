package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	cliutils "github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/cli/tui/styles"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

// ExecuteCmd creates the workflow execute command
func ExecuteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute <workflow-id>",
		Short: "Execute a workflow",
		Long:  "Execute a workflow with optional input parameters.",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowExecute,
	}

	// Add input parameter flags
	cmd.Flags().StringSlice("input", []string{}, "Input parameters in key=value format (can be used multiple times)")
	cmd.Flags().String("input-file", "", "Path to JSON file containing input parameters")

	return cmd
}

// runWorkflowExecute handles the workflow execute command execution
func runWorkflowExecute(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
	}, cmd.ModeHandlers{
		JSON: executeJSONHandler,
		TUI:  executeTUIHandler,
	}, args)
}

// executeJSONHandler handles JSON mode for workflow execute
func executeJSONHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return workflowExecuteJSONHandler(ctx, cobraCmd, executor.GetAuthClient(), args)
}

// executeTUIHandler handles TUI mode for workflow execute
func executeTUIHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return workflowExecuteTUIHandler(ctx, cobraCmd, executor.GetAuthClient(), args)
}

// executeWorkflow handles the common workflow execution logic
func executeWorkflow(
	ctx context.Context,
	cmd *cobra.Command,
	client api.AuthClient,
	workflowID core.ID,
) (*api.ExecutionResult, error) {
	// Parse input parameters
	inputs, err := parseInputParameters(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input parameters: %w", err)
	}

	// Create workflow mutate API client
	apiClient := createWorkflowMutateAPIClient(client)

	// Create execution input
	input := api.ExecutionInput{
		Data: inputs,
	}

	// Start workflow execution
	result, err := apiClient.Execute(ctx, workflowID, input)
	if err != nil {
		return nil, fmt.Errorf("failed to execute workflow: %w", err)
	}

	logger.FromContext(ctx).
		Debug("workflow execution started", "workflow_id", workflowID, "execution_id", result.ExecutionID)
	return result, nil
}

// workflowExecuteJSONHandler handles JSON output mode
func workflowExecuteJSONHandler(ctx context.Context, cmd *cobra.Command, client api.AuthClient, args []string) error {
	workflowID := core.ID(args[0])

	result, err := executeWorkflow(ctx, cmd, client, workflowID)
	if err != nil {
		return err
	}

	// Create JSON formatter
	formatter := cliutils.NewJSONFormatter(true)

	// Format result
	output, err := formatter.FormatSuccess(result, &cliutils.FormatterMetadata{
		Timestamp: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to format result data: %w", err)
	}

	fmt.Println(output)
	return nil
}

// workflowExecuteTUIHandler handles TUI output mode
func workflowExecuteTUIHandler(ctx context.Context, cmd *cobra.Command, client api.AuthClient, args []string) error {
	workflowID := core.ID(args[0])

	result, err := executeWorkflow(ctx, cmd, client, workflowID)
	if err != nil {
		return err
	}

	// Display result
	fmt.Println(styles.SuccessStyle.Render("✓ Workflow execution started"))
	fmt.Printf("Execution ID: %s\n", result.ExecutionID)
	fmt.Printf("Status: %s\n", renderExecutionStatus(result.Status))

	if result.Message != "" {
		fmt.Printf("Message: %s\n", result.Message)
	}

	return nil
}

// parseInputParameters parses input parameters from flags
func parseInputParameters(cmd *cobra.Command) (map[string]any, error) {
	inputs := make(map[string]any)

	// Parse --input flags
	inputFlags, err := cmd.Flags().GetStringSlice("input")
	if err != nil {
		return nil, fmt.Errorf("failed to get input flags: %w", err)
	}

	for _, input := range inputFlags {
		parts := strings.SplitN(input, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid input format: %s (expected key=value)", input)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Try to parse value as JSON first, with explicit type checking
		if gjson.Valid(value) {
			result := gjson.Parse(value)
			// Only use JSON parsing for structured data (objects, arrays, booleans, numbers)
			// String values wrapped in quotes should be treated as JSON strings
			switch result.Type {
			case gjson.String, gjson.Number, gjson.True, gjson.False:
				inputs[key] = result.Value()
			case gjson.JSON:
				// For complex JSON (objects/arrays), use the parsed value
				inputs[key] = result.Value()
			default:
				// For null or other types, treat as string
				inputs[key] = value
			}
		} else {
			// Otherwise treat as string
			inputs[key] = value
		}
	}

	// Parse --input-file flag
	inputFile, err := cmd.Flags().GetString("input-file")
	if err != nil {
		return nil, fmt.Errorf("failed to get input-file flag: %w", err)
	}

	if inputFile != "" {
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read input file: %w", err)
		}

		var fileInputs map[string]any
		if err := json.Unmarshal(data, &fileInputs); err != nil {
			return nil, fmt.Errorf("failed to parse input file as JSON: %w", err)
		}

		// Merge file inputs with flag inputs (flag inputs take precedence)
		for k, v := range fileInputs {
			if _, exists := inputs[k]; !exists {
				inputs[k] = v
			}
		}
	}

	return inputs, nil
}

// renderExecutionStatus renders the execution status with color
func renderExecutionStatus(status api.ExecutionStatus) string {
	switch status {
	case api.ExecutionStatusRunning:
		return styles.InfoStyle.Render(string(status))
	case api.ExecutionStatusCompleted:
		return styles.SuccessStyle.Render(string(status))
	case api.ExecutionStatusFailed:
		return styles.ErrorStyle.Render(string(status))
	case api.ExecutionStatusCancelled:
		return styles.ErrorStyle.Render(string(status))
	default:
		return string(status)
	}
}

// createWorkflowMutateAPIClient creates an API client for workflow mutation operations
func createWorkflowMutateAPIClient(authClient api.AuthClient) api.WorkflowMutateService {
	// Create HTTP client using shared utility
	client := cliutils.CreateHTTPClient(authClient, nil)

	return &workflowMutateAPIService{
		authClient: authClient,
		httpClient: client,
	}
}

// workflowMutateAPIService implements the WorkflowMutateService interface
type workflowMutateAPIService struct {
	authClient api.AuthClient
	httpClient *resty.Client
}

// Execute implements the WorkflowMutateService.Execute method
func (s *workflowMutateAPIService) Execute(
	ctx context.Context,
	id core.ID,
	input api.ExecutionInput,
) (*api.ExecutionResult, error) {
	log := logger.FromContext(ctx)

	// Make the API call
	var result struct {
		Data api.ExecutionResult `json:"data"`
	}

	resp, err := s.httpClient.R().
		SetContext(ctx).
		SetBody(input).
		SetResult(&result).
		Post(fmt.Sprintf("/workflows/%s/execute", id))

	if err != nil {
		// Handle network errors
		if cliutils.IsNetworkError(err) {
			return nil, fmt.Errorf("network error: unable to connect to Compozy server: %w", err)
		}
		if cliutils.IsTimeoutError(err) {
			return nil, fmt.Errorf("request timed out: server may be busy: %w", err)
		}
		return nil, fmt.Errorf("failed to execute workflow: %w", err)
	}

	// Handle HTTP errors
	if resp.StatusCode() >= 400 {
		if resp.StatusCode() == 401 {
			return nil, fmt.Errorf("authentication failed: please check your API key or login credentials")
		}
		if resp.StatusCode() == 403 {
			return nil, fmt.Errorf("permission denied: you don't have access to execute this workflow")
		}
		if resp.StatusCode() == 404 {
			return nil, fmt.Errorf("workflow not found: workflow with ID %s does not exist", id)
		}
		if resp.StatusCode() >= 500 {
			return nil, fmt.Errorf("server error (status %d): try again later", resp.StatusCode())
		}
		return nil, fmt.Errorf("API error: %s (status %d)", resp.String(), resp.StatusCode())
	}

	log.Debug("workflow executed successfully", "workflow_id", id, "execution_id", result.Data.ExecutionID)
	return &result.Data, nil
}
