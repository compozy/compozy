package memory

import (
	"context"
	"fmt"
	"time"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// Config defines the structure for a memory resource configuration.
// This is what gets loaded from YAML files (e.g., in a `memories/` directory).
type Config struct {
	// Resource type identifier, should be "memory".
	// Used by autoloaders to identify the type of this configuration.
	Resource string `json:"resource"              yaml:"resource"              validate:"required,eq=memory"`
	// ID is the unique identifier for this memory resource within the project.
	ID string `json:"id"                    yaml:"id"                    validate:"required"`
	// Description provides a human-readable explanation of the memory resource's purpose.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Version allows tracking changes to the memory resource definition.
	Version string `json:"version,omitempty"     yaml:"version,omitempty"`

	// Type indicates the primary management strategy (e.g., token_based).
	// This refers to memory.MemoryType defined in types.go
	Type memcore.Type `json:"type" yaml:"type" validate:"required,oneof=token_based message_count_based buffer"`

	// MaxTokens is the hard limit on the number of tokens this memory can hold.
	MaxTokens int `json:"max_tokens,omitempty"        yaml:"max_tokens,omitempty"        validate:"omitempty,gt=0"`
	// MaxMessages is the hard limit on the number of messages.
	MaxMessages int `json:"max_messages,omitempty"      yaml:"max_messages,omitempty"      validate:"omitempty,gt=0"`
	// MaxContextRatio specifies the maximum portion of an LLM's context window this memory should aim to use.
	MaxContextRatio float64 `json:"max_context_ratio,omitempty" yaml:"max_context_ratio,omitempty" validate:"omitempty,gt=0,lte=1"`

	// TokenAllocation defines how the token budget is distributed if applicable.
	// Refers to memory.TokenAllocation defined in types.go
	TokenAllocation *memcore.TokenAllocation `json:"token_allocation,omitempty" yaml:"token_allocation,omitempty"`
	// FlushingStrategy defines how memory is managed when limits are approached or reached.
	// Refers to memcore.FlushingStrategyConfig defined in types.go
	Flushing *memcore.FlushingStrategyConfig `json:"flushing,omitempty"         yaml:"flushing,omitempty"` // Renamed from FlushingStrategy in PRD to avoid conflict with the struct type

	// Persistence defines how memory instances are persisted.
	// Refers to memcore.PersistenceConfig defined in types.go
	Persistence memcore.PersistenceConfig `json:"persistence" yaml:"persistence" validate:"required"`

	// PrivacyPolicy defines how sensitive data within this memory resource should be handled.
	// Refers to memcore.PrivacyPolicyConfig defined in types.go
	PrivacyPolicy *memcore.PrivacyPolicyConfig `json:"privacy_policy,omitempty" yaml:"privacy_policy,omitempty"`

	// Locking defines lock timeout settings for memory operations.
	// Refers to memcore.LockConfig defined in types.go
	Locking *memcore.LockConfig `json:"locking,omitempty" yaml:"locking,omitempty"`

	// TokenProvider configures multi-provider token counting with API-based counting.
	// Refers to memcore.TokenProviderConfig defined in types.go
	TokenProvider *memcore.TokenProviderConfig `json:"token_provider,omitempty" yaml:"token_provider,omitempty"`

	// --- Fields for core.Configurable / core.Config compatibility ---
	filePath string        `json:"-" yaml:"-"`
	CWD      *core.PathCWD `json:"-" yaml:"-"`
	// Env      *core.EnvMap  `json:"-" yaml:"-"` // Not typically needed for resource definitions
	// With     *core.Input   `json:"-" yaml:"-"` // Not typically needed for resource definitions
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

// GetAppendLockTTL returns the lock TTL for append operations with a default fallback.
func (c *Config) GetAppendLockTTL() time.Duration {
	if c.Locking != nil && c.Locking.AppendTTL != "" {
		if ttl, err := time.ParseDuration(c.Locking.AppendTTL); err == nil {
			return ttl
		}
	}
	return 30 * time.Second // Default
}

// GetClearLockTTL returns the lock TTL for clear operations with a default fallback.
func (c *Config) GetClearLockTTL() time.Duration {
	if c.Locking != nil && c.Locking.ClearTTL != "" {
		if ttl, err := time.ParseDuration(c.Locking.ClearTTL); err == nil {
			return ttl
		}
	}
	return 10 * time.Second // Default
}

// GetFlushLockTTL returns the lock TTL for flush operations with a default fallback.
func (c *Config) GetFlushLockTTL() time.Duration {
	if c.Locking != nil && c.Locking.FlushTTL != "" {
		if ttl, err := time.ParseDuration(c.Locking.FlushTTL); err == nil {
			return ttl
		}
	}
	return 5 * time.Minute // Default
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

func (c *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("cannot merge memory.Config with type %T", other)
	}
	if c.ID != otherConfig.ID && otherConfig.ID != "" {
		return fmt.Errorf("cannot merge memory configs with different IDs: '%s' and '%s'", c.ID, otherConfig.ID)
	}
	if err := mergo.Merge(c, otherConfig, mergo.WithOverride); err != nil {
		return fmt.Errorf("failed to merge memory configs: %w", err)
	}
	return nil
}

func (c *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(c)
}

func (c *Config) FromMap(data any) error {
	parsedConfig, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	*c = *parsedConfig // Replace current config with parsed one
	return nil
}
