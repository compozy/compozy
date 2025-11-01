package agentaction

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("Should create action with minimal configuration", func(t *testing.T) {
		ctx := t.Context()
		action, err := New(ctx, "test-action",
			WithPrompt("Test prompt"),
		)
		require.NoError(t, err)
		require.NotNil(t, action)
		assert.Equal(t, "test-action", action.ID)
		assert.Equal(t, "Test prompt", action.Prompt)
	})
	t.Run("Should trim whitespace from ID and prompt", func(t *testing.T) {
		ctx := t.Context()
		action, err := New(ctx, "  test-action  ",
			WithPrompt("  Test prompt  "),
		)
		require.NoError(t, err)
		require.NotNil(t, action)
		assert.Equal(t, "test-action", action.ID)
		assert.Equal(t, "Test prompt", action.Prompt)
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		var nilCtx context.Context
		_, err := New(nilCtx, "test-action",
			WithPrompt("Test"),
		)
		require.Error(t, err)
		assert.EqualError(t, err, "context is required")
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := t.Context()
		_, err := New(ctx, "",
			WithPrompt("Test"),
		)
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when ID is whitespace only", func(t *testing.T) {
		ctx := t.Context()
		_, err := New(ctx, "   ",
			WithPrompt("Test"),
		)
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when prompt is empty", func(t *testing.T) {
		ctx := t.Context()
		_, err := New(ctx, "test-action")
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when prompt is whitespace only", func(t *testing.T) {
		ctx := t.Context()
		_, err := New(ctx, "test-action",
			WithPrompt("   "),
		)
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should create action with all options", func(t *testing.T) {
		ctx := t.Context()
		inputSchema := &schema.Schema{"type": "object"}
		outputSchema := &schema.Schema{"type": "object"}
		withInput := &core.Input{"key": "value"}
		tools := []tool.Config{{ID: "tool1"}}
		onSuccess := &core.SuccessTransition{Next: strPtr("next-task")}
		onError := &core.ErrorTransition{Next: strPtr("error-task")}
		retryPolicy := &core.RetryPolicyConfig{MaximumAttempts: 3, InitialInterval: "1s"}
		action, err := New(ctx, "full-action",
			WithPrompt("Complex action"),
			WithInputSchema(inputSchema),
			WithOutputSchema(outputSchema),
			WithWith(withInput),
			WithTools(tools),
			WithOnSuccess(onSuccess),
			WithOnError(onError),
			WithRetryPolicy(retryPolicy),
			WithTimeout("30s"),
		)
		require.NoError(t, err)
		require.NotNil(t, action)
		assert.NotNil(t, action.InputSchema)
		assert.NotNil(t, action.OutputSchema)
		assert.NotNil(t, action.With)
		assert.Len(t, action.Tools, 1)
		assert.NotNil(t, action.OnSuccess)
		assert.NotNil(t, action.OnError)
		assert.NotNil(t, action.RetryPolicy)
		assert.Equal(t, "30s", action.Timeout)
	})
	t.Run("Should create deep copy", func(t *testing.T) {
		ctx := t.Context()
		originalTools := []tool.Config{{ID: "tool1"}}
		action, err := New(ctx, "copy-test",
			WithPrompt("Test prompt"),
			WithTools(originalTools),
		)
		require.NoError(t, err)
		require.NotNil(t, action)
		originalTools[0].ID = "modified"
		assert.Equal(t, "tool1", action.Tools[0].ID)
	})
	t.Run("Should handle multiple error accumulation", func(t *testing.T) {
		ctx := t.Context()
		_, err := New(ctx, "")
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
		assert.GreaterOrEqual(t, len(buildErr.Errors), 2)
	})
	t.Run("Should work with helper functions for transitions", func(t *testing.T) {
		ctx := t.Context()
		action, err := New(ctx, "transition-test",
			WithPrompt("Test prompt"),
			WithOnSuccess(&core.SuccessTransition{Next: strPtr("success-task")}),
			WithOnError(&core.ErrorTransition{Next: strPtr("error-task")}),
		)
		require.NoError(t, err)
		require.NotNil(t, action)
		require.NotNil(t, action.OnSuccess)
		require.NotNil(t, action.OnError)
		assert.Equal(t, "success-task", *action.OnSuccess.Next)
		assert.Equal(t, "error-task", *action.OnError.Next)
	})
	t.Run("Should work with retry policy", func(t *testing.T) {
		ctx := t.Context()
		action, err := New(ctx, "retry-test",
			WithPrompt("Test prompt"),
			WithRetryPolicy(&core.RetryPolicyConfig{
				MaximumAttempts:    5,
				InitialInterval:    "2s",
				BackoffCoefficient: 2.0,
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, action)
		require.NotNil(t, action.RetryPolicy)
		assert.EqualValues(t, 5, action.RetryPolicy.MaximumAttempts)
		assert.Equal(t, "2s", action.RetryPolicy.InitialInterval)
	})
	t.Run("Should work with timeout", func(t *testing.T) {
		ctx := t.Context()
		action, err := New(ctx, "timeout-test",
			WithPrompt("Test prompt"),
			WithTimeout("1m30s"),
		)
		require.NoError(t, err)
		require.NotNil(t, action)
		assert.Equal(t, "1m30s", action.Timeout)
		duration, parseErr := time.ParseDuration(action.Timeout)
		require.NoError(t, parseErr)
		assert.Equal(t, 90*time.Second, duration)
	})
	t.Run("Should work with input and output schemas", func(t *testing.T) {
		ctx := t.Context()
		inputSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{"type": "string"},
			},
		}
		outputSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"quality": map[string]any{"type": "string"},
			},
		}
		action, err := New(ctx, "schema-test",
			WithPrompt("Test prompt"),
			WithInputSchema(inputSchema),
			WithOutputSchema(outputSchema),
		)
		require.NoError(t, err)
		require.NotNil(t, action)
		require.NotNil(t, action.InputSchema)
		require.NotNil(t, action.OutputSchema)
		assert.Equal(t, "object", (*action.InputSchema)["type"])
		assert.Equal(t, "object", (*action.OutputSchema)["type"])
	})
}

func strPtr(s string) *string {
	return &s
}
