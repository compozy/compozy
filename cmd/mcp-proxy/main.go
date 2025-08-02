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

	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/pkg/config"
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
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return setupGlobalConfig(cmd)
		},
	}

	// Add global configuration flags
	helpers.AddGlobalFlags(root)

	// Add MCP proxy specific flags
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

func setupGlobalConfig(cmd *cobra.Command) error {
	// Load environment file if specified
	if err := helpers.LoadEnvironmentFile(cmd); err != nil {
		return fmt.Errorf("failed to load environment file: %w", err)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Extract CLI flag overrides
	cliFlags, err := helpers.ExtractCLIFlags(cmd)
	if err != nil {
		return fmt.Errorf("failed to extract CLI flags: %w", err)
	}

	// Build sources with proper precedence: defaults -> env -> config file -> CLI flags
	sources := []config.Source{
		config.NewDefaultProvider(),
		config.NewEnvProvider(),
	}

	// Add config file if specified
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return fmt.Errorf("failed to get config file: %w", err)
	}
	if configFile != "" {
		sources = append(sources, config.NewYAMLProvider(configFile))
	}

	// Add CLI flags as highest precedence
	if len(cliFlags) > 0 {
		sources = append(sources, config.NewCLIProvider(cliFlags))
	}

	// Initialize global config
	if err := config.Initialize(ctx, nil, sources...); err != nil {
		return fmt.Errorf("failed to initialize global configuration: %w", err)
	}

	// Setup logger based on configuration
	cfg := config.Get()
	logLevel := logger.InfoLevel
	if cfg.CLI.Quiet {
		logLevel = logger.DisabledLevel
	} else if cfg.CLI.Debug {
		logLevel = logger.DebugLevel
	}

	// Override log level if debug flag is set
	if debug, err := cmd.Flags().GetBool("debug"); err == nil && debug {
		logLevel = logger.DebugLevel
	}

	log := logger.SetupLogger(logLevel, false, cfg.CLI.Debug)
	ctx = logger.ContextWithLogger(ctx, log)
	cmd.SetContext(ctx)

	return nil
}

func runMCPProxy(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	log := logger.FromContext(ctx)

	// Get MCP proxy configuration
	cfg := config.Get()
	mcpConfig := cfg.MCPProxy

	// Convert port from int to string as required by MCP proxy
	port := strconv.Itoa(mcpConfig.Port)

	// Set default base URL if not provided
	baseURL := mcpConfig.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}

	proxyConfig := &mcpproxy.Config{
		Host:             mcpConfig.Host,
		Port:             port,
		BaseURL:          baseURL,
		ShutdownTimeout:  mcpConfig.ShutdownTimeout,
		AdminTokens:      mcpConfig.AdminTokens,
		AdminAllowIPs:    mcpConfig.AdminAllowIPs,
		TrustedProxies:   mcpConfig.TrustedProxies,
		GlobalAuthTokens: mcpConfig.GlobalAuthTokens,
	}

	// Validate configuration
	if err := proxyConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Display startup information
	mode := cfg.CLI.Mode
	if mode == "json" {
		fmt.Printf(`{"message":"Starting MCP proxy server","host":"%s","port":"%s","base_url":"%s"}`+"\n",
			proxyConfig.Host, proxyConfig.Port, proxyConfig.BaseURL)
	} else {
		fmt.Printf("🚀 Starting MCP proxy server\n")
		fmt.Printf("📡 Host: %s\n", proxyConfig.Host)
		fmt.Printf("🔌 Port: %s\n", proxyConfig.Port)
		fmt.Printf("🌐 Base URL: %s\n", proxyConfig.BaseURL)
		fmt.Printf("\n💡 Press Ctrl+C to stop the server\n\n")
	}

	log.Info("starting MCP proxy server",
		"host", proxyConfig.Host,
		"port", proxyConfig.Port,
		"base_url", proxyConfig.BaseURL,
	)

	// Run the MCP proxy server
	return mcpproxy.Run(ctx, proxyConfig)
}
