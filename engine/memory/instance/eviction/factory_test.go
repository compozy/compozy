package eviction

import (
	"sort"
	"testing"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/instance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyFactory_NewPolicyFactory(t *testing.T) {
	t.Run("Should create factory with built-in policies", func(t *testing.T) {
		factory := NewPolicyFactory()
		require.NotNil(t, factory)
		require.NotNil(t, factory.policies)
		// Should have built-in policies registered
		available := factory.ListAvailable()
		assert.Contains(t, available, "fifo")
		assert.Contains(t, available, "lru")
		assert.Contains(t, available, "priority")
	})
}

func TestPolicyFactory_Register(t *testing.T) {
	t.Run("Should register new policy", func(t *testing.T) {
		factory := NewPolicyFactory()
		// Create a custom policy
		err := factory.Register("custom", func() instance.EvictionPolicy {
			return &mockEvictionPolicy{name: "custom"}
		})
		require.NoError(t, err)
		// Should be available
		assert.True(t, factory.IsSupported("custom"))
		assert.Contains(t, factory.ListAvailable(), "custom")
	})

	t.Run("Should reject empty policy name", func(t *testing.T) {
		factory := NewPolicyFactory()
		err := factory.Register("", func() instance.EvictionPolicy {
			return &mockEvictionPolicy{}
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "policy name cannot be empty")
	})

	t.Run("Should reject nil creator", func(t *testing.T) {
		factory := NewPolicyFactory()
		err := factory.Register("test", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "policy creator cannot be nil")
	})

	t.Run("Should allow overwriting existing policy", func(t *testing.T) {
		factory := NewPolicyFactory()
		// Register initial policy
		err := factory.Register("test", func() instance.EvictionPolicy {
			return &mockEvictionPolicy{name: "version1"}
		})
		require.NoError(t, err)
		// Overwrite with new implementation
		err = factory.Register("test", func() instance.EvictionPolicy {
			return &mockEvictionPolicy{name: "version2"}
		})
		require.NoError(t, err)
		// Should create the new version
		policy, err := factory.Create("test")
		require.NoError(t, err)
		assert.Equal(t, "version2", policy.GetType())
	})
}

func TestPolicyFactory_Create(t *testing.T) {
	factory := NewPolicyFactory()

	t.Run("Should create FIFO policy", func(t *testing.T) {
		policy, err := factory.Create("fifo")
		require.NoError(t, err)
		require.NotNil(t, policy)
		assert.Equal(t, "fifo", policy.GetType())
		// Should be correct type
		_, ok := policy.(*FIFOEvictionPolicy)
		assert.True(t, ok)
	})

	t.Run("Should create LRU policy", func(t *testing.T) {
		policy, err := factory.Create("lru")
		require.NoError(t, err)
		require.NotNil(t, policy)
		assert.Equal(t, "lru", policy.GetType())
		// Should be correct type
		_, ok := policy.(*LRUEvictionPolicy)
		assert.True(t, ok)
	})

	t.Run("Should create priority policy", func(t *testing.T) {
		policy, err := factory.Create("priority")
		require.NoError(t, err)
		require.NotNil(t, policy)
		assert.Equal(t, "priority", policy.GetType())
		// Should be correct type
		_, ok := policy.(*PriorityEvictionPolicy)
		assert.True(t, ok)
	})

	t.Run("Should return error for unknown policy", func(t *testing.T) {
		policy, err := factory.Create("unknown")
		assert.Error(t, err)
		assert.Nil(t, policy)
		assert.Contains(t, err.Error(), "unknown eviction policy type: unknown")
	})
}

func TestPolicyFactory_CreateOrDefault(t *testing.T) {
	factory := NewPolicyFactory()

	t.Run("Should create requested policy if exists", func(t *testing.T) {
		policy := factory.CreateOrDefault("lru")
		require.NotNil(t, policy)
		assert.Equal(t, "lru", policy.GetType())
	})

	t.Run("Should return FIFO policy for unknown type", func(t *testing.T) {
		policy := factory.CreateOrDefault("unknown")
		require.NotNil(t, policy)
		assert.Equal(t, "fifo", policy.GetType())
		// Should be FIFO type
		_, ok := policy.(*FIFOEvictionPolicy)
		assert.True(t, ok)
	})
}

func TestPolicyFactory_ListAvailable(t *testing.T) {
	t.Run("Should list all available policies sorted", func(t *testing.T) {
		factory := NewPolicyFactory()
		available := factory.ListAvailable()
		// Should have at least the built-in policies
		assert.GreaterOrEqual(t, len(available), 3)
		assert.Contains(t, available, "fifo")
		assert.Contains(t, available, "lru")
		assert.Contains(t, available, "priority")
		// Should be sorted
		assert.True(t, sort.StringsAreSorted(available))
	})

	t.Run("Should include custom registered policies", func(t *testing.T) {
		factory := NewPolicyFactory()
		factory.Register("custom1", func() instance.EvictionPolicy {
			return &mockEvictionPolicy{name: "custom1"}
		})
		factory.Register("custom2", func() instance.EvictionPolicy {
			return &mockEvictionPolicy{name: "custom2"}
		})
		available := factory.ListAvailable()
		assert.Contains(t, available, "custom1")
		assert.Contains(t, available, "custom2")
	})
}

func TestPolicyFactory_IsSupported(t *testing.T) {
	factory := NewPolicyFactory()

	t.Run("Should return true for built-in policies", func(t *testing.T) {
		assert.True(t, factory.IsSupported("fifo"))
		assert.True(t, factory.IsSupported("lru"))
		assert.True(t, factory.IsSupported("priority"))
	})

	t.Run("Should return false for unknown policies", func(t *testing.T) {
		assert.False(t, factory.IsSupported("unknown"))
		assert.False(t, factory.IsSupported(""))
	})

	t.Run("Should return true for registered custom policy", func(t *testing.T) {
		factory.Register("custom", func() instance.EvictionPolicy {
			return &mockEvictionPolicy{name: "custom"}
		})
		assert.True(t, factory.IsSupported("custom"))
	})
}

func TestPolicyFactory_Clear(t *testing.T) {
	t.Run("Should clear all registered policies", func(t *testing.T) {
		factory := NewPolicyFactory()
		// Should have built-in policies initially
		assert.True(t, len(factory.ListAvailable()) > 0)
		// Clear all policies
		factory.Clear()
		// Should have no policies
		assert.Empty(t, factory.ListAvailable())
		assert.False(t, factory.IsSupported("fifo"))
		assert.False(t, factory.IsSupported("lru"))
		assert.False(t, factory.IsSupported("priority"))
	})
}

func TestPolicyFactory_DefaultFactory(t *testing.T) {
	t.Run("Should have default factory instance", func(t *testing.T) {
		require.NotNil(t, DefaultPolicyFactory)
		// Should have built-in policies
		assert.True(t, DefaultPolicyFactory.IsSupported("fifo"))
		assert.True(t, DefaultPolicyFactory.IsSupported("lru"))
		assert.True(t, DefaultPolicyFactory.IsSupported("priority"))
	})
}

func TestPolicyFactory_GlobalFunctions(t *testing.T) {
	t.Run("Should create policy using global function", func(t *testing.T) {
		policy, err := CreatePolicy("fifo")
		require.NoError(t, err)
		require.NotNil(t, policy)
		assert.Equal(t, "fifo", policy.GetType())
	})

	t.Run("Should register policy using global function", func(t *testing.T) {
		// Clear any previous test registrations
		originalPolicies := make(map[string]func() instance.EvictionPolicy)
		for name := range DefaultPolicyFactory.policies {
			originalPolicies[name] = DefaultPolicyFactory.policies[name]
		}
		// Register new policy
		err := RegisterPolicy("test-global", func() instance.EvictionPolicy {
			return &mockEvictionPolicy{name: "test-global"}
		})
		require.NoError(t, err)
		// Should be able to create it
		policy, err := CreatePolicy("test-global")
		require.NoError(t, err)
		assert.Equal(t, "test-global", policy.GetType())
		// Cleanup: restore original policies
		DefaultPolicyFactory.policies = originalPolicies
	})
}

func TestPolicyFactory_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent operations safely", func(t *testing.T) {
		factory := NewPolicyFactory()
		done := make(chan bool, 4)
		// Goroutine 1: Register policies
		go func() {
			for i := 0; i < 50; i++ {
				factory.Register(
					"concurrent"+string(rune('A'+i)),
					func() instance.EvictionPolicy {
						return &mockEvictionPolicy{name: "concurrent"}
					},
				)
			}
			done <- true
		}()
		// Goroutine 2: Create policies
		go func() {
			for i := 0; i < 100; i++ {
				factory.Create("fifo")
				factory.Create("lru")
				factory.Create("priority")
			}
			done <- true
		}()
		// Goroutine 3: List available
		go func() {
			for i := 0; i < 50; i++ {
				factory.ListAvailable()
			}
			done <- true
		}()
		// Goroutine 4: Check support
		go func() {
			for i := 0; i < 100; i++ {
				factory.IsSupported("fifo")
				factory.IsSupported("unknown")
			}
			done <- true
		}()
		// Wait for all goroutines
		for i := 0; i < 4; i++ {
			<-done
		}
		// Should complete without panic
		assert.True(t, true)
	})
}

// mockEvictionPolicy is a simple mock implementation for testing
type mockEvictionPolicy struct {
	name string
}

func (m *mockEvictionPolicy) SelectMessagesToEvict(messages []llm.Message, targetCount int) []llm.Message {
	if len(messages) <= targetCount {
		return nil
	}
	return messages[:len(messages)-targetCount]
}

func (m *mockEvictionPolicy) GetType() string {
	return m.name
}
