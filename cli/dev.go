package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
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

func getDatabaseConfig(cmd *cobra.Command) (*store.Config, error) {
	dbHost, err := cmd.Flags().GetString("db-host")
	if err != nil {
		return nil, fmt.Errorf("failed to get db-host flag: %w", err)
	}
	dbPort, err := cmd.Flags().GetString("db-port")
	if err != nil {
		return nil, fmt.Errorf("failed to get db-port flag: %w", err)
	}
	dbUser, err := cmd.Flags().GetString("db-user")
	if err != nil {
		return nil, fmt.Errorf("failed to get db-user flag: %w", err)
	}
	dbPassword, err := cmd.Flags().GetString("db-password")
	if err != nil {
		return nil, fmt.Errorf("failed to get db-password flag: %w", err)
	}
	dbName, err := cmd.Flags().GetString("db-name")
	if err != nil {
		return nil, fmt.Errorf("failed to get db-name flag: %w", err)
	}
	dbSSLMode, err := cmd.Flags().GetString("db-ssl-mode")
	if err != nil {
		return nil, fmt.Errorf("failed to get db-ssl-mode flag: %w", err)
	}
	dbConnString, err := cmd.Flags().GetString("db-conn-string")
	if err != nil {
		return nil, fmt.Errorf("failed to get db-conn-string flag: %w", err)
	}

	// Use env vars as fallback if flags are empty
	if dbHost == "" {
		dbHost = getEnvOrDefault("DB_HOST", "localhost")
	}
	if dbPort == "" {
		dbPort = getEnvOrDefault("DB_PORT", "5432")
	}
	if dbUser == "" {
		dbUser = getEnvOrDefault("DB_USER", "postgres")
	}
	if dbPassword == "" {
		dbPassword = getEnvOrDefault("DB_PASSWORD", "")
	}
	if dbName == "" {
		dbName = getEnvOrDefault("DB_NAME", "compozy")
	}
	if dbSSLMode == "" {
		dbSSLMode = getEnvOrDefault("DB_SSL_MODE", "disable")
	}
	if dbConnString == "" {
		dbConnString = getEnvOrDefault("DB_CONN_STRING", "")
	}

	return &store.Config{
		ConnString: dbConnString,
		Host:       dbHost,
		Port:       dbPort,
		User:       dbUser,
		Password:   dbPassword,
		DBName:     dbName,
		SSLMode:    dbSSLMode,
	}, nil
}

func getTemporalConfig(cmd *cobra.Command) (*worker.TemporalConfig, error) {
	hostPort, err := cmd.Flags().GetString("temporal-host")
	if err != nil {
		return nil, fmt.Errorf("failed to get temporal-host flag: %w", err)
	}
	namespace, err := cmd.Flags().GetString("temporal-namespace")
	if err != nil {
		return nil, fmt.Errorf("failed to get temporal-namespace flag: %w", err)
	}

	// Use env vars as fallback if flags are empty
	if hostPort == "" {
		temporalHost := getEnvOrDefault("TEMPORAL_HOST", "localhost")
		temporalPort := getEnvOrDefault("TEMPORAL_PORT", "7233")
		hostPort = fmt.Sprintf("%s:%s", temporalHost, temporalPort)
	}
	if namespace == "" {
		namespace = getEnvOrDefault("TEMPORAL_NAMESPACE", "default")
	}
	return &worker.TemporalConfig{
		HostPort:  hostPort,
		Namespace: namespace,
	}, nil
}

