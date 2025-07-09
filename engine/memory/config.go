package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
)

// Config defines the structure for a memory resource configuration.
//
// **Memory System Overview**
//
// The memory system in Compozy provides **stateful context management** for AI agents,
// enabling them to **retain, share, and reference information** across multiple interactions
// within workflows. This allows agents to collaborate effectively by maintaining shared
// knowledge and conversation history.
//
// **Key Features:**
// - **🧠 Persistent Context**: Maintains conversation history and state across agent interactions
// - **🤝 Agent Collaboration**: Multiple agents can read/write to shared memory spaces
// - **🔐 Access Control**: Fine-grained permissions (read-only, read-write) per agent
// - **⚡ Smart Eviction**: Intelligent memory management with token-aware strategies
// - **🔄 Distributed Locking**: Safe concurrent access in multi-agent scenarios
// - **📊 Token Optimization**: Efficient use of LLM context windows
//
// **Memory Namespacing**
//
// Memory instances are created dynamically using templated keys that provide namespace isolation:
// ```yaml
// # In agent configuration:
// memory:
//   - id: conversation_memory
//     key: "chat:{{.workflow.input.user_id}}:{{.workflow.input.session_id}}"
//     mode: read-write
//
// ```
//
// This creates isolated memory spaces per user/session combination, enabling:
// - **User Isolation**: Each user gets their own memory space
// - **Session Management**: Separate contexts for different conversations
// - **Multi-tenancy**: Safe operation across multiple users/organizations
//
// **Agent Collaboration Example**
//
// Multiple agents can share memory to collaborate on complex tasks:
// ```yaml
// # Research agent writes findings to shared memory
// agents:
//
//	researcher:
//	  memory:
//	    - id: project_memory
//	      key: "project:{{.workflow.input.project_id}}"
//	      mode: read-write
//
//	# Analyzer reads research and writes analysis
//	analyzer:
//	  memory:
//	    - id: project_memory
//	      key: "project:{{.workflow.input.project_id}}"
//	      mode: read-write
//
//	# Reporter only reads to generate reports
//	reporter:
//	  memory:
//	    - id: project_memory
//	      key: "project:{{.workflow.input.project_id}}"
//	      mode: read-only
//
// ```
//
// **Complete Configuration Example**
//
// ```yaml
// resource: memory
// id: conversation_memory
// description: Stores conversation history with intelligent summarization
// type: token_based
// max_tokens: 4000
// max_context_ratio: 0.5  # Use max 50% of model's context
//
// # Token budget distribution
// token_allocation:
//
//	short_term: 0.6   # 60% for recent messages
//	long_term: 0.3    # 30% for summaries
//	system: 0.1       # 10% for system context
//
// # Smart memory management
// flushing:
//
//	type: hybrid_summary
//	summarize_threshold: 0.8
//	summary_tokens: 500
//
// # Redis persistence with 24-hour TTL
// persistence:
//
//	type: redis
//	ttl: 24h
//
// # Distributed locking for safety
// locking:
//
//	append_ttl: 30s
//	clear_ttl: 10s
//	flush_ttl: 5m
//
// # Privacy compliance
// privacy_policy:
//
//	redact_patterns: ["\\b\\d{3}-\\d{2}-\\d{4}\\b"]  # SSN
//	non_persistable_message_types: ["payment_info"]
//
// ```
type Config struct {
	// Resource type identifier, **must be "memory"**.
	// This field is used by the autoloader system to identify and properly
	// register this configuration as a memory resource.
	Resource string `json:"resource"                    yaml:"resource"                    mapstructure:"resource"                    validate:"required,eq=memory"`
	// ID is the **unique identifier** for this memory resource within the project.
	// This ID is used by agents to reference the memory in their configuration.
	// - **Examples**: `"user_conversation"`, `"session_context"`, `"agent_workspace"`
	ID string `json:"id"                          yaml:"id"                          mapstructure:"id"                          validate:"required"`
	// Description provides a **human-readable explanation** of the memory resource's purpose.
	// This helps developers understand what kind of data this memory stores and
	// how it should be used within workflows.
	Description string `json:"description,omitempty"       yaml:"description,omitempty"       mapstructure:"description,omitempty"`
	// Version allows **tracking changes** to the memory resource definition.
	// Can be used for migration strategies when memory schema evolves.
	// **Format**: semantic versioning (e.g., `"1.0.0"`, `"2.1.0-beta"`)
	Version string `json:"version,omitempty"           yaml:"version,omitempty"           mapstructure:"version,omitempty"`
	// Type indicates the **primary memory management strategy**:
	// - **`"token_based"`**: Manages memory based on token count limits (recommended for LLM contexts)
	// - **`"message_count_based"`**: Manages memory based on message count limits
	// - **`"buffer"`**: Simple buffer that stores messages up to a limit without sophisticated eviction
	Type memcore.Type `json:"type"                        yaml:"type"                        mapstructure:"type"                        validate:"required,oneof=token_based message_count_based buffer"`
	// MaxTokens is the **hard limit** on the number of tokens this memory can hold.
	// Only applicable when Type is `"token_based"`. When this limit is reached,
	// the flushing strategy determines how to make room for new content.
	//
	// - **Example**: `4000` (roughly equivalent to ~3000 words)
	MaxTokens int `json:"max_tokens,omitempty"        yaml:"max_tokens,omitempty"        mapstructure:"max_tokens,omitempty"        validate:"omitempty,gt=0"`
	// MaxMessages is the **hard limit** on the number of messages this memory can store.
	// Applicable for `"message_count_based"` type or as a secondary limit for `"token_based"`.
	//
	// - **Example**: `100` (keeps last 100 messages in conversation)
	MaxMessages int `json:"max_messages,omitempty"      yaml:"max_messages,omitempty"      mapstructure:"max_messages,omitempty"      validate:"omitempty,gt=0"`
	// MaxContextRatio specifies the **maximum portion** of an LLM's context window this memory should use.
	// Value between 0 and 1. Dynamically calculates MaxTokens based on the model's context window.
	//
	// - **Example**: `0.5` means use at most 50% of the model's context window for memory,
	// leaving the rest for system prompts and current task context.
	MaxContextRatio float64 `json:"max_context_ratio,omitempty" yaml:"max_context_ratio,omitempty" mapstructure:"max_context_ratio,omitempty" validate:"omitempty,gt=0,lte=1"`

	// TokenAllocation defines how the **token budget is distributed** across different categories.
	// Only applicable for `token_based` memory type. All percentages **must sum to 1.0**.
	// ```yaml
	// token_allocation:
	//   short_term: 0.6  # 60% for recent messages
	//   long_term: 0.3   # 30% for summarized context
	//   system: 0.1      # 10% for system prompts
	// ```
	TokenAllocation *memcore.TokenAllocation `json:"token_allocation,omitempty" yaml:"token_allocation,omitempty" mapstructure:"token_allocation,omitempty"`
	// Flushing defines **how memory is managed** when limits are approached or reached.
	// **Available strategies**:
	// - **`"simple_fifo"`**: Removes oldest messages first (fastest, no LLM required)
	// - **`"lru"`**: Removes least recently used messages (tracks access patterns)
	// - **`"hybrid_summary"`**: Summarizes old messages before removal (requires LLM, preserves context)
	// - **`"token_aware_lru"`**: LRU that considers token cost of messages (optimizes token usage)
	Flushing *memcore.FlushingStrategyConfig `json:"flushing,omitempty"         yaml:"flushing,omitempty"         mapstructure:"flushing,omitempty"` // Renamed from FlushingStrategy in PRD to avoid conflict with the struct type

	// Persistence defines **how memory instances are persisted** beyond process lifetime.
	// **Required field** that specifies storage backend and retention policy.
	// **Supported backends**:
	// - **`"redis"`**: Production-grade persistence with distributed locking and TTL support
	// - **`"in_memory"`**: Testing/development only, data lost on restart
	Persistence memcore.PersistenceConfig `json:"persistence" yaml:"persistence" mapstructure:"persistence" validate:"required"`

	// PrivacyPolicy defines **rules for handling sensitive data** within this memory.
	// Can specify redaction patterns, non-persistable message types, and
	// custom redaction strings for **compliance with data protection regulations**.
	// ```yaml
	// privacy_policy:
	//   redact_patterns: ["\\b\\d{3}-\\d{2}-\\d{4}\\b"]  # SSN pattern
	//   non_persistable_message_types: ["payment_info"]
	//   default_redaction_string: "[REDACTED]"
	// ```
	PrivacyPolicy *memcore.PrivacyPolicyConfig `json:"privacy_policy,omitempty" yaml:"privacy_policy,omitempty" mapstructure:"privacy_policy,omitempty"`

	// Locking configures **distributed lock timeouts** for concurrent memory operations.
	// **Critical for preventing race conditions** when multiple agents access the same memory.
	// Timeouts can be configured per operation type:
	// - **`append_ttl`**: Timeout for adding new messages (default: `30s`)
	// - **`clear_ttl`**: Timeout for clearing memory (default: `10s`)
	// - **`flush_ttl`**: Timeout for flush operations (default: `5m`)
	Locking *memcore.LockConfig `json:"locking,omitempty" yaml:"locking,omitempty" mapstructure:"locking,omitempty"`

	// TokenProvider configures **provider-specific token counting** for accurate limits.
	// Supports OpenAI, Anthropic, and other providers with their specific tokenizers.
	// Can specify API keys for **real-time token counting** or fallback strategies.
	TokenProvider *memcore.TokenProviderConfig `json:"token_provider,omitempty" yaml:"token_provider,omitempty" mapstructure:"token_provider,omitempty"`

	// --- Internal fields for framework compatibility ---
	// filePath stores the source configuration file path for debugging
	filePath string `json:"-" yaml:"-"`
	// CWD represents the current working directory context for path resolution
	CWD *core.PathCWD `json:"-" yaml:"-"`
	// ttlManager provides centralized TTL management for lock operations
	ttlManager *TTLManager `json:"-" yaml:"-"`
	// ttlManagerOnce ensures thread-safe initialization of ttlManager
	ttlManagerOnce sync.Once `json:"-" yaml:"-"`
}

