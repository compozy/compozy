package orchestrator

import (
	"context"
	"errors"
	"net"
	"testing"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsToolExecutionError(t *testing.T) {
	t.Run("Should detect structured error payload", func(t *testing.T) {
		errJSON := `{"success":false,"error":{"code":"X","message":"boom","details":"d"}}`
		terr, ok := IsToolExecutionError(errJSON)
		require.True(t, ok)
		assert.Equal(t, "boom", terr.Message)
	})
	t.Run("Should detect unstructured error indicators", func(t *testing.T) {
		terr, ok := IsToolExecutionError("stderr: something failed")
		require.True(t, ok)
		assert.Equal(t, ErrCodeToolExecution, terr.Code)
	})
	t.Run("Should ignore non-error text", func(t *testing.T) {
		_, ok := IsToolExecutionError("all good")
		assert.False(t, ok)
	})
}

func TestErrorHelpers_Wrappers(t *testing.T) {
	t.Run("Should wrap tool error with tool name", func(t *testing.T) {
		err := NewToolError(errors.New("x"), ErrCodeToolInvalidInput, "fmt", map[string]any{"k": "v"})
		require.Error(t, err)
	})
	t.Run("Should wrap validation and MCP errors", func(t *testing.T) {
		verr := NewValidationError(errors.New("bad"), "field", 123)
		require.Error(t, verr)
		merr := WrapMCPError(errors.New("conn"), "dial")
		require.Error(t, merr)
	})
}

func TestIsRetryableErrorWithContext(t *testing.T) {
	ctx := context.Background()
	t.Run("Should be retryable for adapter timeout", func(t *testing.T) {
		err := llmadapter.NewErrorWithCode(llmadapter.ErrCodeTimeout, "t", "prov", nil)
		assert.True(t, isRetryableErrorWithContext(ctx, err))
	})
	t.Run("Should not be retryable for bad request", func(t *testing.T) {
		err := llmadapter.NewErrorWithCode(llmadapter.ErrCodeBadRequest, "b", "prov", nil)
		assert.False(t, isRetryableErrorWithContext(ctx, err))
	})
	t.Run("Should be retryable for net.Error", func(t *testing.T) {
		ne := &net.DNSError{IsTimeout: true}
		assert.True(t, isRetryableErrorWithContext(ctx, ne))
	})
}
