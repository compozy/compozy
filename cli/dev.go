package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

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

// getBasicServerFlags extracts basic server configuration flags
func getBasicServerFlags(cmd *cobra.Command) (host string, port int, cors bool, configFile string, err error) {
	if port, err = cmd.Flags().GetInt("port"); err != nil {
		return "", 0, false, "", fmt.Errorf("failed to get port flag: %w", err)
	}
	if host, err = cmd.Flags().GetString("host"); err != nil {
		return "", 0, false, "", fmt.Errorf("failed to get host flag: %w", err)
	}
	if cors, err = cmd.Flags().GetBool("cors"); err != nil {
		return "", 0, false, "", fmt.Errorf("failed to get cors flag: %w", err)
	}
	if configFile, err = cmd.Flags().GetString("config"); err != nil {
		return "", 0, false, "", fmt.Errorf("failed to get config flag: %w", err)
	}
	return host, port, cors, configFile, nil
}

// setEnvironmentVariablesFromFlags sets environment variables from command flags if not already set
func setEnvironmentVariablesFromFlags(cmd *cobra.Command) error {
	maxNestingDepth, err := cmd.Flags().GetInt("max-nesting-depth")
	if err != nil {
		return fmt.Errorf("failed to get max-nesting-depth flag: %w", err)
	}
	if os.Getenv("MAX_NESTING_DEPTH") == "" {
		os.Setenv("MAX_NESTING_DEPTH", fmt.Sprintf("%d", maxNestingDepth))
	}
	dispatcherHeartbeatInterval, err := cmd.Flags().GetInt("dispatcher-heartbeat-interval")
	if err != nil {
		return fmt.Errorf("failed to get dispatcher-heartbeat-interval flag: %w", err)
	}
	if os.Getenv("DISPATCHER_HEARTBEAT_INTERVAL") == "" {
		os.Setenv("DISPATCHER_HEARTBEAT_INTERVAL", fmt.Sprintf("%d", dispatcherHeartbeatInterval))
	}
	dispatcherHeartbeatTTL, err := cmd.Flags().GetInt("dispatcher-heartbeat-ttl")
	if err != nil {
		return fmt.Errorf("failed to get dispatcher-heartbeat-ttl flag: %w", err)
	}
	if os.Getenv("DISPATCHER_HEARTBEAT_TTL") == "" {
		os.Setenv("DISPATCHER_HEARTBEAT_TTL", fmt.Sprintf("%d", dispatcherHeartbeatTTL))
	}
	dispatcherStaleThreshold, err := cmd.Flags().GetInt("dispatcher-stale-threshold")
	if err != nil {
		return fmt.Errorf("failed to get dispatcher-stale-threshold flag: %w", err)
	}
	if os.Getenv("DISPATCHER_STALE_THRESHOLD") == "" {
		os.Setenv("DISPATCHER_STALE_THRESHOLD", fmt.Sprintf("%d", dispatcherStaleThreshold))
	}
	maxMessageContentLength, err := cmd.Flags().GetInt("max-message-content-length")
	if err != nil {
		return fmt.Errorf("failed to get max-message-content-length flag: %w", err)
	}
	if os.Getenv("MAX_MESSAGE_CONTENT_LENGTH") == "" {
		os.Setenv("MAX_MESSAGE_CONTENT_LENGTH", fmt.Sprintf("%d", maxMessageContentLength))
	}
	maxTotalContentSize, err := cmd.Flags().GetInt("max-total-content-size")
	if err != nil {
		return fmt.Errorf("failed to get max-total-content-size flag: %w", err)
	}
	if os.Getenv("MAX_TOTAL_CONTENT_SIZE") == "" {
		os.Setenv("MAX_TOTAL_CONTENT_SIZE", fmt.Sprintf("%d", maxTotalContentSize))
	}
	return nil
}

