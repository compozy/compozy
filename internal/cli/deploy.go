package cli

import (
	"fmt"

	"github.com/sirupsen/logrus"
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

			cwd, _, configPath, err := GetCommonFlags(cmd)
			if err != nil {
				return err
			}

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
	AddCommonFlags(cmd)
	return cmd
}