// --- Implementation for core.Configurable pattern ---

// GetResource returns the resource type string.
// Task 4.0 description asks for this method.
func (c *Config) GetResource() string {
	if c.Resource == "" {
		return string(core.ConfigMemory) // Default if not set, but validation should catch it.
	}
	return c.Resource
}

// GetID returns the unique ID of this memory resource.
// Task 4.0 description asks for this method.
func (c *Config) GetID() string {
	return c.ID
}

// --- Implementation to satisfy parts of core.Config interface for loading ---
// This makes it compatible with core.LoadConfig and registry systems.

// Component returns the configuration type.
func (c *Config) Component() core.ConfigType {
	return core.ConfigMemory
}

// SetFilePath sets the file path of the loaded configuration.
func (c *Config) SetFilePath(path string) {
	c.filePath = path
}

// GetFilePath returns the file path.
func (c *Config) GetFilePath() string {
	return c.filePath
}

// SetCWD sets the current working directory.
func (c *Config) SetCWD(path string) error {
	cwd, err := core.CWDFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to set CWD for memory config %s: %w", c.ID, err)
	}
	c.CWD = cwd
	return nil
}

// GetCWD returns the current working directory.
func (c *Config) GetCWD() *core.PathCWD {
	return c.CWD
}

// Validate performs validation on the memory resource configuration.
// This will be called by the autoload registry after loading.
func (c *Config) Validate() error {
	// Basic struct validation using tags will be done by schema.NewStructValidator(c) in the loader.
	// Here, we add more complex or cross-field validation.
	if err := c.validateResource(); err != nil {
		return err
	}
	if err := c.validatePersistence(); err != nil {
		return err
	}
	if err := c.validateTokenAllocation(); err != nil {
		return err
	}
	if err := c.validateFlushing(); err != nil {
		return err
	}
	if err := c.validateLocking(); err != nil {
		return err
	}
	return c.validateTokenBased()
}

