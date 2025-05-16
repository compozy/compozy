package cli

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/spf13/cobra"
)

// BuildCmd returns the build command
func BuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the Compozy project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			production, err := cmd.Flags().GetBool("production")
			if err != nil {
				return fmt.Errorf("failed to get production flag: %w", err)
			}

			outDir, err := cmd.Flags().GetString("out-dir")
			if err != nil {
				return fmt.Errorf("failed to get out-dir flag: %w", err)
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

			logger.Info("Building project...")
			logger.Info("Working directory: %s", cwd)
			logger.Info("Config file: %s", configPath)
			logger.Info("Production mode: %v", production)
			logger.Info("Output directory: %s", outDir)

			// TODO: Implement build logic (e.g., compile workflows, generate artifacts)
			return nil
		},
	}

	cmd.Flags().Bool("production", false, "Build in production mode")
	cmd.Flags().String("out-dir", "dist", "Output directory")
	utils.AddCommonFlags(cmd)
	return cmd
}
