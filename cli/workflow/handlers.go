package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// listJSONHandler handles workflow list command in JSON mode
func listJSONHandler(ctx context.Context, _ *cobra.Command, client *Client, _ []string) error {
	workflows, err := client.ListWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	output := struct {
		Workflows []Info `json:"workflows"`
		Count     int    `json:"count"`
	}{
		Workflows: workflows,
		Count:     len(workflows),
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// listTUIHandler handles workflow list command in TUI mode
func listTUIHandler(ctx context.Context, _ *cobra.Command, client *Client, _ []string) error {
	workflows, err := client.ListWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	// Simple table output for now (TUI implementation can be enhanced later)
	if len(workflows) == 0 {
		fmt.Println("No workflows found")
		return nil
	}

	fmt.Printf("%-36s %-30s %-15s %-20s\n", "ID", "NAME", "STATUS", "CREATED")
	fmt.Println(strings.Repeat("-", 101))

	for _, workflow := range workflows {
		fmt.Printf("%-36s %-30s %-15s %-20s\n",
			workflow.ID,
			truncate(workflow.Name, 30),
			workflow.Status,
			formatTimestamp(workflow.CreatedAt))
	}

	fmt.Printf("\nTotal: %d workflows\n", len(workflows))
	return nil
}

// getJSONHandler handles workflow get command in JSON mode
func getJSONHandler(ctx context.Context, _ *cobra.Command, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID is required")
	}

	workflowID := args[0]
	workflow, err := client.GetWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow %s: %w", workflowID, err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(workflow)
}

// getTUIHandler handles workflow get command in TUI mode
func getTUIHandler(ctx context.Context, _ *cobra.Command, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID is required")
	}

	workflowID := args[0]
	workflow, err := client.GetWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow %s: %w", workflowID, err)
	}

	// Simple formatted output for now (TUI implementation can be enhanced later)
	fmt.Printf("Workflow Details\n")
	fmt.Printf("================\n\n")
	fmt.Printf("ID:          %s\n", workflow.ID)
	fmt.Printf("Name:        %s\n", workflow.Name)
	fmt.Printf("Description: %s\n", workflow.Description)
	fmt.Printf("Status:      %s\n", workflow.Status)
	fmt.Printf("Created:     %s\n", formatTimestamp(workflow.CreatedAt))
	fmt.Printf("Updated:     %s\n", formatTimestamp(workflow.UpdatedAt))

	if workflow.Config != nil {
		fmt.Printf("\nConfiguration:\n")
		configJSON, err := json.MarshalIndent(workflow.Config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format configuration: %w", err)
		}
		fmt.Println(string(configJSON))
	}

	return nil
}

// deployJSONHandler handles workflow deploy command in JSON mode
func deployJSONHandler(ctx context.Context, cmd *cobra.Command, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID is required")
	}

	workflowID := args[0]

	// Check dry-run flag
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("failed to get dry-run flag: %w", err)
	}
	if dryRun {
		// Validate only
		result, err := client.ValidateWorkflow(ctx, workflowID)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		output := struct {
			DryRun     bool             `json:"dry_run"`
			Validation ValidationResult `json:"validation"`
		}{
			DryRun:     true,
			Validation: *result,
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}

	// Parse input data
	var input map[string]any
	inputStr, err := cmd.Flags().GetString("input")
	if err != nil {
		return fmt.Errorf("failed to get input flag: %w", err)
	}
	if inputStr != "" {
		if err := json.Unmarshal([]byte(inputStr), &input); err != nil {
			return fmt.Errorf("failed to parse input JSON: %w", err)
		}
	}

	// Execute workflow
	result, err := client.ExecuteWorkflow(ctx, workflowID, input)
	if err != nil {
		return fmt.Errorf("failed to execute workflow: %w", err)
	}

	output := struct {
		Deployed    bool                    `json:"deployed"`
		ExecutionID string                  `json:"execution_id"`
		Status      string                  `json:"status"`
		Result      ExecuteWorkflowResponse `json:"result"`
	}{
		Deployed:    true,
		ExecutionID: result.ExecutionID,
		Status:      result.Status,
		Result:      *result,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// deployTUIHandler handles workflow deploy command in TUI mode
func deployTUIHandler(ctx context.Context, cmd *cobra.Command, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID is required")
	}

	workflowID := args[0]

	// Check dry-run flag
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("failed to get dry-run flag: %w", err)
	}
	if dryRun {
		return handleDryRunValidation(ctx, client, workflowID)
	}

	// Get workflow details first
	workflow, err := client.GetWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow details: %w", err)
	}

	fmt.Printf("🚀 Deploying workflow: %s\n", workflow.Name)
	fmt.Printf("ID: %s\n", workflow.ID)
	fmt.Printf("Status: %s\n\n", workflow.Status)

	// Check force flag
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}
	if !force {
		fmt.Print("Are you sure you want to deploy this workflow? (y/N): ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}
		if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
			fmt.Println("Deployment canceled")
			return nil
		}
	}

	// Parse input data
	var input map[string]any
	inputStr, err := cmd.Flags().GetString("input")
	if err != nil {
		return fmt.Errorf("failed to get input flag: %w", err)
	}
	if inputStr != "" {
		if err := json.Unmarshal([]byte(inputStr), &input); err != nil {
			return fmt.Errorf("failed to parse input JSON: %w", err)
		}
	}

	fmt.Println("⏳ Executing workflow...")
	result, err := client.ExecuteWorkflow(ctx, workflowID, input)
	if err != nil {
		return fmt.Errorf("failed to execute workflow: %w", err)
	}

	fmt.Printf("✅ Workflow deployed successfully!\n")
	fmt.Printf("Execution ID: %s\n", result.ExecutionID)
	fmt.Printf("Status: %s\n", result.Status)

	return nil
}