func (c *Config) validateResource() error {
	if c.Resource != string(core.ConfigMemory) {
		return fmt.Errorf(
			"memory config ID '%s': resource field must be '%s', got '%s'",
			c.ID,
			core.ConfigMemory,
			c.Resource,
		)
	}
	return nil
}

func (c *Config) validatePersistence() error {
	if c.Persistence.TTL != "" {
		parsedTTL, err := time.ParseDuration(c.Persistence.TTL)
		if err != nil {
			return fmt.Errorf(
				"memory config ID '%s': invalid persistence.ttl duration format '%s': %w",
				c.ID,
				c.Persistence.TTL,
				err,
			)
		}
		// TTL "0" means no expiration, which is valid
		if parsedTTL < 0 && c.Persistence.Type != memcore.InMemoryPersistence {
			return fmt.Errorf(
				"memory config ID '%s': persistence.ttl must be non-negative, got '%s'",
				c.ID,
				c.Persistence.TTL,
			)
		}
		// Store the parsed TTL for runtime use
		c.Persistence.ParsedTTL = parsedTTL
	} else if c.Persistence.Type != memcore.InMemoryPersistence {
		return fmt.Errorf(
			"memory config ID '%s': persistence.ttl is required for persistence type '%s'",
			c.ID, c.Persistence.Type)
	}
	return nil
}

