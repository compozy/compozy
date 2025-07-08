package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// Constants for dev server configuration
const (
	// Default timeouts
	defaultToolExecutionTimeout = 60 * time.Second

	// Port scanning
	maxPortScanAttempts = 100

	// Server restart delays
	initialRetryDelay = 500 * time.Millisecond
	maxRetryDelay     = 30 * time.Second

	// File watcher debounce delay
	fileChangeDebounceDelay = 200 * time.Millisecond

	// Default values
	defaultPort              = 3001
	defaultHost              = "0.0.0.0"
	defaultConfigFile        = "compozy.yaml"
	defaultEnvFile           = ".env"
	defaultLogLevel          = "info"
	defaultMaxNesting        = 20
	defaultMaxStringLen      = 10485760 // 10MB
	defaultMaxMsgContent     = 10240    // 10KB
	defaultMaxTotalContent   = 102400   // 100KB
	defaultAsyncWorkers      = 10
	defaultAsyncBuffer       = 1000
	defaultHeartbeatInterval = 30
	defaultHeartbeatTTL      = 300
	defaultStaleThreshold    = 120
)

// ignoredDirs contains directories that should be skipped during file watching
var ignoredDirs = map[string]bool{
	".git":          true,
	"node_modules":  true,
	".idea":         true,
	".vscode":       true,
	"vendor":        true,
	"dist":          true,
	"build":         true,
	"tmp":           true,
	"temp":          true,
	".cache":        true,
	"__pycache__":   true,
	".pytest_cache": true,
	".next":         true,
	".nuxt":         true,
	"coverage":      true,
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

// loadUnifiedConfig loads configuration using pkg/config with CLI flag overrides
func loadUnifiedConfig(ctx context.Context, cmd *cobra.Command, _ string) (*config.Config, error) {
	// Create configuration service
	service := config.NewService()

	// Create sources for configuration loading
	sources := []config.Source{
		config.NewEnvProvider(),
	}

	// Note: Default values are loaded automatically by the loader's Load method,
	// so we don't need to include NewDefaultProvider() in the sources list

	// Note: YAML configuration is handled by the project loader (engine/project),
	// not by the application configuration system (pkg/config)

	// Add CLI source for flag overrides (highest precedence)
	cliFlags := make(map[string]any)
	extractCLIFlags(cmd, cliFlags)
	if len(cliFlags) > 0 {
		sources = append(sources, config.NewCLIProvider(cliFlags))
	}

	// Load configuration from all sources
	cfg, err := service.Load(ctx, sources...)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return cfg, nil
}

// extractCLIFlags extracts changed CLI flags into a map for configuration override
func extractCLIFlags(cmd *cobra.Command, flags map[string]any) {
	// Generic helper to add any flag type
	addFlag := func(flagName, key string, getter func(string) (any, error)) {
		if cmd.Flags().Changed(flagName) {
			if value, err := getter(flagName); err == nil {
				flags[key] = value
			}
		}
	}

	// Define flag extractors with proper type conversion
	getString := func(name string) (any, error) { return cmd.Flags().GetString(name) }
	getInt := func(name string) (any, error) { return cmd.Flags().GetInt(name) }
	getBool := func(name string) (any, error) { return cmd.Flags().GetBool(name) }

	// Flag definitions with their types
	flagDefs := []struct {
		flagName string
		key      string
		getter   func(string) (any, error)
	}{
		// Server flags
		{"host", "host", getString},
		{"port", "port", getInt},
		{"cors", "cors", getBool},

		// Database flags
		{"db-host", "db-host", getString},
		{"db-port", "db-port", getString},
		{"db-user", "db-user", getString},
		{"db-password", "db-password", getString},
		{"db-name", "db-name", getString},
		{"db-ssl-mode", "db-ssl-mode", getString},
		{"db-conn-string", "db-conn-string", getString},

		// Temporal flags
		{"temporal-host", "temporal-host", getString},
		{"temporal-namespace", "temporal-namespace", getString},
		{"temporal-task-queue", "temporal-task-queue", getString},
	}

	// Process all flags
	for _, def := range flagDefs {
		addFlag(def.flagName, def.key, def.getter)
	}
}

// DevCmd returns the dev command
func DevCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the Compozy development server",
		RunE:  handleDevCmd,
	}

	// Server configuration flags
	cmd.Flags().Int("port", defaultPort, "Port to run the development server on")
	cmd.Flags().String("host", defaultHost, "Host to bind the server to")
	cmd.Flags().Bool("cors", false, "Enable CORS")
	cmd.Flags().String("cwd", "", "Working directory for the project")
	cmd.Flags().String("config", defaultConfigFile, "Path to the project configuration file")
	cmd.Flags().String("env-file", defaultEnvFile, "Path to the environment variables file")

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
		Duration("tool-execution-timeout", defaultToolExecutionTimeout,
			"Tool execution timeout (env: TOOL_EXECUTION_TIMEOUT)")

	// Logging configuration flags
	cmd.Flags().String("log-level", defaultLogLevel, "Log level (debug, info, warn, error)")
	cmd.Flags().Bool("log-json", false, "Output logs in JSON format")
	cmd.Flags().Bool("log-source", false, "Include source file and line in logs")
	cmd.Flags().Bool("debug", false, "Enable debug mode (sets log level to debug)")
	cmd.Flags().Bool("watch", false, "Enable file watcher to restart server on change")

	// Task execution configuration flags
	cmd.Flags().
		Int("max-nesting-depth", defaultMaxNesting,
			"Maximum task nesting depth allowed (env: MAX_NESTING_DEPTH)")
	cmd.Flags().
		Int("max-string-length", defaultMaxStringLen,
			"Maximum string length in bytes for template processing (env: MAX_STRING_LENGTH)")

	// Memory content size configuration flags
	cmd.Flags().
		Int("max-message-content-length", defaultMaxMsgContent,
			"Maximum message content length in bytes (env: MAX_MESSAGE_CONTENT_LENGTH)")
	cmd.Flags().
		Int("max-total-content-size", defaultMaxTotalContent,
			"Maximum total content size in bytes (env: MAX_TOTAL_CONTENT_SIZE)")
	// Memory async token counter configuration
	cmd.Flags().Int("async-token-counter-workers", defaultAsyncWorkers,
		"Number of workers for async token counting (env: ASYNC_TOKEN_COUNTER_WORKERS)")
	cmd.Flags().Int("async-token-counter-buffer-size", defaultAsyncBuffer,
		"Buffer size for async token counting queue (env: ASYNC_TOKEN_COUNTER_BUFFER_SIZE)")

	// Dispatcher heartbeat configuration flags
	cmd.Flags().Int("dispatcher-heartbeat-interval", defaultHeartbeatInterval,
		"Dispatcher heartbeat interval in seconds (env: DISPATCHER_HEARTBEAT_INTERVAL)")
	cmd.Flags().
		Int("dispatcher-heartbeat-ttl", defaultHeartbeatTTL,
			"Dispatcher heartbeat TTL in seconds (env: DISPATCHER_HEARTBEAT_TTL)")
	cmd.Flags().
		Int("dispatcher-stale-threshold", defaultStaleThreshold,
			"Dispatcher stale threshold in seconds (env: DISPATCHER_STALE_THRESHOLD)")

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
	// Try to listen on the port with a short timeout
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// findAvailablePort finds the next available port starting from the given port
// It uses an exponential backoff strategy to efficiently find available ports
func findAvailablePort(host string, startPort int) (int, error) {
	// First, try the requested port
	if isPortAvailable(host, startPort) {
		return startPort, nil
	}

	// Common alternative ports for development servers
	commonPorts := []int{3001, 3002, 3003, 4000, 4001, 5000, 5001, 8000, 8001, 8080, 8081, 9000, 9001}
	for _, port := range commonPorts {
		if port != startPort && isPortAvailable(host, port) {
			return port, nil
		}
	}

	// If common ports are taken, scan incrementally from the start port
	// but skip already tried common ports
	triedPorts := make(map[int]bool)
	for _, p := range commonPorts {
		triedPorts[p] = true
	}
	triedPorts[startPort] = true

	for i := 1; i < maxPortScanAttempts; i++ {
		// Try ports in both directions from the start port
		portUp := startPort + i
		portDown := startPort - i

		// Check upward direction
		if portUp <= 65535 && !triedPorts[portUp] && isPortAvailable(host, portUp) {
			return portUp, nil
		}

		// Check downward direction (but stay above privileged ports)
		if portDown >= 1024 && !triedPorts[portDown] && isPortAvailable(host, portDown) {
			return portDown, nil
		}
	}

	return 0, fmt.Errorf("no available port found near %d after checking %d ports", startPort, maxPortScanAttempts)
}

