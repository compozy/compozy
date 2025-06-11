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

func TestRouterStateTransitions(t *testing.T) {
	ctx := context.Background()

	t.Run("Should properly track state transitions during route execution", func(t *testing.T) {
		// Create a router workflow that validates state transitions through execution phases
		builder := testhelpers.NewTestConfigBuilder(t).
			WithTestID("router-state-test").
			WithSimpleTask("start-task", "process", "Analyze state: {{.env.execution_mode}}")

		baseConfig := builder.Build(t)

		routerWorkflow := &wf.Config{
			ID:          "router-state-test",
			Version:     "1.0.0",
			Description: "Router state transition test",
			Opts: wf.Opts{
				Env: &core.EnvMap{
					"execution_mode": "priority",
				},
			},
			Tasks: []task.Config{
				// Start task
				baseConfig.WorkflowConfig.Tasks[0],
				// Router task
				{
					BaseConfig: task.BaseConfig{
						ID:   "router-task",
						Type: task.TaskTypeRouter,
					},
					RouterTask: task.RouterTask{
						Condition: "{{ .env.execution_mode }}",
						Routes: map[string]any{
							"priority": "priority-task",
							"normal":   "normal-task",
							"batch":    "batch-task",
						},
					},
				},
				// Priority route task
				{
					BaseConfig: task.BaseConfig{
						ID:   "priority-task",
						Type: task.TaskTypeBasic,
						Agent: testhelpers.CreateTestAgentConfigWithAction(
							"priority-agent",
							"Handle priority tasks",
							"process",
							"Handle priority request: {{.workflow.input.message}}",
						),
						Outputs: &core.Input{
							"execution_type": "priority",
							"processed":      true,
							"priority_level": "high",
						},
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
			},
			Agents: []agent.Config{
				*testhelpers.CreateTestAgentConfigWithAction("start-task-agent", "Start agent", "process", "Analyze state: {{.env.execution_mode}}"),
				*testhelpers.CreateTestAgentConfigWithAction("priority-agent", "Handle priority tasks", "process", "Handle priority request: {{.workflow.input.message}}"),
			},
		}

		// Update the start task to use proper agent reference and set transition
		routerWorkflow.Tasks[0].Agent = testhelpers.CreateTestAgentConfigWithAction(
			"start-task-agent",
			"Start agent",
			"process",
			"Analyze state: {{.env.execution_mode}}",
		)
		routerWorkflow.Tasks[0].Outputs = &core.Input{
			"analysis_result": "{{ .env.execution_mode }}",
		}
		// Add transition from start task to router task
		routerWorkflow.Tasks[0].OnSuccess = &core.SuccessTransition{
			Next: ptr("router-task"),
		}

		baseConfig.WorkflowConfig = routerWorkflow

		workerBuilder := utils.NewWorkerTestBuilder(t).
			WithTaskQueue("router-state-test-queue").
			WithActivityTimeout(testhelpers.DefaultTestTimeout)

		config := workerBuilder.Build(t)
		config.ContainerTestConfig = baseConfig

		helper := utils.NewWorkerTestHelper(t, config)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow("router-state-test", &core.Input{
			"message": "Test router state transitions",
		})
		require.NotEmpty(t, workflowExecID)

		// Verify workflow completes successfully
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		// Detailed state transition verification
		transitions := []testhelpers.StatusTransition{
			{Status: core.StatusSuccess, Component: "start-task", MaxWait: testhelpers.DefaultTestTimeout},
			{Status: core.StatusSuccess, Component: "router-task", MaxWait: testhelpers.DefaultTestTimeout},
			{Status: core.StatusSuccess, Component: "priority-task", MaxWait: testhelpers.DefaultTestTimeout},
			{Status: core.StatusSuccess, Component: "workflow", MaxWait: testhelpers.DefaultTestTimeout},
		}

		verifier.VerifyStatusTransitionSequence(workflowExecID, transitions)

		// Verify final state
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		// Check router task output and routing
		routerState, exists := state.Tasks["router-task"]
		require.True(t, exists, "Router task should exist")
		require.NotNil(t, routerState.Output, "Router task should have output")

		routerOutput := *routerState.Output
		assert.Equal(t, "priority", routerOutput["condition"])
		assert.Equal(t, "priority-task", routerOutput["route_taken"])
		assert.Equal(t, "conditional", routerOutput["router_type"])

		// Check that priority task was executed successfully
		priorityState, exists := state.Tasks["priority-task"]
		require.True(t, exists, "Priority task should exist")
		assert.Equal(t, core.StatusSuccess, priorityState.Status)
	})

	t.Run("Should maintain state persistence with task dependencies", func(t *testing.T) {
		// Test router with tasks that have dependencies and verify state persistence
		builder := testhelpers.NewTestConfigBuilder(t).
			WithTestID("router-dependency-test").
			WithSimpleTask("initial-task", "process", "Setup for router: {{.env.flow_type}}")

		baseConfig := builder.Build(t)

		routerWorkflow := &wf.Config{
			ID:      "router-dependency-test",
			Version: "1.0.0",
			Opts: wf.Opts{
				Env: &core.EnvMap{
					"flow_type": "sequential",
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
						Condition: "{{ .env.flow_type }}",
						Routes: map[string]any{
							"sequential": "sequential-task",
							"parallel":   "parallel-task",
						},
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "sequential-task",
						Type: task.TaskTypeBasic,
						Agent: testhelpers.CreateTestAgentConfigWithAction(
							"sequential-agent",
							"Process sequentially",
							"process",
							"Handle sequential request: {{.workflow.input.message}}",
						),
						Outputs: &core.Input{
							"execution_order": "sequential",
							"completed":       true,
						},
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "parallel-task",
						Type: task.TaskTypeBasic,
						Agent: testhelpers.CreateTestAgentConfigWithAction(
							"parallel-agent",
							"Process in parallel",
							"process",
							"Handle parallel request: {{.workflow.input.message}}",
						),
						Outputs: &core.Input{
							"execution_order": "parallel",
							"completed":       true,
						},
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
			},
			Agents: []agent.Config{
				*testhelpers.CreateTestAgentConfigWithAction("initial-task-agent", "Initial setup", "process", "Setup for router: {{.env.flow_type}}"),
				*testhelpers.CreateTestAgentConfigWithAction("sequential-agent", "Process sequentially", "process", "Handle sequential request: {{.workflow.input.message}}"),
				*testhelpers.CreateTestAgentConfigWithAction("parallel-agent", "Process in parallel", "process", "Handle parallel request: {{.workflow.input.message}}"),
			},
		}

		// Configure initial task
		routerWorkflow.Tasks[0].Agent = testhelpers.CreateTestAgentConfigWithAction(
			"initial-task-agent",
			"Initial setup",
			"process",
			"Setup for router: {{.env.flow_type}}",
		)
		routerWorkflow.Tasks[0].OnSuccess = &core.SuccessTransition{
			Next: ptr("router-task"),
		}

		baseConfig.WorkflowConfig = routerWorkflow

		workerBuilder := utils.NewWorkerTestBuilder(t).
			WithTaskQueue("router-dependency-test-queue").
			WithActivityTimeout(testhelpers.DefaultTestTimeout)

		config := workerBuilder.Build(t)
		config.ContainerTestConfig = baseConfig

		helper := utils.NewWorkerTestHelper(t, config)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow("router-dependency-test", &core.Input{
			"message": "Test router dependencies",
		})
		require.NotEmpty(t, workflowExecID)

		// Verify workflow completes successfully
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		// Verify state consistency
		verifier.VerifyTaskStateConsistency(workflowExecID)

		// Verify that the correct route was taken
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		routerState, exists := state.Tasks["router-task"]
		require.True(t, exists, "Router task should exist")
		assert.Equal(t, core.StatusSuccess, routerState.Status)

		sequentialState, exists := state.Tasks["sequential-task"]
		require.True(t, exists, "Sequential task should exist")
		assert.Equal(t, core.StatusSuccess, sequentialState.Status)
	})

	t.Run("Should handle state consistency after router execution and cleanup", func(t *testing.T) {
		// Test that state remains consistent after router execution and cleanup
		builder := testhelpers.NewTestConfigBuilder(t).
			WithTestID("router-cleanup-test").
			WithSimpleTask("pre-router", "process", "Prepare for routing: {{.env.cleanup_mode}}")

		baseConfig := builder.Build(t)

		routerWorkflow := &wf.Config{
			ID:      "router-cleanup-test",
			Version: "1.0.0",
			Opts: wf.Opts{
				Env: &core.EnvMap{
					"cleanup_mode": "thorough",
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
						Condition: "{{ .env.cleanup_mode }}",
						Routes: map[string]any{
							"thorough": "cleanup-task",
							"quick":    "quick-task",
						},
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "cleanup-task",
						Type: task.TaskTypeBasic,
						Agent: testhelpers.CreateTestAgentConfigWithAction(
							"cleanup-agent",
							"Perform thorough cleanup",
							"cleanup",
							"Handle cleanup request: {{.workflow.input.message}}",
						),
						Outputs: &core.Input{
							"cleanup_type": "thorough",
							"completed":    true,
							"resources":    "freed",
						},
					},
					BasicTask: task.BasicTask{
						Action: "cleanup",
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "quick-task",
						Type: task.TaskTypeBasic,
						Agent: testhelpers.CreateTestAgentConfigWithAction(
							"quick-agent",
							"Perform quick cleanup",
							"cleanup",
							"Handle cleanup request: {{.workflow.input.message}}",
						),
						Outputs: &core.Input{
							"cleanup_type": "quick",
							"completed":    true,
							"resources":    "minimal",
						},
					},
					BasicTask: task.BasicTask{
						Action: "cleanup",
					},
				},
			},
			Agents: []agent.Config{
				*testhelpers.CreateTestAgentConfigWithAction("pre-router-agent", "Preparation", "process", "Prepare for routing: {{.env.cleanup_mode}}"),
				*testhelpers.CreateTestAgentConfigWithAction("cleanup-agent", "Perform thorough cleanup", "cleanup", "Handle cleanup request: {{.workflow.input.message}}"),
				*testhelpers.CreateTestAgentConfigWithAction("quick-agent", "Perform quick cleanup", "cleanup", "Handle cleanup request: {{.workflow.input.message}}"),
			},
		}

		// Configure pre-router task
		routerWorkflow.Tasks[0].Agent = testhelpers.CreateTestAgentConfigWithAction(
			"pre-router-agent",
			"Preparation",
			"process",
			"Prepare for routing: {{.env.cleanup_mode}}",
		)
		routerWorkflow.Tasks[0].OnSuccess = &core.SuccessTransition{
			Next: ptr("router-task"),
		}

		baseConfig.WorkflowConfig = routerWorkflow

		workerBuilder := utils.NewWorkerTestBuilder(t).
			WithTaskQueue("router-cleanup-test-queue").
			WithActivityTimeout(testhelpers.DefaultTestTimeout)

		config := workerBuilder.Build(t)
		config.ContainerTestConfig = baseConfig

		helper := utils.NewWorkerTestHelper(t, config)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow("router-cleanup-test", &core.Input{
			"message": "Test router cleanup",
		})
		require.NotEmpty(t, workflowExecID)

		// Verify workflow completes successfully
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		// Verify no errors occurred during execution
		verifier.VerifyNoErrors(workflowExecID)

		// Verify correct task count (3 tasks: pre-router, router, cleanup)
		verifier.VerifyTaskCount(workflowExecID, 3)

		// Verify state consistency
		verifier.VerifyTaskStateConsistency(workflowExecID)

		// Verify the thorough cleanup was executed
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		cleanupState, exists := state.Tasks["cleanup-task"]
		require.True(t, exists, "Cleanup task should exist")
		assert.Equal(t, core.StatusSuccess, cleanupState.Status)

		// Quick task should not have been executed
		if quickState, exists := state.Tasks["quick-task"]; exists {
			assert.NotEqual(t, core.StatusSuccess, quickState.Status, "Quick task should not be completed")
		}
	})
}

// ptr is a helper function for creating string pointers
func ptr(s string) *string {
	return &s
}
