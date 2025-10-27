package schedule

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	engineschedule "github.com/compozy/compozy/engine/workflow/schedule"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/testutil"
)

func TestNewTrimsIdentifier(t *testing.T) {
	builder := New("  sched-123  ")
	require.Equal(t, "sched-123", builder.config.ID)
	require.Empty(t, builder.errors)
}

func TestNilBuilderMethodsReturnNil(t *testing.T) {
	var builder *Builder
	require.Nil(t, builder.WithCron("cron"))
	require.Nil(t, builder.WithWorkflow("wf"))
	require.Nil(t, builder.WithInput(map[string]any{"k": "v"}))
	require.Nil(t, builder.WithRetry(1, time.Second))
	require.Nil(t, builder.WithTimezone("UTC"))
	require.Nil(t, builder.WithDescription("desc"))
	require.Nil(t, builder.WithEnabled(true))
	ctx := testutil.NewTestContext(t)
	cfg, err := builder.Build(ctx)
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "schedule builder is required")
}

func TestWithInputClonesMap(t *testing.T) {
	builder := New("sched").
		WithWorkflow("wf").
		WithCron("0 * * * *")
	input := map[string]any{"value": "initial"}
	builder.WithInput(input)
	ctx := testutil.NewTestContext(t)
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Equal(t, "initial", cfg.Input["value"])
	input["value"] = "mutated"
	require.Equal(t, "initial", cfg.Input["value"])
}

func TestWithInputNilClearsConfig(t *testing.T) {
	builder := New("sched").
		WithWorkflow("wf").
		WithCron("0 * * * *")
	builder.WithInput(map[string]any{"k": "v"})
	builder.WithInput(nil)
	ctx := testutil.NewTestContext(t)
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Nil(t, cfg.Input)
}

func TestWithTimezoneAndEnabled(t *testing.T) {
	builder := New("sched").
		WithWorkflow("wf").
		WithCron("0 * * * *").
		WithTimezone("  UTC  ").
		WithEnabled(true)
	ctx := testutil.NewTestContext(t)
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Equal(t, "UTC", cfg.Timezone)
	require.NotNil(t, cfg.Enabled)
	require.True(t, *cfg.Enabled)
	require.True(t, *builder.config.Enabled)
	cfg.Enabled = nil
	require.NotNil(t, builder.config.Enabled)
}

func TestWithCronEmptyAddsError(t *testing.T) {
	builder := New("sched").
		WithWorkflow("wf").
		WithCron(" ")
	ctx := testutil.NewTestContext(t)
	cfg, err := builder.Build(ctx)
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cron expression cannot be empty")
}

func TestWithTimezoneEmptyClearsValue(t *testing.T) {
	builder := New("sched").
		WithWorkflow("wf").
		WithCron("0 * * * *")
	builder.WithTimezone("UTC")
	builder.WithTimezone("  ")
	ctx := testutil.NewTestContext(t)
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Empty(t, cfg.Timezone)
}

func TestBuildValidatesInputs(t *testing.T) {
	builder := New(" ").
		WithWorkflow(" ").
		WithCron("invalid").
		WithRetry(0, 0)
	ctx := testutil.NewTestContext(t)
	cfg, err := builder.Build(ctx)
	require.Nil(t, cfg)
	require.Error(t, err)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Greater(t, len(buildErr.Errors), 0)
	require.Contains(t, buildErr.Error(), "id")
	require.Contains(t, buildErr.Error(), "workflow id")
	require.Contains(t, buildErr.Error(), "cron")
}

func TestBuildRejectsInvalidWorkflowID(t *testing.T) {
	builder := New("sched").
		WithWorkflow("bad id").
		WithCron("0 * * * *")
	ctx := testutil.NewTestContext(t)
	cfg, err := builder.Build(ctx)
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "workflow id is invalid")
}

func TestBuildFailsWithNilContext(t *testing.T) {
	builder := New("sched").
		WithWorkflow("wf").
		WithCron("0 * * * *")
	var missingCtx context.Context
	cfg, err := builder.Build(missingCtx)
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "context is required")
}

func TestBuildClonesResult(t *testing.T) {
	builder := New("sched").
		WithWorkflow("wf").
		WithCron("0 * * * *").
		WithDescription("desc").
		WithRetry(3, time.Minute)
	builder.config.Input = map[string]any{"k": "v"}
	ctx := testutil.NewTestContext(t)
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.IsType(t, &engineschedule.Config{}, cfg)
	cfg.ID = "mutated"
	cfg.Input["k"] = "changed"
	require.Equal(t, "sched", builder.config.ID)
	require.Equal(t, "v", builder.config.Input["k"])
}

func TestBuildCloneFailure(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	builder := New("sched").
		WithWorkflow("wf").
		WithCron("0 * * * *")
	original := cloneScheduleConfig
	cloneScheduleConfig = func(*engineschedule.Config) (*engineschedule.Config, error) {
		return nil, fmt.Errorf("clone error")
	}
	t.Cleanup(func() {
		cloneScheduleConfig = original
	})
	cfg, err := builder.Build(ctx)
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to clone schedule config")
}