// isPathWithinDirectory checks if a given path is within the specified directory
func isPathWithinDirectory(path, dir string) bool {
	// Clean and resolve both paths to absolute paths
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return false
	}

	// Check if the path starts with the directory path
	// Add a trailing separator to the directory to ensure exact match
	// (e.g., /home/user/project matches /home/user/project/file but not /home/user/project2/file)
	if !strings.HasSuffix(absDir, string(filepath.Separator)) {
		absDir += string(filepath.Separator)
	}

	return strings.HasPrefix(absPath, absDir) || absPath == strings.TrimSuffix(absDir, string(filepath.Separator))
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

		// Security: Validate the resolved path to prevent directory traversal
		cleanPath := filepath.Clean(envFile)
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve env file path: %w", err)
		}

		// Ensure the file is within the project directory or its subdirectories
		// This prevents accessing files like /etc/passwd via ../../../etc/passwd
		if !isPathWithinDirectory(absPath, pwd) {
			return "", fmt.Errorf("env file path '%s' is outside the project directory", envFile)
		}

		// Additional validation: ensure it's a regular file (not a directory or symlink)
		fileInfo, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				// File doesn't exist, which is allowed
				return absPath, nil
			}
			return "", fmt.Errorf("failed to stat env file: %w", err)
		}

		if !fileInfo.Mode().IsRegular() {
			return "", fmt.Errorf("env file path '%s' is not a regular file", envFile)
		}

		if err := godotenv.Load(absPath); err != nil {
			// Don't fail if the env file doesn't exist, just log a debug message
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("failed to load env file %s: %w", absPath, err)
			}
		}

		return absPath, nil
	}
	return envFile, nil
}

