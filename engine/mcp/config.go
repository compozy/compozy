package mcp

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"strings"
	"time"

	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
)

const (
	DefaultProtocolVersion = "2025-03-26"
	DefaultTransport       = mcpproxy.TransportSSE
)

// Config represents a remote MCP (Model Context Protocol) server configuration
type Config struct {
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
	if err := c.validateID(); err != nil {
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

	parsedURL, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("invalid mcp url format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("mcp url must use http or https scheme, got: %s", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("mcp url must include a host")
	}

	return nil
}

func (c *Config) validateProxy() error {
	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		return errors.New("MCP_PROXY_URL environment variable is required for MCP server configuration")
	}

	parsedProxyURL, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy url format: %w", err)
	}

	if parsedProxyURL.Scheme != "http" && parsedProxyURL.Scheme != "https" {
		return fmt.Errorf("proxy url must use http or https scheme, got: %s", parsedProxyURL.Scheme)
	}

	if parsedProxyURL.Host == "" {
		return fmt.Errorf("proxy url must include a host")
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
		return fmt.Errorf("invalid transport type: %s (must be 'sse' or 'streamable-http')", c.Transport)
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
func (c *Config) Clone() *Config {
	clone := &Config{
		ID:           c.ID,
		URL:          c.URL,
		Env:          make(map[string]string),
		Proto:        c.Proto,
		Transport:    c.Transport,
		StartTimeout: c.StartTimeout,
		MaxSessions:  c.MaxSessions,
	}
	maps.Copy(clone.Env, c.Env)
	return clone
}

// isValidProtoVersion validates the protocol version format (YYYY-MM-DD)
func isValidProtoVersion(version string) bool {
	parts := strings.Split(version, "-")
	if len(parts) != 3 {
		return false
	}
	// Basic format validation - should be YYYY-MM-DD
	year, month, day := parts[0], parts[1], parts[2]
	if len(year) != 4 || len(month) != 2 || len(day) != 2 {
		return false
	}
	// All parts should be numeric
	for _, part := range parts {
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

// isValidTransport validates the transport type
func isValidTransport(transport mcpproxy.TransportType) bool {
	return transport == mcpproxy.TransportSSE ||
		transport == mcpproxy.TransportStreamableHTTP ||
		transport == mcpproxy.TransportStdio
}
