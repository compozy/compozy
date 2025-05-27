package repo

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func setupAgentRepoTestBed(t *testing.T) *utils.IntegrationTestBed {
	t.Helper()
	componentsToWatch := []core.ComponentType{
		core.ComponentWorkflow,
		core.ComponentTask,
		core.ComponentAgent,
		core.ComponentTool,
	}
	return utils.SetupIntegrationTestBed(t, utils.DefaultTestTimeout, componentsToWatch)
}

func createTestAgentWorkflowExecution(
	t *testing.T,
	tb *utils.IntegrationTestBed,
	env core.EnvMap,
	input *core.Input,
) core.ID {
	t.Helper()

	workflowExecID := core.MustNewID()
	workflowMetadata := &pb.WorkflowMetadata{
		WorkflowId:     "test-workflow",
		WorkflowExecId: string(workflowExecID),
		Time:           timestamppb.Now(),
		Source:         "test",
	}

	workflowConfig := &workflow.Config{
		ID:      "test-workflow",
		Version: "1.0.0",
		Env:     env,
	}
	err := workflowConfig.SetCWD(tb.StateDir)
	require.NoError(t, err)

	_, err = tb.WorkflowRepo.CreateExecution(tb.Ctx, workflowMetadata, workflowConfig, input)
	require.NoError(t, err)

	return workflowExecID
}

func createTestAgentTaskExecution(
	t *testing.T,
	tb *utils.IntegrationTestBed,
	workflowExecID core.ID,
	taskConfig *task.Config,
) core.ID {
	t.Helper()

	taskExecID := core.MustNewID()
	taskMetadata := &pb.TaskMetadata{
		WorkflowId:     "test-workflow",
		WorkflowExecId: string(workflowExecID),
		TaskId:         "test-task",
		TaskExecId:     string(taskExecID),
		Time:           timestamppb.Now(),
		Source:         "test",
	}

	err := taskConfig.SetCWD(tb.StateDir)
	require.NoError(t, err)

	_, err = tb.TaskRepo.CreateExecution(tb.Ctx, taskMetadata, taskConfig)
	require.NoError(t, err)

	return taskExecID
}

func createTestAgentExecution(
	t *testing.T,
	tb *utils.IntegrationTestBed,
	workflowExecID core.ID,
	taskExecID core.ID,
	agentID string,
	agentConfig *agent.Config,
) (core.ID, *agent.Execution) {
	t.Helper()
	agentExecID := core.MustNewID()
	agentMetadata := &pb.AgentMetadata{
		WorkflowId:     "test-workflow",
		WorkflowExecId: string(workflowExecID),
		TaskId:         "test-task",
		TaskExecId:     string(taskExecID),
		AgentId:        agentID,
		AgentExecId:    string(agentExecID),
		Time:           timestamppb.Now(),
		Source:         "test",
	}
	err := agentConfig.SetCWD(tb.StateDir)
	require.NoError(t, err)
	execution, err := tb.AgentRepo.CreateExecution(tb.Ctx, agentMetadata, agentConfig)
	require.NoError(t, err)
	require.NotNil(t, execution)
	return agentExecID, execution
}

