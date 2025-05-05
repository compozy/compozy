package commands

import (
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// BuildCmd returns the build command
func BuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the Compozy project",
		RunE: func(cmd *cobra.Command, args []string) error {
			production, _ := cmd.Flags().GetBool("production")
			outDir, _ := cmd.Flags().GetString("out-dir")
			cwd, _ := cmd.Flags().GetString("cwd")
			config, _ := cmd.Flags().GetString("config")

			// Resolve paths
			if cwd == "" {
				cwd, _ = filepath.Abs(".")
			}
			configPath := filepath.Join(cwd, config)

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
	return cmd
}
