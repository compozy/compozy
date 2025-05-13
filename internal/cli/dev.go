package cli

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/nats"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/project"
	"github.com/compozy/compozy/internal/server"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// setupLogger initializes the logger with the given configuration
func setupLogger(logLevel string, logJSON, logSource bool) {
	var level log.Level
	switch logLevel {
	case "debug":
		level = log.DebugLevel
	case "info":
		level = log.InfoLevel
	case "warn":
		level = log.WarnLevel
	case "error":
		level = log.ErrorLevel
	default:
		level = log.InfoLevel
	}

	// Initialize logger with development-friendly settings
	logger.Init(&logger.Config{
		Level:      level,
		JSON:       logJSON,
		AddSource:  logSource,
		TimeFormat: "15:04:05", // Use time format with seconds
	})
}

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

	cwd, _, _, err := GetCommonFlags(cmd)
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

// getLoggerConfig extracts logger configuration from command flags
func getLoggerConfig(cmd *cobra.Command) (string, bool, bool, error) {
	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return "", false, false, fmt.Errorf("failed to get log-level flag: %w", err)
	}

	logJSON, err := cmd.Flags().GetBool("log-json")
	if err != nil {
		return "", false, false, fmt.Errorf("failed to get log-json flag: %w", err)
	}

	logSource, err := cmd.Flags().GetBool("log-source")
	if err != nil {
		return "", false, false, fmt.Errorf("failed to get log-source flag: %w", err)
	}

	return logLevel, logJSON, logSource, nil
}

// setupNatsServer initializes and returns a NATS server
func setupNatsServer() (*nats.Server, error) {
	natsServer, err := nats.NewNatsServer(nats.DefaultServerOptions())
	if err != nil {
		logger.Error("Failed to setup NATS server", "error", err)
		return nil, err
	}
	return natsServer, nil
}

// loadProjectConfig loads and validates project configuration
func loadProjectConfig(cwd, configPath string) (*project.Config, error) {
	pCWD, err := common.CWDFromPath(cwd)
	if err != nil {
		return nil, err
	}

	logger.Info("Starting compozy server")
	logger.Debug("Loading config file", "path", configPath)

	projectConfig, err := project.Load(pCWD, configPath)
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Set Gin to release mode to reduce debug output
			gin.SetMode(gin.ReleaseMode)

			// Get server configuration
			serverConfig, err := getServerConfig(cmd)
			if err != nil {
				return err
			}

			// Get common flags
			cwd, _, configPath, err := GetCommonFlags(cmd)
			if err != nil {
				return err
			}

			// Setup logger
			logLevel, logJSON, logSource, err := getLoggerConfig(cmd)
			if err != nil {
				return err
			}
			setupLogger(logLevel, logJSON, logSource)

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

			// Load project configuration
			projectConfig, err := loadProjectConfig(cwd, configPath)
			if err != nil {
				return err
			}

			// Load workflows from sources
			workflows, err := projectConfig.WorkflowsFromSources()
			if err != nil {
				logger.Error("Failed to load workflows", "error", err)
				return err
			}

			// Create app state
			appState, err := server.NewAppState(projectConfig, workflows, natsServer)
			if err != nil {
				logger.Error("Failed to create app state", "error", err)
				return err
			}

			// Create and run server
			srv := server.NewServer(serverConfig, appState)
			return srv.Run()
		},
	}

	// Server configuration flags
	cmd.Flags().Int("port", 3001, "Port to run the development server on")
	cmd.Flags().String("host", "0.0.0.0", "Host to bind the server to")
	cmd.Flags().Bool("cors", false, "Enable CORS")
	AddCommonFlags(cmd)

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
