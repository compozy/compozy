package router

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/e2e/utils"
	testhelpers "github.com/compozy/compozy/test/helpers"
)

func TestBasicRoutingFunctionality(t *testing.T) {
	ctx := context.Background()

	t.Run("Should route to admin task for admin user", func(t *testing.T) {
		// Create a simple router workflow using existing test builder patterns
		builder := testhelpers.NewTestConfigBuilder(t).
			WithTestID("simple-router-test").
			WithSimpleTask("start-task", "analyze", "Analyze user type: {{.env.user_type}}")

		// Build base config
		baseConfig := builder.Build(t)

		// Create custom router workflow config
		routerWorkflow := &wf.Config{
			ID:          "simple-router-test",
			Version:     "1.0.0",
			Description: "Simple router test",
			Opts: wf.Opts{
				Env: &core.EnvMap{
					"user_type": "admin",
				},
			},
			Tasks: []task.Config{
				// Start task (use existing task from builder)
				baseConfig.WorkflowConfig.Tasks[0],
				// Router task with proper dependency
				{
					BaseConfig: task.BaseConfig{
						ID:   "router-task",
						Type: task.TaskTypeRouter,
						OnSuccess: &core.SuccessTransition{
							Next: StringPtr("admin-task"), // Set next task based on routing
						},
					},
					RouterTask: task.RouterTask{
						Condition: "{{ .env.user_type }}",
						Routes: map[string]any{
							"admin": "admin-task",
							"user":  "user-task",
						},
					},
				},
				// Admin task
				{
					BaseConfig: task.BaseConfig{
						ID:   "admin-task",
						Type: task.TaskTypeBasic,
						Agent: testhelpers.CreateTestAgentConfigWithAction(
							"admin-agent",
							"Handle admin tasks",
							"process",
							"Handle admin request for user: {{.env.user_type}}",
						),
						Outputs: &core.Input{
							"result":      "admin_workflow_completed",
							"permissions": "full_access",
						},
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
				// User task
				{
					BaseConfig: task.BaseConfig{
						ID:   "user-task",
						Type: task.TaskTypeBasic,
						Agent: testhelpers.CreateTestAgentConfigWithAction(
							"user-agent",
							"Handle user tasks",
							"process",
							"Handle user request for user: {{.env.user_type}}",
						),
						Outputs: &core.Input{
							"result":      "user_workflow_completed",
							"permissions": "limited_access",
						},
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
			},
			Agents: []agent.Config{
				*testhelpers.CreateTestAgentConfigWithAction("start-agent", "Start agent", "analyze", "Analyze user type: {{.env.user_type}}"),
				*testhelpers.CreateTestAgentConfigWithAction("admin-agent", "Handle admin tasks", "process", "Handle admin request for user: {{.env.user_type}}"),
				*testhelpers.CreateTestAgentConfigWithAction("user-agent", "Handle user tasks", "process", "Handle user request for user: {{.env.user_type}}"),
			},
		}

		// Update the start task to use proper agent reference and set transition
		routerWorkflow.Tasks[0].Agent = testhelpers.CreateTestAgentConfigWithAction(
			"start-agent",
			"Start agent",
			"analyze",
			"Analyze user type: {{.env.user_type}}",
		)
		routerWorkflow.Tasks[0].Outputs = &core.Input{
			"user_category": "{{ .env.user_type }}",
		}
		// Add transition from start task to router task
		routerWorkflow.Tasks[0].OnSuccess = &core.SuccessTransition{
			Next: StringPtr("router-task"),
		}

		// Replace base workflow config
		baseConfig.WorkflowConfig = routerWorkflow

		// Create worker test configuration
		workerBuilder := utils.NewWorkerTestBuilder(t).
			WithTaskQueue("router-test-queue").
			WithActivityTimeout(testhelpers.DefaultTestTimeout)

		config := workerBuilder.Build(t)
		config.ContainerTestConfig = baseConfig

		helper := utils.NewWorkerTestHelper(t, config)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow("simple-router-test", &core.Input{})
		require.NotEmpty(t, workflowExecID)

		// Verify workflow completes successfully
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		// Verify router and task execution
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		// Check router task output
		routerState, exists := state.Tasks["router-task"]
		require.True(t, exists, "Router task should exist")
		require.NotNil(t, routerState.Output, "Router task should have output")

		routerOutput := *routerState.Output
		assert.Equal(t, "admin", routerOutput["condition"])
		assert.Equal(t, "admin-task", routerOutput["route_taken"])
		assert.Equal(t, "conditional", routerOutput["router_type"])

		// Check that admin task was executed
		adminState, exists := state.Tasks["admin-task"]
		require.True(t, exists, "Admin task should exist")
		assert.Equal(t, core.StatusSuccess, adminState.Status)

		// Check that user task was NOT executed (or at least not completed)
		if userState, exists := state.Tasks["user-task"]; exists {
			assert.NotEqual(t, core.StatusSuccess, userState.Status, "User task should not be completed")
		}
	})

	t.Run("Should route to user task for regular user", func(t *testing.T) {
		// Similar test but with user_type set to "user"
		builder := testhelpers.NewTestConfigBuilder(t).
			WithTestID("user-router-test").
			WithSimpleTask("start-task", "analyze", "Analyze user type: {{.env.user_type}}")

		baseConfig := builder.Build(t)

		routerWorkflow := &wf.Config{
			ID:      "user-router-test",
			Version: "1.0.0",
			Opts: wf.Opts{
				Env: &core.EnvMap{
					"user_type": "user", // Different user type
				},
			},
			Tasks: []task.Config{
				baseConfig.WorkflowConfig.Tasks[0],
				{
					BaseConfig: task.BaseConfig{
						ID:   "router-task",
						Type: task.TaskTypeRouter,
						OnSuccess: &core.SuccessTransition{
							Next: StringPtr("user-task"), // Set next task based on routing
						},
					},
					RouterTask: task.RouterTask{
						Condition: "{{ .env.user_type }}",
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
							"Handle admin request for user: {{.env.user_type}}",
						),
						Outputs: &core.Input{
							"result": "admin_workflow_completed",
						},
					},
					BasicTask: task.BasicTask{
						Action: "process",
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
							"Handle user request for user: {{.env.user_type}}",
						),
						Outputs: &core.Input{
							"result": "user_workflow_completed",
						},
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
			},
			Agents: []agent.Config{
				*testhelpers.CreateTestAgentConfigWithAction("start-agent", "Start agent", "analyze", "Analyze user type: {{.env.user_type}}"),
				*testhelpers.CreateTestAgentConfigWithAction("admin-agent", "Handle admin tasks", "process", "Handle admin request for user: {{.env.user_type}}"),
				*testhelpers.CreateTestAgentConfigWithAction("user-agent", "Handle user tasks", "process", "Handle user request for user: {{.env.user_type}}"),
			},
		}

		routerWorkflow.Tasks[0].Agent = testhelpers.CreateTestAgentConfigWithAction(
			"start-agent",
			"Start agent",
			"analyze",
			"Analyze user type: {{.env.user_type}}",
		)
		routerWorkflow.Tasks[0].Outputs = &core.Input{
			"user_category": "{{ .env.user_type }}",
		}
		// Add transition from start task to router task
		routerWorkflow.Tasks[0].OnSuccess = &core.SuccessTransition{
			Next: StringPtr("router-task"),
		}

		baseConfig.WorkflowConfig = routerWorkflow

		workerBuilder := utils.NewWorkerTestBuilder(t).
			WithTaskQueue("user-router-test-queue").
			WithActivityTimeout(testhelpers.DefaultTestTimeout)

		config := workerBuilder.Build(t)
		config.ContainerTestConfig = baseConfig

		helper := utils.NewWorkerTestHelper(t, config)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow("user-router-test", &core.Input{})
		require.NotEmpty(t, workflowExecID)

		// Verify workflow completes successfully
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		// Verify router routed to user task
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		routerState, exists := state.Tasks["router-task"]
		require.True(t, exists, "Router task should exist")
		require.NotNil(t, routerState.Output, "Router task should have output")

		routerOutput := *routerState.Output
		assert.Equal(t, "user", routerOutput["condition"])
		assert.Equal(t, "user-task", routerOutput["route_taken"])

		// Check that user task was executed
		userState, exists := state.Tasks["user-task"]
		require.True(t, exists, "User task should exist")
		assert.Equal(t, core.StatusSuccess, userState.Status)

		// Check that admin task was NOT executed
		if adminState, exists := state.Tasks["admin-task"]; exists {
			assert.NotEqual(t, core.StatusSuccess, adminState.Status, "Admin task should not be completed")
		}
	})
}

// StringPtr helper function for creating string pointers
func StringPtr(s string) *string {
	return &s
}
