package toolcontext

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/infra/server/router/routertest"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task/testutil"
	"github.com/stretchr/testify/require"
)

func TestAppStateRoundTrip(t *testing.T) {
	ctx := context.Background()
	state := routertest.NewTestAppState(t)
	resultCtx := WithAppState(ctx, state)
	got, ok := GetAppState(resultCtx)
	require.True(t, ok)
	require.Equal(t, state, got)
	_, ok = GetAppState(context.Background())
	require.False(t, ok)
}

func TestTaskRepoRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := testutil.NewInMemoryRepo()
	resultCtx := WithTaskRepo(ctx, repo)
	got, ok := GetTaskRepo(resultCtx)
	require.True(t, ok)
	require.Equal(t, repo, got)
	_, ok = GetTaskRepo(context.Background())
	require.False(t, ok)
}

func TestResourceStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := resources.NewMemoryResourceStore()
	resultCtx := WithResourceStore(ctx, store)
	got, ok := GetResourceStore(resultCtx)
	require.True(t, ok)
	require.Equal(t, store, got)
	_, ok = GetResourceStore(context.Background())
	require.False(t, ok)
}

func TestPlannerDisableFlag(t *testing.T) {
	ctx := context.Background()
	require.False(t, PlannerToolsDisabled(ctx))
	disabledCtx := DisablePlannerTools(ctx)
	require.True(t, PlannerToolsDisabled(disabledCtx))
	require.False(t, PlannerToolsDisabled(context.Background()))
}