func getServerConfig(ctx context.Context, cmd *cobra.Command, envFilePath string) (*server.Config, error) {
	log := logger.FromContext(ctx)
	host, port, cors, configFile, err := getBasicServerFlags(cmd)
	if err != nil {
		return nil, err
	}
	CWD, _, err := utils.GetConfigCWD(cmd)
	if err != nil {
		return nil, err
	}
	availablePort, err := findAvailablePort(host, port)
	if err != nil {
		return nil, fmt.Errorf("no free port found near %d: %w", port, err)
	}
	if availablePort != port {
		log.Info("Port unavailable, using alternative port", "requested_port", port, "available_port", availablePort)
	}
	if err := setEnvironmentVariablesFromFlags(cmd); err != nil {
		return nil, err
	}
	return &server.Config{
		CWD:         CWD,
		Host:        host,
		Port:        availablePort,
		CORSEnabled: cors,
		ConfigFile:  configFile,
		EnvFilePath: envFilePath,
	}, nil
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

func getToolExecutionTimeout(cmd *cobra.Command) (time.Duration, error) {
	timeout, err := cmd.Flags().GetDuration("tool-execution-timeout")
	if err != nil {
		return 0, fmt.Errorf("failed to get tool-execution-timeout flag: %w", err)
	}

	// Use env var as fallback if flag was not explicitly set
	if !cmd.Flags().Changed("tool-execution-timeout") {
		if envTimeout := os.Getenv("TOOL_EXECUTION_TIMEOUT"); envTimeout != "" {
			parsedTimeout, err := time.ParseDuration(envTimeout)
			if err != nil {
				return 0, fmt.Errorf("failed to parse TOOL_EXECUTION_TIMEOUT: %w", err)
			}
			timeout = parsedTimeout
		}
	}

	// Validate timeout is positive
	if timeout <= 0 {
		return 0, fmt.Errorf("tool execution timeout must be positive, got %v", timeout)
	}

	return timeout, nil
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

	// Tool execution configuration flags
	cmd.Flags().
		Duration("tool-execution-timeout", 60*time.Second, "Tool execution timeout (env: TOOL_EXECUTION_TIMEOUT)")

	// Logging configuration flags
	cmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.Flags().Bool("log-json", false, "Output logs in JSON format")
	cmd.Flags().Bool("log-source", false, "Include source file and line in logs")
	cmd.Flags().Bool("debug", false, "Enable debug mode (sets log level to debug)")
	cmd.Flags().Bool("watch", false, "Enable file watcher to restart server on change")

	// Task execution configuration flags
	cmd.Flags().Int("max-nesting-depth", 20, "Maximum task nesting depth allowed (env: MAX_NESTING_DEPTH)")

	// Memory content size configuration flags
	cmd.Flags().
		Int("max-message-content-length", 10240, "Maximum message content length in bytes (env: MAX_MESSAGE_CONTENT_LENGTH)")
	cmd.Flags().
		Int("max-total-content-size", 102400, "Maximum total content size in bytes (env: MAX_TOTAL_CONTENT_SIZE)")

	// Dispatcher heartbeat configuration flags
	cmd.Flags().Int("dispatcher-heartbeat-interval", 30,
		"Dispatcher heartbeat interval in seconds (env: DISPATCHER_HEARTBEAT_INTERVAL)")
	cmd.Flags().
		Int("dispatcher-heartbeat-ttl", 300, "Dispatcher heartbeat TTL in seconds (env: DISPATCHER_HEARTBEAT_TTL)")
	cmd.Flags().
		Int("dispatcher-stale-threshold", 120, "Dispatcher stale threshold in seconds (env: DISPATCHER_STALE_THRESHOLD)")

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

// isPortAvailable checks if a port is available for binding
func isPortAvailable(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// findAvailablePort finds the next available port starting from the given port
func findAvailablePort(host string, startPort int) (int, error) {
	maxAttempts := 100 // Prevent infinite loops
	for i := 0; i < maxAttempts; i++ {
		port := startPort + i
		if isPortAvailable(host, port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found near %d", startPort)
}

func loadEnvFile(cmd *cobra.Command) (string, error) {
	envFile, err := cmd.Flags().GetString("env-file")
	if err != nil {
		return "", fmt.Errorf("failed to get env-file flag: %w", err)
	}

	if envFile != "" {
		// Get the current working directory before any cwd changes
		pwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}

		// Resolve env file path relative to original working directory
		if !filepath.IsAbs(envFile) {
			envFile = filepath.Join(pwd, envFile)
		}

		if err := godotenv.Load(envFile); err != nil {
			// Don't fail if the env file doesn't exist, just log a debug message
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("failed to load env file %s: %w", envFile, err)
			}
		}
	}
	return envFile, nil
}

func handleDevCmd(cmd *cobra.Command, _ []string) error {
	gin.SetMode(gin.ReleaseMode)

	// Load environment variables from the specified file first
	envFilePath, err := loadEnvFile(cmd)
	if err != nil {
		return err
	}

	// Setup logging
	logLevel, logJSON, logSource, err := logger.GetLoggerConfig(cmd)
	if err != nil {
		return err
	}
	log := logger.SetupLogger(logLevel, logJSON, logSource)

	// Create context with logger
	ctx := context.Background()
	ctx = logger.ContextWithLogger(ctx, log)

	// Get server configuration
	scfg, err := getServerConfig(ctx, cmd, envFilePath)
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

	// Get tool execution timeout
	toolTimeout, err := getToolExecutionTimeout(cmd)
	if err != nil {
		return err
	}

	// Set tool execution timeout as environment variable
	if err := os.Setenv("TOOL_EXECUTION_TIMEOUT", toolTimeout.String()); err != nil {
		return fmt.Errorf("failed to set TOOL_EXECUTION_TIMEOUT environment variable: %w", err)
	}

	watch, err := cmd.Flags().GetBool("watch")
	if err != nil {
		return fmt.Errorf("failed to get watch flag: %w", err)
	}
	if watch {
		watcher, err := setupWatcher(ctx, scfg.CWD)
		if err != nil {
			return err
		}
		defer watcher.Close()
		restartChan := make(chan bool, 1)
		go startWatcher(ctx, watcher, restartChan)
		return runAndWatchServer(ctx, scfg, tcfg, dbCfg, restartChan)
	}

	srv := server.NewServer(ctx, scfg, tcfg, dbCfg)
	return srv.Run()
}

func setupWatcher(ctx context.Context, cwd string) (*fsnotify.Watcher, error) {
	log := logger.FromContext(ctx)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}
	// Close watcher when context is canceled to prevent goroutine leaks
	go func() {
		<-ctx.Done()
		_ = watcher.Close()
	}()
	if err := filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml") {
			if err := watcher.Add(path); err != nil {
				log.Warn("Failed to watch file", "path", path, "error", err)
			}
		}
		return nil
	}); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to walk project directory: %w", err)
	}
	return watcher, nil
}

