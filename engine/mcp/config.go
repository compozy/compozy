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
	DefaultProtocolVersion = "2025-03-26"
	DefaultTransport       = mcpproxy.TransportSSE
)

// Config represents a remote MCP (Model Context Protocol) server configuration
type Config struct {
	Resource     string                 `yaml:"resource,omitempty"      json:"resource,omitempty"`
	ID           string                 `yaml:"id"                      json:"id"`
	URL          string                 `yaml:"url"                     json:"url"`
	Command      string                 `yaml:"command,omitempty"       json:"command,omitempty"`
	Env          map[string]string      `yaml:"env,omitempty"           json:"env,omitempty"`
	Proto        string                 `yaml:"proto,omitempty"         json:"proto,omitempty"`
	Transport    mcpproxy.TransportType `yaml:"transport,omitempty"     json:"transport,omitempty"`
	StartTimeout time.Duration          `yaml:"start_timeout,omitempty" json:"start_timeout,omitempty"`
	MaxSessions  int                    `yaml:"max_sessions,omitempty"  json:"max_sessions,omitempty"`
}

// SetDefaults sets default values for optional configuration fields
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

// Validate validates the MCP configuration
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

// validateURLFormat validates a URL format and scheme for the given context
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

// Clone creates a deep copy of the MCP configuration
func (c *Config) Clone() (*Config, error) {
	if c == nil {
		return nil, nil
	}
	return core.DeepCopy(c)
}

// isValidProtoVersion validates the protocol version format (YYYY-MM-DD)
func isValidProtoVersion(version string) bool {
	_, err := time.Parse("2006-01-02", version)
	return err == nil
}

// isValidTransport validates the transport type
func isValidTransport(transport mcpproxy.TransportType) bool {
	return transport == mcpproxy.TransportSSE ||
		transport == mcpproxy.TransportStreamableHTTP ||
		transport == mcpproxy.TransportStdio
}
