package llm

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/config"
)

// Config represents the configuration for the LLM service
type Config struct {
	// MCP proxy configuration
	ProxyURL string
	// Caching configuration
	CacheTTL time.Duration
	// Timeout configuration
	Timeout time.Duration
	// Tool execution configuration
	MaxConcurrentTools int
	// MaxToolIterations caps the maximum number of tool-iteration loops per request.
	// If zero or negative, provider-specific or default limits apply.
	MaxToolIterations int
	// MaxSequentialToolErrors caps how many sequential failures are tolerated
	// for the same tool (or content-level error) before aborting the task.
	// When <= 0, a default of 8 is used (see DefaultConfig).
	MaxSequentialToolErrors int
	// Retry configuration
	RetryAttempts      int
	RetryBackoffBase   time.Duration
	RetryBackoffMax    time.Duration
	RetryJitter        bool
	RetryJitterPercent int
	// Feature flags
	EnableStructuredOutput bool
	EnableToolCaching      bool
	// LLM factory for creating clients
	LLMFactory llmadapter.Factory
	// Memory provider for agent memory support
	MemoryProvider MemoryProvider
	// ResolvedTools contains pre-resolved tools from hierarchical inheritance
	ResolvedTools []tool.Config
	// AllowedMCPNames restricts MCP tool advertisement/lookup to these MCP IDs.
	// When empty, all discovered MCP tools are eligible.
	AllowedMCPNames []string
	// FailOnMCPRegistrationError enforces fail-fast behavior when registering
	// agent-declared MCPs. When true, NewService returns error on registration failure.
	FailOnMCPRegistrationError bool
	// RegisterMCPs contains additional MCP configurations to register with the
	// proxy for this service instance (e.g., workflow-level MCPs). These are
	// merged with agent-declared MCPs for registration.
	RegisterMCPs []mcp.Config
}