func startWatcher(ctx context.Context, watcher *fsnotify.Watcher, restartChan chan bool) {
	log := logger.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Info("Context canceled, stopping file watcher")
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				log.Debug("Detected file change, sending restart signal...", "file", event.Name)
				select {
				case restartChan <- true:
				default: // restart already pending
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Error("Watcher error", "error", err)
		}
	}
}

func runAndWatchServer(
	ctx context.Context,
	scfg *server.Config,
	tcfg *worker.TemporalConfig,
	dbCfg *store.Config,
	restartChan chan bool,
) error {
	log := logger.FromContext(ctx)
	var retryDelay = 500 * time.Millisecond
	const maxRetryDelay = 30 * time.Second
	for {
		// Find available port on each restart in case the original port becomes free
		availablePort, err := findAvailablePort(scfg.Host, scfg.Port)
		if err != nil {
			return fmt.Errorf("no free port found near %d: %w", scfg.Port, err)
		}
		if availablePort != scfg.Port {
			log.Info("port conflict on restart, using next available port",
				"original_port", scfg.Port,
				"available_port", availablePort)
			scfg.Port = availablePort
		}

		srv := server.NewServer(ctx, scfg, tcfg, dbCfg)
		serverErrChan := make(chan error, 1)
		go func() {
			serverErrChan <- srv.Run()
		}()
		log.Info("Server started. Watching for file changes.")
		select {
		case <-restartChan:
			log.Info("Restart signal received. Shutting down server...")
			srv.Shutdown()
			<-serverErrChan // Wait for shutdown to complete
			log.Info("Server shut down. Restarting...")
			// Reset retry delay on successful file-based restart
			retryDelay = 500 * time.Millisecond
			// Drain the channel in case of multiple file change events
			for len(restartChan) > 0 {
				<-restartChan
			}
			continue // Restart the loop
		case err := <-serverErrChan:
			if err != nil {
				log.Error("Server stopped with error", "error", err)
				// Use exponential back-off to prevent tight restart loops on server failures
				log.Debug("Waiting before retry...", "delay", retryDelay)
				time.Sleep(retryDelay)
				// Double the delay for next retry, up to maximum
				retryDelay *= 2
				if retryDelay > maxRetryDelay {
					retryDelay = maxRetryDelay
				}
				continue // Retry after back-off
			}
			log.Info("Server stopped.")
			return nil
		}
	}
}
