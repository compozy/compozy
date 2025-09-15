package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	toolcfg "github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
	"github.com/compozy/compozy/test/integration/worker/helpers"
)

// TestToolInheritance_Runtime verifies that tool resolution runs during actual execution
// by creating a real Temporal workflow, exercising ExecuteTask.createLLMService path,
// and persisting state into the database. This ensures integration beyond unit resolver checks.
func TestToolInheritance_Runtime(t *testing.T) {
	t.Run("Should execute workflow successfully with inherited project tools", func(t *testing.T) {
		// Initialize global app config used by activities
		// Build a very small workflow with one basic task using an agent without explicit tools
		basicTask := helpers.CreateBasicTaskConfig("runtime-basic-task")
		fixture := helpers.CreateBasicWorkflowFixture("tool-inheritance-runtime", basicTask)

		// Prepare agent without tools (to trigger inheritance path)
		agentCfg := helpers.CreateBasicAgentConfig()
		// Make sure agent has no explicit tools and set CWD
		agentCfg.Tools = nil
		_ = agentCfg.SetCWD(".")

		// Attach agent to basic task and set an action ID that exists in the agent
		basicTask.Agent = agentCfg
		basicTask.Action = "process_message"

		// Create workflow config
		wfCfg := &workflow.Config{
			ID:    fixture.Workflow.ID,
			Tasks: []task.Config{*basicTask},
		}
		_ = wfCfg.SetCWD(".")

		// Create project config with shared tools to be inherited
		prjTools := []toolcfg.Config{
			CreateTestTool("shared-a", "Shared tool A"),
			CreateTestTool("shared-b", "Shared tool B"),
		}
		prjCfg := CreateTestProjectConfig(prjTools)

		// Set up repositories and runtime similar to worker helpers
		ctx := context.Background()
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		t.Cleanup(cleanup)

		// Create test suite and worker environment
		var testSuite testsuite.WorkflowTestSuite
		env := testSuite.NewTestWorkflowEnvironment()

		// Create runtime and config store
		rt := helpers.CreateMockRuntime(t)
		cfgStore := services.NewTestConfigStore(t)
		t.Cleanup(func() { _ = cfgStore.Close() })

		// Build Activities with our custom project/workflow configs
		activities, err := worker.NewActivities(
			ctx,
			prjCfg,
			[]*workflow.Config{wfCfg},
			workflowRepo,
			taskRepo,
			rt,
			cfgStore,
			nil, // signal dispatcher
			nil, // redis cache
			nil, // memory manager
			tplengine.NewEngine(tplengine.FormatJSON),
		)
		require.NoError(t, err)

		// Register activities (common set used by worker tests)
		helpers.RegisterCommonActivities(env, activities)

		// Prepare workflow input
		workflowExecID := core.MustNewID()
		temporalInput := worker.WorkflowInput{
			WorkflowID:     wfCfg.ID,
			WorkflowExecID: workflowExecID,
			InitialTaskID:  basicTask.ID,
		}

		// Execute workflow
		env.ExecuteWorkflow(worker.CompozyWorkflow, temporalInput)
		require.True(t, env.IsWorkflowCompleted())
		require.NoError(t, env.GetWorkflowError())

		// Verify final state persisted
		state, err := workflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, state)
		require.Equal(t, string(core.StatusSuccess), string(state.Status))
	})
}