// setupDevEnvironment prepares the development environment including loading env file and setting up logging
func setupDevEnvironment(cmd *cobra.Command) (context.Context, string, error) {
	gin.SetMode(gin.ReleaseMode)
	envFilePath, err := loadEnvFile(cmd)
	if err != nil {
		return nil, "", err
	}
	logLevel, logJSON, logSource, err := logger.GetLoggerConfig(cmd)
	if err != nil {
		return nil, "", err
	}
	log := logger.SetupLogger(logLevel, logJSON, logSource)
	ctx := context.Background()
	ctx = logger.ContextWithLogger(ctx, log)
	return ctx, envFilePath, nil
}

// setupLegacyEnvironment sets up OpenAI and tool execution timeout environment variables
func setupLegacyEnvironment(cmd *cobra.Command) error {
	openaiAPIKey, err := getOpenAIConfig(cmd)
	if err != nil {
		return err
	}
	if openaiAPIKey != "" {
		if err := os.Setenv("OPENAI_API_KEY", openaiAPIKey); err != nil {
			return fmt.Errorf("failed to set OPENAI_API_KEY environment variable: %w", err)
		}
	}
	toolTimeout, err := getToolExecutionTimeout(cmd)
	if err != nil {
		return err
	}
	if err := os.Setenv("TOOL_EXECUTION_TIMEOUT", toolTimeout.String()); err != nil {
		return fmt.Errorf("failed to set TOOL_EXECUTION_TIMEOUT environment variable: %w", err)
	}
	return nil
}

