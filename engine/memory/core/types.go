package core

import (
	"fmt"
	"math"
	"time"
)

// Type defines the type of memory strategy being used.
type Type string

const (
	// TokenBasedMemory indicates memory management primarily driven by token counts.
	TokenBasedMemory Type = "token_based"
	// MessageCountBasedMemory indicates memory management primarily driven by message counts.
	MessageCountBasedMemory Type = "message_count_based"
	// BufferMemory simply stores messages up to a limit without sophisticated eviction.
	BufferMemory Type = "buffer"
)

// FlushingStrategyType defines the type of flushing strategy.
type FlushingStrategyType string

const (
	// TokenCountFlushing triggers flushing based on token count limits.
	TokenCountFlushing FlushingStrategyType = "token_count"
	// MessageCountFlushing triggers flushing based on message count limits.
	MessageCountFlushing FlushingStrategyType = "message_count"
	// HybridSummaryFlushing uses rule-based summarization for older messages.
	HybridSummaryFlushing FlushingStrategyType = "hybrid_summary"
	// SimpleFIFOFlushing evicts the oldest messages without summarization.
	SimpleFIFOFlushing FlushingStrategyType = "simple_fifo"
	// TimeBased triggers flushing based on time intervals.
	TimeBased FlushingStrategyType = "time_based"
	// FIFOFlushing is an alias for SimpleFIFOFlushing.
	FIFOFlushing FlushingStrategyType = "fifo"
)

// Resource holds the static configuration for a memory resource,
// typically loaded from a project's configuration files.
type Resource struct {
	ID          string `yaml:"id"                           json:"id"                           validate:"required"`
	Description string `yaml:"description,omitempty"        json:"description,omitempty"`
	// Type indicates the primary management strategy (e.g., token_based).
	Type Type `yaml:"type"                         json:"type"                         validate:"required,oneof=token_based message_count_based buffer"`
	// Model specifies the LLM model name for proper token counting (e.g., "gpt-4-turbo", "claude-3-opus").
	Model string `yaml:"model,omitempty"              json:"model,omitempty"`
	// ModelContextSize is the maximum context window size for the specified model.
	// If not provided, defaults will be used or fetched from model registry.
	ModelContextSize int `yaml:"model_context_size,omitempty" json:"model_context_size,omitempty"`

	// MaxTokens is the hard limit on the number of tokens this memory can hold.
	// Used if Type is token_based.
	MaxTokens int `yaml:"max_tokens,omitempty"        json:"max_tokens,omitempty"        validate:"omitempty,gt=0"`
	// MaxMessages is the hard limit on the number of messages.
	// Used if Type is message_count_based or as a secondary limit for token_based.
	MaxMessages int `yaml:"max_messages,omitempty"      json:"max_messages,omitempty"      validate:"omitempty,gt=0"`
	// MaxContextRatio specifies the maximum portion of an LLM's context window this memory should aim to use.
	// Value between 0 and 1. If set, overrides MaxTokens based on the model's context window.
	// Example: 0.8 means use at most 80% of the model's context window.
	MaxContextRatio float64 `yaml:"max_context_ratio,omitempty" json:"max_context_ratio,omitempty" validate:"omitempty,gt=0,lte=1"`

	// EvictionPolicy defines the strategy for removing messages when limits are reached.
	// Common values: "FIFO" (First In First Out), "Priority" (based on message importance).
	EvictionPolicy string `yaml:"eviction_policy,omitempty" json:"eviction_policy,omitempty" validate:"omitempty,oneof=FIFO Priority"`

	// TokenAllocation defines how the token budget is distributed if applicable.
	TokenAllocation *TokenAllocation `yaml:"token_allocation,omitempty"  json:"token_allocation,omitempty"`
	// FlushingStrategy defines how memory is managed when limits are approached or reached.
	FlushingStrategy *FlushingStrategyConfig `yaml:"flushing_strategy,omitempty" json:"flushing_strategy,omitempty"`

	// Persistence configuration
	Persistence PersistenceConfig `yaml:"persistence" json:"persistence" validate:"required"`

	// TTL configuration
	// AppendTTL extends the TTL by this duration on each append operation.
	// Default is 30 minutes if not specified.
	AppendTTL string `yaml:"append_ttl,omitempty" json:"append_ttl,omitempty"`
	// ClearTTL sets the TTL to this duration when the memory is cleared.
	// Default is 5 minutes if not specified.
	ClearTTL string `yaml:"clear_ttl,omitempty"  json:"clear_ttl,omitempty"`
	// FlushTTL sets the TTL to this duration after a flush operation.
	// Default is 1 hour if not specified.
	FlushTTL string `yaml:"flush_ttl,omitempty"  json:"flush_ttl,omitempty"`

	// PrivacyPolicy defines rules for handling sensitive data in memory.
	PrivacyPolicy *PrivacyPolicyConfig `yaml:"privacy_policy,omitempty" json:"privacy_policy,omitempty"`

	// Advanced configuration
	// TokenCounter specifies a custom token counting implementation.
	// If not set, defaults to the model's standard tokenizer.
	TokenCounter string `yaml:"token_counter,omitempty" json:"token_counter,omitempty"`
	// Metadata allows for custom key-value pairs specific to the application.
	Metadata map[string]any `yaml:"metadata,omitempty"      json:"metadata,omitempty"`

	// DisableFlush completely disables automatic flushing for this resource.
	DisableFlush bool `yaml:"disable_flush,omitempty" json:"disable_flush,omitempty"`

	// Parse the TTL durations
	ParsedAppendTTL time.Duration `yaml:"-" json:"-"`
	ParsedClearTTL  time.Duration `yaml:"-" json:"-"`
	ParsedFlushTTL  time.Duration `yaml:"-" json:"-"`
}