func TestAgentRepository_CreateExecution(t *testing.T) {
	tb := setupAgentRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should create agent execution successfully", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestAgentWorkflowExecution(
			t, tb, core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{"workflow_input": "test_data"},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{"TASK_VAR": "task_value"},
		}
		taskExecID := createTestAgentTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create agent config
		agentConfig := &agent.Config{
			ID:           "code-assistant",
			Instructions: "You are a helpful coding assistant",
			Config: agent.ProviderConfig{
				Provider:    agent.ProviderAnthropic,
				Model:       agent.ModelClaude3Opus,
				Temperature: 0.7,
				MaxTokens:   4000,
			},
			Env: core.EnvMap{"AGENT_VAR": "agent_value"},
		}

		// Create agent execution
		agentExecID, execution := createTestAgentExecution(t, tb, workflowExecID, taskExecID, "code-assistant", agentConfig)

		// Verify execution properties
		assert.Equal(t, agentExecID, execution.GetID())
		assert.Equal(t, "code-assistant", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
		assert.Equal(t, workflowExecID, execution.GetWorkflowExecID())
		assert.Equal(t, taskExecID, execution.TaskExecID)
		assert.NotNil(t, execution.GetEnv())
		assert.Equal(t, "agent_value", execution.GetEnv().Prop("AGENT_VAR"))
		assert.Equal(t, "task_value", execution.GetEnv().Prop("TASK_VAR"))
		assert.Equal(t, "workflow_value", execution.GetEnv().Prop("WORKFLOW_VAR"))
	})

	t.Run("Should handle execution creation with actions", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestAgentWorkflowExecution(
			t, tb, core.EnvMap{},
			&core.Input{},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		taskExecID := createTestAgentTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create agent config with actions
		agentConfig := &agent.Config{
			ID:           "code-reviewer",
			Instructions: "You are a code review assistant",
			Config: agent.ProviderConfig{
				Provider:    agent.ProviderAnthropic,
				Model:       agent.ModelClaude3Sonnet,
				Temperature: 0.5,
				MaxTokens:   2000,
			},
			Actions: []*agent.ActionConfig{
				{
					ID:     "review-code",
					Prompt: "Review the following code for quality and best practices",
					InputSchema: &schema.InputSchema{
						Schema: schema.Schema{
							"type": "object",
							"properties": map[string]any{
								"code": map[string]any{
									"type":        "string",
									"description": "The code to review",
								},
								"language": map[string]any{
									"type":        "string",
									"description": "The programming language",
								},
							},
							"required": []string{"code"},
						},
					},
				},
			},
			Env: core.EnvMap{},
		}

		// Create agent execution
		agentExecID, execution := createTestAgentExecution(t, tb, workflowExecID, taskExecID, "code-reviewer", agentConfig)

		// Verify execution properties
		assert.Equal(t, agentExecID, execution.GetID())
		assert.Equal(t, "code-reviewer", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
	})
}

func TestAgentRepository_LoadExecution(t *testing.T) {
	tb := setupAgentRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load existing execution", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestAgentWorkflowExecution(
			t, tb, core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{"workflow_input": "test_data"},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{"TASK_VAR": "task_value"},
		}
		taskExecID := createTestAgentTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create agent config
		agentConfig := &agent.Config{
			ID:           "code-assistant",
			Instructions: "You are a helpful coding assistant",
			Config: agent.ProviderConfig{
				Provider:    agent.ProviderAnthropic,
				Model:       agent.ModelClaude3Opus,
				Temperature: 0.7,
				MaxTokens:   4000,
			},
			Env: core.EnvMap{"AGENT_VAR": "agent_value"},
		}

		// Create agent execution
		agentExecID, createdExecution := createTestAgentExecution(t, tb, workflowExecID, taskExecID, "code-assistant", agentConfig)

		// Load the execution
		loadedExecution, err := tb.AgentRepo.GetExecution(tb.Ctx, agentExecID)
		require.NoError(t, err)
		require.NotNil(t, loadedExecution)

		// Verify loaded execution matches created execution
		assert.Equal(t, createdExecution.GetID(), loadedExecution.GetID())
		assert.Equal(t, createdExecution.GetComponentID(), loadedExecution.GetComponentID())
		assert.Equal(t, createdExecution.GetStatus(), loadedExecution.GetStatus())
		assert.Equal(t, createdExecution.GetWorkflowExecID(), loadedExecution.GetWorkflowExecID())
		assert.Equal(t, createdExecution.TaskExecID, loadedExecution.TaskExecID)
	})

	t.Run("Should return error for non-existent execution", func(t *testing.T) {
		nonExistentAgentExecID := core.MustNewID()
		execution, err := tb.AgentRepo.GetExecution(tb.Ctx, nonExistentAgentExecID)
		assert.Error(t, err)
		assert.Nil(t, execution)
	})
}

