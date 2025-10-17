package llm

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	orchestratorpkg "github.com/compozy/compozy/engine/llm/orchestrator"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/config"
)

const (
	defaultStructuredOutputRetries = 2
	defaultMaxConsecutiveSuccesses = 3
	defaultNoProgressThreshold     = 3
	defaultRestartStallThreshold   = 2
	defaultMaxLoopRestarts         = 1
	defaultCompactionThreshold     = 0.85
	defaultCompactionCooldown      = 2
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
	// MaxConsecutiveSuccesses controls how many consecutive successes without progress
	// are allowed before considering the conversation stalled.
	MaxConsecutiveSuccesses int
	// EnableProgressTracking toggles loop-level progress monitoring to detect stalls.
	EnableProgressTracking bool
	// NoProgressThreshold defines how many turns without observable progress are tolerated.
	NoProgressThreshold int
	// EnableLoopRestarts toggles restart of the orchestration loop when progress stalls.
	EnableLoopRestarts bool
	// RestartStallThreshold controls how many stalled iterations trigger a restart.
	RestartStallThreshold int
	// MaxLoopRestarts caps how many restarts are attempted per orchestration run.
	MaxLoopRestarts int
	// EnableContextCompaction toggles summary-based memory compaction when context usage grows.
	EnableContextCompaction bool
	// ContextCompactionThreshold expresses the context usage (0-1) that should trigger compaction.
	ContextCompactionThreshold float64
	// ContextCompactionCooldown controls how many iterations to wait between compaction attempts.
	ContextCompactionCooldown int
	// EnableDynamicPromptState toggles inclusion of runtime state in the system prompt.
	EnableDynamicPromptState bool
	// ToolCallCaps defines default and per-tool invocation caps enforced during orchestration.
	ToolCallCaps orchestratorpkg.ToolCallCaps
	// FinalizeOutputRetryAttempts overrides structured retry attempts for finalizing JSON outputs.
	FinalizeOutputRetryAttempts int
	// OrchestratorMiddlewares registers middleware hooks executed during orchestration.
	OrchestratorMiddlewares []orchestratorpkg.Middleware
	// StructuredOutputRetryAttempts controls how many times the orchestrator
	// will retry to obtain a valid structured response before failing.
	// Acceptable range: 0–10. When 0 or negative, falls back to default (2).
	StructuredOutputRetryAttempts int
	// Retry configuration
	RetryAttempts                     int
	RetryBackoffBase                  time.Duration
	RetryBackoffMax                   time.Duration
	RetryJitter                       bool
	RetryJitterPercent                int
	TelemetryContextWarningThresholds []float64
	// Feature flags
	EnableStructuredOutput bool
	EnableToolCaching      bool
	ProjectRoot            string
	// LLM factory for creating clients
	LLMFactory llmadapter.Factory
	// RateLimiter coordinates provider concurrency throttles shared across orchestrations.
	// When nil, requests execute without centralized throttling.
	RateLimiter *llmadapter.RateLimiterRegistry
	// Memory provider for agent memory support
	MemoryProvider MemoryProvider
	// Knowledge contains resolved knowledge context for retrieval during orchestration.
	Knowledge *KnowledgeRuntimeConfig
	// ResolvedTools contains pre-resolved tools from hierarchical inheritance
	ResolvedTools []tool.Config
	// AllowedMCPNames restricts MCP tool advertisement/lookup to these MCP IDs.
	// When empty, all discovered MCP tools are eligible.
	AllowedMCPNames []string
	// DeniedMCPNames excludes MCP tool advertisement/lookup to these MCP IDs.
	DeniedMCPNames []string
	// FailOnMCPRegistrationError enforces fail-fast behavior when registering
	// agent-declared MCPs. When true, NewService returns error on registration failure.
	FailOnMCPRegistrationError bool
	// RegisterMCPs contains additional MCP configurations to register with the
	// proxy for this service instance (e.g., workflow-level MCPs). These are
	// merged with agent-declared MCPs for registration.
	RegisterMCPs []mcp.Config
	// ToolEnvironment provides dependency access for builtin tools.
	ToolEnvironment toolenv.Environment
}

