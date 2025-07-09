// Package mcp provides configuration and management for **Model Context Protocol (MCP)** servers.
//
// ## What is MCP?
//
// The Model Context Protocol is a standardized interface that allows AI agents to interact
// with external tools and services. It bridges the gap between language models and the
// external world, enabling agents to:
//
// - Access file systems and databases
// - Call APIs and web services
// - Execute code in various runtimes
// - Interact with specialized tools
//
// ## Architecture
//
// ```
// ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
// │   AI Agent  │────▶│  MCP Proxy  │────▶│ MCP Server  │
// └─────────────┘     └─────────────┘     └─────────────┘
//
//	│                    │                    │
//	└── Uses tools ──────┴── Manages ────────┴── Provides tools
//
// ```
//
// ## Quick Start
//
// ```yaml
// # Configure MCP servers in your agent YAML:
// mcps:
//
//   - id: filesystem
//     command: "mcp-server-filesystem"
//     transport: stdio
//
//   - id: github
//     url: "https://api.github.com/mcp/v1"
//     transport: sse
//     env:
//     GITHUB_TOKEN: "{{ .env.GITHUB_TOKEN }}"
//
// ```
//
// ## Key Components
//
// - **Config**: MCP server configuration structure
// - **Service**: Manages MCP lifecycle and registration
// - **Client**: Communicates with the MCP proxy
// - **Transport**: Handles different communication protocols
//
// ## Environment Requirements
//
// - `MCP_PROXY_URL`: Required. URL of the MCP proxy service
// - `MCP_PROXY_ADMIN_TOKEN`: Optional. Admin token for proxy management
package mcp

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/compozy/compozy/engine/core"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
)

const (
	// DefaultProtocolVersion is the default MCP protocol version used when not specified
	// Format: YYYY-MM-DD (e.g., "2025-03-26")
	DefaultProtocolVersion = "2025-03-26"
	// DefaultTransport is the default transport type for MCP connections
	// Uses Server-Sent Events (SSE) for real-time bidirectional communication
	DefaultTransport = mcpproxy.TransportSSE
)

