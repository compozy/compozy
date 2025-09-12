package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
)

const (
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
)

// NewMCPProxyCommand creates the mcp-proxy command using the unified command pattern
func NewMCPProxyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp-proxy",
		Short: "Run the MCP proxy server",
		Long:  "Start the MCP proxy server to provide HTTP access to MCP servers",
		RunE:  executeMCPProxyCommand,
	}

	// MCP proxy specific debugging flag
	cmd.Flags().Bool("debug", false, "Enable debug mode (sets log level to debug)")

	return cmd
}

// executeMCPProxyCommand handles the mcp-proxy command execution using the unified executor pattern
func executeMCPProxyCommand(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: false,
	}, cmd.ModeHandlers{
		JSON: handleMCPProxyJSON,
		TUI:  handleMCPProxyTUI,
	}, args)
}

// handleMCPProxyJSON handles mcp-proxy command in JSON mode
func handleMCPProxyJSON(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return runMCPProxyJSON(ctx, cobraCmd, executor, args)
}

// handleMCPProxyTUI handles mcp-proxy command in TUI mode
func handleMCPProxyTUI(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return runMCPProxyTUI(ctx, cobraCmd, executor, args)
}

// runMCPProxyJSON handles JSON mode execution
func runMCPProxyJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	log := logger.FromContext(ctx)
	log.Debug("executing mcp-proxy command in JSON mode")

	return runMCPProxy(ctx, cobraCmd, executor, func(proxyConfig *mcpproxy.Config) error {
		// Output JSON response indicating server start
		response := map[string]any{
			"message":  "Starting MCP proxy server",
			"host":     proxyConfig.Host,
			"port":     proxyConfig.Port,
			"base_url": proxyConfig.BaseURL,
		}
		return outputMCPProxyJSON(response)
	})
}

// runMCPProxyTUI handles TUI mode execution
func runMCPProxyTUI(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	log := logger.FromContext(ctx)
	log.Debug("executing mcp-proxy command in TUI mode")

	return runMCPProxy(ctx, cobraCmd, executor, func(_ *mcpproxy.Config) error {
		fmt.Printf("\n💡 Press Ctrl+C to stop the server\n\n")
		return nil
	})
}

// runMCPProxy handles the common MCP proxy execution logic
func runMCPProxy(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	outputHandler func(*mcpproxy.Config) error,
) error {
	// Get command-specific configuration from context-backed config
	proxyConfig := buildMCPProxyConfig(ctx, cobraCmd)

	// Setup logging using global configuration
	loggerInstance := setupMCPProxyLogging(cobraCmd, executor)

	// Create context with cancellation and logger
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx = logger.ContextWithLogger(ctx, loggerInstance)

	if err := proxyConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Handle mode-specific output
	if err := outputHandler(proxyConfig); err != nil {
		return err
	}

	// Run the MCP proxy server
	return mcpproxy.Run(ctx, proxyConfig)
}

// buildMCPProxyConfig builds MCP proxy configuration from unified config system
func buildMCPProxyConfig(ctx context.Context, _ *cobra.Command) *mcpproxy.Config {
	// Get unified configuration
	cfg := config.FromContext(ctx)
	mcpConfig := cfg.MCPProxy

	// Convert port from int to string as required by MCP proxy
	port := strconv.Itoa(mcpConfig.Port)

	// Set default base URL if not provided
	baseURL := mcpConfig.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}

	return &mcpproxy.Config{
		Host:            mcpConfig.Host,
		Port:            port,
		BaseURL:         baseURL,
		ShutdownTimeout: mcpConfig.ShutdownTimeout,
	}
}

// Note: Environment file loading is handled through the global --env-file flag and executor

// setupMCPProxyLogging configures logging for the MCP proxy using global configuration
func setupMCPProxyLogging(cobraCmd *cobra.Command, _ *cmd.CommandExecutor) logger.Logger {
	cfg := config.FromContext(cobraCmd.Context())
	logLevel := cfg.Runtime.LogLevel
	if envLevel := os.Getenv("MCP_PROXY_LOG_LEVEL"); envLevel != "" {
		v := strings.ToLower(envLevel)
		switch v {
		case logLevelDebug, logLevelInfo, logLevelWarn, logLevelError:
			logLevel = v
		}
	}
	if cfg.CLI.Debug {
		logLevel = logLevelDebug
	}
	if debug, err := cobraCmd.Flags().GetBool("debug"); err == nil && debug {
		logLevel = logLevelDebug
	}
	return logger.SetupLogger(logger.LogLevel(logLevel), false, false)
}

// outputMCPProxyJSON outputs a JSON response for mcp-proxy command
func outputMCPProxyJSON(data any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
