package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Config holds the MCP proxy configuration
type Config struct {
	// Server settings
	Host            string
	Port            int
	BaseURL         string
	ShutdownTimeout time.Duration

	// Authentication
	AdminTokens      []string
	AdminAllowIPs    []string
	TrustedProxies   []string
	GlobalAuthTokens []string

	// Logging
	LogLevel string
	Debug    bool
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Host:             "127.0.0.1",
		Port:             6001,
		BaseURL:          "", // Auto-generated if empty
		ShutdownTimeout:  10 * time.Second,
		LogLevel:         "info",
		Debug:            false,
		AdminAllowIPs:    []string{"127.0.0.1/32", "::1/128"},
		AdminTokens:      []string{},
		TrustedProxies:   []string{},
		GlobalAuthTokens: []string{},
	}
}

// LoadConfig loads configuration from environment variables and CLI flags
func LoadConfig(cmd *cobra.Command) (*Config, error) {
	cfg := DefaultConfig()

	// Load from environment variables first
	loadFromEnv(cfg)

	// Then override with CLI flags if provided
	if err := loadFromFlags(cfg, cmd); err != nil {
		return nil, fmt.Errorf("failed to load flags: %w", err)
	}

	// Validate port range
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535, got %d", cfg.Port)
	}

	// Auto-generate base URL if not provided
	if cfg.BaseURL == "" {
		cfg.BaseURL = fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
	}

	return cfg, nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(cfg *Config) {
	// Server settings
	if val := os.Getenv("MCP_PROXY_HOST"); val != "" {
		cfg.Host = val
	}
	if val := os.Getenv("MCP_PROXY_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cfg.Port = port
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Invalid MCP_PROXY_PORT value '%s': %v\n", val, err)
		}
	}
	if val := os.Getenv("MCP_PROXY_BASE_URL"); val != "" {
		cfg.BaseURL = val
	}
	if val := os.Getenv("MCP_PROXY_SHUTDOWN_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			cfg.ShutdownTimeout = duration
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Invalid MCP_PROXY_SHUTDOWN_TIMEOUT value '%s': %v\n", val, err)
		}
	}

	// Authentication
	if val := os.Getenv("MCP_PROXY_ADMIN_TOKENS"); val != "" {
		cfg.AdminTokens = splitAndTrim(val)
	}
	if val := os.Getenv("MCP_PROXY_ADMIN_ALLOW_IPS"); val != "" {
		cfg.AdminAllowIPs = splitAndTrim(val)
	}
	if val := os.Getenv("MCP_PROXY_TRUSTED_PROXIES"); val != "" {
		cfg.TrustedProxies = splitAndTrim(val)
	}
	if val := os.Getenv("MCP_PROXY_GLOBAL_AUTH_TOKENS"); val != "" {
		cfg.GlobalAuthTokens = splitAndTrim(val)
	}

	// Logging
	if val := os.Getenv("MCP_PROXY_LOG_LEVEL"); val != "" {
		cfg.LogLevel = val
	}
	if val := os.Getenv("MCP_PROXY_DEBUG"); val != "" {
		cfg.Debug = val == "true" || val == "1"
	}
}

// loadFromFlags loads configuration from CLI flags
func loadFromFlags(cfg *Config, cmd *cobra.Command) error {
	// Load server flags
	if err := loadServerFlags(cfg, cmd); err != nil {
		return err
	}

	// Load authentication flags
	if err := loadAuthFlags(cfg, cmd); err != nil {
		return err
	}

	// Load logging flags
	return loadLoggingFlags(cfg, cmd)
}

// loadServerFlags loads server-related flags
func loadServerFlags(cfg *Config, cmd *cobra.Command) error {
	if cmd.Flags().Changed("host") {
		host, err := cmd.Flags().GetString("host")
		if err != nil {
			return fmt.Errorf("failed to get host flag: %w", err)
		}
		cfg.Host = host
	}
	if cmd.Flags().Changed("port") {
		port, err := cmd.Flags().GetInt("port")
		if err != nil {
			return fmt.Errorf("failed to get port flag: %w", err)
		}
		cfg.Port = port
	}
	if cmd.Flags().Changed("base-url") {
		baseURL, err := cmd.Flags().GetString("base-url")
		if err != nil {
			return fmt.Errorf("failed to get base-url flag: %w", err)
		}
		cfg.BaseURL = baseURL
	}
	if cmd.Flags().Changed("shutdown-timeout") {
		timeout, err := cmd.Flags().GetDuration("shutdown-timeout")
		if err != nil {
			return fmt.Errorf("failed to get shutdown-timeout flag: %w", err)
		}
		cfg.ShutdownTimeout = timeout
	}
	return nil
}

// loadAuthFlags loads authentication-related flags
func loadAuthFlags(cfg *Config, cmd *cobra.Command) error {
	if cmd.Flags().Changed("admin-tokens") {
		tokens, err := cmd.Flags().GetStringSlice("admin-tokens")
		if err != nil {
			return fmt.Errorf("failed to get admin-tokens flag: %w", err)
		}
		cfg.AdminTokens = tokens
	}
	if cmd.Flags().Changed("admin-allow-ips") {
		ips, err := cmd.Flags().GetStringSlice("admin-allow-ips")
		if err != nil {
			return fmt.Errorf("failed to get admin-allow-ips flag: %w", err)
		}
		cfg.AdminAllowIPs = ips
	}
	if cmd.Flags().Changed("trusted-proxies") {
		proxies, err := cmd.Flags().GetStringSlice("trusted-proxies")
		if err != nil {
			return fmt.Errorf("failed to get trusted-proxies flag: %w", err)
		}
		cfg.TrustedProxies = proxies
	}
	if cmd.Flags().Changed("global-auth-tokens") {
		tokens, err := cmd.Flags().GetStringSlice("global-auth-tokens")
		if err != nil {
			return fmt.Errorf("failed to get global-auth-tokens flag: %w", err)
		}
		cfg.GlobalAuthTokens = tokens
	}
	return nil
}

// loadLoggingFlags loads logging-related flags
func loadLoggingFlags(cfg *Config, cmd *cobra.Command) error {
	if cmd.Flags().Changed("log-level") {
		logLevel, err := cmd.Flags().GetString("log-level")
		if err != nil {
			return fmt.Errorf("failed to get log-level flag: %w", err)
		}
		cfg.LogLevel = logLevel
	}
	if cmd.Flags().Changed("debug") {
		debug, err := cmd.Flags().GetBool("debug")
		if err != nil {
			return fmt.Errorf("failed to get debug flag: %w", err)
		}
		cfg.Debug = debug
		if debug {
			cfg.LogLevel = "debug"
		}
	}
	return nil
}

// splitAndTrim splits a comma-separated string and trims whitespace
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