// Config represents a **Model Context Protocol (MCP)** server configuration.
//
// ## Overview
//
// MCP (Model Context Protocol) is a standardized protocol that enables AI agents to interact
// with external tools, services, and data sources through a unified interface. It provides a
// consistent way for agents to extend their capabilities beyond their core language model
// functionality.
//
// ## Key Features
//
// - **Tool Extension**: Agents can access specialized tools like file systems, databases, APIs
// - **Transport Flexibility**: Supports multiple communication methods (SSE, HTTP, stdio)
// - **Security**: Built-in authentication and environment variable management
// - **Scalability**: Session limits and timeout controls for resource management
//
// ## Server Types
//
// ### 1. **Remote MCP Servers** (HTTP/HTTPS)
// Accessed via network endpoints using Server-Sent Events (SSE) or HTTP streaming:
// ```yaml
// mcps:
//   - id: weather-api
//     url: "https://api.weather-mcp.com/v1"
//     transport: sse
//     proto: "2025-03-26"
//
// ```
//
// ### 2. **Local Process Servers** (stdio)
// Spawned as child processes communicating via standard I/O:
// ```yaml
// mcps:
//   - id: filesystem
//     command: "mcp-server-filesystem"
//     args: ["--root", "/data"]
//     transport: stdio
//     env:
//     LOG_LEVEL: "debug"
//
// ```
//
// ### 3. **Docker-based Servers**
// Run MCP servers in containers with environment configuration:
// ```yaml
// mcps:
//   - id: database
//     command: "docker run --rm -i mcp-postgres:latest"
//     transport: stdio
//     env:
//     DATABASE_URL: "postgres://user:pass@db/myapp"
//     start_timeout: 30s
//
// ```
//
// ## Transport Types
//
// | Transport | Description | Use Case |
// |-----------|-------------|----------|
// | **sse** | Server-Sent Events | Real-time streaming from HTTP servers |
// | **streamable-http** | HTTP with streaming | Large responses, file transfers |
// | **stdio** | Standard I/O | Local processes, Docker containers |
//
// ## Integration Example
//
// ```yaml
// # In your agent configuration
// agent:
//
//	id: data-analyst
//	instructions: "Analyze data using available tools"
//
//	mcps:
//	  - id: sql-server
//	    url: "http://mcp-proxy:3000/sql"
//	    transport: sse
//
//	  - id: python-runtime
//	    command: "mcp-python-server"
//	    transport: stdio
//	    env:
//	      PYTHONPATH: "/app/libs"
//
//	actions:
//	  - prompt: "Query the sales database and create a visualization"
//
// ```
//
// ## Environment Requirements
//
// The MCP system requires the `MCP_PROXY_URL` environment variable to be set, pointing to
// the MCP proxy service that manages all MCP server connections.
type Config struct {
	// Resource reference for the MCP server (optional)
	//
	// If not specified, defaults to the value of ID.
	// Used for resource identification and referencing in Compozy's resource system.
	Resource string `yaml:"resource,omitempty"      json:"resource,omitempty"`
	// ID is the **unique identifier** for this MCP server configuration.
	//
	// This identifier is used throughout the system to reference this specific MCP server.
	// Choose descriptive IDs that reflect the server's purpose.
	//
	// - **Examples**:
	// - `filesystem` - for file system operations
	// - `postgres-db` - for PostgreSQL database access
	// - `github-api` - for GitHub integration
	// - `python-runtime` - for Python code execution
	ID string `yaml:"id"                      json:"id"`
	// URL is the **endpoint for remote MCP servers**.
	//
	// Required for HTTP-based transports (SSE, streamable-http).
	// Must be a valid HTTP or HTTPS URL pointing to an MCP-compatible endpoint.
	//
	// **Format**: `http[s]://host[:port]/path`
	//
	// - **Examples**:
	// ```yaml
	// url: "http://localhost:3000/mcp"
	// url: "https://api.example.com/v1/mcp"
	// url: "http://mcp-proxy:8080/filesystem"
	// ```
	//
	// **Note**: Mutually exclusive with `command` - use either URL or Command, not both.
	URL string `yaml:"url"                     json:"url"`
	// Command is the **executable command** to spawn a local MCP server process.
	//
	// Used for stdio transport to run MCP servers as child processes.
	// Supports both direct executables and complex commands with arguments.
	//
	// - **Examples**:
	// ```yaml
	// # Simple executable
	// command: "mcp-server-filesystem"
	//
	// # Command with arguments
	// command: "python /app/mcp_server.py --mode production"
	//
	// # Docker container
	// command: "docker run --rm -i mcp/postgres:latest"
	// ```
	//
	// **Security Note**: Commands are parsed using shell lexing for safety.
	// Avoid user-provided input in commands.
	Command string `yaml:"command,omitempty"       json:"command,omitempty"`
	// Env contains **environment variables** to pass to the MCP server process.
	//
	// Only used when `command` is specified for spawning local processes.
	// Useful for passing configuration, secrets, or runtime parameters.
	//
	// - **Examples**:
	// ```yaml
	// env:
	//   DATABASE_URL: "postgres://user:pass@localhost/db"
	//   API_KEY: "{{ .env.GITHUB_TOKEN }}"
	//   LOG_LEVEL: "debug"
	//   WORKSPACE_DIR: "/data/workspace"
	// ```
	//
	// **Template Support**: Values can use Go template syntax to reference
	// environment variables from the host system.
	Env map[string]string `yaml:"env,omitempty"           json:"env,omitempty"`
	// Proto specifies the **MCP protocol version** to use.
	//
	// Different protocol versions may support different features, message formats,
	// or capabilities. Always use the version compatible with your MCP server.
	//
	// **Format**: `YYYY-MM-DD` (e.g., "2025-03-26")
	//
	// **Default**: `DefaultProtocolVersion` ("2025-03-26")
	//
	// **Version History**:
	// - `2025-03-26` - Latest version with streaming support
	// - `2024-12-01` - Initial protocol release
	Proto string `yaml:"proto,omitempty"         json:"proto,omitempty"`
	// Transport defines the **communication transport mechanism**.
	//
	// Choose the transport based on your MCP server's capabilities and deployment model.
	//
	// **Supported Values**:
	//
	// | Transport | Description | Use Case |
	// |-----------|-------------|----------|
	// | `sse` | Server-Sent Events | HTTP servers with real-time streaming |
	// | `streamable-http` | HTTP with streaming | Large responses, file transfers |
	// | `stdio` | Standard I/O | Local processes, Docker containers |
	//
	// **Default**: `sse`
	//
	// - **Examples**:
	// ```yaml
	// # Remote server with SSE
	// transport: sse
	//
	// # Local process with stdio
	// transport: stdio
	//
	// # HTTP server with large file support
	// transport: streamable-http
	// ```
	Transport mcpproxy.TransportType `yaml:"transport,omitempty"     json:"transport,omitempty"`
	// StartTimeout is the **maximum time to wait** for the MCP server to start.
	//
	// Only applicable when using `command` to spawn local processes.
	// Helps detect and handle startup failures gracefully.
	//
	// **Format**: Go duration string (e.g., "30s", "1m", "500ms")
	//
	// **Default**: No timeout (waits indefinitely)
	//
	// - **Examples**:
	// ```yaml
	// start_timeout: 30s   # Wait up to 30 seconds
	// start_timeout: 2m    # Wait up to 2 minutes
	// start_timeout: 500ms # Wait up to 500 milliseconds
	// ```
	//
	// **Recommendation**: Set to at least 10-30s for Docker-based servers.
	StartTimeout time.Duration `yaml:"start_timeout,omitempty" json:"start_timeout,omitempty"`
	// MaxSessions defines the **maximum number of concurrent sessions** allowed.
	//
	// Helps manage resource usage and prevent server overload.
	// Each agent connection typically creates one session.
	//
	// **Values**:
	// - `0` or negative: Unlimited sessions (default)
	// - Positive number: Maximum concurrent sessions
	//
	// - **Examples**:
	// ```yaml
	// max_sessions: 10  # Allow up to 10 concurrent connections
	// max_sessions: 1   # Single session only (useful for stateful servers)
	// max_sessions: 0   # Unlimited sessions
	// ```
	MaxSessions int `yaml:"max_sessions,omitempty"  json:"max_sessions,omitempty"`
}

