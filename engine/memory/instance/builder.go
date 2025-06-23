package instance

import (
	"fmt"

	"github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	"go.temporal.io/sdk/client"
)

// BuilderOptions holds options for creating a memory instance.
type BuilderOptions struct {
	InstanceID        string // Resolved unique key for the instance
	ResourceID        string // ID of the memory.Config definition
	ProjectID         string // Optional project ID
	ResourceConfig    *core.Resource
	Store             core.Store
	LockManager       LockManager
	TokenCounter      core.TokenCounter
	FlushingStrategy  FlushStrategy
	TemporalClient    client.Client
	TemporalTaskQueue string
	PrivacyManager    any // Will be properly typed when privacy is migrated
	Logger            logger.Logger
}

// Builder provides a fluent interface for creating memory instances
type Builder struct {
	opts *BuilderOptions
}

// NewBuilder creates a new instance builder
func NewBuilder() *Builder {
	return &Builder{
		opts: &BuilderOptions{},
	}
}

// WithInstanceID sets the instance ID
func (b *Builder) WithInstanceID(id string) *Builder {
	b.opts.InstanceID = id
	return b
}

// WithResourceID sets the resource ID
func (b *Builder) WithResourceID(id string) *Builder {
	b.opts.ResourceID = id
	return b
}

// WithProjectID sets the project ID
func (b *Builder) WithProjectID(id string) *Builder {
	b.opts.ProjectID = id
	return b
}

// WithResourceConfig sets the resource configuration
func (b *Builder) WithResourceConfig(config *core.Resource) *Builder {
	b.opts.ResourceConfig = config
	return b
}

// WithStore sets the store implementation
func (b *Builder) WithStore(store core.Store) *Builder {
	b.opts.Store = store
	return b
}

// WithLockManager sets the lock manager
func (b *Builder) WithLockManager(lm LockManager) *Builder {
	b.opts.LockManager = lm
	return b
}

// WithTokenCounter sets the token counter
func (b *Builder) WithTokenCounter(tc core.TokenCounter) *Builder {
	b.opts.TokenCounter = tc
	return b
}

// WithFlushingStrategy sets the flushing strategy
func (b *Builder) WithFlushingStrategy(fs FlushStrategy) *Builder {
	b.opts.FlushingStrategy = fs
	return b
}

// WithTemporalClient sets the temporal client
func (b *Builder) WithTemporalClient(tc client.Client) *Builder {
	b.opts.TemporalClient = tc
	return b
}

// WithTemporalTaskQueue sets the temporal task queue
func (b *Builder) WithTemporalTaskQueue(queue string) *Builder {
	b.opts.TemporalTaskQueue = queue
	return b
}

// WithPrivacyManager sets the privacy manager
func (b *Builder) WithPrivacyManager(pm any) *Builder {
	b.opts.PrivacyManager = pm
	return b
}

// WithLogger sets the logger
func (b *Builder) WithLogger(log logger.Logger) *Builder {
	b.opts.Logger = log
	return b
}

// Validate validates the builder options
func (b *Builder) Validate() error {
	if b.opts.InstanceID == "" {
		return fmt.Errorf("instance ID cannot be empty")
	}
	if b.opts.ResourceConfig == nil {
		return fmt.Errorf("resource config cannot be nil for instance %s", b.opts.InstanceID)
	}
	if b.opts.Store == nil {
		return fmt.Errorf("memory store cannot be nil for instance %s", b.opts.InstanceID)
	}
	if b.opts.LockManager == nil {
		return fmt.Errorf("lock manager cannot be nil for instance %s", b.opts.InstanceID)
	}
	if b.opts.TokenCounter == nil {
		return fmt.Errorf("token counter cannot be nil for instance %s", b.opts.InstanceID)
	}
	if b.opts.FlushingStrategy == nil {
		return fmt.Errorf("flushing strategy cannot be nil for instance %s", b.opts.InstanceID)
	}
	if b.opts.TemporalClient == nil {
		return fmt.Errorf("temporal client cannot be nil for instance %s", b.opts.InstanceID)
	}
	if b.opts.TemporalTaskQueue == "" {
		b.opts.TemporalTaskQueue = "memory-operations" // Default task queue
	}
	if b.opts.Logger == nil {
		b.opts.Logger = logger.NewForTests() // Default to test logger if none provided
	}
	return nil
}

// Build creates the memory instance
func (b *Builder) Build() (Instance, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return NewMemoryInstance(b.opts)
}
