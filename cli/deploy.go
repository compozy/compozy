package cli

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/spf13/cobra"
)

// DeployCmd returns the deploy command
func DeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the Compozy project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := cmd.Flags().GetString("env")
			if err != nil {
				return fmt.Errorf("failed to get env flag: %w", err)
			}

			yes, err := cmd.Flags().GetBool("yes")
			if err != nil {
				return fmt.Errorf("failed to get yes flag: %w", err)
			}

			cwd, _, configPath, err := utils.GetCommonFlags(cmd)
			if err != nil {
				return err
			}

			logLevel, logJSON, logSource, err := logger.GetLoggerConfig(cmd)
			if err != nil {
				return err
			}
			logger.SetupLogger(logLevel, logJSON, logSource)

			logger.Info("Deploying project...")
			logger.Info("Working directory: %s", cwd)
			logger.Info("Config file: %s", configPath)
			logger.Info("Environment: %s", env)
			logger.Info("Skip confirmation: %v", yes)

			// TODO: Implement deploy logic (e.g., deploy to specified environment)
			return nil
		},
	}

	cmd.Flags().String("env", "", "Environment to deploy to")
	cmd.Flags().Bool("yes", false, "Skip confirmation prompt")
	utils.AddCommonFlags(cmd)
	return cmd
}