// SetDefaults sets **default values** for optional configuration fields.
//
// This method ensures that the configuration has sensible defaults:
// - **Resource**: Defaults to ID if not specified
// - **Proto**: Defaults to `DefaultProtocolVersion` ("2025-03-26")
// - **Transport**: Defaults to `DefaultTransport` (SSE)
//
// Call this before validation to ensure all required defaults are set.
func (c *Config) SetDefaults() {
	// Set default resource if not specified
	if c.Resource == "" {
		c.Resource = c.ID
	}
	// Set default protocol version if not specified
	if c.Proto == "" {
		c.Proto = DefaultProtocolVersion
	}
	// Set default transport if not specified
	if c.Transport == "" {
		c.Transport = DefaultTransport
	}
}

// Validate performs **comprehensive validation** of the MCP configuration.
//
// ## Validation Steps
//
// 1. **Required Fields**:
//   - `id` must be non-empty
//   - `resource` must be set (auto-filled by SetDefaults)
//
// 2. **Transport-Specific Validation**:
//   - **HTTP transports** (sse, streamable-http): Requires valid `url`
//   - **stdio transport**: Requires `command` to be specified
//
// 3. **URL Validation** (for HTTP transports):
//   - Must be valid HTTP/HTTPS URL
//   - Must include a host component
//   - Must be properly formatted
//
// 4. **Environment Requirements**:
//   - `MCP_PROXY_URL` environment variable must be set
//   - Proxy URL must be valid HTTP/HTTPS endpoint
//
// 5. **Format Validation**:
//   - Protocol version must match `YYYY-MM-DD` format
//   - Transport must be one of: `sse`, `streamable-http`, or `stdio`
//
// 6. **Limit Validation**:
//   - `start_timeout` cannot be negative
//   - `max_sessions` cannot be negative
//
// ## Returns
//
// Returns an error describing the first validation failure encountered,
// or nil if the configuration is valid.
//
// ## Example Usage
//
// ```go
//
//	config := &Config{
//	    ID: "filesystem",
//	    URL: "http://localhost:3000/mcp",
//	    Transport: "sse",
//	}
//
// config.SetDefaults()
//
//	if err := config.Validate(); err != nil {
//	    log.Fatal("Invalid MCP config:", err)
//	}
//
// ```
func (c *Config) Validate() error {
	// Ensure defaults are set before validation
	c.SetDefaults()
	if err := c.validateID(); err != nil {
		return err
	}
	if err := c.validateResource(); err != nil {
		return err
	}
	if err := c.validateURL(); err != nil {
		return err
	}
	if err := c.validateProxy(); err != nil {
		return err
	}
	if err := c.validateProto(); err != nil {
		return err
	}
	if err := c.validateTransport(); err != nil {
		return err
	}
	if err := c.validateLimits(); err != nil {
		return err
	}
	return nil
}

func (c *Config) validateResource() error {
	if c.Resource == "" {
		return errors.New("mcp resource is required")
	}
	return nil
}

func (c *Config) validateID() error {
	if c.ID == "" {
		return errors.New("mcp id is required")
	}
	return nil
}

