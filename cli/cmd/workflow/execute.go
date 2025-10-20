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
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

// ExecuteCmd creates the workflow execute command
func ExecuteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute <workflow-id>",
		Short: "Execute a workflow",
		Long: `Execute a workflow with input parameters.

Input can be provided in two formats:
  --json    : Complete JSON object for complex inputs
  --param   : Individual key=value pairs for simple inputs

Examples:
  # Using JSON for complex input
  compozy workflow execute greeter --json='{"name":"World","style":"friendly"}'
  
  # Using params for simple input
  compozy workflow execute simple --param name=World --param count=5
  
  # Using input file
  compozy workflow execute complex --input-file=input.json`,
		Args: cobra.ExactArgs(1),
		RunE: runWorkflowExecute,
	}
	cmd.Flags().String("json", "", "Input parameters as a JSON object")
	cmd.Flags().StringSlice("param", []string{}, "Input parameters in key=value format (can be used multiple times)")
	cmd.Flags().String("input-file", "", "Path to JSON file containing input parameters")
	mustMarkFlagsMutuallyExclusive(cmd, "json", "param")
	return cmd
}

func mustMarkFlagsMutuallyExclusive(cmd *cobra.Command, flagNames ...string) {
	ctx := cmd.Context()
	defer func() {
		if r := recover(); r != nil {
			logger.FromContext(ctx).Debug(
				"failed to mark flags mutually exclusive",
				"flags", flagNames,
				"error", r,
			)
		}
	}()
	cmd.MarkFlagsMutuallyExclusive(flagNames...)
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
	inputs, err := parseInputParameters(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input parameters: %w", err)
	}
	httpClient := cliutils.CreateHTTPClient(client, nil)
	apiClient := api.NewWorkflowMutateService(client, httpClient)
	input := api.ExecutionInput{
		Data: inputs,
	}
	result, err := apiClient.Execute(ctx, workflowID, input)
	if err != nil {
		return nil, fmt.Errorf("failed to execute workflow: %w", err)
	}
	return result, nil
}

// workflowExecuteJSONHandler handles JSON output mode
func workflowExecuteJSONHandler(ctx context.Context, cmd *cobra.Command, client api.AuthClient, args []string) error {
	workflowID := core.ID(args[0])
	result, err := executeWorkflow(ctx, cmd, client, workflowID)
	if err != nil {
		return err
	}
	formatter := cliutils.NewJSONFormatter(true)
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
	if err := parseJSONFlag(cmd, inputs); err != nil {
		return nil, err
	}
	if err := parseParamFlags(cmd, inputs); err != nil {
		return nil, err
	}
	if err := parseInputFileFlag(cmd, inputs); err != nil {
		return nil, err
	}
	return inputs, nil
}

// parseJSONFlag parses the --json flag and adds to inputs map
func parseJSONFlag(cmd *cobra.Command, inputs map[string]any) error {
	jsonInput, err := cmd.Flags().GetString("json")
	if err != nil {
		return fmt.Errorf("failed to get json flag: %w", err)
	}
	if jsonInput != "" {
		if err := json.Unmarshal([]byte(jsonInput), &inputs); err != nil {
			return fmt.Errorf("failed to parse --json input: %w", err)
		}
	}
	return nil
}

// parseParamFlags parses the --param flags and adds to inputs map
func parseParamFlags(cmd *cobra.Command, inputs map[string]any) error {
	paramFlags, err := cmd.Flags().GetStringSlice("param")
	if err != nil {
		return fmt.Errorf("failed to get param flags: %w", err)
	}
	for _, param := range paramFlags {
		key, value, err := parseKeyValue(param)
		if err != nil {
			return err
		}
		inputs[key] = parseParamValue(value)
	}
	return nil
}

// parseKeyValue splits a key=value string
func parseKeyValue(param string) (string, string, error) {
	parts := strings.SplitN(param, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid param format: %s (expected key=value)", param)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

// parseParamValue parses a parameter value with type detection
func parseParamValue(value string) any {
	result := gjson.Parse(value)
	if result.Type == gjson.Null {
		return value
	}
	switch result.Type {
	case gjson.Number:
		return result.Float()
	case gjson.True, gjson.False:
		return result.Bool()
	case gjson.String:
		return result.String()
	case gjson.JSON:
		return result.Value()
	default:
		return value
	}
}

// parseInputFileFlag parses the --input-file flag and merges with inputs
func parseInputFileFlag(cmd *cobra.Command, inputs map[string]any) error {
	inputFile, err := cmd.Flags().GetString("input-file")
	if err != nil {
		return fmt.Errorf("failed to get input-file flag: %w", err)
	}
	if inputFile == "" {
		return nil
	}
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}
	var fileInputs map[string]any
	if err := json.Unmarshal(data, &fileInputs); err != nil {
		return fmt.Errorf("failed to parse input file as JSON: %w", err)
	}
	for k, v := range fileInputs {
		if _, exists := inputs[k]; !exists {
			inputs[k] = v
		}
	}
	return nil
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
		return styles.WarningStyle.Render(fmt.Sprintf("Unknown status: %s", status))
	}
}