func handleDevCmd(cmd *cobra.Command, _ []string) error {
	ctx, envFilePath, err := setupDevEnvironment(cmd)
	if err != nil {
		return err
	}
	log := logger.FromContext(ctx)
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return fmt.Errorf("failed to get config flag: %w", err)
	}
	cfg, err := loadUnifiedConfig(ctx, cmd, configFile)
	if err != nil {
		return fmt.Errorf("failed to load unified configuration: %w", err)
	}
	CWD, _, err := GetConfigCWD(cmd)
	if err != nil {
		return err
	}
	availablePort, err := findAvailablePort(cfg.Server.Host, cfg.Server.Port)
	if err != nil {
		return fmt.Errorf("no free port found near %d: %w", cfg.Server.Port, err)
	}
	if availablePort != cfg.Server.Port {
		log.Info("Port unavailable, using alternative port",
			"requested_port", cfg.Server.Port, "available_port", availablePort)
		cfg.Server.Port = availablePort
	}
	if err := setupLegacyEnvironment(cmd); err != nil {
		return err
	}
	watch, err := cmd.Flags().GetBool("watch")
	if err != nil {
		return fmt.Errorf("failed to get watch flag: %w", err)
	}
	if watch {
		return runWithWatcher(ctx, cfg, CWD, configFile, envFilePath)
	}
	srv := server.NewServer(ctx, cfg, CWD, configFile, envFilePath)
	return srv.Run()
}

// runWithWatcher sets up file watching and runs the server with restart capability
func runWithWatcher(ctx context.Context, cfg *config.Config, cwd, configFile, envFilePath string) error {
	watcher, err := setupWatcher(ctx, cwd)
	if err != nil {
		return err
	}
	defer watcher.Close()
	restartChan := make(chan bool, 1)
	go startWatcher(ctx, watcher, restartChan)
	return runAndWatchServer(ctx, cfg, cwd, configFile, envFilePath, restartChan)
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

	// For large projects, watch directories instead of individual files
	// This is more efficient and scales better
	dirsToWatch := make(map[string]bool)
	fileCount := 0

	if err := filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip ignored directories
		if info.IsDir() {
			baseName := filepath.Base(path)
			if ignoredDirs[baseName] {
				return filepath.SkipDir
			}
		}

		// Count YAML files and track their parent directories
		if !info.IsDir() && (filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml") {
			fileCount++
			dir := filepath.Dir(path)
			dirsToWatch[dir] = true
		}
		return nil
	}); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to walk project directory: %w", err)
	}

	// Watch directories containing YAML files instead of individual files
	// This is more efficient for large projects
	for dir := range dirsToWatch {
		if err := watcher.Add(dir); err != nil {
			log.Warn("Failed to watch directory", "path", dir, "error", err)
		}
	}

	log.Info("File watcher initialized",
		"yaml_files", fileCount,
		"watched_directories", len(dirsToWatch))

	return watcher, nil
}

func startWatcher(ctx context.Context, watcher *fsnotify.Watcher, restartChan chan bool) {
	log := logger.FromContext(ctx)

	// Debounce timer to batch multiple file changes
	var debounceTimer *time.Timer
	var pendingRestart bool
	debounceMutex := &sync.Mutex{}

	triggerRestart := func() {
		debounceMutex.Lock()
		defer debounceMutex.Unlock()

		if pendingRestart {
			select {
			case restartChan <- true:
				log.Debug("Sending restart signal after debounce")
			default:
				// Restart already pending
			}
			pendingRestart = false
		}
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("Context canceled, stopping file watcher")
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				// Only react to YAML file changes
				ext := filepath.Ext(event.Name)
				if ext == ".yaml" || ext == ".yml" {
					log.Debug("Detected file change, debouncing...", "file", event.Name)

					debounceMutex.Lock()
					pendingRestart = true

					// Reset the debounce timer
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(fileChangeDebounceDelay, triggerRestart)
					debounceMutex.Unlock()
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
	cfg *config.Config,
	cwd, configFile, envFilePath string,
	restartChan chan bool,
) error {
	log := logger.FromContext(ctx)
	var retryDelay = initialRetryDelay
	const maxRetryDelay = maxRetryDelay
	for {
		// Find available port on each restart in case the original port becomes free
		availablePort, err := findAvailablePort(cfg.Server.Host, cfg.Server.Port)
		if err != nil {
			return fmt.Errorf("no free port found near %d: %w", cfg.Server.Port, err)
		}
		if availablePort != cfg.Server.Port {
			log.Info("port conflict on restart, using next available port",
				"original_port", cfg.Server.Port,
				"available_port", availablePort)
			cfg.Server.Port = availablePort
		}

		srv := server.NewServer(ctx, cfg, cwd, configFile, envFilePath)
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
			retryDelay = initialRetryDelay
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