func TestAgentRepository_LoadExecutionsMapByWorkflowExecID(t *testing.T) {
	tb := setupAgentRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load executions JSON for existing workflow execution", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestAgentWorkflowExecution(
			t, tb, core.EnvMap{},
			&core.Input{},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		taskExecID := createTestAgentTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create multiple agent executions for the same workflow execution
		agentConfig1 := &agent.Config{
			ID:           "agent-1",
			Instructions: "First agent",
			Config: agent.ProviderConfig{
				Provider: agent.ProviderAnthropic,
				Model:    agent.ModelClaude3Opus,
			},
			Env: core.EnvMap{},
		}
		agentExecID1, _ := createTestAgentExecution(t, tb, workflowExecID, taskExecID, "agent-1", agentConfig1)

		agentConfig2 := &agent.Config{
			ID:           "agent-2",
			Instructions: "Second agent",
			Config: agent.ProviderConfig{
				Provider: agent.ProviderAnthropic,
				Model:    agent.ModelClaude3Sonnet,
			},
			Env: core.EnvMap{},
		}
		agentExecID2, _ := createTestAgentExecution(t, tb, workflowExecID, taskExecID, "agent-2", agentConfig2)

		// Load executions JSON
		executionsJSON, err := tb.AgentRepo.LoadExecutionsMapByWorkflowExecID(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, executionsJSON)

		// Verify we have both executions
		assert.Len(t, executionsJSON, 2)

		// Verify execution data
		exec1, exists := executionsJSON[agentExecID1].(map[core.ID]any)
		assert.True(t, exists)
		assert.Equal(t, "agent-1", exec1[core.ID("agent_id")])
		assert.Equal(t, agentExecID1, exec1[core.ID("agent_exec_id")])
		assert.Equal(t, core.StatusPending, exec1[core.ID("status")])

		exec2, exists := executionsJSON[agentExecID2].(map[core.ID]any)
		assert.True(t, exists)
		assert.Equal(t, "agent-2", exec2[core.ID("agent_id")])
		assert.Equal(t, agentExecID2, exec2[core.ID("agent_exec_id")])
		assert.Equal(t, core.StatusPending, exec2[core.ID("status")])
	})

	t.Run("Should return empty map for workflow execution with no agents", func(t *testing.T) {
		nonExistentWorkflowExecID := core.MustNewID()

		executionsJSON, err := tb.AgentRepo.LoadExecutionsMapByWorkflowExecID(tb.Ctx, nonExistentWorkflowExecID)
		require.NoError(t, err)
		assert.Empty(t, executionsJSON)
	})
}

