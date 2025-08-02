//	@title			MCP Proxy API
//	@version		1.0
//	@description	Model Context Protocol Proxy for Compozy
//	@termsOfService	https://github.com/compozy/compozy

//	@contact.name	Compozy Support
//	@contact.url	https://github.com/compozy/compozy
//	@contact.email	support@compozy.com

//	@license.name	BSL-1.1
//	@license.url	https://github.com/compozy/compozy/blob/main/LICENSE

//	@BasePath	/

//	@tag.name			admin
//	@tag.description	Administrative operations for MCP server management

//	@tag.name			proxy
//	@tag.description	Proxy operations for MCP server communication

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/compozy/compozy/pkg/version"
)

func main() {
	cmd := createRootCommand()
	if err := cmd.Execute(); err != nil {
		// Exit with error code 1 if command execution fails
		os.Exit(1)
	}
}

func createRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "compozy-mcp-proxy",
		Short: "MCP Proxy Server - Model Context Protocol gateway for Compozy",
		Long: `MCP Proxy Server provides HTTP access to Model Context Protocol servers.
It acts as a gateway between AI agents and MCP servers, managing connections,
authentication, and routing requests.`,
		RunE: runMCPProxy,
	}

	// Add MCP proxy specific flags
	root.Flags().String("host", "", "Host interface for MCP proxy server to bind to")
	root.Flags().Int("port", 0, "Port for MCP proxy server to listen on")
	root.Flags().String("base-url", "", "Base URL for MCP proxy server (auto-generated if empty)")
	root.Flags().Duration("shutdown-timeout", 0, "Maximum time to wait for graceful shutdown")

	// Authentication flags
	root.Flags().StringSlice("admin-tokens", nil, "Admin authentication tokens (comma-separated)")
	root.Flags().StringSlice("admin-allow-ips", nil,
		"IP addresses/CIDR blocks allowed for admin access (comma-separated)")
	root.Flags().StringSlice("trusted-proxies", nil,
		"Trusted proxy IP addresses/CIDR blocks (comma-separated)")
	root.Flags().StringSlice("global-auth-tokens", nil,
		"Global authentication tokens for all MCP clients (comma-separated)")

	// Logging flags
	root.Flags().String("log-level", "", "Log level (debug, info, warn, error)")
	root.Flags().Bool("debug", false, "Enable debug mode (sets log level to debug)")

	// Add version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(_ *cobra.Command, _ []string) {
			info := version.Get()
			fmt.Printf("compozy-mcp-proxy version %s\n", info.Version)
			fmt.Printf("commit: %s\n", info.CommitHash)
			fmt.Printf("built: %s\n", info.BuildDate)
		},
	}
	root.AddCommand(versionCmd)

	return root
}

func runMCPProxy(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Load configuration
	cfg, err := LoadConfig(cmd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Setup logger
	logLevel := parseLogLevel(cfg.LogLevel)
	log := logger.SetupLogger(logLevel, false, cfg.Debug)
	ctx = logger.ContextWithLogger(ctx, log)

	// Debug: log the configuration values
	log.Debug("MCP proxy configuration loaded",
		"host", cfg.Host,
		"port", cfg.Port,
		"baseURL", cfg.BaseURL)

	// Convert port from int to string as required by MCP proxy
	port := strconv.Itoa(cfg.Port)

	proxyConfig := &mcpproxy.Config{
		Host:             cfg.Host,
		Port:             port,
		BaseURL:          cfg.BaseURL,
		ShutdownTimeout:  cfg.ShutdownTimeout,
		AdminTokens:      cfg.AdminTokens,
		AdminAllowIPs:    cfg.AdminAllowIPs,
		TrustedProxies:   cfg.TrustedProxies,
		GlobalAuthTokens: cfg.GlobalAuthTokens,
	}

	// Validate configuration
	if err := proxyConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Display startup information
	fmt.Printf("🚀 Starting MCP proxy server\n")
	fmt.Printf("📡 Host: %s\n", proxyConfig.Host)
	fmt.Printf("🔌 Port: %s\n", proxyConfig.Port)
	fmt.Printf("🌐 Base URL: %s\n", proxyConfig.BaseURL)
	fmt.Printf("\n💡 Press Ctrl+C to stop the server\n\n")

	log.Info("starting MCP proxy server",
		"host", proxyConfig.Host,
		"port", proxyConfig.Port,
		"base_url", proxyConfig.BaseURL,
	)

	// Run the MCP proxy server
	return mcpproxy.Run(ctx, proxyConfig)
}

// parseLogLevel converts string log level to logger.LogLevel
func parseLogLevel(level string) logger.LogLevel {
	switch level {
	case string(logger.DebugLevel):
		return logger.DebugLevel
	case string(logger.InfoLevel):
		return logger.InfoLevel
	case string(logger.WarnLevel):
		return logger.WarnLevel
	case string(logger.ErrorLevel):
		return logger.ErrorLevel
	default:
		return logger.InfoLevel
	}
}
