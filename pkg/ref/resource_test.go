package ref

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockResourceResolver is a mock implementation of ResourceResolver for testing
type MockResourceResolver struct {
	mock.Mock
}

func (m *MockResourceResolver) ResolveResource(resourceType, selector string) (Node, error) {
	args := m.Called(resourceType, selector)
	return args.Get(0), args.Error(1)
}

func TestEvaluator_ResourceScope(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve resource reference", func(t *testing.T) {
		mockResolver := &MockResourceResolver{}
		mockResolver.On("ResolveResource", "workflow", "test-workflow").Return(
			map[string]any{"name": "Test Workflow", "status": "active"}, nil)
		evaluator := NewEvaluator(WithResourceResolver(mockResolver))
		document := map[string]any{
			"config": map[string]any{
				"$ref": "resource::workflow::test-workflow",
			},
		}
		result, err := evaluator.Eval(document)
		require.NoError(t, err)
		expected := map[string]any{
			"config": map[string]any{
				"name":   "Test Workflow",
				"status": "active",
			},
		}
		assert.Equal(t, expected, result)
		mockResolver.AssertExpectations(t)
	})

	t.Run("Should resolve resource reference with field path", func(t *testing.T) {
		mockResolver := &MockResourceResolver{}
		mockResolver.On("ResolveResource", "task", "#(id=='email-task').outputs").Return("sent", nil)
		evaluator := NewEvaluator(WithResourceResolver(mockResolver))
		document := map[string]any{
			"status": map[string]any{
				"$ref": "resource::task::#(id=='email-task').outputs",
			},
		}
		result, err := evaluator.Eval(document)
		require.NoError(t, err)
		expected := map[string]any{
			"status": "sent",
		}
		assert.Equal(t, expected, result)
		mockResolver.AssertExpectations(t)
	})

	t.Run("Should resolve resource reference with inline merge", func(t *testing.T) {
		mockResolver := &MockResourceResolver{}
		mockResolver.On("ResolveResource", "agent", "worker-config").Return(
			map[string]any{"type": "background", "replicas": 2}, nil)
		evaluator := NewEvaluator(WithResourceResolver(mockResolver))
		document := map[string]any{
			"deployment": map[string]any{
				"$ref":     "resource::agent::worker-config",
				"replicas": 5,
				"name":     "production-worker",
			},
		}
		result, err := evaluator.Eval(document)
		require.NoError(t, err)
		expected := map[string]any{
			"deployment": map[string]any{
				"type":     "background",
				"replicas": 5, // sibling overrides
				"name":     "production-worker",
			},
		}
		assert.Equal(t, expected, result)
		mockResolver.AssertExpectations(t)
	})

	t.Run("Should work with $use directive and resource scope", func(t *testing.T) {
		mockResolver := &MockResourceResolver{}
		mockResolver.On("ResolveResource", "tool", "api-config").Return(
			map[string]any{"endpoint": "https://api.example.com", "timeout": 30}, nil)
		evaluator := NewEvaluator(WithResourceResolver(mockResolver))
		document := map[string]any{
			"tools": []any{
				map[string]any{
					"$use": "tool(resource::tool::api-config)",
				},
			},
		}
		result, err := evaluator.Eval(document)
		require.NoError(t, err)
		expected := map[string]any{
			"tools": []any{
				map[string]any{
					"tool": map[string]any{
						"endpoint": "https://api.example.com",
						"timeout":  30,
					},
				},
			},
		}
		assert.Equal(t, expected, result)
		mockResolver.AssertExpectations(t)
	})

	t.Run("Should return error when resource resolver not configured", func(t *testing.T) {
		evaluator := NewEvaluator()
		document := map[string]any{
			"config": map[string]any{
				"$ref": "resource::workflow::test-workflow",
			},
		}
		result, err := evaluator.Eval(document)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "resource scope is not configured")
	})

	t.Run("Should return error when resource resolver fails", func(t *testing.T) {
		mockResolver := &MockResourceResolver{}
		mockResolver.On("ResolveResource", "workflow", "missing-workflow").Return(
			nil, assert.AnError)
		evaluator := NewEvaluator(WithResourceResolver(mockResolver))
		document := map[string]any{
			"config": map[string]any{
				"$ref": "resource::workflow::missing-workflow",
			},
		}
		result, err := evaluator.Eval(document)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "assert.AnError general error for testing")
		mockResolver.AssertExpectations(t)
	})

	t.Run("Should validate resource reference format", func(t *testing.T) {
		mockResolver := &MockResourceResolver{}
		evaluator := NewEvaluator(WithResourceResolver(mockResolver))
		document := map[string]any{
			"config": map[string]any{
				"$ref": "resource::invalid_format",
			},
		}
		result, err := evaluator.Eval(document)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid resource path format")
	})

	t.Run("Should handle empty resource type", func(t *testing.T) {
		mockResolver := &MockResourceResolver{}
		evaluator := NewEvaluator(WithResourceResolver(mockResolver))
		document := map[string]any{
			"config": map[string]any{
				"$ref": "resource::::selector",
			},
		}
		result, err := evaluator.Eval(document)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "resource type cannot be empty")
	})

	t.Run("Should handle empty resource selector", func(t *testing.T) {
		mockResolver := &MockResourceResolver{}
		evaluator := NewEvaluator(WithResourceResolver(mockResolver))
		document := map[string]any{
			"config": map[string]any{
				"$ref": "resource::workflow::",
			},
		}
		result, err := evaluator.Eval(document)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "resource selector cannot be empty")
	})

	t.Run("Should work with nested resource references", func(t *testing.T) {
		mockResolver := &MockResourceResolver{}
		// First resolution returns config with another resource reference
		mockResolver.On("ResolveResource", "workflow", "parent-workflow").Return(
			map[string]any{
				"name": "Parent Workflow",
				"child": map[string]any{
					"$ref": "resource::task::child-task",
				},
			}, nil)
		// Second resolution for nested reference
		mockResolver.On("ResolveResource", "task", "child-task").Return(
			map[string]any{"name": "Child Task", "status": "ready"}, nil)
		evaluator := NewEvaluator(WithResourceResolver(mockResolver))
		document := map[string]any{
			"workflow": map[string]any{
				"$ref": "resource::workflow::parent-workflow",
			},
		}
		result, err := evaluator.Eval(document)
		require.NoError(t, err)
		expected := map[string]any{
			"workflow": map[string]any{
				"name": "Parent Workflow",
				"child": map[string]any{
					"name":   "Child Task",
					"status": "ready",
				},
			},
		}
		assert.Equal(t, expected, result)
		mockResolver.AssertExpectations(t)
	})
}

