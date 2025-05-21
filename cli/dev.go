package cli

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/pkg/app"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/compozy/compozy/server"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// getServerConfig extracts server configuration from command flags
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

	cwd, _, err := utils.GetConfigCWD(cmd)
	if err != nil {
		return nil, err
	}

	serverConfig := &server.Config{
		CWD:         cwd,
		Host:        host,
		Port:        port,
		CORSEnabled: cors,
	}
	return serverConfig, nil
}

// setupNatsServer initializes and returns a NATS server
func setupNatsServer() (*nats.Server, error) {
	opts := nats.DefaultServerOptions()
	opts.EnableJetStream = true
	natsServer, err := nats.NewNatsServer(opts)
	if err != nil {
		logger.Error("Failed to setup NATS server", "error", err)
		return nil, err
	}
	return natsServer, nil
}

// loadProjectConfig loads and validates project configuration
func loadProjectConfig(cmd *cobra.Command) (*project.Config, error) {
	cwd, config, err := utils.GetConfigCWD(cmd)
	if err != nil {
		return nil, err
	}

	pCWD, err := common.CWDFromPath(cwd)
	if err != nil {
		return nil, err
	}

	logger.Info("Starting compozy server")
	logger.Debug("Loading config file", "config_file", config)

	projectConfig, err := project.Load(pCWD, config)
	if err != nil {
		logger.Error("Failed to load project config", "error", err)
		return nil, err
	}

	if err := projectConfig.Validate(); err != nil {
		logger.Error("Invalid project config", "error", err)
		return nil, err
	}

	return projectConfig, nil
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
	// Set Gin to release mode to reduce debug output
	gin.SetMode(gin.ReleaseMode)

	// Get server configuration
	serverConfig, err := getServerConfig(cmd)
	if err != nil {
		return err
	}

	// Setup logger
	logLevel, logJSON, logSource, err := logger.GetLoggerConfig(cmd)
	if err != nil {
		return err
	}
	logger.SetupLogger(logLevel, logJSON, logSource)

	// Load project configuration
	projectConfig, err := loadProjectConfig(cmd)
	if err != nil {
		return err
	}

	// Setup NATS server
	natsServer, err := setupNatsServer()
	if err != nil {
		return err
	}
	defer func() {
		if err := natsServer.Shutdown(); err != nil {
			logger.Error("Error shutting down NATS server", "error", err)
		}
	}()

	// Load workflows from sources
	workflows, err := projectConfig.WorkflowsFromSources()
	if err != nil {
		logger.Error("Failed to load workflows", "error", err)
		return err
	}

	// Create app state
	appState, err := app.NewState(projectConfig, workflows, natsServer)
	if err != nil {
		logger.Error("Failed to create app state", "error", err)
		return err
	}

	// Create and run server
	srv := server.NewServer(serverConfig, appState)
	return srv.Run()
}
