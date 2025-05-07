package cli

import (
	"path/filepath"

	"github.com/compozy/compozy/internal/parser/project"
	"github.com/compozy/compozy/internal/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// DevCmd returns the dev command
func DevCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the Compozy development server",
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")
			host, _ := cmd.Flags().GetString("host")
			cors, _ := cmd.Flags().GetBool("cors")
			cwd, _ := cmd.Flags().GetString("cwd")
			config, _ := cmd.Flags().GetString("config")

			// Resolve paths
			if cwd == "" {
				cwd, _ = filepath.Abs(".")
			}
			configPath := filepath.Join(cwd, config)

			logrus.Info("Starting Compozy server...")
			logrus.Infof("Loading config file: %s", configPath)

			// Load project configuration
			projectConfig, err := project.Load(configPath)
			if err != nil {
				return err
			}

			// Validate project configuration
			if err := projectConfig.Validate(); err != nil {
				return err
			}

			// Load workflows from sources
			workflows, err := projectConfig.WorkflowsFromSources()
			if err != nil {
				return err
			}

			// Create app state
			appState, err := server.NewAppState(cwd, workflows)
			if err != nil {
				return err
			}

			// Create server configuration
			serverConfig := &server.ServerConfig{
				CWD:         cwd,
				Host:        host,
				Port:        port,
				CORSEnabled: cors,
			}

			// Create and run server
			srv := server.NewServer(serverConfig, appState)
			return srv.Run()
		},
	}

	cmd.Flags().Int("port", 3001, "Port to run the development server on")
	cmd.Flags().String("host", "0.0.0.0", "Host to bind the server to")
	cmd.Flags().Bool("cors", false, "Enable CORS")
	cmd.Flags().String("cwd", "", "Working directory for the project")
	cmd.Flags().String("config", "compozy.yaml", "Path to the project configuration file")
	return cmd
}