// PrivacyPolicyConfig defines rules for handling sensitive data.
type PrivacyPolicyConfig struct {
	// RedactPatterns is a list of regex patterns to apply for redacting content.
	RedactPatterns []string `yaml:"redact_patterns,omitempty"               json:"redact_patterns,omitempty"`
	// NonPersistableMessageTypes is a list of message types/roles that should not be persisted.
	NonPersistableMessageTypes []string `yaml:"non_persistable_message_types,omitempty" json:"non_persistable_message_types,omitempty"`
	// DefaultRedactionString is the string to replace redacted content with. Defaults to "[REDACTED]".
	DefaultRedactionString string `yaml:"default_redaction_string,omitempty"      json:"default_redaction_string,omitempty"`
}

// String returns the string representation of Type.
func (mt Type) String() string {
	return string(mt)
}

// Constants for persistence types - moved to PersistenceType constants below

// MessageWithTokens represents a message with its token count
type MessageWithTokens struct {
	Message    any // Using interface{} to avoid import cycle with llm package
	TokenCount int
}

// Constants for default values
const (
	// DefaultMaxTokens is the default maximum number of tokens if not specified
	DefaultMaxTokens = 2000
	// DefaultMaxMessages is the default maximum number of messages if not specified
	DefaultMaxMessages = 100
	// DefaultEvictionPolicy is FIFO by default
	DefaultEvictionPolicy = "FIFO"
	// DefaultFlushingStrategy is simple FIFO by default
	DefaultFlushingStrategy = SimpleFIFOFlushing
)

// Validate validates the Resource configuration
func (r *Resource) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("resource ID is required")
	}
	if r.Type == "" {
		return fmt.Errorf("resource type is required")
	}
	switch r.Type {
	case TokenBasedMemory:
		if r.MaxTokens == 0 && r.MaxContextRatio == 0 {
			return fmt.Errorf("token-based memory requires either max_tokens or max_context_ratio")
		}
	case MessageCountBasedMemory:
		if r.MaxMessages == 0 {
			return fmt.Errorf("message-count-based memory requires max_messages")
		}
	case BufferMemory:
		// Buffer memory can work with either limit
	default:
		return fmt.Errorf("invalid memory type: %s", r.Type)
	}
	if r.MaxContextRatio > 1 {
		return fmt.Errorf("max_context_ratio must be between 0 and 1")
	}
	return nil
}

// GetEffectiveMaxTokens calculates the effective max tokens based on configuration
func (r *Resource) GetEffectiveMaxTokens() int {
	if r.MaxTokens > 0 {
		return r.MaxTokens
	}
	if r.MaxContextRatio > 0 && r.ModelContextSize > 0 {
		return int(math.Floor(float64(r.ModelContextSize) * r.MaxContextRatio))
	}
	return DefaultMaxTokens
}

// GetEffectiveMaxMessages returns the effective max messages limit
func (r *Resource) GetEffectiveMaxMessages() int {
	if r.MaxMessages > 0 {
		return r.MaxMessages
	}
	return DefaultMaxMessages
}

// TokenAllocation defines percentages for distributing a token budget
// across different categories of memory content.
// All values should sum to 1.0 if used.
type TokenAllocation struct {
	// ShortTerm is the percentage of tokens allocated for recent messages.
	ShortTerm float64 `yaml:"short_term"             json:"short_term"             validate:"gte=0,lte=1"`
	// LongTerm is the percentage of tokens allocated for summarized or older important context.
	LongTerm float64 `yaml:"long_term"              json:"long_term"              validate:"gte=0,lte=1"`
	// System is the percentage of tokens reserved for system prompts or critical instructions.
	System float64 `yaml:"system"                 json:"system"                 validate:"gte=0,lte=1"`
	// UserDefined is a map for additional custom allocations if needed.
	UserDefined map[string]float64 `yaml:"user_defined,omitempty" json:"user_defined,omitempty"`
}

// Validate ensures all token allocation percentages sum to 1.0
func (ta *TokenAllocation) Validate() error {
	sum := ta.ShortTerm + ta.LongTerm + ta.System
	for _, v := range ta.UserDefined {
		sum += v
	}
	if math.Abs(sum-1.0) > 0.001 { // Allow small floating point errors
		return fmt.Errorf("token allocation percentages must sum to 1.0, got %f", sum)
	}
	return nil
}

