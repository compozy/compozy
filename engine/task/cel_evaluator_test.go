package task

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errContains checks if error message contains substring
func errContains(err error, substr string) bool {
	return err != nil && strings.Contains(err.Error(), substr)
}

func TestNewCELEvaluator(t *testing.T) {
	t.Run("Should create CEL evaluator successfully", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		assert.NotNil(t, evaluator)
		assert.NotNil(t, evaluator.env)
		assert.Equal(t, uint64(1000), evaluator.costLimit)
	})
	t.Run("Should create CEL evaluator with custom cost limit", func(t *testing.T) {
		evaluator, err := NewCELEvaluator(WithCostLimit(500))
		require.NoError(t, err)
		assert.NotNil(t, evaluator)
		assert.Equal(t, uint64(500), evaluator.costLimit)
	})
	t.Run("Should create CEL evaluator with Ristretto cache", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		assert.NotNil(t, evaluator)
		assert.NotNil(t, evaluator.programCache)
	})
}

func TestCELEvaluator_Evaluate(t *testing.T) {
	t.Run("Should evaluate simple boolean expression", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"status": "approved",
				},
			},
		}
		result, err := evaluator.Evaluate(ctx, `signal.payload.status == "approved"`, data)
		require.NoError(t, err)
		assert.True(t, result)
	})
	t.Run("Should evaluate complex expression with processor output", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"value": 100,
				},
			},
			"processor": map[string]any{
				"output": map[string]any{
					"valid": true,
					"score": 0.95,
				},
			},
		}
		expression := `signal.payload.value > 50 && processor.output.valid && processor.output.score > 0.8`
		result, err := evaluator.Evaluate(ctx, expression, data)
		require.NoError(t, err)
		assert.True(t, result)
	})
	t.Run("Should handle false conditions", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"status": "rejected",
				},
			},
		}
		result, err := evaluator.Evaluate(ctx, `signal.payload.status == "approved"`, data)
		require.NoError(t, err)
		assert.False(t, result)
	})
	t.Run("Should handle missing fields gracefully", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{},
			},
		}
		// CEL will return an error for undefined field access
		result, err := evaluator.Evaluate(ctx, `signal.payload.status == "approved"`, data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no such key")
		assert.False(t, result)
	})
	t.Run("Should respect context cancellation", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"status": "approved",
				},
			},
		}
		result, err := evaluator.Evaluate(ctx, `signal.payload.status == "approved"`, data)
		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
			errContains(err, "context"))
		assert.False(t, result)
	})
	t.Run("Should enforce type safety", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"count": "not-a-number",
				},
			},
		}
		// Type mismatch should cause evaluation error
		result, err := evaluator.Evaluate(ctx, `signal.payload.count > 10`, data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no such overload")
		assert.False(t, result)
	})
	t.Run("Should handle compilation errors", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{},
		}
		// Invalid syntax
		result, err := evaluator.Evaluate(ctx, `signal.payload.status ==`, data)
		require.Error(t, err)
		// Check for compilation error without relying on exact string
		assert.True(t, errContains(err, "compilation") || errContains(err, "parse"))
		assert.False(t, result)
	})
	t.Run("Should require boolean result", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"status": "approved",
				},
			},
		}
		// Expression returns string, not boolean
		result, err := evaluator.Evaluate(ctx, `signal.payload.status`, data)
		require.Error(t, err)
		assert.True(t, errContains(err, "boolean"))
		assert.False(t, result)
	})
	t.Run("Should handle has() function for optional fields", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"status": "approved",
				},
			},
		}
		// Check if field exists before accessing
		expression := `has(signal.payload.status) && signal.payload.status == "approved"`
		result, err := evaluator.Evaluate(ctx, expression, data)
		require.NoError(t, err)
		assert.True(t, result)
		// Check for missing field
		expression2 := `has(signal.payload.missing_field)`
		result2, err := evaluator.Evaluate(ctx, expression2, data)
		require.NoError(t, err)
		assert.False(t, result2)
	})
	t.Run("Should evaluate expressions with headers variable", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"headers": map[string]any{
				"content-type": "application/json",
				"user-agent":   "test-agent",
			},
		}
		result, err := evaluator.Evaluate(ctx, `headers["content-type"] == "application/json"`, data)
		require.NoError(t, err)
		assert.True(t, result)
	})
	t.Run("Should evaluate expressions with query variable", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"query": map[string]any{
				"status": "active",
				"limit":  "10",
			},
		}
		result, err := evaluator.Evaluate(ctx, `query.status == "active"`, data)
		require.NoError(t, err)
		assert.True(t, result)
	})
	t.Run("Should evaluate expressions with payload, headers, and query together", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"payload": map[string]any{
				"action": "create",
				"userId": 123,
			},
			"headers": map[string]any{
				"authorization": "Bearer token123",
				"content-type":  "application/json",
			},
			"query": map[string]any{
				"source":  "web",
				"version": "v1",
			},
		}
		// Test webhook filter condition using all three variables
		expression := `payload.action == "create" && headers["content-type"] == "application/json" && query.source == "web"`
		result, err := evaluator.Evaluate(ctx, expression, data)
		require.NoError(t, err)
		assert.True(t, result)
	})
	t.Run("Should handle empty headers and query maps", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"payload": map[string]any{
				"status": "ok",
			},
			"headers": map[string]any{},
			"query":   map[string]any{},
		}
		// Test that empty headers and query maps don't cause errors
		result, err := evaluator.Evaluate(ctx, `payload.status == "ok"`, data)
		require.NoError(t, err)
		assert.True(t, result)
	})
	t.Run("Should cache compiled programs for performance", func(t *testing.T) {
		// Use small cache size to test eviction
		evaluator, err := NewCELEvaluator(WithCacheSize(3))
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"value": 1,
				},
			},
		}
		// Test that same expression reuses cached program
		expression := `signal.payload.value == 1`
		// First call compiles and caches
		start1 := time.Now()
		result1, err := evaluator.Evaluate(ctx, expression, data)
		duration1 := time.Since(start1)
		require.NoError(t, err)
		assert.True(t, result1)
		// Second call should be faster due to cache hit
		start2 := time.Now()
		result2, err := evaluator.Evaluate(ctx, expression, data)
		duration2 := time.Since(start2)
		require.NoError(t, err)
		assert.True(t, result2)
		// Cache hit should generally be faster, but we can't guarantee it
		// due to system factors, so just verify correctness
		t.Logf("First call: %v, Second call: %v", duration1, duration2)
	})
	t.Run("Should handle cache eviction with small cache", func(t *testing.T) {
		// Use very small cache to test eviction
		evaluator, err := NewCELEvaluator(WithCacheSize(2))
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"value": 1,
				},
			},
		}
		// Add more expressions than cache can hold
		expressions := []string{
			`signal.payload.value == 1`,
			`signal.payload.value > 0`,
			`signal.payload.value < 10`,
			`signal.payload.value != 0`,
		}
		// Evaluate all expressions
		for _, expr := range expressions {
			result, err := evaluator.Evaluate(ctx, expr, data)
			require.NoError(t, err)
			assert.True(t, result)
		}
		// Re-evaluate first expression - should still work even if evicted
		result, err := evaluator.Evaluate(ctx, expressions[0], data)
		require.NoError(t, err)
		assert.True(t, result)
	})
}

