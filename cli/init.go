package cli

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/spf13/cobra"
)

// InitCmd returns the init command
func InitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new Compozy project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return fmt.Errorf("failed to get name flag: %w", err)
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

			logger.Info("Initializing project...")
			logger.Info("Working directory: %s", cwd)
			logger.Info("Config file: %s", configPath)
			logger.Info("Project name: %s", name)

			// TODO: Implement init logic (e.g., create project structure, config file)
			return nil
		},
	}

	cmd.Flags().String("name", "", "Project name")
	utils.AddCommonFlags(cmd)
	return cmd
}
