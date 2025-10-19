package worker_test

import (
	"context"
	"testing"
	"time"

	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/compozy/compozy/test/integration/worker/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// sentinelKey is used to ensure the original context flows end-to-end
type sentinelKey struct{}

func TestWorker_ContextPropagation_E2E(t *testing.T) {
	t.Run("Should preserve context values across worker call chain", func(t *testing.T) {
		base := context.WithValue(t.Context(), sentinelKey{}, "sentinel")
		mgr := config.NewManager(t.Context(), nil)
		_, err := mgr.Load(base, config.NewDefaultProvider())
		require.NoError(t, err)
		ctx := config.ContextWithManager(base, mgr)
		tple := tplengine.NewEngine(tplengine.FormatJSON)
		toolEnv := toolenv.New(nil, nil, nil)
		acts, err := worker.NewActivities(
			ctx,
			nil, // projectConfig
			nil, // workflows
			nil, // workflowRepo
			nil, // taskRepo
			&helpers.NoopUsageMetrics{},
			providermetrics.Nop(),
			nil,  // runtime
			nil,  // configStore
			nil,  // signalDispatcher
			nil,  // redisCache
			nil,  // memoryManager
			tple, // templateEngine (required by factory construction)
			toolEnv,
		)
		require.NoError(t, err)
		input := &activities.EvaluateConditionInput{Expression: "true"}
		res, err := acts.EvaluateCondition(ctx, input)
		require.NoError(t, err)
		assert.True(t, res)
		val := ctx.Value(sentinelKey{})
		assert.Equal(t, "sentinel", val)
		_ = mgr.Close(ctx)
	})
}

// readRuntimeEnvActivity returns cfg.Runtime.Environment from activity context
func readRuntimeEnvActivity(ctx context.Context) (string, error) {
	cfg := config.FromContext(ctx)
	return cfg.Runtime.Environment, nil
}

// workflow calling readRuntimeEnvActivity and returning the value
func cfgPropagationWorkflow(ctx workflow.Context) (string, error) {
	opts := workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Second}
	ctx = workflow.WithActivityOptions(ctx, opts)
	var env string
	if err := workflow.ExecuteActivity(ctx, readRuntimeEnvActivity).Get(ctx, &env); err != nil {
		return "", err
	}
	return env, nil
}

func TestWorker_GlobalConfigConsistency_Temporal(t *testing.T) {
	t.Run("Should preserve pkg/config value inside Temporal activity", func(t *testing.T) {
		t.Setenv("RUNTIME_ENVIRONMENT", "staging")
		base := t.Context()
		mgr := config.NewManager(t.Context(), nil)
		_, err := mgr.Load(base, config.NewDefaultProvider(), config.NewEnvProvider())
		require.NoError(t, err)
		ctx := config.ContextWithManager(base, mgr)
		suite := &testsuite.WorkflowTestSuite{}
		helper := helpers.NewTemporalHelper(t, suite, "test-task-queue")
		defer helper.Cleanup(t)
		helper.RegisterActivity(readRuntimeEnvActivity)
		helper.RegisterWorkflow(cfgPropagationWorkflow)
		helper.ExecuteWorkflowSync(cfgPropagationWorkflow)
		require.True(t, helper.IsWorkflowCompleted())
		var got string
		require.NoError(t, helper.GetWorkflowResult(&got))
		want := config.FromContext(ctx).Runtime.Environment
		assert.Equal(t, want, got)
		_ = mgr.Close(ctx)
	})
}
