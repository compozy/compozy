package toolcontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlannerDisableFlag(t *testing.T) {
	ctx := context.Background()
	require.False(t, PlannerToolsDisabled(ctx))
	disabledCtx := DisablePlannerTools(ctx)
	require.True(t, PlannerToolsDisabled(disabledCtx))
	require.False(t, PlannerToolsDisabled(context.Background()))
}

func TestAgentOrchestratorDepth(t *testing.T) {
	ctx := context.Background()
	t.Run("ShouldDefaultToZero", func(t *testing.T) {
		require.Equal(t, 0, AgentOrchestratorDepth(ctx))
	})
	t.Run("ShouldIncrementDepth", func(t *testing.T) {
		levelOne := IncrementAgentOrchestratorDepth(ctx)
		require.Equal(t, 1, AgentOrchestratorDepth(levelOne))
		levelTwo := IncrementAgentOrchestratorDepth(levelOne)
		require.Equal(t, 2, AgentOrchestratorDepth(levelTwo))
	})
}