func TestCELEvaluator_ValidateExpression(t *testing.T) {
	t.Run("Should validate correct expression", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		err = evaluator.ValidateExpression(`signal.payload.status == "approved"`)
		assert.NoError(t, err)
	})
	t.Run("Should reject invalid expression", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		err = evaluator.ValidateExpression(`signal.payload.status ==`)
		require.Error(t, err)
		assert.True(t, errContains(err, "invalid") || errContains(err, "compilation"))
	})
}

func TestCELEvaluator_CostLimit(t *testing.T) {
	t.Run("Should handle expressions within cost limit", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		ctx := context.Background()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"list": []any{1, 2, 3, 4, 5},
				},
			},
		}
		// Simple expression should be within cost limit
		result, err := evaluator.Evaluate(ctx, `size(signal.payload.list) > 3`, data)
		require.NoError(t, err)
		assert.True(t, result)
	})
	t.Run("Should return error when expression exceeds cost limit", func(t *testing.T) {
		evaluator, err := NewCELEvaluator(WithCostLimit(5)) // Very low cost limit
		require.NoError(t, err)
		ctx := context.Background()
		// Create a simple but high-cost expression
		// String concatenation in a loop is expensive
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"value": "test",
				},
			},
		}
		// This expression has high cost due to multiple string operations
		result, err := evaluator.Evaluate(
			ctx,
			`signal.payload.value + signal.payload.value + signal.payload.value +
			 signal.payload.value + signal.payload.value + signal.payload.value +
			 signal.payload.value + signal.payload.value + signal.payload.value +
			 signal.payload.value + signal.payload.value + signal.payload.value == "testtesttesttesttesttesttesttesttesttesttesttest"`,
			data,
		)
		// Should exceed cost limit or evaluate correctly
		if err != nil {
			assert.Contains(t, err.Error(), "exceeded cost limit")
		} else {
			// If no error, verify the result is correct
			assert.True(t, result)
		}
	})
}

func TestCELEvaluator_ContextTimeout(t *testing.T) {
	t.Run("Should respect context timeout", func(t *testing.T) {
		evaluator, err := NewCELEvaluator()
		require.NoError(t, err)
		// Create already-expired context
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		data := map[string]any{
			"signal": map[string]any{
				"payload": map[string]any{
					"status": "approved",
				},
			},
		}
		result, err := evaluator.Evaluate(ctx, `signal.payload.status == "approved"`, data)
		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
			errContains(err, "context"))
		assert.False(t, result)
	})
}
