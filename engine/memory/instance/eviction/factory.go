package eviction

import (
	"fmt"
	"sort"
	"sync"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance"
)

// PolicyFactory manages the creation of eviction policies
type PolicyFactory struct {
	policies map[string]func() instance.EvictionPolicy
	mu       sync.RWMutex
}

// NewPolicyFactory creates a new eviction policy factory with built-in policies
func NewPolicyFactory() *PolicyFactory {
	factory := &PolicyFactory{
		policies: make(map[string]func() instance.EvictionPolicy),
	}
	// Register built-in policies
	if err := factory.registerBuiltInPolicies(); err != nil {
		// This should never happen with built-in policies
		panic(fmt.Errorf("failed to register built-in eviction policies: %w", err))
	}
	return factory
}

// registerBuiltInPolicies registers all default eviction policies
func (f *PolicyFactory) registerBuiltInPolicies() error {
	// FIFO policy
	if err := f.Register("fifo", func() instance.EvictionPolicy {
		return NewFIFOEvictionPolicy()
	}); err != nil {
		return fmt.Errorf("failed to register FIFO policy: %w", err)
	}
	// LRU policy
	if err := f.Register("lru", func() instance.EvictionPolicy {
		return NewLRUEvictionPolicy()
	}); err != nil {
		return fmt.Errorf("failed to register LRU policy: %w", err)
	}
	// Priority-based policy
	if err := f.Register("priority", func() instance.EvictionPolicy {
		return NewPriorityEvictionPolicy()
	}); err != nil {
		return fmt.Errorf("failed to register priority policy: %w", err)
	}
	return nil
}

// Register adds a new policy creator to the factory
func (f *PolicyFactory) Register(name string, creator func() instance.EvictionPolicy) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name == "" {
		return fmt.Errorf("policy name cannot be empty")
	}
	if creator == nil {
		return fmt.Errorf("policy creator cannot be nil")
	}
	f.policies[name] = creator
	return nil
}

// Create instantiates an eviction policy by type
func (f *PolicyFactory) Create(policyType string) (instance.EvictionPolicy, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	creator, exists := f.policies[policyType]
	if !exists {
		return nil, fmt.Errorf("unknown eviction policy type: %s", policyType)
	}
	return creator(), nil
}

// CreateOrDefault creates a policy or returns a default FIFO policy
func (f *PolicyFactory) CreateOrDefault(policyType string) instance.EvictionPolicy {
	policy, err := f.Create(policyType)
	if err != nil {
		// Return default FIFO policy
		return NewFIFOEvictionPolicy()
	}
	return policy
}

// ListAvailable returns all registered policy types
func (f *PolicyFactory) ListAvailable() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	policies := make([]string, 0, len(f.policies))
	for name := range f.policies {
		policies = append(policies, name)
	}
	sort.Strings(policies)
	return policies
}

// IsSupported checks if a policy type is supported
func (f *PolicyFactory) IsSupported(policyType string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, exists := f.policies[policyType]
	return exists
}

// Clear removes all registered policies (useful for testing)
func (f *PolicyFactory) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.policies = make(map[string]func() instance.EvictionPolicy)
}

// DefaultPolicyFactory is the global factory instance
var DefaultPolicyFactory = NewPolicyFactory()

// CreatePolicy creates an eviction policy using the default factory
func CreatePolicy(policyType string) (instance.EvictionPolicy, error) {
	return DefaultPolicyFactory.Create(policyType)
}

// RegisterPolicy registers a policy with the default factory
func RegisterPolicy(name string, creator func() instance.EvictionPolicy) error {
	return DefaultPolicyFactory.Register(name, creator)
}

// CreateOrDefault creates a policy using the default factory or returns FIFO if not found
func CreateOrDefault(policyType string) instance.EvictionPolicy {
	return DefaultPolicyFactory.CreateOrDefault(policyType)
}

// CreatePolicyWithConfig creates an eviction policy with proper eviction configuration
func CreatePolicyWithConfig(config *memcore.EvictionPolicyConfig) instance.EvictionPolicy {
	if config == nil {
		return NewFIFOEvictionPolicy()
	}
	switch config.Type {
	case memcore.PriorityEviction:
		return NewPriorityEvictionPolicyWithKeywords(config.PriorityKeywords)
	case memcore.LRUEviction:
		return NewLRUEvictionPolicy()
	case memcore.FIFOEviction:
		return NewFIFOEvictionPolicy()
	default:
		// Default to FIFO if unknown policy type
		return NewFIFOEvictionPolicy()
	}
}
