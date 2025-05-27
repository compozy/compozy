package cli

import (
	"fmt"

	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

func getServerConfig(cmd *cobra.Command) (*server.Config, error) {
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		return nil, fmt.Errorf("failed to get port flag: %w", err)
	}
	host, err := cmd.Flags().GetString("host")
	if err != nil {
		return nil, fmt.Errorf("failed to get host flag: %w", err)
	}
	cors, err := cmd.Flags().GetBool("cors")
	if err != nil {
		return nil, fmt.Errorf("failed to get cors flag: %w", err)
	}
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return nil, fmt.Errorf("failed to get config flag: %w", err)
	}
	cwd, _, err := utils.GetConfigCWD(cmd)
	if err != nil {
		return nil, err
	}
	serverConfig := &server.Config{
		CWD:         cwd,
		Host:        host,
		Port:        port,
		CORSEnabled: cors,
		ConfigFile:  configFile,
	}
	return serverConfig, nil
}

// DevCmd returns the dev command
func DevCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the Compozy development server",
		RunE:  handleDevCmd,
	}

	// Server configuration flags
	cmd.Flags().Int("port", 3001, "Port to run the development server on")
	cmd.Flags().String("host", "0.0.0.0", "Host to bind the server to")
	cmd.Flags().Bool("cors", false, "Enable CORS")
	cmd.Flags().String("cwd", "", "Working directory for the project")
	cmd.Flags().String("config", "compozy.yaml", "Path to the project configuration file")

	// Logging configuration flags
	cmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.Flags().Bool("log-json", false, "Output logs in JSON format")
	cmd.Flags().Bool("log-source", false, "Include source file and line in logs")
	cmd.Flags().Bool("debug", false, "Enable debug mode (sets log level to debug)")

	// Set debug flag to override log level
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		debug, err := cmd.Flags().GetBool("debug")
		if err != nil {
			return fmt.Errorf("failed to get debug flag: %w", err)
		}

		if debug {
			return cmd.Flags().Set("log-level", "debug")
		}
		return nil
	}

	return cmd
}

func handleDevCmd(cmd *cobra.Command, _ []string) error {
	gin.SetMode(gin.ReleaseMode)
	scfg, err := getServerConfig(cmd)
	if err != nil {
		return err
	}

	logLevel, logJSON, logSource, err := logger.GetLoggerConfig(cmd)
	if err != nil {
		return err
	}
	logger.SetupLogger(logLevel, logJSON, logSource)
	srv := server.NewServer(*scfg)
	return srv.Run()
}
