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

func TestSignalBuilderBuildSend(t *testing.T) {
	t.Parallel()

	cfg, err := NewSignal("dispatch").
		WithSignalID("order-ready").
		WithMode(SignalModeSend).
		WithData(map[string]any{"order_id": 42}).
		OnSuccess("next-task").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, enginetask.TaskTypeSignal, cfg.Type)
	require.NotNil(t, cfg.Signal)
	assert.Equal(t, "order-ready", cfg.Signal.ID)
	assert.Equal(t, 42, cfg.Signal.Payload["order_id"])

	require.NotNil(t, cfg.OnSuccess)
	require.NotNil(t, cfg.OnSuccess.Next)
	assert.Equal(t, "next-task", *cfg.OnSuccess.Next)
}

func TestSignalBuilderBuildWait(t *testing.T) {
	t.Parallel()

	cfg, err := NewSignal("waiter").
		Wait("inventory-ready").
		WithTimeout(5 * time.Minute).
		OnError("timeout-handler").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, enginetask.TaskTypeWait, cfg.Type)
	assert.Nil(t, cfg.Signal)
	assert.Equal(t, "inventory-ready", cfg.WaitFor)

	timeout, parseErr := core.ParseHumanDuration(cfg.Timeout)
	require.NoError(t, parseErr)
	assert.Equal(t, 5*time.Minute, timeout)

	require.NotNil(t, cfg.OnError)
	require.NotNil(t, cfg.OnError.Next)
	assert.Equal(t, "timeout-handler", *cfg.OnError.Next)
}

func TestSignalBuilderSendConvenience(t *testing.T) {
	t.Parallel()

	cfg, err := NewSignal("sender").
		Send("data-ready", map[string]any{"payload": "value"}).
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, enginetask.TaskTypeSignal, cfg.Type)
	require.NotNil(t, cfg.Signal)
	assert.Equal(t, "data-ready", cfg.Signal.ID)
	assert.Equal(t, "value", cfg.Signal.Payload["payload"])
}

func TestSignalBuilderWaitConvenience(t *testing.T) {
	t.Parallel()

	cfg, err := NewSignal("receiver").
		Wait("approval-required").
		WithTimeout(time.Minute).
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, enginetask.TaskTypeWait, cfg.Type)
	assert.Equal(t, "approval-required", cfg.WaitFor)
}

func TestSignalBuilderMissingSignalID(t *testing.T) {
	t.Parallel()

	_, err := NewSignal("   ").
		WithMode(SignalModeSend).
		WithData(map[string]any{"sample": true}).
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.NotNil(t, buildErr)
	assert.Contains(t, err.Error(), "signal_id")
}

func TestSignalBuilderSendWithoutData(t *testing.T) {
	t.Parallel()

	_, err := NewSignal("sender").
		WithSignalID("ready").
		WithMode(SignalModeSend).
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.NotNil(t, buildErr)
	assert.Contains(t, err.Error(), "requires data payload")
}

func TestSignalBuilderWaitWithDataFails(t *testing.T) {
	t.Parallel()

	_, err := NewSignal("receiver").
		Wait("ready").
		WithData(map[string]any{"unexpected": true}).
		WithTimeout(time.Second).
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.NotNil(t, buildErr)
	assert.Contains(t, err.Error(), "does not accept data payload")
}

func TestSignalBuilderInvalidMode(t *testing.T) {
	t.Parallel()

	_, err := NewSignal("signal").
		WithSignalID("mode-test").
		WithMode(SignalMode("invalid")).
		WithData(map[string]any{"value": 1}).
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.NotNil(t, buildErr)
	assert.Contains(t, err.Error(), "invalid signal mode")
}

func TestSignalBuilderAggregatesErrors(t *testing.T) {
	t.Parallel()

	_, err := NewSignal("   ").
		WithMode(SignalMode("invalid")).
		WithData(nil).
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotNil(t, buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 3)
}