func DefaultConfig() *Config {
	return &Config{
		ProxyURL:                   "http://localhost:6001",
		CacheTTL:                   5 * time.Minute,
		Timeout:                    300 * time.Second,
		MaxConcurrentTools:         10,
		MaxToolIterations:          10,
		MaxSequentialToolErrors:    8,
		RetryAttempts:              3,
		RetryBackoffBase:           100 * time.Millisecond,
		RetryBackoffMax:            10 * time.Second,
		RetryJitter:                true,
		RetryJitterPercent:         10,
		EnableStructuredOutput:     true,
		EnableToolCaching:          true,
		FailOnMCPRegistrationError: false,
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

// WithAllowedMCPNames sets an allowlist of MCP IDs to restrict which MCP tools
// are advertised and callable for this service instance.
func WithAllowedMCPNames(mcpIDs []string) Option {
	return func(c *Config) {
		// Shallow copy is sufficient; values are strings
		c.AllowedMCPNames = nil
		if len(mcpIDs) > 0 {
			// Deduplicate and normalize
			seen := make(map[string]struct{})
			out := make([]string, 0, len(mcpIDs))
			for _, id := range mcpIDs {
				nid := strings.ToLower(strings.TrimSpace(id))
				if nid == "" {
					continue
				}
				if _, ok := seen[nid]; ok {
					continue
				}
				seen[nid] = struct{}{}
				out = append(out, nid)
			}
			c.AllowedMCPNames = out
		}
	}
}

// WithStrictMCPRegistration sets fail-fast behavior for MCP registration errors.
func WithStrictMCPRegistration(strict bool) Option {
	return func(c *Config) {
		c.FailOnMCPRegistrationError = strict
	}
}

// WithRegisterMCPs adds MCP configurations to be registered with the proxy
// in addition to any MCPs declared on the agent configuration (e.g., workflow MCPs).
func WithRegisterMCPs(mcps []mcp.Config) Option {
	return func(c *Config) {
		if len(mcps) == 0 {
			return
		}
		// Deep copy selected map fields to avoid aliasing
		c.RegisterMCPs = make([]mcp.Config, 0, len(mcps))
		for i := range mcps {
			dst := mcps[i]
			if mcps[i].Headers != nil {
				dst.Headers = make(map[string]string, len(mcps[i].Headers))
				for k, v := range mcps[i].Headers {
					dst.Headers[k] = v
				}
			}
			if mcps[i].Env != nil {
				dst.Env = make(map[string]string, len(mcps[i].Env))
				for k, v := range mcps[i].Env {
					dst.Env[k] = v
				}
			}
			c.RegisterMCPs = append(c.RegisterMCPs, dst)
		}
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
		// Deep copy each tool to prevent external mutation of nested fields
		c.ResolvedTools = make([]tool.Config, 0, len(tools))
		for i := range tools {
			// Use core.DeepCopy to clone the element; on failure, fall back to value copy
			if cloned, err := core.DeepCopy(tools[i]); err == nil {
				c.ResolvedTools = append(c.ResolvedTools, cloned)
			} else {
				c.ResolvedTools = append(c.ResolvedTools, tools[i])
			}
		}
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
		if appConfig.LLM.MaxSequentialToolErrors > 0 {
			c.MaxSequentialToolErrors = appConfig.LLM.MaxSequentialToolErrors
		}
		if appConfig.LLM.RetryJitterPercent > 0 {
			c.RetryJitterPercent = appConfig.LLM.RetryJitterPercent
		}

		// Propagate MCP-related options
		if len(appConfig.LLM.AllowedMCPNames) > 0 {
			WithAllowedMCPNames(appConfig.LLM.AllowedMCPNames)(c)
		}
		c.FailOnMCPRegistrationError = appConfig.LLM.FailOnMCPRegistrationError

		// Convert RegisterMCPs from generic maps into typed mcp.Configs (best-effort)
		if len(appConfig.LLM.RegisterMCPs) > 0 {
			converted := mcp.ConvertRegisterMCPsFromMaps(appConfig.LLM.RegisterMCPs)
			if len(converted) > 0 {
				c.RegisterMCPs = converted
			}
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
		// First pass: check structural issues (empty IDs, duplicates)
		seenIDs := make(map[string]bool)
		for i := range c.ResolvedTools {
			t := &c.ResolvedTools[i]
			if strings.TrimSpace(t.ID) == "" {
				return fmt.Errorf("resolved tool at index %d has empty ID", i)
			}
			if seenIDs[t.ID] {
				return fmt.Errorf("duplicate tool ID '%s' found in resolved tools", t.ID)
			}
			seenIDs[t.ID] = true
		}

		// Second pass: validate each tool only after ID set and uniqueness guaranteed
		for i := range c.ResolvedTools {
			t := &c.ResolvedTools[i]
			if err := t.Validate(); err != nil {
				return fmt.Errorf("resolved tool '%s' validation failed: %w", t.ID, err)
			}
		}
	}

	return nil
}

// CreateMCPClient creates an MCP client from the configuration
func (c *Config) CreateMCPClient() (*mcp.Client, error) {
	if c.ProxyURL == "" {
		return nil, fmt.Errorf("proxy URL is required for MCP client creation")
	}
	// Normalize URL by adding http:// prefix if no scheme is present
	u := c.ProxyURL
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "http://" + u
	}
	if _, err := url.ParseRequestURI(u); err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}
	client := mcp.NewProxyClient(u, c.Timeout)
	if client == nil {
		return nil, fmt.Errorf("failed to create MCP proxy client")
	}
	// Align retry behavior with configured LLM retry settings
	// RetryAttempts maps to retries (not attempts). Base and Max are respected.
	attempts := c.RetryAttempts
	if attempts < 0 {
		attempts = 0
	}
	retries := uint64(attempts) //nolint:gosec // G115: bounds checked above and values come from validated config
	// Configure retry with jitter percentage (capped inside the client)
	jp := uint64(0)
	if c.RetryJitter && c.RetryJitterPercent > 0 {
		jp = uint64(c.RetryJitterPercent)
	}
	client.ConfigureRetry(
		retries,
		c.RetryBackoffBase,
		c.RetryBackoffMax,
		c.RetryJitter,
		jp,
	)
	return client, nil
}
