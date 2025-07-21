package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
)

// NewMCPProxyCommand creates the mcp-proxy command using the unified command pattern
func NewMCPProxyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp-proxy",
		Short: "Run the MCP proxy server",
		Long:  "Start the MCP proxy server to provide HTTP access to MCP servers",
		RunE:  executeMCPProxyCommand,
	}

	// Command-specific flags (MCP proxy specific configuration)
	cmd.Flags().String("host", "0.0.0.0", "Host to bind the server to (env: MCP_PROXY_HOST)")
	cmd.Flags().String("port", "8081", "Port to run the MCP proxy server on (env: MCP_PROXY_PORT)")
	cmd.Flags().String("base-url", "", "Base URL for the MCP proxy server (env: MCP_PROXY_BASE_URL)")

	// Security configuration flags (MCP proxy specific)
	cmd.Flags().StringSlice("admin-tokens", []string{}, "Admin API tokens")
	cmd.Flags().StringSlice("admin-allow-ips", []string{}, "Admin API allowed IP addresses/CIDR blocks")
	cmd.Flags().StringSlice("trusted-proxies", []string{}, "Trusted proxy IP addresses/CIDR blocks")
	cmd.Flags().StringSlice("global-auth-tokens", []string{}, "Global auth tokens for all MCP clients")

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

	return runMCPProxy(ctx, cobraCmd, executor, func(proxyConfig *mcpproxy.Config) error {
		fmt.Printf("🚀 Starting MCP proxy server\n")
		fmt.Printf("📡 Host: %s\n", proxyConfig.Host)
		fmt.Printf("🔌 Port: %s\n", proxyConfig.Port)
		fmt.Printf("🌐 Base URL: %s\n", proxyConfig.BaseURL)
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
	// Get command-specific configuration directly from flags
	proxyConfig := buildMCPProxyConfig(cobraCmd)

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

// buildMCPProxyConfig builds MCP proxy configuration from command flags and environment variables
func buildMCPProxyConfig(cobraCmd *cobra.Command) *mcpproxy.Config {
	// Get flag values with environment fallbacks
	host := getStringFlagOrEnv(cobraCmd, "host", "MCP_PROXY_HOST", "0.0.0.0")
	port := getStringFlagOrEnv(cobraCmd, "port", "MCP_PROXY_PORT", "8081")
	baseURL := getStringFlagOrEnv(cobraCmd, "base-url", "MCP_PROXY_BASE_URL", "")

	// Get slice flags (these flags are defined in the command, so errors are unlikely)
	adminTokens := getStringSliceFlag(cobraCmd, "admin-tokens")
	adminAllowIPs := getStringSliceFlag(cobraCmd, "admin-allow-ips")
	trustedProxies := getStringSliceFlag(cobraCmd, "trusted-proxies")
	globalAuthTokens := getStringSliceFlag(cobraCmd, "global-auth-tokens")

	// Set default base URL if not provided
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}

	return &mcpproxy.Config{
		Host:             host,
		Port:             port,
		BaseURL:          baseURL,
		AdminTokens:      adminTokens,
		AdminAllowIPs:    adminAllowIPs,
		TrustedProxies:   trustedProxies,
		GlobalAuthTokens: globalAuthTokens,
	}
}

// getStringFlagOrEnv gets a string flag value with environment variable fallback
func getStringFlagOrEnv(cobraCmd *cobra.Command, flagName, envVar, defaultValue string) string {
	// First check if flag was explicitly set
	if value, err := cobraCmd.Flags().GetString(flagName); err == nil && cobraCmd.Flags().Changed(flagName) {
		return value
	}

	// Then check environment variable
	if value := os.Getenv(envVar); value != "" {
		return value
	}

	// Return default value
	return defaultValue
}

// getStringSliceFlag gets a string slice flag value, returning empty slice on error
func getStringSliceFlag(cobraCmd *cobra.Command, flagName string) []string {
	if value, err := cobraCmd.Flags().GetStringSlice(flagName); err == nil {
		return value
	}
	return []string{}
}

// Note: Environment file loading is handled through the global --env-file flag and executor

// setupMCPProxyLogging configures logging for the MCP proxy using global configuration
func setupMCPProxyLogging(cobraCmd *cobra.Command, _ *cmd.CommandExecutor) logger.Logger {
	cfg := config.Get()
	logLevel := cfg.Runtime.LogLevel

	// Override log level if debug flag is set
	if debug, err := cobraCmd.Flags().GetBool("debug"); err == nil && debug {
		logLevel = "debug"
	}

	// Use global configuration for logging setup
	return logger.SetupLogger(logger.LogLevel(logLevel), false, false)
}

// outputMCPProxyJSON outputs a JSON response for mcp-proxy command
func outputMCPProxyJSON(data any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
