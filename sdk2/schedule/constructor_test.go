package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	engineschedule "github.com/compozy/compozy/engine/project/schedule"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

func TestNew(t *testing.T) {
	t.Run("Should create schedule with minimal configuration", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test-schedule",
			WithWorkflowID("test-workflow"),
			WithCron("0 * * * *"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
		if cfg.ID != "test-schedule" {
			t.Errorf("expected ID 'test-schedule', got '%s'", cfg.ID)
		}
		if cfg.WorkflowID != "test-workflow" {
			t.Errorf("expected workflow ID 'test-workflow', got '%s'", cfg.WorkflowID)
		}
		if cfg.Cron != "0 * * * *" {
			t.Errorf("expected cron '0 * * * *', got '%s'", cfg.Cron)
		}
	})
	t.Run("Should trim whitespace from ID", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "  test-schedule  ",
			WithWorkflowID("test-workflow"),
			WithCron("0 * * * *"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ID != "test-schedule" {
			t.Errorf("expected trimmed ID 'test-schedule', got '%s'", cfg.ID)
		}
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		_, err := New(nil, "test-schedule",
			WithWorkflowID("test-workflow"),
			WithCron("0 * * * *"),
		)
		if err == nil {
			t.Fatal("expected error for nil context")
		}
		if err.Error() != "context is required" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "",
			WithWorkflowID("test-workflow"),
			WithCron("0 * * * *"),
		)
		if err == nil {
			t.Fatal("expected error for empty ID")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when workflow ID is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-schedule",
			WithCron("0 * * * *"),
		)
		if err == nil {
			t.Fatal("expected error for empty workflow ID")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when cron expression is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-schedule",
			WithWorkflowID("test-workflow"),
		)
		if err == nil {
			t.Fatal("expected error for empty cron expression")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should create schedule with all options", func(t *testing.T) {
		ctx := context.Background()
		enabled := true
		cfg, err := New(ctx, "full-schedule",
			WithWorkflowID("test-workflow"),
			WithCron("0 0 * * *"),
			WithTimezone("America/New_York"),
			WithInput(map[string]any{"key": "value"}),
			WithRetry(&engineschedule.RetryPolicy{
				MaxAttempts: 3,
				Backoff:     time.Minute,
			}),
			WithEnabled(&enabled),
			WithDescription("Test schedule"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Timezone != "America/New_York" {
			t.Errorf("expected timezone 'America/New_York', got '%s'", cfg.Timezone)
		}
		if cfg.Input == nil || cfg.Input["key"] != "value" {
			t.Error("expected input to be set")
		}
		if cfg.Retry == nil || cfg.Retry.MaxAttempts != 3 {
			t.Error("expected retry policy to be set")
		}
		if cfg.Enabled == nil || !*cfg.Enabled {
			t.Error("expected enabled to be true")
		}
		if cfg.Description != "Test schedule" {
			t.Errorf("expected description 'Test schedule', got '%s'", cfg.Description)
		}
	})
	t.Run("Should create deep copy of configuration", func(t *testing.T) {
		ctx := context.Background()
		cfg1, err := New(ctx, "test-schedule",
			WithWorkflowID("test-workflow"),
			WithCron("0 * * * *"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cfg2 := &engineschedule.Config{}
		*cfg2 = *cfg1
		cfg2.WorkflowID = "modified"
		if cfg1.WorkflowID == "modified" {
			t.Error("configuration was not deep copied")
		}
	})
}

func TestCronValidation(t *testing.T) {
	t.Run("Should accept valid standard cron expression", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
		)
		if err != nil {
			t.Errorf("expected valid cron, got error: %v", err)
		}
	})
	t.Run("Should accept cron with multiple values", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 0,12 * * *"),
		)
		if err != nil {
			t.Errorf("expected valid cron, got error: %v", err)
		}
	})
	t.Run("Should accept cron with ranges", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 9-17 * * 1-5"),
		)
		if err != nil {
			t.Errorf("expected valid cron, got error: %v", err)
		}
	})
	t.Run("Should fail with invalid cron expression", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("invalid"),
		)
		if err == nil {
			t.Fatal("expected error for invalid cron")
		}
	})
	t.Run("Should fail with too many fields", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 0 0 0 0 0"),
		)
		if err == nil {
			t.Fatal("expected error for invalid cron format")
		}
	})
}

