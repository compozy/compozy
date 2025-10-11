package toolcontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlannerDisableFlag(t *testing.T) {
	t.Run("ShouldDefaultToEnabled", func(t *testing.T) {
		ctx := context.Background()
		require.False(t, PlannerToolsDisabled(ctx))
	})
	t.Run("ShouldBeDisabledWhenContextDisabled", func(t *testing.T) {
		ctx := context.Background()
		disabledCtx := DisablePlannerTools(ctx)
		require.True(t, PlannerToolsDisabled(disabledCtx))
	})
	t.Run("ShouldNotLeakToNewBackgroundContext", func(t *testing.T) {
		require.False(t, PlannerToolsDisabled(context.Background()))
	})
}