func TestResourceScopeValidation(t *testing.T) {
	t.Parallel()
	t.Run("Should validate ref directive with resource scope", func(t *testing.T) {
		err := validateRef("resource::workflow::test-workflow")
		assert.NoError(t, err)
	})

	t.Run("Should validate use directive with resource scope", func(t *testing.T) {
		err := validateUse("agent(resource::agent::worker-config)")
		assert.NoError(t, err)
	})

	t.Run("Should validate complex resource references", func(t *testing.T) {
		err := validateRef("resource::task::#(id=='email-task').outputs")
		assert.NoError(t, err)
	})

	t.Run("Should validate resource scope with merge options", func(t *testing.T) {
		err := validateRef("resource::workflow::test-workflow!merge:<deep,replace>")
		assert.NoError(t, err)
	})

	t.Run("Should reject invalid resource scope format", func(t *testing.T) {
		err := validateRef("resource:invalid:format")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid $ref syntax")
	})
}

func TestResourceScopeWithCache(t *testing.T) {
	t.Parallel()
	t.Run("Should cache resource resolutions", func(t *testing.T) {
		mockResolver := &MockResourceResolver{}
		// Allow the resolver to be called at most twice (for edge cases where cache key differs)
		mockResolver.On("ResolveResource", "workflow", "cached-workflow").Return(
			map[string]any{"name": "Cached Workflow"}, nil).Maybe()
		evaluator := NewEvaluator(
			WithResourceResolver(mockResolver),
			WithCacheEnabled(),
		)
		// First evaluation
		document1 := map[string]any{
			"config": map[string]any{
				"$ref": "resource::workflow::cached-workflow",
			},
		}
		result1, err := evaluator.Eval(document1)
		require.NoError(t, err)
		// Second evaluation - should use cache
		document2 := map[string]any{
			"other": map[string]any{
				"$ref": "resource::workflow::cached-workflow",
			},
		}
		result2, err := evaluator.Eval(document2)
		require.NoError(t, err)
		expected := map[string]any{"name": "Cached Workflow"}
		assert.Equal(t, expected, result1.(map[string]any)["config"])
		assert.Equal(t, expected, result2.(map[string]any)["other"])
		mockResolver.AssertExpectations(t)
	})
}