func (c *Config) validateTokenAllocation() error {
	if c.TokenAllocation == nil {
		return nil
	}
	sum := c.TokenAllocation.ShortTerm + c.TokenAllocation.LongTerm + c.TokenAllocation.System
	for _, v := range c.TokenAllocation.UserDefined {
		sum += v
	}
	const tolerance = 0.001
	if sum < 1.0-tolerance || sum > 1.0+tolerance {
		return fmt.Errorf(
			"memory config ID '%s': token allocation sum (%.3f) must be approximately 1.0",
			c.ID, sum,
		)
	}
	return nil
}

func (c *Config) validateFlushing() error {
	if c.Flushing == nil || c.Flushing.Type != memcore.HybridSummaryFlushing {
		return nil
	}
	if c.Flushing.SummarizeThreshold <= 0 || c.Flushing.SummarizeThreshold > 1 {
		return fmt.Errorf(
			"memory config ID '%s': flushing.summarize_threshold (%.2f) must be > 0 and <= 1",
			c.ID,
			c.Flushing.SummarizeThreshold,
		)
	}
	if c.Flushing.SummaryTokens <= 0 {
		return fmt.Errorf(
			"memory config ID '%s': flushing.summary_tokens (%d) must be > 0",
			c.ID,
			c.Flushing.SummaryTokens,
		)
	}
	return nil
}

func (c *Config) validateLocking() error {
	if c.Locking == nil {
		return nil
	}
	if c.Locking.AppendTTL != "" {
		if _, err := time.ParseDuration(c.Locking.AppendTTL); err != nil {
			return fmt.Errorf(
				"memory config ID '%s': invalid locking.append_ttl duration format '%s': %w",
				c.ID,
				c.Locking.AppendTTL,
				err,
			)
		}
	}
	if c.Locking.ClearTTL != "" {
		if _, err := time.ParseDuration(c.Locking.ClearTTL); err != nil {
			return fmt.Errorf(
				"memory config ID '%s': invalid locking.clear_ttl duration format '%s': %w",
				c.ID,
				c.Locking.ClearTTL,
				err,
			)
		}
	}
	if c.Locking.FlushTTL != "" {
		if _, err := time.ParseDuration(c.Locking.FlushTTL); err != nil {
			return fmt.Errorf(
				"memory config ID '%s': invalid locking.flush_ttl duration format '%s': %w",
				c.ID,
				c.Locking.FlushTTL,
				err,
			)
		}
	}
	return nil
}

func (c *Config) validateTokenBased() error {
	if c.Type == memcore.TokenBasedMemory {
		if c.MaxTokens <= 0 && c.MaxContextRatio <= 0 && c.MaxMessages <= 0 {
			return fmt.Errorf(
				"memory config ID '%s': token_based memory must have at least one limit configured "+
					"(max_tokens, max_context_ratio, or max_messages)",
				c.ID,
			)
		}
	}
	return nil
}

// TTLManager centralizes **TTL (Time-To-Live) management** for memory operations.
// It provides consistent timeout values for distributed lock operations,
// ensuring that locks are **automatically released** if operations fail or timeout.
// This prevents deadlocks and ensures **system resilience** in distributed environments.
//
// **Why TTL Management Matters**:
// - **Prevents Deadlocks**: Ensures locks don't persist indefinitely
// - **Handles Failures Gracefully**: Auto-releases locks if agents crash
// - **Enables Concurrent Access**: Multiple agents can safely share memory
// - **Optimizes Performance**: Different operations get appropriate timeouts
type TTLManager struct {
	// appendTTL defines how long append operations can hold a lock
	appendTTL time.Duration
	// clearTTL defines how long clear operations can hold a lock
	clearTTL time.Duration
	// flushTTL defines how long flush operations can hold a lock
	flushTTL time.Duration
}

