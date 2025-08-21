package auth

import (
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/cmd/auth/handlers"
	"github.com/spf13/cobra"
)

// BootstrapCmd returns the bootstrap command for initial admin setup
func BootstrapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap initial admin user",
		Long: `Create the initial admin user for the system.
This is a one-time operation that should only be run during initial setup.
If an admin user already exists, this command will fail.`,
		Example: `  # Bootstrap with interactive prompts
  compozy auth bootstrap

  # Bootstrap with email specified
  compozy auth bootstrap --email admin@company.com

  # Bootstrap with force flag (skip confirmation)
  compozy auth bootstrap --email admin@company.com --force`,
		RunE: runBootstrap,
	}

	// Add flags
	cmd.Flags().String("email", "", "Admin user email address")
	cmd.Flags().Bool("force", false, "Skip confirmation prompt")
	cmd.Flags().Bool("check", false, "Check if system is already bootstrapped")

	return cmd
}

// runBootstrap handles the bootstrap command execution
func runBootstrap(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(
		cobraCmd,
		cmd.ExecutorOptions{RequireAuth: false},
		cmd.ModeHandlers{
			JSON: handlers.BootstrapJSON,
			TUI:  handlers.BootstrapTUI,
		},
		args,
	)
}
