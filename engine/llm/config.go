package llm

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/pkg/config"
)

// Config represents the configuration for the LLM service
type Config struct {
	// MCP proxy configuration
	ProxyURL   string
	AdminToken string
	// Caching configuration
	CacheTTL time.Duration
	// Timeout configuration
	Timeout time.Duration
	// Tool execution configuration
	MaxConcurrentTools int
	// Feature flags
	EnableStructuredOutput bool
	EnableToolCaching      bool
	// LLM factory for creating clients
	LLMFactory llmadapter.Factory
	// Memory provider for agent memory support
	MemoryProvider MemoryProvider
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		ProxyURL:               "http://localhost:8081",
		AdminToken:             "",
		CacheTTL:               5 * time.Minute,
		Timeout:                30 * time.Second,
		MaxConcurrentTools:     10,
		EnableStructuredOutput: true,
		EnableToolCaching:      true,
	}
}

// Option represents a configuration option
type Option func(*Config)

// WithProxyURL sets the MCP proxy URL
func WithProxyURL(url string) Option {
	return func(c *Config) {
		c.ProxyURL = url
	}
}

// WithAdminToken sets the admin token for the MCP proxy
func WithAdminToken(token string) Option {
	return func(c *Config) {
		c.AdminToken = token
	}
}

// WithCacheTTL sets the cache TTL for tools
func WithCacheTTL(ttl time.Duration) Option {
	return func(c *Config) {
		c.CacheTTL = ttl
	}
}

// WithTimeout sets the timeout for LLM operations
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// WithMaxConcurrentTools sets the maximum number of concurrent tool executions
func WithMaxConcurrentTools(maxTools int) Option {
	return func(c *Config) {
		c.MaxConcurrentTools = maxTools
	}
}

// WithStructuredOutput enables or disables structured output
func WithStructuredOutput(enabled bool) Option {
	return func(c *Config) {
		c.EnableStructuredOutput = enabled
	}
}

// WithToolCaching enables or disables tool caching
func WithToolCaching(enabled bool) Option {
	return func(c *Config) {
		c.EnableToolCaching = enabled
	}
}

// WithLLMFactory sets the LLM factory for creating clients
func WithLLMFactory(factory llmadapter.Factory) Option {
	return func(c *Config) {
		c.LLMFactory = factory
	}
}

// WithMemoryProvider sets the memory provider for agent memory support
func WithMemoryProvider(provider MemoryProvider) Option {
	return func(c *Config) {
		c.MemoryProvider = provider
	}
}

// WithAppConfig sets configuration values from the application config
func WithAppConfig(appConfig *config.Config) Option {
	return func(c *Config) {
		if appConfig == nil {
			return
		}

		if appConfig.LLM.ProxyURL != "" {
			c.ProxyURL = appConfig.LLM.ProxyURL
		}
		if appConfig.LLM.AdminToken.Value() != "" {
			c.AdminToken = appConfig.LLM.AdminToken.Value()
		}
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if strings.TrimSpace(c.ProxyURL) == "" {
		return fmt.Errorf("proxy URL cannot be empty")
	}
	if c.CacheTTL < 0 {
		return fmt.Errorf("cache TTL cannot be negative")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if c.MaxConcurrentTools <= 0 {
		return fmt.Errorf("max concurrent tools must be positive")
	}
	return nil
}

// CreateMCPClient creates an MCP client from the configuration
func (c *Config) CreateMCPClient() (*mcp.Client, error) {
	if c.ProxyURL == "" {
		return nil, fmt.Errorf("proxy URL is required for MCP client creation")
	}
	if _, err := url.ParseRequestURI(c.ProxyURL); err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}
	client := mcp.NewProxyClient(c.ProxyURL, c.AdminToken, c.Timeout)
	if client == nil {
		return nil, fmt.Errorf("failed to create MCP proxy client")
	}
	return client, nil
}
