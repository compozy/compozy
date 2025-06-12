package mcp

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultProtocolVersion is the default MCP protocol version
	DefaultProtocolVersion = "2025-03-26"
	// Transport types
	TransportSSE            = "sse"
	TransportStreamableHTTP = "streamable_http"
	DefaultTransport        = TransportSSE
)

// Config represents a remote MCP (Model Context Protocol) server configuration
type Config struct {
	ID           string            `yaml:"id"                      json:"id"`
	URL          string            `yaml:"url"                     json:"url"`
	Env          map[string]string `yaml:"env,omitempty"           json:"env,omitempty"`
	Proto        string            `yaml:"proto,omitempty"         json:"proto,omitempty"`
	Transport    string            `yaml:"transport,omitempty"     json:"transport,omitempty"`
	StartTimeout time.Duration     `yaml:"start_timeout,omitempty" json:"start_timeout,omitempty"`
	MaxSessions  int               `yaml:"max_sessions,omitempty"  json:"max_sessions,omitempty"`
}

// Validate validates the MCP configuration
func (c *Config) Validate() error {
	if c.ID == "" {
		return errors.New("mcp id is required")
	}

	if c.URL == "" {
		return errors.New("mcp url is required")
	}

	// Validate HTTP URL format
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

	// Set default protocol version if not specified
	if c.Proto == "" {
		c.Proto = DefaultProtocolVersion
	}

	// Validate protocol version format
	if !isValidProtoVersion(c.Proto) {
		return fmt.Errorf("invalid protocol version: %s", c.Proto)
	}

	// Set default transport if not specified
	if c.Transport == "" {
		c.Transport = DefaultTransport
	}

	// Validate transport type
	if !isValidTransport(c.Transport) {
		return fmt.Errorf("invalid transport type: %s (must be 'sse' or 'streamable_http')", c.Transport)
	}

	// Validate timeout and session limits
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

	for k, v := range c.Env {
		clone.Env[k] = v
	}

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
func isValidTransport(transport string) bool {
	return transport == TransportSSE || transport == TransportStreamableHTTP
}
