package cli

import (
	"fmt"

	"github.com/sirupsen/logrus"
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

			cwd, _, configPath, err := GetCommonFlags(cmd)
			if err != nil {
				return err
			}

			logrus.Info("Building project...")
			logrus.Infof("Working directory: %s", cwd)
			logrus.Infof("Config file: %s", configPath)
			logrus.Infof("Production mode: %v", production)
			logrus.Infof("Output directory: %s", outDir)

			// TODO: Implement build logic (e.g., compile workflows, generate artifacts)
			return nil
		},
	}

	cmd.Flags().Bool("production", false, "Build in production mode")
	cmd.Flags().String("out-dir", "dist", "Output directory")
	AddCommonFlags(cmd)
	return cmd
}