type KnowledgeRuntimeConfig struct {
	ProjectID              string
	Definitions            knowledge.Definitions
	WorkflowKnowledgeBases []knowledge.BaseConfig
	ProjectBinding         []core.KnowledgeBinding
	WorkflowBinding        []core.KnowledgeBinding
	InlineBinding          []core.KnowledgeBinding
	RuntimeEmbedders       map[string]*knowledge.EmbedderConfig
	RuntimeVectorDBs       map[string]*knowledge.VectorDBConfig
	RuntimeKnowledgeBases  map[string]*knowledge.BaseConfig
	RuntimeWorkflowKBs     map[string]*knowledge.BaseConfig
}

func DefaultConfig() *Config {
	return &Config{
		ProxyURL:                          "http://localhost:6001",
		CacheTTL:                          5 * time.Minute,
		Timeout:                           300 * time.Second,
		MaxConcurrentTools:                10,
		MaxToolIterations:                 10,
		MaxSequentialToolErrors:           8,
		MaxConsecutiveSuccesses:           defaultMaxConsecutiveSuccesses,
		NoProgressThreshold:               defaultNoProgressThreshold,
		EnableLoopRestarts:                false,
		RestartStallThreshold:             0,
		MaxLoopRestarts:                   0,
		EnableContextCompaction:           false,
		ContextCompactionThreshold:        0,
		ContextCompactionCooldown:         0,
		StructuredOutputRetryAttempts:     defaultStructuredOutputRetries,
		FinalizeOutputRetryAttempts:       0,
		RetryAttempts:                     3,
		RetryBackoffBase:                  100 * time.Millisecond,
		RetryBackoffMax:                   10 * time.Second,
		RetryJitter:                       true,
		RetryJitterPercent:                10,
		TelemetryContextWarningThresholds: []float64{0.7, 0.85},
		EnableStructuredOutput:            true,
		EnableToolCaching:                 true,
		FailOnMCPRegistrationError:        false,
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

// WithMaxConsecutiveSuccesses sets the maximum allowed consecutive successes without progress.
func WithMaxConsecutiveSuccesses(value int) Option {
	return func(c *Config) {
		c.MaxConsecutiveSuccesses = value
	}
}

// WithProgressTracking enables or disables loop progress tracking.
func WithProgressTracking(enabled bool) Option {
	return func(c *Config) {
		c.EnableProgressTracking = enabled
	}
}

// WithNoProgressThreshold sets the threshold for consecutive iterations without progress.
func WithNoProgressThreshold(threshold int) Option {
	return func(c *Config) {
		c.NoProgressThreshold = threshold
	}
}

// WithDynamicPromptState toggles runtime prompt state injection.
func WithDynamicPromptState(enabled bool) Option {
	return func(c *Config) {
		c.EnableDynamicPromptState = enabled
	}
}

// WithToolCallCaps sets default and per-tool invocation limits.
func WithToolCallCaps(caps orchestratorpkg.ToolCallCaps) Option {
	return func(cCfg *Config) {
		var overrides map[string]int
		if len(caps.Overrides) > 0 {
			overrides = core.CopyMap(caps.Overrides)
		}
		cCfg.ToolCallCaps = orchestratorpkg.ToolCallCaps{
			Default:   caps.Default,
			Overrides: overrides,
		}
	}
}

// WithFinalizeOutputRetries sets retries for finalizing structured output.
func WithFinalizeOutputRetries(attempts int) Option {
	return func(c *Config) {
		c.FinalizeOutputRetryAttempts = attempts
	}
}

// WithOrchestratorMiddlewares registers middleware hooks for orchestrator runs.
func WithOrchestratorMiddlewares(middlewares ...orchestratorpkg.Middleware) Option {
	return func(c *Config) {
		if len(middlewares) == 0 {
			c.OrchestratorMiddlewares = nil
			return
		}
		out := make([]orchestratorpkg.Middleware, 0, len(middlewares))
		for _, mw := range middlewares {
			if mw == nil {
				continue
			}
			out = append(out, mw)
		}
		c.OrchestratorMiddlewares = out
	}
}

// WithStructuredOutputRetries sets the structured output retry attempts
func WithStructuredOutputRetries(attempts int) Option {
	return func(c *Config) {
		if attempts < 0 {
			attempts = 0 // let effective default kick in
		}
		c.StructuredOutputRetryAttempts = attempts
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

// WithToolEnvironment injects the tool environment used during builtin registration.
func WithToolEnvironment(env toolenv.Environment) Option {
	return func(c *Config) {
		c.ToolEnvironment = env
	}
}

// WithProjectRoot sets the project root directory used for orchestrator diagnostics.
func WithProjectRoot(root string) Option {
	return func(c *Config) {
		c.ProjectRoot = strings.TrimSpace(root)
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

// WithDeniedMCPNames sets a deny list of MCP IDs to restrict which MCP tools are advertised.
func WithDeniedMCPNames(mcpIDs []string) Option {
	return func(c *Config) {
		c.DeniedMCPNames = nil
		if len(mcpIDs) == 0 {
			return
		}
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
		c.DeniedMCPNames = out
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
				dst.Headers = core.CopyMap(mcps[i].Headers)
			}
			if mcps[i].Env != nil {
				dst.Env = core.CopyMap(mcps[i].Env)
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

// WithKnowledgeContext wires knowledge runtime context for retrieval-aware prompts.
func WithKnowledgeContext(cfg *KnowledgeRuntimeConfig) Option {
	return func(c *Config) {
		c.Knowledge = cloneKnowledgeConfig(cfg)
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

func cloneKnowledgeConfig(cfg *KnowledgeRuntimeConfig) *KnowledgeRuntimeConfig {
	if cfg == nil {
		return nil
	}
	cloned, err := core.DeepCopy(*cfg)
	if err != nil {
		return cloneKnowledgeConfigFallback(cfg)
	}
	cloned.ProjectID = strings.TrimSpace(cloned.ProjectID)
	return &cloned
}

func cloneKnowledgeConfigFallback(cfg *KnowledgeRuntimeConfig) *KnowledgeRuntimeConfig {
	fallback := KnowledgeRuntimeConfig{
		ProjectID:       strings.TrimSpace(cfg.ProjectID),
		ProjectBinding:  append([]core.KnowledgeBinding(nil), cfg.ProjectBinding...),
		WorkflowBinding: append([]core.KnowledgeBinding(nil), cfg.WorkflowBinding...),
		InlineBinding:   append([]core.KnowledgeBinding(nil), cfg.InlineBinding...),
	}
	if defsCopy, copyErr := core.DeepCopy(cfg.Definitions); copyErr == nil {
		fallback.Definitions = defsCopy
	} else {
		fallback.Definitions = cfg.Definitions
	}
	if len(cfg.WorkflowKnowledgeBases) > 0 {
		if basesCopy, copyErr := core.DeepCopy(cfg.WorkflowKnowledgeBases); copyErr == nil {
			fallback.WorkflowKnowledgeBases = basesCopy
		} else {
			fallback.WorkflowKnowledgeBases = append([]knowledge.BaseConfig{}, cfg.WorkflowKnowledgeBases...)
		}
	}
	fallback.RuntimeEmbedders = cloneEmbedderOverrides(cfg.RuntimeEmbedders)
	fallback.RuntimeVectorDBs = cloneVectorOverrides(cfg.RuntimeVectorDBs)
	fallback.RuntimeKnowledgeBases = cloneKnowledgeOverrides(cfg.RuntimeKnowledgeBases)
	fallback.RuntimeWorkflowKBs = cloneKnowledgeOverrides(cfg.RuntimeWorkflowKBs)
	return &fallback
}

func cloneEmbedderOverrides(src map[string]*knowledge.EmbedderConfig) map[string]*knowledge.EmbedderConfig {
	if len(src) == 0 {
		return nil
	}
	if out, err := core.DeepCopy(src); err == nil {
		return out
	}
	out := make(map[string]*knowledge.EmbedderConfig, len(src))
	for key, cfg := range src {
		if cfg == nil {
			out[key] = nil
			continue
		}
		copyCfg := *cfg
		out[key] = &copyCfg
	}
	return out
}

func cloneVectorOverrides(src map[string]*knowledge.VectorDBConfig) map[string]*knowledge.VectorDBConfig {
	if len(src) == 0 {
		return nil
	}
	if out, err := core.DeepCopy(src); err == nil {
		return out
	}
	out := make(map[string]*knowledge.VectorDBConfig, len(src))
	for key, cfg := range src {
		if cfg == nil {
			out[key] = nil
			continue
		}
		copyCfg := *cfg
		out[key] = &copyCfg
	}
	return out
}

func cloneKnowledgeOverrides(src map[string]*knowledge.BaseConfig) map[string]*knowledge.BaseConfig {
	if len(src) == 0 {
		return nil
	}
	if out, err := core.DeepCopy(src); err == nil {
		return out
	}
	out := make(map[string]*knowledge.BaseConfig, len(src))
	for key, cfg := range src {
		if cfg == nil {
			out[key] = nil
			continue
		}
		copyCfg := *cfg
		out[key] = &copyCfg
	}
	return out
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
		applyLLMCoreEndpoints(c, &appConfig.LLM)
		applyLLMRetryConfig(c, &appConfig.LLM)
		applyLLMToolLimits(c, &appConfig.LLM)
		applyLLMTelemetryConfig(c, &appConfig.LLM)
		applyLLMMCPOptions(c, &appConfig.LLM)
		applyLLMRateLimiter(c, &appConfig.LLM)
	}
}

func applyLLMCoreEndpoints(c *Config, llm *config.LLMConfig) {
	if llm.ProxyURL != "" {
		c.ProxyURL = llm.ProxyURL
	}
	if llm.ProviderTimeout > 0 {
		c.Timeout = llm.ProviderTimeout
	} else if llm.MCPClientTimeout > 0 {
		c.Timeout = llm.MCPClientTimeout
	}
}

func applyLLMRetryConfig(c *Config, llm *config.LLMConfig) {
	if llm.RetryAttempts > 0 {
		c.RetryAttempts = llm.RetryAttempts
	}
	if llm.RetryBackoffBase > 0 {
		c.RetryBackoffBase = llm.RetryBackoffBase
	}
	if llm.RetryBackoffMax > 0 {
		c.RetryBackoffMax = llm.RetryBackoffMax
	}
	c.RetryJitter = llm.RetryJitter
	if llm.RetryJitterPercent > 0 {
		c.RetryJitterPercent = llm.RetryJitterPercent
	}
}

func applyLLMToolLimits(c *Config, llm *config.LLMConfig) {
	if llm.MaxConcurrentTools > 0 {
		c.MaxConcurrentTools = llm.MaxConcurrentTools
	}
	if llm.MaxToolIterations > 0 {
		c.MaxToolIterations = llm.MaxToolIterations
	}
	if llm.MaxSequentialToolErrors > 0 {
		c.MaxSequentialToolErrors = llm.MaxSequentialToolErrors
	}
	if llm.MaxConsecutiveSuccesses > 0 {
		c.MaxConsecutiveSuccesses = llm.MaxConsecutiveSuccesses
	}
	if llm.NoProgressThreshold > 0 {
		c.NoProgressThreshold = llm.NoProgressThreshold
	}
	c.EnableProgressTracking = llm.EnableProgressTracking
	c.EnableLoopRestarts = llm.EnableLoopRestarts
	if llm.RestartStallThreshold > 0 {
		c.RestartStallThreshold = llm.RestartStallThreshold
	}
	if llm.MaxLoopRestarts > 0 {
		c.MaxLoopRestarts = llm.MaxLoopRestarts
	}
	c.EnableContextCompaction = llm.EnableContextCompaction
	if llm.ContextCompactionThreshold > 0 {
		c.ContextCompactionThreshold = llm.ContextCompactionThreshold
	}
	if llm.ContextCompactionCooldown > 0 {
		c.ContextCompactionCooldown = llm.ContextCompactionCooldown
	}
	c.EnableDynamicPromptState = llm.EnableDynamicPromptState
	if llm.ToolCallCaps.Default > 0 || len(llm.ToolCallCaps.Overrides) > 0 {
		c.ToolCallCaps = orchestratorpkg.ToolCallCaps{
			Default:   llm.ToolCallCaps.Default,
			Overrides: cloneToolCapOverrides(llm.ToolCallCaps.Overrides),
		}
	}
	if llm.FinalizeOutputRetryAttempts > 0 {
		c.FinalizeOutputRetryAttempts = llm.FinalizeOutputRetryAttempts
	}
	if llm.StructuredOutputRetryAttempts > 0 {
		c.StructuredOutputRetryAttempts = llm.StructuredOutputRetryAttempts
	}
}

func applyLLMTelemetryConfig(c *Config, llm *config.LLMConfig) {
	if len(llm.ContextWarningThresholds) > 0 {
		c.TelemetryContextWarningThresholds = append([]float64(nil), llm.ContextWarningThresholds...)
	}
}

func cloneToolCapOverrides(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	return core.CopyMap(src)
}

func applyLLMMCPOptions(c *Config, llm *config.LLMConfig) {
	if len(llm.AllowedMCPNames) > 0 {
		WithAllowedMCPNames(llm.AllowedMCPNames)(c)
	}
	if len(llm.DeniedMCPNames) > 0 {
		WithDeniedMCPNames(llm.DeniedMCPNames)(c)
	}
	c.FailOnMCPRegistrationError = llm.FailOnMCPRegistrationError
	if len(llm.RegisterMCPs) > 0 {
		converted := mcp.ConvertRegisterMCPsFromMaps(llm.RegisterMCPs)
		if len(converted) > 0 {
			c.RegisterMCPs = converted
		}
	}
}

func applyLLMRateLimiter(c *Config, llm *config.LLMConfig) {
	if llm == nil {
		return
	}
	if llm.RateLimiting.Enabled {
		c.RateLimiter = llmadapter.NewRateLimiterRegistry(llm.RateLimiting)
		return
	}
	c.RateLimiter = nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if err := c.validateBasics(); err != nil {
		return err
	}
	if err := c.validateResolvedTools(); err != nil {
		return err
	}
	if err := c.validateToolCaps(); err != nil {
		return err
	}
	c.applyDefaultLimits()
	return nil
}

func (c *Config) validateBasics() error {
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

func (c *Config) validateResolvedTools() error {
	if len(c.ResolvedTools) == 0 {
		return nil
	}
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
	for i := range c.ResolvedTools {
		t := &c.ResolvedTools[i]
		if err := t.Validate(); err != nil {
			return fmt.Errorf("resolved tool '%s' validation failed: %w", t.ID, err)
		}
	}
	return nil
}

func (c *Config) validateToolCaps() error {
	if c.FinalizeOutputRetryAttempts < 0 {
		return fmt.Errorf("finalize output retry attempts cannot be negative")
	}
	if c.ToolCallCaps.Default < 0 {
		return fmt.Errorf("default tool call cap cannot be negative")
	}
	for name, limit := range c.ToolCallCaps.Overrides {
		if limit < 0 {
			return fmt.Errorf("tool call cap for %s cannot be negative", name)
		}
	}
	return nil
}

func (c *Config) applyDefaultLimits() {
	if c.StructuredOutputRetryAttempts <= 0 {
		c.StructuredOutputRetryAttempts = defaultStructuredOutputRetries
	} else if c.StructuredOutputRetryAttempts > 10 {
		c.StructuredOutputRetryAttempts = 10
	}
	if c.MaxConsecutiveSuccesses <= 0 {
		c.MaxConsecutiveSuccesses = defaultMaxConsecutiveSuccesses
	}
	if c.NoProgressThreshold <= 0 {
		c.NoProgressThreshold = defaultNoProgressThreshold
	}
	if c.FinalizeOutputRetryAttempts > 10 {
		c.FinalizeOutputRetryAttempts = 10
	}
	if c.EnableLoopRestarts {
		if c.RestartStallThreshold <= 0 {
			c.RestartStallThreshold = defaultRestartStallThreshold
		}
		if c.MaxLoopRestarts <= 0 {
			c.MaxLoopRestarts = defaultMaxLoopRestarts
		}
	} else {
		if c.MaxLoopRestarts < 0 {
			c.MaxLoopRestarts = 0
		}
		if c.RestartStallThreshold < 0 {
			c.RestartStallThreshold = 0
		}
	}
	if c.EnableContextCompaction {
		if c.ContextCompactionThreshold <= 0 {
			c.ContextCompactionThreshold = defaultCompactionThreshold
		}
		if c.ContextCompactionCooldown <= 0 {
			c.ContextCompactionCooldown = defaultCompactionCooldown
		}
	} else {
		c.ContextCompactionThreshold = 0
		if c.ContextCompactionCooldown < 0 {
			c.ContextCompactionCooldown = 0
		}
	}
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
	attempts := max(c.RetryAttempts, 0)
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
