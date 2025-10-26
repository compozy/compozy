package task

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitBuilderBuildWithDuration(t *testing.T) {
	t.Parallel()

	cfg, err := NewWait("pause-briefly").
		WithSignal("wait::pause").
		WithDuration(30 * time.Second).
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "pause-briefly", cfg.ID)
	assert.Equal(t, enginetask.TaskTypeWait, cfg.Type)
	assert.Equal(t, "wait::pause", cfg.WaitFor)
	assert.Equal(t, defaultWaitCondition, cfg.Condition)

	duration, parseErr := core.ParseHumanDuration(cfg.Timeout)
	require.NoError(t, parseErr)
	assert.Equal(t, 30*time.Second, duration)
}

func TestWaitBuilderBuildWithCondition(t *testing.T) {
	t.Parallel()

	expression := `signal.payload.status == "approved"`
	cfg, err := NewWait("wait-approval").
		WithSignal("approval-signal").
		WithCondition(expression).
		WithTimeout(2 * time.Minute).
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, enginetask.TaskTypeWait, cfg.Type)
	assert.Equal(t, "approval-signal", cfg.WaitFor)
	assert.Equal(t, expression, cfg.Condition)

	timeout, parseErr := core.ParseHumanDuration(cfg.Timeout)
	require.NoError(t, parseErr)
	assert.Equal(t, 2*time.Minute, timeout)
}

func TestWaitBuilderBuildFailsWithDurationAndCondition(t *testing.T) {
	t.Parallel()

	_, err := NewWait("conflict").
		WithSignal("conflict-signal").
		WithDuration(10 * time.Second).
		WithCondition("signal.payload.ready").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "cannot specify both duration and condition")
}

func TestWaitBuilderBuildFailsWithoutDurationOrCondition(t *testing.T) {
	t.Parallel()

	_, err := NewWait("incomplete").
		WithSignal("incomplete").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "require either a duration or a condition")
}

func TestWaitBuilderBuildFailsWithNegativeDuration(t *testing.T) {
	t.Parallel()

	_, err := NewWait("negative").
		WithSignal("negative").
		WithDuration(-5 * time.Second).
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "duration must be positive")
}

func TestWaitBuilderAggregatesErrors(t *testing.T) {
	t.Parallel()

	_, err := NewWait("   ").
		WithSignal("   ").
		WithCondition("   ").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotNil(t, buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 3)
}
