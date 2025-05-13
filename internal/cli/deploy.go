package cli

import (
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// DeployCmd returns the deploy command
func DeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the Compozy project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, _ := cmd.Flags().GetString("env")
			yes, _ := cmd.Flags().GetBool("yes")
			cwd, _ := cmd.Flags().GetString("cwd")
			config, _ := cmd.Flags().GetString("config")

			// Resolve paths
			if cwd == "" {
				cwd, _ = filepath.Abs(".")
			}
			configPath := filepath.Join(cwd, config)

			logrus.Info("Deploying project...")
			logrus.Infof("Working directory: %s", cwd)
			logrus.Infof("Config file: %s", configPath)
			logrus.Infof("Environment: %s", env)
			logrus.Infof("Skip confirmation: %v", yes)

			// TODO: Implement deploy logic (e.g., deploy to specified environment)
			return nil
		},
	}

	cmd.Flags().String("env", "", "Environment to deploy to")
	cmd.Flags().Bool("yes", false, "Skip confirmation prompt")
	return cmd
}
