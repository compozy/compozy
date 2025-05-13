package cli

import (
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// InitCmd returns the init command
func InitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new Compozy project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, _ := cmd.Flags().GetString("name")
			cwd, _ := cmd.Flags().GetString("cwd")
			config, _ := cmd.Flags().GetString("config")

			// Resolve paths
			if cwd == "" {
				cwd, _ = filepath.Abs(".")
			}
			configPath := filepath.Join(cwd, config)

			logrus.Info("Initializing project...")
			logrus.Infof("Working directory: %s", cwd)
			logrus.Infof("Config file: %s", configPath)
			logrus.Infof("Project name: %s", name)

			// TODO: Implement init logic (e.g., create project structure, config file)
			return nil
		},
	}

	cmd.Flags().String("name", "", "Project name")
	return cmd
}