// NewTTLManager creates a TTL manager from lock configuration.
// If no configuration is provided, **sensible defaults** are used:
// - **Append operations**: `30 seconds` (quick operation, should complete fast)
// - **Clear operations**: `10 seconds` (even quicker, just clearing data)
// - **Flush operations**: `5 minutes` (may involve summarization with LLM calls)
func NewTTLManager(lockConfig *memcore.LockConfig, memoryID string) *TTLManager {
	tm := &TTLManager{
		// Default TTLs
		appendTTL: 30 * time.Second,
		clearTTL:  10 * time.Second,
		flushTTL:  5 * time.Minute,
	}
	if lockConfig == nil {
		return tm
	}
	// Parse configured TTLs with error logging
	log := logger.FromContext(context.Background())
	tm.appendTTL = parseTTLWithDefault(lockConfig.AppendTTL, tm.appendTTL, "append", memoryID, log)
	tm.clearTTL = parseTTLWithDefault(lockConfig.ClearTTL, tm.clearTTL, "clear", memoryID, log)
	tm.flushTTL = parseTTLWithDefault(lockConfig.FlushTTL, tm.flushTTL, "flush", memoryID, log)
	return tm
}

// parseTTLWithDefault parses a TTL string with **fallback and error logging**.
// This helper function ensures **robust TTL parsing** with graceful degradation.
// If the TTL string is empty or invalid, it falls back to the provided default
// and logs a warning for debugging purposes.
//
// **Parameters**:
//   - `ttlStr`: Duration string to parse (e.g., `"30s"`, `"5m"`, `"1h"`)
//   - `defaultTTL`: Fallback duration if parsing fails
//   - `operation`: Operation name for logging context (e.g., `"append"`, `"clear"`, `"flush"`)
//   - `memoryID`: Memory resource ID for logging context
//   - `log`: Logger instance for error reporting
func parseTTLWithDefault(
	ttlStr string,
	defaultTTL time.Duration,
	operation, memoryID string,
	log logger.Logger,
) time.Duration {
	if ttlStr == "" {
		return defaultTTL
	}
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		log.Error("Failed to parse lock TTL",
			"memory_id", memoryID,
			"operation", operation,
			"ttl_string", ttlStr,
			"error", err)
		return defaultTTL
	}
	return ttl
}

// GetAppendTTL returns the TTL for **append operations**.
// This timeout is used when acquiring distributed locks for adding new messages to memory.
// **Default**: `30 seconds` if not configured.
func (tm *TTLManager) GetAppendTTL() time.Duration {
	return tm.appendTTL
}

// GetClearTTL returns the TTL for **clear operations**.
// This timeout is used when acquiring distributed locks for clearing memory contents.
// Clear operations should be fast, so this has a **shorter default timeout**.
// **Default**: `10 seconds` if not configured.
func (tm *TTLManager) GetClearTTL() time.Duration {
	return tm.clearTTL
}

// GetFlushTTL returns the TTL for **flush operations**.
// This timeout is used when acquiring distributed locks for flush operations,
// which may involve **LLM-based summarization** and can take longer.
// **Default**: `5 minutes` if not configured.
func (tm *TTLManager) GetFlushTTL() time.Duration {
	return tm.flushTTL
}

// initTTLManager lazily initializes the TTLManager instance once.
// Uses sync.Once to ensure thread-safe initialization in concurrent environments.
func (c *Config) initTTLManager() {
	c.ttlManagerOnce.Do(func() {
		c.ttlManager = NewTTLManager(c.Locking, c.ID)
	})
}

// GetAppendLockTTL returns the lock TTL for **append operations** with a default fallback.
// This method is **thread-safe** and lazily initializes the TTL manager on first use.
// The returned duration should be used as the timeout when acquiring distributed
// locks for append operations to **prevent indefinite lock holding**.
func (c *Config) GetAppendLockTTL() time.Duration {
	c.initTTLManager()
	return c.ttlManager.GetAppendTTL()
}

// GetClearLockTTL returns the lock TTL for **clear operations** with a default fallback.
// This method is **thread-safe** and lazily initializes the TTL manager on first use.
// Clear operations typically complete quickly, so this returns a **shorter timeout**
// to ensure locks are released promptly if operations fail.
func (c *Config) GetClearLockTTL() time.Duration {
	c.initTTLManager()
	return c.ttlManager.GetClearTTL()
}

