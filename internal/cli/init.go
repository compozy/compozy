package cli

import (
	"fmt"

	"github.com/sirupsen/logrus"
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

			cwd, _, configPath, err := GetCommonFlags(cmd)
			if err != nil {
				return err
			}

			logrus.Info("Initializing project...")
			logrus.Infof("Working directory: %s", cwd)
			logrus.Infof("Config file: %s", configPath)
			logrus.Infof("Project name: %s", name)

			// TODO: Implement init logic (e.g., create project structure, config file)
			return nil
		},
	}

	cmd.Flags().String("name", "", "Project name")
	AddCommonFlags(cmd)
	return cmd
}
