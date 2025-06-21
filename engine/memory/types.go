package memory

import "time"

// MemoryType defines the type of memory strategy being used.
type MemoryType string

const (
	// TokenBasedMemory indicates memory management primarily driven by token counts.
	TokenBasedMemory MemoryType = "token_based"
	// MessageCountBasedMemory indicates memory management primarily driven by message counts.
	MessageCountBasedMemory MemoryType = "message_count_based"
	// BufferMemory simply stores messages up to a limit without sophisticated eviction.
	BufferMemory MemoryType = "buffer"
)

// FlushingStrategyType defines the type of flushing strategy.
type FlushingStrategyType string

const (
	// HybridSummaryFlushing uses rule-based summarization for older messages.
	HybridSummaryFlushing FlushingStrategyType = "hybrid_summary"
	// SimpleFIFOFlushing evicts the oldest messages without summarization.
	SimpleFIFOFlushing FlushingStrategyType = "simple_fifo"
)

// MemoryResource holds the static configuration for a memory resource,
// typically loaded from a project's configuration files.
type MemoryResource struct {
	ID          string `yaml:"id" json:"id" validate:"required"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Type indicates the primary management strategy (e.g., token_based).
	Type MemoryType `yaml:"type" json:"type" validate:"required,oneof=token_based message_count_based buffer"`

	// MaxTokens is the hard limit on the number of tokens this memory can hold.
	// Used if Type is token_based.
	MaxTokens int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty" validate:"omitempty,gt=0"`
	// MaxMessages is the hard limit on the number of messages.
	// Used if Type is message_count_based or as a secondary limit for token_based.
	MaxMessages int `yaml:"max_messages,omitempty" json:"max_messages,omitempty" validate:"omitempty,gt=0"`
	// MaxContextRatio specifies the maximum portion of an LLM's context window this memory should aim to use.
	// E.g., 0.8 means 80% of the model's context.
	// This might be used to dynamically calculate MaxTokens if not explicitly set.
	MaxContextRatio float64 `yaml:"max_context_ratio,omitempty" json:"max_context_ratio,omitempty" validate:"omitempty,gt=0,lte=1"`

	// TokenAllocation defines how the token budget is distributed if applicable.
	TokenAllocation *TokenAllocation `yaml:"token_allocation,omitempty" json:"token_allocation,omitempty"`
	// FlushingStrategy defines how memory is managed when limits are approached or reached.
	FlushingStrategy *FlushingStrategyConfig `yaml:"flushing_strategy,omitempty" json:"flushing_strategy,omitempty"`

	Persistence PersistenceConfig `yaml:"persistence" json:"persistence" validate:"required"`

	// PrivacyPolicy defines how sensitive data within this memory resource should be handled.
	PrivacyPolicy *PrivacyPolicyConfig `yaml:"privacy_policy,omitempty" json:"privacy_policy,omitempty"`

	// Version can be used for tracking changes to the memory resource definition.
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// TokenAllocation defines percentages for distributing a token budget
// across different categories of memory content.
// All values should sum to 1.0 if used.
type TokenAllocation struct {
	// ShortTerm is the percentage of tokens allocated for recent messages.
	ShortTerm float64 `yaml:"short_term" json:"short_term" validate:"gte=0,lte=1"`
	// LongTerm is the percentage of tokens allocated for summarized or older important context.
	LongTerm float64 `yaml:"long_term" json:"long_term" validate:"gte=0,lte=1"`
	// System is the percentage of tokens reserved for system prompts or critical instructions.
	System float64 `yaml:"system" json:"system" validate:"gte=0,lte=1"`
	// UserDefined is a map for additional custom allocations if needed.
	UserDefined map[string]float64 `yaml:"user_defined,omitempty" json:"user_defined,omitempty"`
}

// FlushingStrategyConfig holds the configuration for how memory is flushed or trimmed.
type FlushingStrategyConfig struct {
	// Type is the kind of flushing strategy to apply (e.g., hybrid_summary).
	Type FlushingStrategyType `yaml:"type" json:"type" validate:"required,oneof=hybrid_summary simple_fifo"`
	// SummarizeThreshold is the percentage of MaxTokens/MaxMessages at which summarization should trigger.
	// E.g., 0.8 means trigger summarization when memory is 80% full. Only for hybrid_summary.
	SummarizeThreshold float64 `yaml:"summarize_threshold,omitempty" json:"summarize_threshold,omitempty" validate:"omitempty,gt=0,lte=1"`
	// SummaryTokens is the target token count for generated summaries. Only for hybrid_summary.
	SummaryTokens int `yaml:"summary_tokens,omitempty" json:"summary_tokens,omitempty" validate:"omitempty,gt=0"`
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
	Type PersistenceType `yaml:"type" json:"type" validate:"required,oneof=redis in_memory"`
	// TTL is the time-to-live for memory instances in this resource.
	// Parsed as a duration string (e.g., "24h", "30m").
	TTL string `yaml:"ttl" json:"ttl" validate:"required"`
	// ParsedTTL is the parsed duration of TTL.
	ParsedTTL time.Duration `yaml:"-" json:"-"`
	// CircuitBreaker configures resilience for persistence operations.
	CircuitBreaker *CircuitBreakerConfig `yaml:"circuit_breaker,omitempty" json:"circuit_breaker,omitempty"`
}

// CircuitBreakerConfig defines parameters for a circuit breaker pattern.
type CircuitBreakerConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Timeout       string `yaml:"timeout" json:"timeout" validate:"omitempty,duration"` // e.g., "100ms"
	MaxFailures   int    `yaml:"max_failures" json:"max_failures" validate:"omitempty,gt=0"`
	ResetTimeout  string `yaml:"reset_timeout" json:"reset_timeout" validate:"omitempty,duration"` // e.g., "30s"
	ParsedTimeout time.Duration `yaml:"-" json:"-"`
	ParsedResetTimeout time.Duration `yaml:"-" json:"-"`
}

