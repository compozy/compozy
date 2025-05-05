package commands

import (
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// DevCmd returns the dev command
func DevCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the Compozy development server",
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetUint16("port")
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

			// TODO: Load project config (requires parser package)
			// TODO: Implement server startup (requires server package)
			logrus.Infof("Server would run on %s:%d with CORS=%v", host, port, cors)

			return nil
		},
	}

	cmd.Flags().Uint16("port", 3001, "Port to run the development server on")
	cmd.Flags().String("host", "0.0.0.0", "Host to bind the server to")
	cmd.Flags().Bool("cors", false, "Enable CORS")
	return cmd
}
