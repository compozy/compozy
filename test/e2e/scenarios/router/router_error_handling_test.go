package router

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/e2e/utils"
	testhelpers "github.com/compozy/compozy/test/helpers"
)

func TestRouterErrorHandling(t *testing.T) {
	ctx := context.Background()

	t.Run("Should handle invalid route condition gracefully", func(t *testing.T) {
		builder := testhelpers.NewTestConfigBuilder(t).
			WithTestID("invalid-condition-test").
			WithSimpleTask("start-task", "analyze", "Start analysis")

		baseConfig := builder.Build(t)

		routerWorkflow := &wf.Config{
			ID:      "invalid-condition-test",
			Version: "1.0.0",
			Opts: wf.Opts{
				Env: &core.EnvMap{
					"user_type": "admin",
				},
			},
			Tasks: []task.Config{
				baseConfig.WorkflowConfig.Tasks[0],
				{
					BaseConfig: task.BaseConfig{
						ID:   "router-task",
						Type: task.TaskTypeRouter,
					},
					RouterTask: task.RouterTask{
						Condition: "{{ .invalid.template.reference }}", // Invalid template
						Routes: map[string]any{
							"admin": "admin-task",
							"user":  "user-task",
						},
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "admin-task",
						Type: task.TaskTypeBasic,
						Agent: testhelpers.CreateTestAgentConfigWithAction(
							"admin-agent",
							"Handle admin tasks",
							"process",
							"Handle admin request",
						),
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
			},
			Agents: []agent.Config{
				*testhelpers.CreateTestAgentConfigWithAction("start-agent", "Start agent", "analyze", "Start analysis"),
				*testhelpers.CreateTestAgentConfigWithAction("admin-agent", "Handle admin tasks", "process", "Handle admin request"),
			},
		}

		// Set up task transitions
		routerWorkflow.Tasks[0].Agent = testhelpers.CreateTestAgentConfigWithAction(
			"start-agent",
			"Start agent",
			"analyze",
			"Start analysis",
		)
		routerWorkflow.Tasks[0].OnSuccess = &core.SuccessTransition{
			Next: stringPtr("router-task"),
		}

		baseConfig.WorkflowConfig = routerWorkflow

		workerBuilder := utils.NewWorkerTestBuilder(t).
			WithTaskQueue("invalid-condition-test-queue").
			WithActivityTimeout(testhelpers.DefaultTestTimeout)

		config := workerBuilder.Build(t)
		config.ContainerTestConfig = baseConfig

		helper := utils.NewWorkerTestHelper(t, config)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow("invalid-condition-test", &core.Input{})
		require.NotEmpty(t, workflowExecID)

		// Verify workflow handles error gracefully
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusFailed, 10*time.Second)

		// Note: router-task fails during normalization before task state creation,
		// so no task state record exists to verify. The workflow failure is sufficient
		// to verify the error handling behavior.
	})

	t.Run("Should handle non-existent route gracefully", func(t *testing.T) {
		builder := testhelpers.NewTestConfigBuilder(t).
			WithTestID("missing-route-test").
			WithSimpleTask("start-task", "analyze", "Start analysis")

		baseConfig := builder.Build(t)

		routerWorkflow := &wf.Config{
			ID:      "missing-route-test",
			Version: "1.0.0",
			Opts: wf.Opts{
				Env: &core.EnvMap{
					"user_type": "super_admin", // This route doesn't exist
				},
			},
			Tasks: []task.Config{
				baseConfig.WorkflowConfig.Tasks[0],
				{
					BaseConfig: task.BaseConfig{
						ID:   "router-task",
						Type: task.TaskTypeRouter,
					},
					RouterTask: task.RouterTask{
						Condition: "{{ .env.user_type }}",
						Routes: map[string]any{
							"admin": "admin-task",
							"user":  "user-task",
							// "super_admin" route missing
						},
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "admin-task",
						Type: task.TaskTypeBasic,
						Agent: testhelpers.CreateTestAgentConfigWithAction(
							"admin-agent",
							"Handle admin tasks",
							"process",
							"Handle admin request",
						),
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
			},
			Agents: []agent.Config{
				*testhelpers.CreateTestAgentConfigWithAction("start-agent", "Start agent", "analyze", "Start analysis"),
				*testhelpers.CreateTestAgentConfigWithAction("admin-agent", "Handle admin tasks", "process", "Handle admin request"),
			},
		}

		// Set up task transitions
		routerWorkflow.Tasks[0].Agent = testhelpers.CreateTestAgentConfigWithAction(
			"start-agent",
			"Start agent",
			"analyze",
			"Start analysis",
		)
		routerWorkflow.Tasks[0].OnSuccess = &core.SuccessTransition{
			Next: stringPtr("router-task"),
		}

		baseConfig.WorkflowConfig = routerWorkflow

		workerBuilder := utils.NewWorkerTestBuilder(t).
			WithTaskQueue("missing-route-test-queue").
			WithActivityTimeout(testhelpers.DefaultTestTimeout)

		config := workerBuilder.Build(t)
		config.ContainerTestConfig = baseConfig

		helper := utils.NewWorkerTestHelper(t, config)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow("missing-route-test", &core.Input{})
		require.NotEmpty(t, workflowExecID)

		// Verify workflow handles error gracefully
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusFailed, 10*time.Second)

		// Verify router task failed with appropriate error
		verifier.VerifyTaskStateEventually(
			workflowExecID,
			"router-task",
			core.StatusFailed,
			testhelpers.DefaultTestTimeout,
		)
	})

	t.Run("Should handle route to non-existent task gracefully", func(t *testing.T) {
		builder := testhelpers.NewTestConfigBuilder(t).
			WithTestID("invalid-target-test").
			WithSimpleTask("start-task", "analyze", "Start analysis")

		baseConfig := builder.Build(t)

		routerWorkflow := &wf.Config{
			ID:      "invalid-target-test",
			Version: "1.0.0",
			Opts: wf.Opts{
				Env: &core.EnvMap{
					"user_type": "admin",
				},
			},
			Tasks: []task.Config{
				baseConfig.WorkflowConfig.Tasks[0],
				{
					BaseConfig: task.BaseConfig{
						ID:   "router-task",
						Type: task.TaskTypeRouter,
					},
					RouterTask: task.RouterTask{
						Condition: "{{ .env.user_type }}",
						Routes: map[string]any{
							"admin": "non-existent-task", // Task doesn't exist
							"user":  "user-task",
						},
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "user-task",
						Type: task.TaskTypeBasic,
						Agent: testhelpers.CreateTestAgentConfigWithAction(
							"user-agent",
							"Handle user tasks",
							"process",
							"Handle user request",
						),
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
			},
			Agents: []agent.Config{
				*testhelpers.CreateTestAgentConfigWithAction("start-agent", "Start agent", "analyze", "Start analysis"),
				*testhelpers.CreateTestAgentConfigWithAction("user-agent", "Handle user tasks", "process", "Handle user request"),
			},
		}

		// Set up task transitions
		routerWorkflow.Tasks[0].Agent = testhelpers.CreateTestAgentConfigWithAction(
			"start-agent",
			"Start agent",
			"analyze",
			"Start analysis",
		)
		routerWorkflow.Tasks[0].OnSuccess = &core.SuccessTransition{
			Next: stringPtr("router-task"),
		}

		baseConfig.WorkflowConfig = routerWorkflow

		workerBuilder := utils.NewWorkerTestBuilder(t).
			WithTaskQueue("invalid-target-test-queue").
			WithActivityTimeout(testhelpers.DefaultTestTimeout)

		config := workerBuilder.Build(t)
		config.ContainerTestConfig = baseConfig

		helper := utils.NewWorkerTestHelper(t, config)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow("invalid-target-test", &core.Input{})
		require.NotEmpty(t, workflowExecID)

		// Verify workflow handles error gracefully
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusFailed, 10*time.Second)

		// Verify router task executed but workflow failed due to invalid target
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		// Router task might succeed but workflow should fail when trying to execute non-existent task
		assert.Equal(t, core.StatusFailed, state.Status)
	})

	t.Run("Should handle empty routes configuration", func(t *testing.T) {
		builder := testhelpers.NewTestConfigBuilder(t).
			WithTestID("empty-routes-test").
			WithSimpleTask("start-task", "analyze", "Start analysis")

		baseConfig := builder.Build(t)

		routerWorkflow := &wf.Config{
			ID:      "empty-routes-test",
			Version: "1.0.0",
			Opts: wf.Opts{
				Env: &core.EnvMap{
					"user_type": "admin",
				},
			},
			Tasks: []task.Config{
				baseConfig.WorkflowConfig.Tasks[0],
				{
					BaseConfig: task.BaseConfig{
						ID:   "router-task",
						Type: task.TaskTypeRouter,
					},
					RouterTask: task.RouterTask{
						Condition: "{{ .env.user_type }}",
						Routes:    map[string]any{}, // Empty routes
					},
				},
			},
			Agents: []agent.Config{
				*testhelpers.CreateTestAgentConfigWithAction("start-agent", "Start agent", "analyze", "Start analysis"),
			},
		}

		// Set up task transitions
		routerWorkflow.Tasks[0].Agent = testhelpers.CreateTestAgentConfigWithAction(
			"start-agent",
			"Start agent",
			"analyze",
			"Start analysis",
		)
		routerWorkflow.Tasks[0].OnSuccess = &core.SuccessTransition{
			Next: stringPtr("router-task"),
		}

		baseConfig.WorkflowConfig = routerWorkflow

		workerBuilder := utils.NewWorkerTestBuilder(t).
			WithTaskQueue("empty-routes-test-queue").
			WithActivityTimeout(testhelpers.DefaultTestTimeout)

		config := workerBuilder.Build(t)
		config.ContainerTestConfig = baseConfig

		helper := utils.NewWorkerTestHelper(t, config)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow("empty-routes-test", &core.Input{})
		require.NotEmpty(t, workflowExecID)

		// Verify workflow handles error gracefully
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusFailed, 10*time.Second)

		// Verify router task failed due to empty routes
		verifier.VerifyTaskStateEventually(
			workflowExecID,
			"router-task",
			core.StatusFailed,
			testhelpers.DefaultTestTimeout,
		)
	})
}

// stringPtr is a helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