// PrivacyPolicyConfig defines rules for handling sensitive data.
type PrivacyPolicyConfig struct {
	// RedactPatterns is a list of regex patterns to apply for redacting content.
	RedactPatterns []string `yaml:"redact_patterns,omitempty" json:"redact_patterns,omitempty"`
	// NonPersistableMessageTypes is a list of message types/roles that should not be persisted.
	NonPersistableMessageTypes []string `yaml:"non_persistable_message_types,omitempty" json:"non_persistable_message_types,omitempty"`
	// DefaultRedactionString is the string to replace redacted content with. Defaults to "[REDACTED]".
	DefaultRedactionString string `yaml:"default_redaction_string,omitempty" json:"default_redaction_string,omitempty"`
}

// MemoryReference is used in Agent configuration to point to a MemoryResource
// and define how the agent interacts with it.
type MemoryReference struct {
	ID   string `yaml:"id" json:"id" validate:"required"`
	// Mode defines access permissions (e.g., "read-write", "read-only").
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty" validate:"omitempty,oneof=read-write read-only"`
	// Key is a template string that resolves to the actual memory instance key.
	// e.g., "support-{{ .workflow.input.conversationId }}"
	Key string `yaml:"key" json:"key" validate:"required"`
	// ResolvedKey is the key after template evaluation.
	ResolvedKey string `yaml:"-" json:"-"`
}

// MessagesWithTokensToLLMMessages converts a slice of MessageWithTokens back to llm.Message.
func MessagesWithTokensToLLMMessages(mwt []MessageWithTokens) []llm.Message {
	if mwt == nil {
		return nil
	}
	msgs := make([]llm.Message, len(mwt))
	for i, msgWithToken := range mwt {
		msgs[i] = msgWithToken.Message
	}
	return msgs
}

// LLMMessagesToMessagesWithTokens is a helper primarily for testing or initial setup,
// as TokenMemoryManager.CalculateMessagesWithTokens is the main way to create MessageWithTokens.
// This version assumes token counts are unknown (set to 0).
func LLMMessagesToMessagesWithTokens(msgs []llm.Message) []MessageWithTokens {
	if msgs == nil {
		return nil
	}
	mwt := make([]MessageWithTokens, len(msgs))
	for i, msg := range msgs {
		mwt[i] = MessageWithTokens{Message: msg, TokenCount: 0} // TokenCount would be calculated later
	}
	return mwt
}

// String returns the string representation of MemoryType.
func (mt MemoryType) String() string {
	return string(mt)
}
