package workflow

import (
	"github.com/compozy/compozy/cli/auth"
	"github.com/spf13/cobra"
)

// Cmd returns the workflow command group
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage workflows and their executions",
		Long: `Commands for listing, viewing, executing, and validating workflows.
		
This command group provides comprehensive workflow management capabilities
including interactive TUI interfaces and CI/CD-friendly JSON output modes.`,
	}

	// Add subcommands
	cmd.AddCommand(
		ListCmd(),
		GetCmd(),
		DeployCmd(),
		ValidateCmd(),
	)

	return cmd
}

// ListCmd returns the workflow list command
func ListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all workflows",
		Long: `List all workflows with their basic information.
		
In TUI mode, displays an interactive table with sorting and filtering capabilities.
In JSON mode, outputs structured data suitable for scripting and automation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteCommand(cmd, ModeHandlers{
				JSON: listJSONHandler,
				TUI:  listTUIHandler,
			}, args)
		},
	}

	// Add common mode flags
	auth.AddModeFlags(cmd)

	return cmd
}

// GetCmd returns the workflow get command
func GetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <workflow-id>",
		Short: "Get detailed information about a specific workflow",
		Long: `Get detailed information about a specific workflow including its configuration,
current status, and execution history.
		
In TUI mode, displays a formatted, styled view with syntax highlighting.
In JSON mode, outputs complete workflow details in structured format.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteCommand(cmd, ModeHandlers{
				JSON: getJSONHandler,
				TUI:  getTUIHandler,
			}, args)
		},
	}

	// Add common mode flags
	auth.AddModeFlags(cmd)

	return cmd
}

// DeployCmd returns the workflow deploy command
func DeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy <workflow-id>",
		Short: "Deploy a workflow with validation and confirmation",
		Long: `Deploy a workflow after performing validation checks and obtaining user confirmation.
		
This command validates the workflow configuration, checks for potential issues,
and provides interactive confirmation before deployment.
		
In TUI mode, shows validation results and interactive confirmation prompts.
In JSON mode, performs validation and returns deployment status.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteCommand(cmd, ModeHandlers{
				JSON: deployJSONHandler,
				TUI:  deployTUIHandler,
			}, args)
		},
	}

	// Add deployment-specific flags
	cmd.Flags().Bool("force", false, "Skip confirmation prompts")
	cmd.Flags().Bool("dry-run", false, "Validate without deploying")
	cmd.Flags().String("input", "", "Input data as JSON string")
	cmd.Flags().String("input-file", "", "Input data from file")

	// Add common mode flags
	auth.AddModeFlags(cmd)

	return cmd
}

// ValidateCmd returns the workflow validate command
func ValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <workflow-id>",
		Short: "Validate a workflow configuration locally",
		Long: `Validate a workflow configuration locally without deploying it.
		
This command performs comprehensive validation checks including:
- YAML syntax validation
- Schema validation against workflow specifications
- Dependency and reference validation
- Resource and permission checks
		
In TUI mode, displays validation results with styled output and error highlighting.
In JSON mode, outputs structured validation results with detailed error information.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteCommand(cmd, ModeHandlers{
				JSON: validateJSONHandler,
				TUI:  validateTUIHandler,
			}, args)
		},
	}

	// Add validation-specific flags
	cmd.Flags().Bool("strict", false, "Enable strict validation mode")
	cmd.Flags().String("schema", "", "Custom schema file for validation")

	// Add common mode flags
	auth.AddModeFlags(cmd)

	return cmd
}
