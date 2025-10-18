package orchestrator

import (
	"strings"
	"time"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
)

const (
	defaultMaxConcurrentTools      = 10
	defaultRetryAttempts           = 3
	defaultRetryBackoffBase        = 100 * time.Millisecond
	defaultRetryBackoffMax         = 10 * time.Second
	defaultStructuredOutputRetries = 2
	defaultMaxToolIterations       = 10
	defaultMaxSequentialToolErrors = 8
	defaultMaxConsecutiveSuccesses = 3
	defaultNoProgressThreshold     = 3
	defaultMaxLoopRestarts         = 1
	defaultRestartAfterStall       = 2
	defaultCompactionThreshold     = 0.85
	defaultCompactionCooldown      = 2
)

type settings struct {
	timeout                        time.Duration
	maxConcurrentTools             int
	maxToolIterations              int
	maxSequentialToolErrors        int
	finalizeOutputRetries          int
	retryAttempts                  int
	retryBackoffBase               time.Duration
	retryBackoffMax                time.Duration
	retryJitter                    bool
	maxConsecutiveSuccesses        int
	enableProgressTracking         bool
	noProgressThreshold            int
	enableLoopRestarts             bool
	restartAfterStall              int
	maxLoopRestarts                int
	enableContextCompaction        bool
	compactionThreshold            float64
	compactionCooldown             int
	enableAgentCallCompletionHints bool
	enableDynamicPromptState       bool
	projectRoot                    string
	toolCaps                       toolCallCaps
	middlewares                    []Middleware
	restartAfterStallRequested     int
	restartThresholdClamped        bool
	rateLimiter                    *llmadapter.RateLimiterRegistry
	providerMetrics                providermetrics.Recorder
}

func buildSettings(cfg *Config) settings {
	if cfg == nil {
		cfg = &Config{}
	}
	finalizeRetries := cfg.FinalizeOutputRetryAttempts
	if finalizeRetries <= 0 {
		finalizeRetries = cfg.StructuredOutputRetryAttempts
	}
	s := settings{
		timeout:                        cfg.Timeout,
		maxConcurrentTools:             cfg.MaxConcurrentTools,
		maxToolIterations:              cfg.MaxToolIterations,
		maxSequentialToolErrors:        cfg.MaxSequentialToolErrors,
		finalizeOutputRetries:          finalizeRetries,
		retryAttempts:                  cfg.RetryAttempts,
		retryBackoffBase:               cfg.RetryBackoffBase,
		retryBackoffMax:                cfg.RetryBackoffMax,
		retryJitter:                    cfg.RetryJitter,
		maxConsecutiveSuccesses:        cfg.MaxConsecutiveSuccesses,
		enableProgressTracking:         cfg.EnableProgressTracking,
		noProgressThreshold:            cfg.NoProgressThreshold,
		enableLoopRestarts:             cfg.EnableLoopRestarts,
		restartAfterStall:              cfg.RestartStallThreshold,
		maxLoopRestarts:                cfg.MaxLoopRestarts,
		enableContextCompaction:        cfg.EnableContextCompaction,
		compactionThreshold:            cfg.ContextCompactionThreshold,
		compactionCooldown:             cfg.ContextCompactionCooldown,
		enableAgentCallCompletionHints: cfg.EnableAgentCallCompletionHints,
		enableDynamicPromptState:       cfg.EnableDynamicPromptState,
		projectRoot:                    cfg.ProjectRoot,
		toolCaps:                       newToolCallCaps(cfg.ToolCallCaps),
		middlewares:                    cloneMiddlewares(cfg.Middlewares),
		restartAfterStallRequested:     cfg.RestartStallThreshold,
		rateLimiter:                    cfg.RateLimiter,
		providerMetrics:                cfg.ProviderMetrics,
	}

	normalizeNumericDefaults(cfg, &s)
	if s.providerMetrics == nil {
		s.providerMetrics = providermetrics.Nop()
	}
	return s
}

func normalizeNumericDefaults(cfg *Config, s *settings) {
	s.maxConcurrentTools = defaultInt(s.maxConcurrentTools, defaultMaxConcurrentTools)
	s.maxToolIterations = defaultInt(s.maxToolIterations, defaultMaxToolIterations)
	s.maxSequentialToolErrors = defaultInt(s.maxSequentialToolErrors, defaultMaxSequentialToolErrors)
	s.finalizeOutputRetries = defaultInt(s.finalizeOutputRetries, defaultStructuredOutputRetries)
	s.retryAttempts = defaultInt(s.retryAttempts, defaultRetryAttempts)
	s.retryBackoffBase = defaultDuration(s.retryBackoffBase, defaultRetryBackoffBase)
	s.retryBackoffMax = defaultDuration(s.retryBackoffMax, defaultRetryBackoffMax)
	s.maxConsecutiveSuccesses = defaultInt(s.maxConsecutiveSuccesses, defaultMaxConsecutiveSuccesses)
	s.noProgressThreshold = defaultInt(s.noProgressThreshold, defaultNoProgressThreshold)
	s.restartAfterStall = defaultInt(s.restartAfterStall, defaultRestartAfterStall)
	if s.restartAfterStall > s.noProgressThreshold {
		s.restartAfterStall = s.noProgressThreshold
		s.restartThresholdClamped = true
	}
	s.maxLoopRestarts = normalizeMaxRestarts(cfg, s.maxLoopRestarts)
	if !s.enableContextCompaction {
		s.compactionThreshold = 0
		s.compactionCooldown = 0
	} else {
		s.compactionThreshold = normalizeThreshold(s.compactionThreshold, defaultCompactionThreshold)
		s.compactionCooldown = defaultInt(s.compactionCooldown, defaultCompactionCooldown)
	}
}

func normalizeMaxRestarts(cfg *Config, current int) int {
	if current < 0 {
		return 0
	}
	if current == 0 && cfg.EnableLoopRestarts {
		return defaultMaxLoopRestarts
	}
	return current
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func normalizeThreshold(value, fallback float64) float64 {
	if value <= 0 {
		value = fallback
	}
	if value > 1 {
		value = 1
	}
	return value
}

type toolCallCaps struct {
	defaultLimit int
	overrides    map[string]int
}

func newToolCallCaps(cfg ToolCallCaps) toolCallCaps {
	caps := toolCallCaps{
		defaultLimit: cfg.Default,
	}
	if len(cfg.Overrides) == 0 {
		return caps
	}
	caps.overrides = make(map[string]int, len(cfg.Overrides))
	for name, limit := range cfg.Overrides {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		caps.overrides[key] = limit
	}
	return caps
}

func (c toolCallCaps) limitFor(name string) int {
	if name == "" {
		return c.defaultLimit
	}
	if c.overrides != nil {
		if limit, ok := c.overrides[strings.ToLower(strings.TrimSpace(name))]; ok {
			return limit
		}
	}
	return c.defaultLimit
}

func cloneMiddlewares(input []Middleware) []Middleware {
	if len(input) == 0 {
		return nil
	}
	out := make([]Middleware, 0, len(input))
	for _, m := range input {
		if m == nil {
			continue
		}
		out = append(out, m)
	}
	return out
}