// validateJSONHandler handles workflow validate command in JSON mode
func validateJSONHandler(ctx context.Context, _ *cobra.Command, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID is required")
	}

	workflowID := args[0]
	result, err := client.ValidateWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// validateTUIHandler handles workflow validate command in TUI mode
func validateTUIHandler(ctx context.Context, _ *cobra.Command, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID is required")
	}

	workflowID := args[0]

	fmt.Printf("🔍 Validating workflow %s...\n\n", workflowID)

	result, err := client.ValidateWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if result.Valid {
		fmt.Println("✅ Workflow validation passed")
		fmt.Println("The workflow configuration is valid and ready for deployment.")
	} else {
		fmt.Println("❌ Workflow validation failed")
		fmt.Printf("Found %d error(s) that must be fixed before deployment.\n", len(result.Errors))
	}

	if len(result.Errors) > 0 {
		fmt.Println("\n🚨 Errors:")
		for i, err := range result.Errors {
			fmt.Printf("  %d. %s\n", i+1, err.Message)
			if err.Field != "" {
				fmt.Printf("     Field: %s\n", err.Field)
			}
			if err.Code != "" {
				fmt.Printf("     Code: %s\n", err.Code)
			}
			fmt.Println()
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Println("⚠️  Warnings:")
		for i, warning := range result.Warnings {
			fmt.Printf("  %d. %s\n", i+1, warning.Message)
			if warning.Field != "" {
				fmt.Printf("     Field: %s\n", warning.Field)
			}
			if warning.Code != "" {
				fmt.Printf("     Code: %s\n", warning.Code)
			}
			fmt.Println()
		}
	}

	return nil
}

// Helper functions

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// handleDryRunValidation handles dry-run validation for workflows
func handleDryRunValidation(ctx context.Context, client *Client, workflowID string) error {
	fmt.Printf("🔍 Validating workflow %s...\n\n", workflowID)

	result, err := client.ValidateWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if result.Valid {
		fmt.Println("✅ Workflow validation passed")
	} else {
		fmt.Println("❌ Workflow validation failed")
		if len(result.Errors) > 0 {
			fmt.Println("\nErrors:")
			for _, err := range result.Errors {
				fmt.Printf("  - %s: %s\n", err.Field, err.Message)
			}
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, warning := range result.Warnings {
			fmt.Printf("  - %s: %s\n", warning.Field, warning.Message)
		}
	}

	return nil
}

// formatTimestamp formats a timestamp string for display
func formatTimestamp(timestamp string) string {
	// Simple formatting for now - can be enhanced with proper time parsing
	if len(timestamp) > 19 {
		return timestamp[:19] // Take first 19 characters (YYYY-MM-DD HH:MM:SS)
	}
	return timestamp
}
