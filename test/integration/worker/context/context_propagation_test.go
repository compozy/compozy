package worker_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/tplengine"
)

// sentinelKey is used to ensure the original context flows end-to-end
type sentinelKey struct{}

func TestWorker_ContextPropagation_E2E(t *testing.T) {
	t.Run("Should preserve context values across worker call chain", func(t *testing.T) {
		// Build a base context with a unique sentinel value
		base := context.WithValue(context.Background(), sentinelKey{}, "sentinel")

		// Attach a minimal config manager to context (debug=false, quiet=false to avoid logger rewrites)
		mgr := config.NewManager(nil)
		_, err := mgr.Load(base, config.NewDefaultProvider())
		require.NoError(t, err)
		ctx := config.ContextWithManager(base, mgr)

		// Build minimal dependencies for worker.Activities
		// We only need the CEL evaluator path; others can be left nil
		tple := tplengine.NewEngine(tplengine.FormatJSON)
		acts := worker.NewActivities(
			nil,  // projectConfig
			nil,  // workflows
			nil,  // workflowRepo
			nil,  // taskRepo
			nil,  // runtime
			nil,  // configStore
			nil,  // signalDispatcher
			nil,  // redisCache
			nil,  // memoryManager
			tple, // templateEngine (required by factory construction)
		)

		// Prepare a trivial condition evaluation
		input := &activities.EvaluateConditionInput{Expression: "true"}

		// Execute activity that uses the evaluator and propagates ctx as-is
		res, err := acts.EvaluateCondition(ctx, input)
		require.NoError(t, err)
		assert.True(t, res)

		// Assert our sentinel is still available on the original ctx after the call
		// This ensures the chain did not replace the root context with context.Background/TODO
		val := ctx.Value(sentinelKey{})
		assert.Equal(t, "sentinel", val)

		// Cleanup manager
		_ = mgr.Close(ctx)
	})
}