func TestAgentRepository_CreateExecution_TemplateNormalization(t *testing.T) {
	tb := setupAgentRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should parse templates in agent input during execution creation", func(t *testing.T) {
		// Create workflow execution with input data
		workflowExecID := createTestAgentWorkflowExecution(
			t, tb, core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{
				"user_name": "John Doe",
				"user_id":   123,
			},
		)

		// Create task execution with templates
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			With: &core.Input{
				"task_data": "{{ .trigger.input.user_name }}",
			},
			Env: core.EnvMap{"TASK_VAR": "task_value"},
		}
		taskExecID := createTestAgentTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create test agent config with template input
		agentConfig := &agent.Config{
			ID:           "template-agent",
			Instructions: "You are processing data for {{ .trigger.input.user_name }}",
			Config: agent.ProviderConfig{
				Provider: agent.ProviderAnthropic,
				Model:    agent.ModelClaude3Opus,
			},
			With: &core.Input{
				"greeting":     "Hello, {{ .trigger.input.user_name }}!",
				"user_id":      "{{ .trigger.input.user_id }}",
				"env_message":  "Environment: {{ .env.WORKFLOW_VAR }}",
				"static_value": "no template here",
			},
			Env: core.EnvMap{
				"AGENT_VAR":   "agent_value",
				"DYNAMIC_VAR": "{{ .trigger.input.user_name }}_processed",
			},
		}

		// Create execution
		_, execution := createTestAgentExecution(t, tb, workflowExecID, taskExecID, "template-agent", agentConfig)

		// Verify templates were parsed in input
		input := execution.GetInput()
		assert.Equal(t, "Hello, John Doe!", input.Prop("greeting"))
		assert.Equal(t, "123", input.Prop("user_id")) // Numbers become strings in templates
		assert.Equal(t, "Environment: workflow_value", input.Prop("env_message"))
		assert.Equal(t, "no template here", input.Prop("static_value"))

		// Verify templates were parsed in environment
		env := execution.GetEnv()
		assert.Equal(t, "agent_value", env.Prop("AGENT_VAR"))
		assert.Equal(t, "John Doe_processed", env.Prop("DYNAMIC_VAR"))
		assert.Equal(t, "workflow_value", env.Prop("WORKFLOW_VAR")) // From workflow
		assert.Equal(t, "task_value", env.Prop("TASK_VAR"))         // From task
	})

	t.Run("Should handle nested templates in agent configuration", func(t *testing.T) {
		// Create workflow execution with nested input data
		workflowExecID := createTestAgentWorkflowExecution(
			t, tb, core.EnvMap{"API_BASE": "https://api.example.com"},
			&core.Input{
				"user": map[string]any{
					"profile": map[string]any{
						"name":  "Jane Smith",
						"email": "jane@example.com",
					},
					"id": "user123",
				},
				"action": "create",
			},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		taskExecID := createTestAgentTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create agent config with nested templates
		agentConfig := &agent.Config{
			ID:           "nested-template-agent",
			Instructions: "Process user {{ .trigger.input.user.profile.name }}",
			Config: agent.ProviderConfig{
				Provider: agent.ProviderAnthropic,
				Model:    agent.ModelClaude3Opus,
			},
			With: &core.Input{
				"api_config": map[string]any{
					"endpoint": "{{ .env.API_BASE }}/users/{{ .trigger.input.user.id }}",
					"headers": map[string]any{
						"X-User-Email": "{{ .trigger.input.user.profile.email }}",
						"Content-Type": "application/json",
					},
				},
				"user_display": "{{ .trigger.input.user.profile.name }}",
				"action_type":  "{{ .trigger.input.action }}",
			},
			Env: core.EnvMap{},
		}

		// Create execution
		_, execution := createTestAgentExecution(t, tb, workflowExecID, taskExecID, "nested-template-agent", agentConfig)

		// Verify nested templates were parsed
		input := execution.GetInput()

		apiConfig, ok := input.Prop("api_config").(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "https://api.example.com/users/user123", apiConfig["endpoint"])

		headers, ok := apiConfig["headers"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "jane@example.com", headers["X-User-Email"])
		assert.Equal(t, "application/json", headers["Content-Type"])

		assert.Equal(t, "Jane Smith", input.Prop("user_display"))
		assert.Equal(t, "create", input.Prop("action_type"))
	})

	t.Run("Should handle environment variable merging with templates", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestAgentWorkflowExecution(
			t, tb, core.EnvMap{
				"WORKFLOW_ENV": "from_workflow",
				"SHARED_VAR":   "workflow_value",
			},
			&core.Input{
				"service": "user-service",
			},
		)

		// Create task execution with environment
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env: core.EnvMap{
				"TASK_ENV":   "from_task",
				"SHARED_VAR": "task_value", // Should override workflow value
			},
		}
		taskExecID := createTestAgentTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create agent config with environment merging and templates
		agentConfig := &agent.Config{
			ID:           "env-merge-agent",
			Instructions: "Process service {{ .trigger.input.service }}",
			Config: agent.ProviderConfig{
				Provider: agent.ProviderAnthropic,
				Model:    agent.ModelClaude3Opus,
			},
			With: &core.Input{},
			Env: core.EnvMap{
				"AGENT_ENV":    "from_agent",
				"SHARED_VAR":   "agent_value", // Should override task value
				"SERVICE_URL":  "https://{{ .trigger.input.service }}.example.com",
				"COMBINED_VAR": "{{ .env.WORKFLOW_ENV }}_and_{{ .env.TASK_ENV }}_and_{{ .env.AGENT_ENV }}",
			},
		}

		// Create execution
		_, execution := createTestAgentExecution(t, tb, workflowExecID, taskExecID, "env-merge-agent", agentConfig)

		// Verify environment variable merging and template parsing
		env := execution.GetEnv()

		// Workflow env should be present
		assert.Equal(t, "from_workflow", env.Prop("WORKFLOW_ENV"))

		// Task env should be present
		assert.Equal(t, "from_task", env.Prop("TASK_ENV"))

		// Agent env should be present
		assert.Equal(t, "from_agent", env.Prop("AGENT_ENV"))

		// Agent should override task and workflow for shared variables
		assert.Equal(t, "agent_value", env.Prop("SHARED_VAR"))

		// Templates should be parsed
		assert.Equal(t, "https://user-service.example.com", env.Prop("SERVICE_URL"))

		// Complex template combining multiple env vars should work
		combinedVar := env.Prop("COMBINED_VAR")
		assert.Contains(t, combinedVar, "from_workflow")
		assert.Contains(t, combinedVar, "from_task")
		assert.Contains(t, combinedVar, "from_agent")
	})
}