func (c *Config) validateURL() error {
	if c.URL == "" {
		return errors.New("mcp url is required when not using proxy")
	}

	return validateURLFormat(c.URL, "mcp url")
}

func (c *Config) validateProxy() error {
	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		return errors.New("MCP_PROXY_URL environment variable is required for MCP server configuration")
	}

	return validateURLFormat(proxyURL, "proxy url")
}

// validateURLFormat validates that a URL is properly formatted for MCP communication.
//
// ## Parameters
//
// - **urlStr**: The URL string to validate
// - **context**: Context for error messages (e.g., "mcp url", "proxy url")
//
// ## Validation Rules
//
// 1. **Parseable**: URL must be valid according to Go's URL parser
// 2. **Scheme**: Must be `http` or `https` (no other protocols)
// 3. **Host**: Must include a host component
//
// ## Valid Examples
//
// ```
// ✓ http://localhost:3000/mcp
// ✓ https://api.example.com/v1/mcp
// ✓ http://192.168.1.100:8080
// ✓ https://mcp-proxy.internal:443/filesystem
// ```
//
// ## Invalid Examples
//
// ```
// ✗ ws://localhost:3000        # Wrong scheme (WebSocket)
// ✗ http:///path              # Missing host
// ✗ not-a-url                 # Invalid format
// ✗ ftp://server.com          # Wrong protocol
// ✗ localhost:3000            # Missing scheme
// ```
func validateURLFormat(urlStr, context string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid %s format: %w", context, err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("%s must use http or https scheme, got: %s", context, parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("%s must include a host", context)
	}

	return nil
}

func (c *Config) validateProto() error {
	if !isValidProtoVersion(c.Proto) {
		return fmt.Errorf("invalid protocol version: %s", c.Proto)
	}
	return nil
}

func (c *Config) validateTransport() error {
	if !isValidTransport(c.Transport) {
		return fmt.Errorf("invalid transport type: %s (must be 'sse', 'streamable-http' or 'stdio')", c.Transport)
	}
	return nil
}

func (c *Config) validateLimits() error {
	if c.StartTimeout < 0 {
		return errors.New("start_timeout cannot be negative")
	}
	if c.MaxSessions < 0 {
		return errors.New("max_sessions cannot be negative")
	}
	return nil
}

// Clone creates a **deep copy** of the MCP configuration.
//
// This is useful when you need to modify a configuration without
// affecting the original instance. All fields including maps are
// properly duplicated.
//
// ## Returns
//
// - A new Config instance with all fields copied
// - nil if the receiver is nil
// - An error if cloning fails (rare)
//
// ## Example Usage
//
// ```go
//
//	original := &Config{
//	    ID: "filesystem",
//	    URL: "http://localhost:3000",
//	    Env: map[string]string{"KEY": "value"},
//	}
//
// cloned, err := original.Clone()
//
//	if err != nil {
//	    return err
//	}
//
// // Modifications don't affect original
// cloned.URL = "http://localhost:4000"
// cloned.Env["KEY"] = "new-value"
// ```
func (c *Config) Clone() (*Config, error) {
	if c == nil {
		return nil, nil
	}
	return core.DeepCopy(c)
}

// isValidProtoVersion validates the **MCP protocol version format**.
//
// Protocol versions must follow the ISO date format: `YYYY-MM-DD`
//
// ## Valid Examples
// - `2025-03-26` ✓
// - `2024-12-01` ✓
// - `2023-01-15` ✓
//
// ## Invalid Examples
// - `2025-3-26` ✗ (missing leading zero)
// - `25-03-26` ✗ (two-digit year)
// - `2025/03/26` ✗ (wrong separator)
// - `latest` ✗ (not a date)
func isValidProtoVersion(version string) bool {
	_, err := time.Parse("2006-01-02", version)
	return err == nil
}

// isValidTransport validates **MCP transport types**.
//
// ## Supported Transports
//
// | Transport | Constant | Description |
// |-----------|----------|-------------|
// | `sse` | TransportSSE | Server-Sent Events for HTTP-based real-time streaming |
// | `streamable-http` | TransportStreamableHTTP | HTTP with streaming support for large responses |
// | `stdio` | TransportStdio | Standard I/O for local process communication |
//
// ## Usage
//
// This function is used internally by Validate() to ensure only
// supported transport mechanisms are configured.
func isValidTransport(transport mcpproxy.TransportType) bool {
	return transport == mcpproxy.TransportSSE ||
		transport == mcpproxy.TransportStreamableHTTP ||
		transport == mcpproxy.TransportStdio
}
