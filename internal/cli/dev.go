package cli

import (
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
			natsServer, err := nats.NewNatsServer(nats.DefaultServerOptions())
			if err != nil {
				logger.Error("Failed to setup NATS server", "error", err)
				return err
			}
			defer natsServer.Shutdown()

			// Resolve paths
			pCWD, err := common.CWDFromPath(cwd)
			if err != nil {
				return err
			}

			logger.Info("Starting compozy server")
			logger.Debug("Loading config file", "path", config)

			// Load project configuration
			projectConfig, err := project.Load(pCWD, config)
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
			appState, err := server.NewAppState(projectConfig, workflows, natsServer)
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
	cmd.Flags().String("config", "./compozy.yaml", "Path to the project configuration file")

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
