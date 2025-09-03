package llm

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/tool"
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
	// MaxToolIterations caps the maximum number of tool-iteration loops per request.
	// If zero or negative, provider-specific or default limits apply.
	MaxToolIterations int
	// Retry configuration
	RetryAttempts    int
	RetryBackoffBase time.Duration
	RetryBackoffMax  time.Duration
	RetryJitter      bool
	// Feature flags
	EnableStructuredOutput bool
	EnableToolCaching      bool
	// LLM factory for creating clients
	LLMFactory llmadapter.Factory
	// Memory provider for agent memory support
	MemoryProvider MemoryProvider
	// ResolvedTools contains pre-resolved tools from hierarchical inheritance
	ResolvedTools []tool.Config
}

// DefaultConfig returns a default configuration
// defaultTimeout is the default timeout for LLM operations (5 minutes)
// This aligns with MCP proxy SLAs and provider timeouts for long-running operations
const defaultTimeout = 300 * time.Second

func DefaultConfig() *Config {
	return &Config{
		ProxyURL:               "http://localhost:6001",
		AdminToken:             "",
		CacheTTL:               5 * time.Minute,
		Timeout:                defaultTimeout,
		MaxConcurrentTools:     10,
		MaxToolIterations:      10,
		RetryAttempts:          3,
		RetryBackoffBase:       100 * time.Millisecond,
		RetryBackoffMax:        10 * time.Second,
		RetryJitter:            true,
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

// WithResolvedTools sets the pre-resolved tools from hierarchical inheritance
// The slice is copied to prevent external mutation after construction
func WithResolvedTools(tools []tool.Config) Option {
	return func(c *Config) {
		if len(tools) == 0 {
			c.ResolvedTools = nil
			return
		}
		c.ResolvedTools = append(make([]tool.Config, 0, len(tools)), tools...)
	}
}

// WithRetryAttempts sets the number of retry attempts for LLM operations
func WithRetryAttempts(attempts int) Option {
	return func(c *Config) {
		c.RetryAttempts = attempts
	}
}

// WithRetryBackoffBase sets the base delay for exponential backoff retry strategy
func WithRetryBackoffBase(base time.Duration) Option {
	return func(c *Config) {
		c.RetryBackoffBase = base
	}
}

// WithRetryBackoffMax sets the maximum delay between retry attempts
func WithRetryBackoffMax(maxDelay time.Duration) Option {
	return func(c *Config) {
		c.RetryBackoffMax = maxDelay
	}
}

// WithRetryJitter enables or disables random jitter in retry delays
func WithRetryJitter(enabled bool) Option {
	return func(c *Config) {
		c.RetryJitter = enabled
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
		if appConfig.LLM.RetryAttempts > 0 {
			c.RetryAttempts = appConfig.LLM.RetryAttempts
		}
		if appConfig.LLM.RetryBackoffBase > 0 {
			c.RetryBackoffBase = appConfig.LLM.RetryBackoffBase
		}
		if appConfig.LLM.RetryBackoffMax > 0 {
			c.RetryBackoffMax = appConfig.LLM.RetryBackoffMax
		}
		c.RetryJitter = appConfig.LLM.RetryJitter
		if appConfig.LLM.MaxConcurrentTools > 0 {
			c.MaxConcurrentTools = appConfig.LLM.MaxConcurrentTools
		}
		if appConfig.LLM.MaxToolIterations > 0 {
			c.MaxToolIterations = appConfig.LLM.MaxToolIterations
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

	// Validate pre-resolved tools if present
	if len(c.ResolvedTools) > 0 {
		seenIDs := make(map[string]bool)
		for i := range c.ResolvedTools {
			tool := &c.ResolvedTools[i]
			if strings.TrimSpace(tool.ID) == "" {
				return fmt.Errorf("resolved tool at index %d has empty ID", i)
			}
			if seenIDs[tool.ID] {
				return fmt.Errorf("duplicate tool ID '%s' found in resolved tools", tool.ID)
			}
			seenIDs[tool.ID] = true
		}
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
