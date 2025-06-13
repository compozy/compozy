package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/spf13/cobra"
)

// MCPProxyCmd returns the mcp-proxy command
func MCPProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp-proxy",
		Short: "Run the MCP proxy server",
		Long:  "Start the MCP proxy server to provide HTTP access to MCP servers",
		RunE:  handleMCPProxyCmd,
	}

	// Server configuration flags
	cmd.Flags().String("host", "0.0.0.0", "Host to bind the server to")
	cmd.Flags().String("port", "8081", "Port to run the MCP proxy server on")
	cmd.Flags().String("base-url", "http://localhost:8081", "Base URL for the MCP proxy server")
	cmd.Flags().String("env-file", ".env", "Path to the environment variables file")

	// Security configuration flags
	cmd.Flags().StringSlice("admin-tokens", []string{}, "Admin API tokens")
	cmd.Flags().StringSlice("admin-allow-ips", []string{}, "Admin API allowed IP addresses/CIDR blocks")
	cmd.Flags().StringSlice("trusted-proxies", []string{}, "Trusted proxy IP addresses/CIDR blocks")
	cmd.Flags().StringSlice("global-auth-tokens", []string{}, "Global auth tokens for all MCP clients")

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

func handleMCPProxyCmd(cmd *cobra.Command, _ []string) error {
	// Load environment variables from the specified file first
	if err := loadEnvFile(cmd); err != nil {
		return err
	}

	// Setup logging
	if err := setupMCPProxyLogging(cmd); err != nil {
		return err
	}

	// Parse configuration from flags and environment
	config, err := parseMCPProxyConfig(cmd)
	if err != nil {
		return err
	}

	// Create context with cancellation
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("Starting MCP proxy server", "host", config.Host, "port", config.Port)

	// Run the MCP proxy server
	return mcpproxy.Run(ctx, config)
}

// setupMCPProxyLogging configures logging for the MCP proxy
func setupMCPProxyLogging(cmd *cobra.Command) error {
	logLevel, logJSON, logSource, err := logger.GetLoggerConfig(cmd)
	if err != nil {
		return err
	}
	return logger.SetupLogger(logLevel, logJSON, logSource)
}

// parseMCPProxyConfig extracts configuration from flags and environment variables
func parseMCPProxyConfig(cmd *cobra.Command) (*mcpproxy.Config, error) {
	// Get flag values
	host, err := cmd.Flags().GetString("host")
	if err != nil {
		return nil, fmt.Errorf("failed to get host flag: %w", err)
	}
	port, err := cmd.Flags().GetString("port")
	if err != nil {
		return nil, fmt.Errorf("failed to get port flag: %w", err)
	}
	baseURL, err := cmd.Flags().GetString("base-url")
	if err != nil {
		return nil, fmt.Errorf("failed to get base-url flag: %w", err)
	}
	adminTokens, err := cmd.Flags().GetStringSlice("admin-tokens")
	if err != nil {
		return nil, fmt.Errorf("failed to get admin-tokens flag: %w", err)
	}
	adminAllowIPs, err := cmd.Flags().GetStringSlice("admin-allow-ips")
	if err != nil {
		return nil, fmt.Errorf("failed to get admin-allow-ips flag: %w", err)
	}
	trustedProxies, err := cmd.Flags().GetStringSlice("trusted-proxies")
	if err != nil {
		return nil, fmt.Errorf("failed to get trusted-proxies flag: %w", err)
	}
	globalAuthTokens, err := cmd.Flags().GetStringSlice("global-auth-tokens")
	if err != nil {
		return nil, fmt.Errorf("failed to get global-auth-tokens flag: %w", err)
	}

	// Use environment variables as fallback
	if host == "" {
		host = getEnvOrDefault("MCP_PROXY_HOST", "0.0.0.0")
	}
	if port == "" {
		port = getEnvOrDefault("MCP_PROXY_PORT", "8081")
	}
	if baseURL == "" {
		baseURL = getEnvOrDefault("MCP_PROXY_BASE_URL", fmt.Sprintf("http://localhost:%s", port))
	}

	return &mcpproxy.Config{
		Host:             host,
		Port:             port,
		BaseURL:          baseURL,
		AdminTokens:      adminTokens,
		AdminAllowIPs:    adminAllowIPs,
		TrustedProxies:   trustedProxies,
		GlobalAuthTokens: globalAuthTokens,
	}, nil
}