// GetFlushLockTTL returns the lock TTL for **flush operations** with a default fallback.
// This method is **thread-safe** and lazily initializes the TTL manager on first use.
// Flush operations may involve **LLM summarization** and can take significantly longer,
// so this returns a more **generous timeout** to accommodate complex operations.
func (c *Config) GetFlushLockTTL() time.Duration {
	c.initTTLManager()
	return c.ttlManager.GetFlushTTL()
}

// --- Methods below are part of core.Config but might not be fully relevant for a simple resource definition ---
// Implement them minimally if core.LoadConfig or registry expects them.

func (c *Config) GetEnv() core.EnvMap {
	// Memory resources typically don't have their own env vars in this way.
	return core.EnvMap{}
}

func (c *Config) GetInput() *core.Input {
	// Memory resources don't take dynamic inputs like workflows/tasks.
	return &core.Input{}
}

func (c *Config) ValidateInput(_ context.Context, _ *core.Input) error {
	return nil // No input schema to validate against
}

func (c *Config) ValidateOutput(_ context.Context, _ *core.Output) error {
	return nil // No output schema
}

func (c *Config) HasSchema() bool {
	return false // No input/output JSON schema
}

// copyConfigFields copies all fields except sync-related ones.
// This helper method is used during FromMap operations to ensure that
// **sync.Once and other synchronization primitives are not copied**,
// which would break their **thread-safety guarantees**.
func (c *Config) copyConfigFields(from *Config) {
	c.Resource = from.Resource
	c.ID = from.ID
	c.Description = from.Description
	c.Version = from.Version
	c.Type = from.Type
	c.MaxTokens = from.MaxTokens
	c.MaxMessages = from.MaxMessages
	c.MaxContextRatio = from.MaxContextRatio
	c.TokenAllocation = from.TokenAllocation
	c.Flushing = from.Flushing
	c.Persistence = from.Persistence
	c.PrivacyPolicy = from.PrivacyPolicy
	c.Locking = from.Locking
	c.TokenProvider = from.TokenProvider
	c.filePath = from.filePath
	c.CWD = from.CWD
	// Deliberately not copying ttlManager and ttlManagerOnce
}

// Merge merges another memory configuration into this one.
// This is useful for **layering configurations**, such as applying environment-specific
// overrides to a base configuration. The merge operation:
// - **Validates** that both configs have the same ID (or the other has no ID)
// - **Deep copies** the other config to avoid mutations
// - **Preserves sync primitives** (ttlManager, ttlManagerOnce) in the target
// - Uses **mergo.WithOverride** to replace values rather than append to slices
func (c *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("cannot merge memory.Config with type %T", other)
	}
	if c.ID != otherConfig.ID && otherConfig.ID != "" {
		return fmt.Errorf("cannot merge memory configs with different IDs: '%s' and '%s'", c.ID, otherConfig.ID)
	}
	// Deep copy the other config to avoid mutating it
	copiedOther, err := core.DeepCopy(otherConfig)
	if err != nil {
		return fmt.Errorf("failed to deep copy config: %w", err)
	}
	// Clear sync fields from the copy to prevent mergo from copying them
	copiedOther.ttlManager = nil
	copiedOther.ttlManagerOnce = sync.Once{}
	if err := mergo.Merge(c, copiedOther, mergo.WithOverride); err != nil {
		return fmt.Errorf("failed to merge memory configs: %w", err)
	}
	// The sync fields in c remain untouched since we cleared them in copiedOther
	return nil
}

// AsMap converts the memory configuration to a **map representation**.
// This is useful for **serialization**, debugging, or passing configuration
// to external systems that expect map-based data structures.
func (c *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(c)
}

// FromMap populates the memory configuration from a **map representation**.
// This is the inverse of AsMap and is typically used when loading configuration
// from **dynamic sources** like databases or API responses. The method:
// - **Parses** the map into a temporary Config struct
// - **Deep copies** the result to handle nested structures
// - Uses **copyConfigFields** to preserve sync primitives
func (c *Config) FromMap(data any) error {
	parsedConfig, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	// Deep copy the parsed config to handle nested structures properly
	copiedConfig, err := core.DeepCopy(parsedConfig)
	if err != nil {
		return fmt.Errorf("failed to deep copy config: %w", err)
	}
	// Use helper method to copy all fields except sync fields
	c.copyConfigFields(copiedConfig)
	return nil
}
