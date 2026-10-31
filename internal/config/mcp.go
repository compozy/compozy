package config

import "time"

const (
	// DefaultMCPStdioIdleTimeout closes a stdio MCP server process after this long without requests.
	DefaultMCPStdioIdleTimeout = 10 * time.Minute
	// DefaultMCPStdioConnectTimeout bounds one stdio MCP server launch plus its MCP handshake.
	DefaultMCPStdioConnectTimeout = 60 * time.Second
	// DefaultMCPStatusProbeTimeout bounds one settings status probe of an MCP server.
	DefaultMCPStatusProbeTimeout = 15 * time.Second
	// DefaultMCPStdioMaxConcurrentCalls bounds in-flight requests on one stdio MCP server process.
	DefaultMCPStdioMaxConcurrentCalls = 8
	// MaxMCPStdioMaxConcurrentCalls caps the configurable per-process request concurrency.
	MaxMCPStdioMaxConcurrentCalls = 64
)

// MCPConfig controls global MCP client behavior.
type MCPConfig struct {
	OAuth              MCPOAuthConfig `toml:"oauth"`
	Stdio              MCPStdioConfig `toml:"stdio"`
	StatusProbeTimeout time.Duration  `toml:"status_probe_timeout"`
}

// MCPOAuthConfig controls the Compozy MCP OAuth client identity.
type MCPOAuthConfig struct {
	ClientMetadataURL string `toml:"client_metadata_url,omitempty"`
	RedirectURI       string `toml:"redirect_uri,omitempty"`
}

// MCPStdioConfig controls how the daemon reuses stdio MCP server processes.
type MCPStdioConfig struct {
	IdleTimeout        time.Duration `toml:"idle_timeout"`
	ConnectTimeout     time.Duration `toml:"connect_timeout"`
	MaxConcurrentCalls int           `toml:"max_concurrent_calls"`
}

// DefaultMCPStdioConfig returns the stdio MCP process reuse defaults.
func DefaultMCPStdioConfig() MCPStdioConfig {
	return MCPStdioConfig{
		IdleTimeout:        DefaultMCPStdioIdleTimeout,
		ConnectTimeout:     DefaultMCPStdioConnectTimeout,
		MaxConcurrentCalls: DefaultMCPStdioMaxConcurrentCalls,
	}
}
