package agentaction

import (
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

func TestNew(t *testing.T) {
	t.Run("Should create action with minimal configuration", func(t *testing.T) {
		ctx := t.Context()
		action, err := New(ctx, "test-action",
			WithPrompt("Test prompt"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action == nil {
			t.Fatal("expected action, got nil")
		}
		if action.ID != "test-action" {
			t.Errorf("expected ID 'test-action', got '%s'", action.ID)
		}
		if action.Prompt != "Test prompt" {
			t.Errorf("expected prompt, got '%s'", action.Prompt)
		}
	})
	t.Run("Should trim whitespace from ID and prompt", func(t *testing.T) {
		ctx := t.Context()
		action, err := New(ctx, "  test-action  ",
			WithPrompt("  Test prompt  "),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action.ID != "test-action" {
			t.Errorf("expected trimmed ID 'test-action', got '%s'", action.ID)
		}
		if action.Prompt != "Test prompt" {
			t.Errorf("expected trimmed prompt, got '%s'", action.Prompt)
		}
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		_, err := New(nil, "test-action",
			WithPrompt("Test"),
		)
		if err == nil {
			t.Fatal("expected error for nil context")
		}
		if err.Error() != "context is required" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := t.Context()
		_, err := New(ctx, "",
			WithPrompt("Test"),
		)
		if err == nil {
			t.Fatal("expected error for empty ID")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when ID is whitespace only", func(t *testing.T) {
		ctx := t.Context()
		_, err := New(ctx, "   ",
			WithPrompt("Test"),
		)
		if err == nil {
			t.Fatal("expected error for whitespace-only ID")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when prompt is empty", func(t *testing.T) {
		ctx := t.Context()
		_, err := New(ctx, "test-action")
		if err == nil {
			t.Fatal("expected error for empty prompt")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when prompt is whitespace only", func(t *testing.T) {
		ctx := t.Context()
		_, err := New(ctx, "test-action",
			WithPrompt("   "),
		)
		if err == nil {
			t.Fatal("expected error for whitespace-only prompt")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action.InputSchema == nil {
			t.Error("expected input schema to be set")
		}
		if action.OutputSchema == nil {
			t.Error("expected output schema to be set")
		}
		if action.With == nil {
			t.Error("expected with to be set")
		}
		if len(action.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(action.Tools))
		}
		if action.OnSuccess == nil {
			t.Error("expected on_success to be set")
		}
		if action.OnError == nil {
			t.Error("expected on_error to be set")
		}
		if action.RetryPolicy == nil {
			t.Error("expected retry_policy to be set")
		}
		if action.Timeout != "30s" {
			t.Errorf("expected timeout '30s', got '%s'", action.Timeout)
		}
	})
	t.Run("Should create deep copy", func(t *testing.T) {
		ctx := t.Context()
		originalTools := []tool.Config{{ID: "tool1"}}
		action, err := New(ctx, "copy-test",
			WithPrompt("Test prompt"),
			WithTools(originalTools),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		originalTools[0].ID = "modified"
		if action.Tools[0].ID == "modified" {
			t.Error("expected deep copy, but got shallow copy")
		}
	})
	t.Run("Should handle multiple error accumulation", func(t *testing.T) {
		ctx := t.Context()
		_, err := New(ctx, "")
		if err == nil {
			t.Fatal("expected error for invalid configuration")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Fatalf("expected BuildError, got %T", err)
		}
		if len(buildErr.Errors) < 2 {
			t.Errorf("expected at least 2 errors (ID and prompt), got %d", len(buildErr.Errors))
		}
	})
	t.Run("Should work with helper functions for transitions", func(t *testing.T) {
		ctx := t.Context()
		action, err := New(ctx, "transition-test",
			WithPrompt("Test prompt"),
			WithOnSuccess(&core.SuccessTransition{Next: strPtr("success-task")}),
			WithOnError(&core.ErrorTransition{Next: strPtr("error-task")}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action.OnSuccess == nil || *action.OnSuccess.Next != "success-task" {
			t.Error("expected success transition to be set correctly")
		}
		if action.OnError == nil || *action.OnError.Next != "error-task" {
			t.Error("expected error transition to be set correctly")
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action.RetryPolicy == nil {
			t.Fatal("expected retry policy to be set")
		}
		if action.RetryPolicy.MaximumAttempts != 5 {
			t.Errorf("expected max attempts 5, got %d", action.RetryPolicy.MaximumAttempts)
		}
		if action.RetryPolicy.InitialInterval != "2s" {
			t.Errorf("expected initial interval '2s', got '%s'", action.RetryPolicy.InitialInterval)
		}
	})
	t.Run("Should work with timeout", func(t *testing.T) {
		ctx := t.Context()
		action, err := New(ctx, "timeout-test",
			WithPrompt("Test prompt"),
			WithTimeout("1m30s"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action.Timeout != "1m30s" {
			t.Errorf("expected timeout '1m30s', got '%s'", action.Timeout)
		}
		duration, parseErr := time.ParseDuration(action.Timeout)
		if parseErr != nil {
			t.Errorf("timeout should be parseable: %v", parseErr)
		}
		if duration != 90*time.Second {
			t.Errorf("expected duration 90s, got %v", duration)
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action.InputSchema == nil {
			t.Error("expected input schema to be set")
		}
		if (*action.InputSchema)["type"] != "object" {
			t.Error("expected input schema type to be object")
		}
		if action.OutputSchema == nil {
			t.Error("expected output schema to be set")
		}
		if (*action.OutputSchema)["type"] != "object" {
			t.Error("expected output schema type to be object")
		}
	})
}

func strPtr(s string) *string {
	return &s
}
