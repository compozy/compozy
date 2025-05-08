package cli

import (
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/nats"
	"github.com/compozy/compozy/internal/parser/project"
	"github.com/compozy/compozy/internal/server"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// setupLogger initializes the logger with the given configuration
func setupLogger(logLevel string, logJSON, logSource bool) {
	// Parse log level
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

// handleNatsLogMessage converts and logs a NATS log message using our logger
func handleNatsLogMessage(msg *nats.LogMessage) {
	// Convert NATS log level to our logger level
	var logLevel log.Level
	switch msg.Level {
	case nats.DebugLevel:
		logLevel = log.DebugLevel
	case nats.InfoLevel:
		logLevel = log.InfoLevel
	case nats.WarnLevel:
		logLevel = log.WarnLevel
	case nats.ErrorLevel:
		logLevel = log.ErrorLevel
	default:
		logLevel = log.InfoLevel
	}

	// Add context fields if present
	fields := make([]any, 0)
	if msg.Context != nil {
		for k, v := range msg.Context {
			fields = append(fields, k, v)
		}
	}

	// Log the message with appropriate level
	switch logLevel {
	case log.DebugLevel:
		logger.Debug(msg.Message, fields...)
	case log.InfoLevel:
		logger.Info(msg.Message, fields...)
	case log.WarnLevel:
		logger.Warn(msg.Message, fields...)
	case log.ErrorLevel:
		logger.Error(msg.Message, fields...)
	}
}

// setupNatsServer starts the NATS server and sets up log message subscription
func setupNatsServer() (*nats.NatsServer, error) {
	// Start NATS server
	natsServer, err := nats.NewNatsServer(nats.DefaultServerOptions())
	if err != nil {
		return nil, err
	}

	// Subscribe to log messages
	_, err = natsServer.SubscribeToLogs(handleNatsLogMessage)
	if err != nil {
		natsServer.Shutdown() // Clean up on error
		return nil, err
	}

	return natsServer, nil
}

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
			logLevel, _ := cmd.Flags().GetString("log-level")
			logJSON, _ := cmd.Flags().GetBool("log-json")
			logSource, _ := cmd.Flags().GetBool("log-source")

			// Set Gin to release mode to reduce debug output
			gin.SetMode(gin.ReleaseMode)

			// Setup logger
			setupLogger(logLevel, logJSON, logSource)

			// Setup NATS server
			natsServer, err := setupNatsServer()
			if err != nil {
				logger.Error("Failed to setup NATS server", "error", err)
				return err
			}
			defer natsServer.Shutdown()

			// Resolve paths
			if cwd == "" {
				var err error
				cwd, err = filepath.Abs(".")
				if err != nil {
					logger.Error("Failed to resolve current directory", "error", err)
					return err
				}
			}
			configPath := filepath.Join(cwd, config)

			logger.Info("Starting compozy server")
			logger.Debug("Loading config file", "path", configPath)

			// Load project configuration
			projectConfig, err := project.Load(configPath)
			if err != nil {
				logger.Error("Failed to load project config", "error", err)
				return err
			}

			// Validate project configuration
			if err := projectConfig.Validate(); err != nil {
				logger.Error("Invalid project config", "error", err)
				return err
			}

			// Load workflows from sources
			workflows, err := projectConfig.WorkflowsFromSources()
			if err != nil {
				logger.Error("Failed to load workflows", "error", err)
				return err
			}

			// Create app state
			appState, err := server.NewAppState(cwd, workflows)
			if err != nil {
				logger.Error("Failed to create app state", "error", err)
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
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if debug, _ := cmd.Flags().GetBool("debug"); debug {
			return cmd.Flags().Set("log-level", "debug")
		}
		return nil
	}

	return cmd
}
