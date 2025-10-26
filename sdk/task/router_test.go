package task

import (
	"testing"

	enginetask "github.com/compozy/compozy/engine/task"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouterBuilderBuildSuccess(t *testing.T) {
	t.Parallel()

	cfg, err := NewRouter("route-decider").
		WithCondition("input.kind").
		AddRoute("invoice", "handle-invoice").
		AddRoute("receipt", "handle-receipt").
		WithDefault("fallback-handler").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, enginetask.TaskTypeRouter, cfg.Type)
	assert.Equal(t, "input.kind", cfg.Condition)

	routes := cfg.Routes
	require.Len(t, routes, 3)

	invoice, ok := routes["invoice"].(string)
	require.True(t, ok)
	require.Equal(t, "handle-invoice", invoice)

	receipt, ok := routes["receipt"].(string)
	require.True(t, ok)
	require.Equal(t, "handle-receipt", receipt)

	defaultRoute, ok := routes[defaultRouteKey].(string)
	require.True(t, ok)
	assert.Equal(t, "fallback-handler", defaultRoute)
}

func TestRouterBuilderTrimsInputs(t *testing.T) {
	t.Parallel()

	cfg, err := NewRouter("  router  ").
		WithCondition("  input.route  ").
		AddRoute("  approved  ", "  approve-task  ").
		Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "router", cfg.ID)
	assert.Equal(t, "input.route", cfg.Condition)
	value, ok := cfg.Routes["approved"].(string)
	require.True(t, ok)
	assert.Equal(t, "approve-task", value)
}

func TestRouterBuilderBuildFailsWithoutRoutes(t *testing.T) {
	t.Parallel()

	_, err := NewRouter("empty").
		WithCondition("input.route").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "router tasks require at least one route")
}

func TestRouterBuilderBuildFailsWithEmptyTaskID(t *testing.T) {
	t.Parallel()

	_, err := NewRouter("bad").
		WithCondition("input.status").
		AddRoute("approved", "   ").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "task id cannot be empty")
	assert.Contains(t, err.Error(), "router tasks require at least one route")
}

func TestRouterBuilderBuildFailsWithDuplicateRoutes(t *testing.T) {
	t.Parallel()

	_, err := NewRouter("duplicated").
		WithCondition("input.status").
		AddRoute("approved", "approve").
		AddRoute("approved", "other").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "duplicate route condition")
}

func TestRouterBuilderBuildAggregatesErrors(t *testing.T) {
	t.Parallel()

	_, err := NewRouter("   ").
		WithCondition("   ").
		AddRoute("   ", "   ").
		WithDefault("   ").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotNil(t, buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 3)
}