func getOpenAIConfig(cmd *cobra.Command) (string, error) {
	apiKey, err := cmd.Flags().GetString("openai-api-key")
	if err != nil {
		return "", fmt.Errorf("failed to get openai-api-key flag: %w", err)
	}

	// Use env vars as fallback if flag is empty
	if apiKey == "" {
		apiKey = getEnvOrDefault("OPENAI_API_KEY", "")
	}

	return apiKey, nil
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
	cmd.Flags().String("env-file", ".env", "Path to the environment variables file")

	// Database configuration flags
	cmd.Flags().String("db-host", "", "Database host (env: DB_HOST)")
	cmd.Flags().String("db-port", "", "Database port (env: DB_PORT)")
	cmd.Flags().String("db-user", "", "Database user (env: DB_USER)")
	cmd.Flags().String("db-password", "", "Database password (env: DB_PASSWORD)")
	cmd.Flags().String("db-name", "", "Database name (env: DB_NAME)")
	cmd.Flags().String("db-ssl-mode", "", "Database SSL mode (env: DB_SSL_MODE)")
	cmd.Flags().String("db-conn-string", "", "Database connection string (env: DB_CONN_STRING)")

	// Temporal configuration flags
	cmd.Flags().String("temporal-host", "", "Temporal host:port (env: TEMPORAL_HOST:TEMPORAL_PORT)")
	cmd.Flags().String("temporal-namespace", "", "Temporal namespace (env: TEMPORAL_NAMESPACE)")
	cmd.Flags().String("temporal-task-queue", "", "Temporal task queue name (env: TEMPORAL_TASK_QUEUE)")

	// OpenAI configuration flags
	cmd.Flags().String("openai-api-key", "", "OpenAI API key (env: OPENAI_API_KEY)")

	// Logging configuration flags
	cmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.Flags().Bool("log-json", false, "Output logs in JSON format")
	cmd.Flags().Bool("log-source", false, "Include source file and line in logs")
	cmd.Flags().Bool("debug", false, "Enable debug mode (sets log level to debug)")
	cmd.Flags().Bool("watch", false, "Enable file watcher to restart server on change")

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

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func loadEnvFile(cmd *cobra.Command) error {
	envFile, err := cmd.Flags().GetString("env-file")
	if err != nil {
		return fmt.Errorf("failed to get env-file flag: %w", err)
	}

	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			// Don't fail if the env file doesn't exist, just log a debug message
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to load env file %s: %w", envFile, err)
			}
		}
	}
	return nil
}

func handleDevCmd(cmd *cobra.Command, _ []string) error {
	gin.SetMode(gin.ReleaseMode)

	// Load environment variables from the specified file first
	if err := loadEnvFile(cmd); err != nil {
		return err
	}

	// Get server configuration
	scfg, err := getServerConfig(cmd)
	if err != nil {
		return err
	}

	// Get database configuration
	dbCfg, err := getDatabaseConfig(cmd)
	if err != nil {
		return err
	}

	// Get Temporal configuration
	tcfg, err := getTemporalConfig(cmd)
	if err != nil {
		return err
	}

	// Get OpenAI configuration
	openaiAPIKey, err := getOpenAIConfig(cmd)
	if err != nil {
		return err
	}

	// Set OpenAI API key as environment variable if provided
	if openaiAPIKey != "" {
		if err := os.Setenv("OPENAI_API_KEY", openaiAPIKey); err != nil {
			return fmt.Errorf("failed to set OPENAI_API_KEY environment variable: %w", err)
		}
	}

	// Setup logging
	logLevel, logJSON, logSource, err := logger.GetLoggerConfig(cmd)
	if err != nil {
		return err
	}
	if err := logger.SetupLogger(logLevel, logJSON, logSource); err != nil {
		return err
	}

	watch, err := cmd.Flags().GetBool("watch")
	if err != nil {
		return fmt.Errorf("failed to get watch flag: %w", err)
	}
	if watch {
		watcher, err := setupWatcher(scfg.CWD)
		if err != nil {
			return err
		}
		defer watcher.Close()
		restartChan := make(chan bool)
		go startWatcher(watcher, restartChan)
		return runAndWatchServer(scfg, tcfg, dbCfg, restartChan)
	}

	srv := server.NewServer(*scfg, tcfg, dbCfg)
	return srv.Run()
}

func setupWatcher(cwd string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}
	if err := filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml") {
			if err := watcher.Add(path); err != nil {
				logger.Warn("Failed to watch file", "path", path, "error", err)
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to walk project directory: %w", err)
	}
	return watcher, nil
}

func startWatcher(watcher *fsnotify.Watcher, restartChan chan bool) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				logger.Info("Detected file change, sending restart signal...", "file", event.Name)
				restartChan <- true
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Error("Watcher error", "error", err)
		}
	}
}

func runAndWatchServer(
	scfg *server.Config,
	tcfg *worker.TemporalConfig,
	dbCfg *store.Config,
	restartChan chan bool,
) error {
	for {
		srv := server.NewServer(*scfg, tcfg, dbCfg)
		serverErrChan := make(chan error, 1)
		go func() {
			serverErrChan <- srv.Run()
		}()
		logger.Info("Server started. Watching for file changes.")
		select {
		case <-restartChan:
			logger.Info("Restart signal received. Shutting down server...")
			srv.Shutdown()
			<-serverErrChan // Wait for shutdown to complete
			logger.Info("Server shut down. Restarting...")
			// Drain the channel in case of multiple file change events
			for len(restartChan) > 0 {
				<-restartChan
			}
			continue // Restart the loop
		case err := <-serverErrChan:
			if err != nil {
				logger.Error("Server stopped with error", "error", err)
				return err
			}
			logger.Info("Server stopped.")
			return nil
		}
	}
}
