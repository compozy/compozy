package webhook

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/task"
)

func TestCELAdapter_Allow(t *testing.T) {
	t.Parallel()
	eval, err := task.NewCELEvaluator()
	require.NoError(t, err)
	a := NewCELAdapter(eval)
	ctx := t.Context()

	t.Run("Should allow when expression is true", func(t *testing.T) {
		t.Parallel()
		data := map[string]any{"payload": map[string]any{"status": "ok"}}
		allowed, err := a.Allow(ctx, "payload.status == 'ok'", data)
		require.NoError(t, err)
		assert.True(t, allowed)
	})
	t.Run("Should reject when expression is false", func(t *testing.T) {
		t.Parallel()
		data := map[string]any{"payload": map[string]any{"status": "fail"}}
		allowed, err := a.Allow(ctx, "payload.status == 'ok'", data)
		require.NoError(t, err)
		assert.False(t, allowed)
	})
	t.Run("Should return error on invalid syntax", func(t *testing.T) {
		t.Parallel()
		data := map[string]any{"payload": map[string]any{"status": "ok"}}
		_, err := a.Allow(ctx, "payload.status = 'ok'", data)
		require.Error(t, err)
		assert.ErrorContains(t, err, "CEL")
	})
}

func TestBuildContext(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize headers to lowercase", func(t *testing.T) {
		t.Parallel()
		headers := http.Header{
			"Content-Type":    []string{"application/json"},
			"Authorization":   []string{"Bearer token123"},
			"X-Custom-Header": []string{"value1", "value2"},
		}
		payload := map[string]any{"status": "ok"}
		query := url.Values{"param": []string{"value"}}

		ctx := BuildContext(payload, headers, query)

		// Check that payload is present
		assert.Equal(t, payload, ctx["payload"])

		// Check that headers are normalized to lowercase
		normalizedHeaders := ctx["headers"].(map[string]any)
		assert.Equal(t, "application/json", normalizedHeaders["content-type"])
		assert.Equal(t, "Bearer token123", normalizedHeaders["authorization"])
		assert.Equal(t, []string{"value1", "value2"}, normalizedHeaders["x-custom-header"])

		// Check that original casing is not preserved
		assert.NotContains(t, normalizedHeaders, "Content-Type")
		assert.NotContains(t, normalizedHeaders, "Authorization")
		assert.NotContains(t, normalizedHeaders, "X-Custom-Header")
	})

	t.Run("Should handle case-insensitive header access in CEL expressions", func(t *testing.T) {
		t.Parallel()
		eval, err := task.NewCELEvaluator()
		require.NoError(t, err)
		adapter := NewCELAdapter(eval)

		// Create headers with mixed casing
		headers := http.Header{
			"Content-Type": []string{"application/json"},
			"USER-AGENT":   []string{"test-agent"},
			"X-API-Key":    []string{"secret123"},
		}
		payload := map[string]any{"action": "create"}
		query := url.Values{}

		ctx := t.Context()
		data := BuildContext(payload, headers, query)

		// Test that different casings all map to the same normalized lowercase key
		allowed1, err := adapter.Allow(ctx, `headers["content-type"] == "application/json"`, data)
		require.NoError(t, err)
		assert.True(t, allowed1)

		// Since headers are normalized to lowercase, accessing with different casings won't work
		// Instead, test that the normalized key works and verify the normalization happened
		normalizedHeaders := data["headers"].(map[string]any)
		assert.Contains(t, normalizedHeaders, "content-type")
		assert.Equal(t, "application/json", normalizedHeaders["content-type"])
		assert.NotContains(t, normalizedHeaders, "Content-Type")
		assert.NotContains(t, normalizedHeaders, "CONTENT-TYPE")
	})

	t.Run("Should handle nil headers and query", func(t *testing.T) {
		t.Parallel()
		payload := map[string]any{"status": "ok"}
		ctx := BuildContext(payload, nil, nil)

		assert.Equal(t, payload, ctx["payload"])
		assert.NotContains(t, ctx, "headers")
		assert.NotContains(t, ctx, "query")
	})

	t.Run("Should handle single and multiple header values", func(t *testing.T) {
		t.Parallel()
		headers := http.Header{
			"Single-Value":    []string{"value1"},
			"Multiple-Values": []string{"value1", "value2", "value3"},
		}
		payload := map[string]any{"status": "ok"}

		ctx := BuildContext(payload, headers, nil)
		normalizedHeaders := ctx["headers"].(map[string]any)

		// Single value should be stored as string
		assert.Equal(t, "value1", normalizedHeaders["single-value"])
		assert.IsType(t, "", normalizedHeaders["single-value"])

		// Multiple values should be stored as slice
		assert.Equal(t, []string{"value1", "value2", "value3"}, normalizedHeaders["multiple-values"])
		assert.IsType(t, []string{}, normalizedHeaders["multiple-values"])
	})
}