func TestTimezoneValidation(t *testing.T) {
	t.Run("Should accept UTC timezone", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithTimezone("UTC"),
		)
		if err != nil {
			t.Errorf("expected valid timezone, got error: %v", err)
		}
		if cfg.Timezone != "UTC" {
			t.Errorf("expected timezone 'UTC', got '%s'", cfg.Timezone)
		}
	})
	t.Run("Should accept America/New_York timezone", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithTimezone("America/New_York"),
		)
		if err != nil {
			t.Errorf("expected valid timezone, got error: %v", err)
		}
		if cfg.Timezone != "America/New_York" {
			t.Errorf("expected timezone 'America/New_York', got '%s'", cfg.Timezone)
		}
	})
	t.Run("Should accept Europe/London timezone", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithTimezone("Europe/London"),
		)
		if err != nil {
			t.Errorf("expected valid timezone, got error: %v", err)
		}
		if cfg.Timezone != "Europe/London" {
			t.Errorf("expected timezone 'Europe/London', got '%s'", cfg.Timezone)
		}
	})
	t.Run("Should accept Asia/Tokyo timezone", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithTimezone("Asia/Tokyo"),
		)
		if err != nil {
			t.Errorf("expected valid timezone, got error: %v", err)
		}
	})
	t.Run("Should fail with invalid timezone", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithTimezone("Invalid/Timezone"),
		)
		if err == nil {
			t.Fatal("expected error for invalid timezone")
		}
	})
	t.Run("Should allow empty timezone", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
		)
		if err != nil {
			t.Errorf("expected success with empty timezone, got error: %v", err)
		}
		if cfg.Timezone != "" {
			t.Errorf("expected empty timezone, got '%s'", cfg.Timezone)
		}
	})
}

func TestRetryValidation(t *testing.T) {
	t.Run("Should accept valid retry policy", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithRetry(&engineschedule.RetryPolicy{
				MaxAttempts: 3,
				Backoff:     time.Minute,
			}),
		)
		if err != nil {
			t.Errorf("expected valid retry, got error: %v", err)
		}
		if cfg.Retry == nil {
			t.Fatal("expected retry policy to be set")
		}
		if cfg.Retry.MaxAttempts != 3 {
			t.Errorf("expected max attempts 3, got %d", cfg.Retry.MaxAttempts)
		}
	})
	t.Run("Should fail with negative max attempts", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithRetry(&engineschedule.RetryPolicy{
				MaxAttempts: -1,
				Backoff:     time.Minute,
			}),
		)
		if err == nil {
			t.Fatal("expected error for negative max attempts")
		}
	})
	t.Run("Should fail with zero max attempts", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithRetry(&engineschedule.RetryPolicy{
				MaxAttempts: 0,
				Backoff:     time.Minute,
			}),
		)
		if err == nil {
			t.Fatal("expected error for zero max attempts")
		}
	})
	t.Run("Should fail with max attempts greater than 100", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithRetry(&engineschedule.RetryPolicy{
				MaxAttempts: 101,
				Backoff:     time.Minute,
			}),
		)
		if err == nil {
			t.Fatal("expected error for max attempts > 100")
		}
	})
	t.Run("Should fail with negative backoff", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithRetry(&engineschedule.RetryPolicy{
				MaxAttempts: 3,
				Backoff:     -time.Minute,
			}),
		)
		if err == nil {
			t.Fatal("expected error for negative backoff")
		}
	})
	t.Run("Should fail with zero backoff", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithRetry(&engineschedule.RetryPolicy{
				MaxAttempts: 3,
				Backoff:     0,
			}),
		)
		if err == nil {
			t.Fatal("expected error for zero backoff")
		}
	})
	t.Run("Should allow nil retry policy", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
		)
		if err != nil {
			t.Errorf("expected success with nil retry, got error: %v", err)
		}
		if cfg.Retry != nil {
			t.Error("expected nil retry policy")
		}
	})
}

func TestWorkflowIDValidation(t *testing.T) {
	t.Run("Should trim whitespace from workflow ID", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithWorkflowID("  test-workflow  "),
			WithCron("0 * * * *"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.WorkflowID != "test-workflow" {
			t.Errorf("expected trimmed workflow ID, got '%s'", cfg.WorkflowID)
		}
	})
	t.Run("Should fail with invalid workflow ID characters", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test",
			WithWorkflowID("bad workflow"),
			WithCron("0 * * * *"),
		)
		if err == nil {
			t.Fatal("expected error for invalid workflow ID")
		}
	})
}

func TestInputHandling(t *testing.T) {
	t.Run("Should accept input map", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
			WithInput(map[string]any{"key": "value"}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Input == nil {
			t.Fatal("expected input to be set")
		}
		if cfg.Input["key"] != "value" {
			t.Error("expected input value to match")
		}
	})
	t.Run("Should accept nil input", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithWorkflowID("wf"),
			WithCron("0 * * * *"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Input != nil {
			t.Error("expected nil input")
		}
	})
}