// FlushingStrategyConfig holds the configuration for how memory is flushed or trimmed.
type FlushingStrategyConfig struct {
	// Type is the kind of flushing strategy to apply (e.g., hybrid_summary).
	Type FlushingStrategyType `yaml:"type"                               json:"type"                               validate:"required,oneof=hybrid_summary simple_fifo"`
	// SummarizeThreshold is the percentage of MaxTokens/MaxMessages at which summarization should trigger.
	// E.g., 0.8 means trigger summarization when memory is 80% full. Only for hybrid_summary.
	SummarizeThreshold float64 `yaml:"summarize_threshold,omitempty"      json:"summarize_threshold,omitempty"      validate:"omitempty,gt=0,lte=1"`
	// SummaryTokens is the target token count for generated summaries. Only for hybrid_summary.
	SummaryTokens int `yaml:"summary_tokens,omitempty"           json:"summary_tokens,omitempty"           validate:"omitempty,gt=0"`
	// SummarizeOldestPercent is the percentage of the oldest messages to summarize. Only for hybrid_summary.
	// E.g., 0.3 means summarize the oldest 30% of messages.
	SummarizeOldestPercent float64 `yaml:"summarize_oldest_percent,omitempty" json:"summarize_oldest_percent,omitempty" validate:"omitempty,gt=0,lte=1"`
}

// PersistenceType defines the backend used for storing memory.
type PersistenceType string

const (
	// RedisPersistence uses Redis as the backend.
	RedisPersistence PersistenceType = "redis"
	// InMemoryPersistence uses in-memory storage (mainly for testing or ephemeral tasks).
	InMemoryPersistence PersistenceType = "in_memory"
)

// PersistenceConfig defines how memory instances are persisted.
type PersistenceConfig struct {
	Type PersistenceType `yaml:"type"                      json:"type"                      validate:"required,oneof=redis in_memory"`
	// TTL is the time-to-live for memory instances in this resource.
	// Parsed as a duration string (e.g., "24h", "30m").
	TTL string `yaml:"ttl"                       json:"ttl"                       validate:"required"`
	// ParsedTTL is the parsed duration of TTL.
	ParsedTTL time.Duration `yaml:"-"                         json:"-"`
	// CircuitBreaker configures resilience for persistence operations.
	CircuitBreaker *CircuitBreakerConfig `yaml:"circuit_breaker,omitempty" json:"circuit_breaker,omitempty"`
}

// CircuitBreakerConfig defines parameters for a circuit breaker pattern.
type CircuitBreakerConfig struct {
	Enabled            bool          `yaml:"enabled"       json:"enabled"`
	Timeout            string        `yaml:"timeout"       json:"timeout"       validate:"omitempty,duration"` // e.g., "100ms"
	MaxFailures        int           `yaml:"max_failures"  json:"max_failures"  validate:"omitempty,gt=0"`
	ResetTimeout       string        `yaml:"reset_timeout" json:"reset_timeout" validate:"omitempty,duration"` // e.g., "30s"
	ParsedTimeout      time.Duration `yaml:"-"             json:"-"`
	ParsedResetTimeout time.Duration `yaml:"-"             json:"-"`
}

// LockConfig defines lock timeout settings for memory operations.
type LockConfig struct {
	// AppendTTL is the lock timeout for append operations (default: "30s")
	AppendTTL string `yaml:"append_ttl,omitempty" json:"append_ttl,omitempty" validate:"omitempty,duration"`
	// ClearTTL is the lock timeout for clear operations (default: "10s")
	ClearTTL string `yaml:"clear_ttl,omitempty"  json:"clear_ttl,omitempty"  validate:"omitempty,duration"`
	// FlushTTL is the lock timeout for flush operations (default: "5m")
	FlushTTL string `yaml:"flush_ttl,omitempty"  json:"flush_ttl,omitempty"  validate:"omitempty,duration"`
	// Parsed durations for internal use
	ParsedAppendTTL time.Duration `yaml:"-"                    json:"-"`
	ParsedClearTTL  time.Duration `yaml:"-"                    json:"-"`
	ParsedFlushTTL  time.Duration `yaml:"-"                    json:"-"`
}

// String returns the string representation of the FlushingStrategyType.
func (f FlushingStrategyType) String() string {
	switch f {
	case TokenCountFlushing:
		return "token_count"
	case MessageCountFlushing:
		return "message_count"
	case HybridSummaryFlushing:
		return "hybrid_summary"
	case TimeBased:
		return "time_based"
	case FIFOFlushing:
		return "fifo"
	case SimpleFIFOFlushing:
		return "simple_fifo"
	default:
		return "unknown"
	}
}

// IsValid returns true if the FlushingStrategyType is a valid strategy.
func (f FlushingStrategyType) IsValid() bool {
	switch f {
	case TokenCountFlushing, MessageCountFlushing, HybridSummaryFlushing, TimeBased, FIFOFlushing, SimpleFIFOFlushing:
		return true
	default:
		return false
	}
}
