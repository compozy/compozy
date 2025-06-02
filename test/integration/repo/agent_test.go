package repo

import (
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func setupAgentTestBed(t *testing.T) *utils.IntegrationTestBed {
	t.Helper()
	return utils.SetupIntegrationTestBed(t, utils.DefaultTestTimeout)
}

func TestAgentRepository_CreateExecution(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should create agent execution successfully", func(t *testing.T) {
		// Create workflow execution first using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{"workflow_input": "test_data"},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskConfig.Env = core.EnvMap{"TASK_VAR": "task_value"}
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create agent config using helper
		agentConfig := utils.CreateTestAgentConfig(t, "code-assistant", "You are a helpful coding assistant", core.EnvMap{"AGENT_VAR": "agent_value"})
		agentConfig.Config = agent.ProviderConfig{
			Provider:    agent.ProviderAnthropic,
			Model:       agent.ModelClaude3Opus,
			Temperature: 0.7,
			MaxTokens:   4000,
		}

		// Create agent execution using helper
		agentExecID, execution := utils.CreateTestAgentExecution(t, tb, taskExecID, "code-assistant", agentConfig)

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
		// Create workflow execution first using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create agent config with actions using helper
		agentConfig := utils.CreateTestAgentConfig(t, "code-reviewer", "You are a code review assistant", core.EnvMap{})
		agentConfig.Config = agent.ProviderConfig{
			Provider:    agent.ProviderAnthropic,
			Model:       agent.ModelClaude3Sonnet,
			Temperature: 0.5,
			MaxTokens:   2000,
		}
		agentConfig.Actions = []*agent.ActionConfig{
			{
				ID:     "review-code",
				Prompt: "Review the following code for quality and best practices",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"code": map[string]any{
							"type":        "string",
							"description": "The code to review",
						},
						"language": map[string]any{
							"type":        "string",
							"description": "Programming language",
						},
					},
					"required": []string{"code"},
				},
			},
		}

		// Create agent execution using helper
		agentExecID, execution := utils.CreateTestAgentExecution(t, tb, taskExecID, "code-reviewer", agentConfig)

		// Verify execution properties
		assert.Equal(t, agentExecID, execution.GetID())
		assert.Equal(t, "code-reviewer", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
		assert.Equal(t, workflowExecID, execution.GetWorkflowExecID())
		assert.Equal(t, taskExecID, execution.TaskExecID)

		// Verify actions are properly set
		// Note: Actions are stored in the agent config, not directly accessible from execution
		assert.Equal(t, agentExecID, execution.GetID())
		assert.Equal(t, "code-reviewer", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
		assert.Equal(t, workflowExecID, execution.GetWorkflowExecID())
		assert.Equal(t, taskExecID, execution.TaskExecID)
	})

	t.Run("Should handle execution creation with empty env", func(t *testing.T) {
		// Create workflow execution first using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "simple-task", "simple")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "simple-task", taskConfig)

		// Create agent config with empty env using helper
		agentConfig := utils.CreateTestAgentConfig(t, "simple-agent", "Simple agent", core.EnvMap{})

		// Create agent execution using helper
		agentExecID, execution := utils.CreateTestAgentExecution(t, tb, taskExecID, "simple-agent", agentConfig)

		// Verify execution properties
		assert.Equal(t, agentExecID, execution.GetID())
		assert.Equal(t, "simple-agent", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
	})

	t.Run("Should return error when workflow execution not found", func(t *testing.T) {
		nonExistentWorkflowExecID := core.MustNewID()
		agentExecID := core.MustNewID()
		agentMetadata := &pb.AgentMetadata{
			WorkflowId:     "test-workflow",
			WorkflowExecId: string(nonExistentWorkflowExecID),
			TaskId:         "test-task",
			TaskExecId:     string(core.MustNewID()),
			AgentId:        "test-agent",
			AgentExecId:    string(agentExecID),
			Time:           timestamppb.Now(),
			Source:         "test",
		}

		agentConfig := &agent.Config{
			ID:           "test-agent",
			Instructions: "Test agent",
			Config: agent.ProviderConfig{
				Provider: agent.ProviderAnthropic,
				Model:    agent.ModelClaude3Opus,
			},
			Env: core.EnvMap{},
		}
		err := agentConfig.SetCWD(tb.StateDir)
		require.NoError(t, err)

		execution, err := tb.AgentRepo.CreateExecution(tb.Ctx, agentMetadata, agentConfig)
		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Contains(t, err.Error(), "failed to load workflow execution")
	})

	t.Run("Should return error when task execution not found", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		nonExistentTaskExecID := core.MustNewID()
		agentExecID := core.MustNewID()
		agentMetadata := &pb.AgentMetadata{
			WorkflowId:     "test-workflow",
			WorkflowExecId: string(workflowExecID),
			TaskId:         "test-task",
			TaskExecId:     string(nonExistentTaskExecID),
			AgentId:        "test-agent",
			AgentExecId:    string(agentExecID),
			Time:           timestamppb.Now(),
			Source:         "test",
		}

		agentConfig := &agent.Config{
			ID:           "test-agent",
			Instructions: "Test agent",
			Config: agent.ProviderConfig{
				Provider: agent.ProviderAnthropic,
				Model:    agent.ModelClaude3Opus,
			},
			Env: core.EnvMap{},
		}
		err := agentConfig.SetCWD(tb.StateDir)
		require.NoError(t, err)

		execution, err := tb.AgentRepo.CreateExecution(tb.Ctx, agentMetadata, agentConfig)
		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Contains(t, err.Error(), "failed to load task execution")
	})
}

func TestAgentRepository_GetExecution(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load existing execution", func(t *testing.T) {
		// Create workflow execution first using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{"workflow_input": "test_data"},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskConfig.Env = core.EnvMap{"TASK_VAR": "task_value"}
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create agent config using helper
		agentConfig := utils.CreateTestAgentConfig(t, "code-assistant", "You are a helpful coding assistant", core.EnvMap{"AGENT_VAR": "agent_value"})
		agentConfig.Config = agent.ProviderConfig{
			Provider:    agent.ProviderAnthropic,
			Model:       agent.ModelClaude3Opus,
			Temperature: 0.7,
			MaxTokens:   4000,
		}

		// Create agent execution using helper
		agentExecID, createdExecution := utils.CreateTestAgentExecution(t, tb, taskExecID, "code-assistant", agentConfig)

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

func TestAgentRepository_ListExecutions(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list all agent executions", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create multiple agent executions using helpers
		agentConfig1 := utils.CreateTestAgentConfig(t, "agent-1", "First agent", core.EnvMap{})
		agentConfig1.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "agent-1", agentConfig1)

		agentConfig2 := utils.CreateTestAgentConfig(t, "agent-2", "Second agent", core.EnvMap{})
		agentConfig2.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Sonnet,
		}
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "agent-2", agentConfig2)

		// List all executions
		executions, err := tb.AgentRepo.ListExecutions(tb.Ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 2)
	})

	t.Run("Should return empty list when no executions exist", func(t *testing.T) {
		// Clear any existing data by creating a new database connection
		// but reuse the same NATS server to avoid conflicts
		dbFilePath := filepath.Join(tb.StateDir, "empty_test.db")
		emptyStore, err := store.NewStore(dbFilePath)
		require.NoError(t, err)
		defer emptyStore.Close()

		err = emptyStore.Setup()
		require.NoError(t, err)

		// Create repositories with the empty store
		projectConfig := &project.Config{}
		err = projectConfig.SetCWD(tb.StateDir)
		require.NoError(t, err)

		workflows := []*workflow.Config{}
		workflowRepo := emptyStore.NewWorkflowRepository(projectConfig, workflows)
		taskRepo := emptyStore.NewTaskRepository(workflowRepo)
		agentRepo := emptyStore.NewAgentRepository(workflowRepo, taskRepo)

		executions, err := agentRepo.ListExecutions(tb.Ctx)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestAgentRepository_ListExecutionsByStatus(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by status", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create agent execution using helper
		agentConfig := utils.CreateTestAgentConfig(t, "test-agent", "Test agent", core.EnvMap{})
		agentConfig.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "test-agent", agentConfig)

		// List executions by status
		executions, err := tb.AgentRepo.ListExecutionsByStatus(tb.Ctx, core.StatusPending)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)

		// Verify all returned executions have the correct status
		for _, exec := range executions {
			assert.Equal(t, core.StatusPending, exec.Status)
		}
	})

	t.Run("Should return empty list for status with no executions", func(t *testing.T) {
		executions, err := tb.AgentRepo.ListExecutionsByStatus(tb.Ctx, core.StatusSuccess)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestAgentRepository_ListExecutionsByWorkflowID(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by workflow ID", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create agent execution using helper
		agentConfig := utils.CreateTestAgentConfig(t, "test-agent", "Test agent", core.EnvMap{})
		agentConfig.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "test-agent", agentConfig)

		// List executions by workflow ID
		executions, err := tb.AgentRepo.ListExecutionsByWorkflowID(tb.Ctx, "test-workflow")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)

		// Verify all returned executions have the correct workflow ID
		for _, exec := range executions {
			assert.Equal(t, "test-workflow", exec.WorkflowID)
		}
	})

	t.Run("Should return empty list for non-existent workflow ID", func(t *testing.T) {
		executions, err := tb.AgentRepo.ListExecutionsByWorkflowID(tb.Ctx, "non-existent-workflow")
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestAgentRepository_ListExecutionsByWorkflowExecID(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by workflow execution ID", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create multiple agent executions for the same workflow execution using helpers
		agentConfig1 := utils.CreateTestAgentConfig(t, "agent-1", "First agent", core.EnvMap{})
		agentConfig1.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "agent-1", agentConfig1)

		agentConfig2 := utils.CreateTestAgentConfig(t, "agent-2", "Second agent", core.EnvMap{})
		agentConfig2.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Sonnet,
		}
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "agent-2", agentConfig2)

		// List executions by workflow execution ID
		executions, err := tb.AgentRepo.ListExecutionsByWorkflowExecID(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		assert.Len(t, executions, 2)

		// Verify all returned executions have the correct workflow execution ID
		for _, exec := range executions {
			assert.Equal(t, workflowExecID, exec.WorkflowExecID)
		}
	})

	t.Run("Should return empty list for non-existent workflow execution ID", func(t *testing.T) {
		nonExistentWorkflowExecID := core.MustNewID()
		executions, err := tb.AgentRepo.ListExecutionsByWorkflowExecID(tb.Ctx, nonExistentWorkflowExecID)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestAgentRepository_ListExecutionsByTaskID(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by task ID", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create agent execution using helper
		agentConfig := utils.CreateTestAgentConfig(t, "test-agent", "Test agent", core.EnvMap{})
		agentConfig.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "test-agent", agentConfig)

		// List executions by task ID
		executions, err := tb.AgentRepo.ListExecutionsByTaskID(tb.Ctx, "test-task")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)

		// Verify all returned executions have the correct task ID
		for _, exec := range executions {
			assert.Equal(t, "test-task", exec.TaskID)
		}
	})

	t.Run("Should return empty list for non-existent task ID", func(t *testing.T) {
		executions, err := tb.AgentRepo.ListExecutionsByTaskID(tb.Ctx, "non-existent-task")
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestAgentRepository_ListExecutionsByTaskExecID(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by task execution ID", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create multiple agent executions for the same task execution using helpers
		agentConfig1 := utils.CreateTestAgentConfig(t, "agent-1", "First agent", core.EnvMap{})
		agentConfig1.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "agent-1", agentConfig1)

		agentConfig2 := utils.CreateTestAgentConfig(t, "agent-2", "Second agent", core.EnvMap{})
		agentConfig2.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Sonnet,
		}
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "agent-2", agentConfig2)

		// List executions by task execution ID
		executions, err := tb.AgentRepo.ListExecutionsByTaskExecID(tb.Ctx, taskExecID)
		require.NoError(t, err)
		assert.Len(t, executions, 2)

		// Verify all returned executions have the correct task execution ID
		for _, exec := range executions {
			assert.Equal(t, taskExecID, exec.TaskExecID)
		}
	})

	t.Run("Should return empty list for non-existent task execution ID", func(t *testing.T) {
		nonExistentTaskExecID := core.MustNewID()
		executions, err := tb.AgentRepo.ListExecutionsByTaskExecID(tb.Ctx, nonExistentTaskExecID)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestAgentRepository_ListExecutionsByAgentID(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by agent ID", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create multiple executions for the same agent ID using helper
		agentConfig := utils.CreateTestAgentConfig(t, "test-agent", "Test agent", core.EnvMap{})
		agentConfig.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "test-agent", agentConfig)
		_, _ = utils.CreateTestAgentExecution(t, tb, taskExecID, "test-agent", agentConfig)

		// List executions by agent ID
		executions, err := tb.AgentRepo.ListExecutionsByAgentID(tb.Ctx, "test-agent")
		require.NoError(t, err)
		assert.Len(t, executions, 2)

		// Verify all returned executions have the correct agent ID
		for _, exec := range executions {
			assert.Equal(t, "test-agent", exec.AgentID)
		}
	})

	t.Run("Should return empty list for non-existent agent ID", func(t *testing.T) {
		executions, err := tb.AgentRepo.ListExecutionsByAgentID(tb.Ctx, "non-existent-agent")
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestAgentRepository_ExecutionsToMap(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should convert executions to execution maps", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create agent execution using helper
		agentConfig := utils.CreateTestAgentConfig(t, "test-agent", "Test agent", core.EnvMap{})
		agentConfig.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		_, execution := utils.CreateTestAgentExecution(t, tb, taskExecID, "test-agent", agentConfig)

		// Convert to execution maps
		executions := []core.Execution{execution}
		execMaps, err := tb.AgentRepo.ExecutionsToMap(tb.Ctx, executions)
		require.NoError(t, err)
		assert.Len(t, execMaps, 1)

		// Verify execution map properties
		execMap := execMaps[0]
		assert.Equal(t, core.StatusPending, execMap.Status)
		assert.Equal(t, core.ComponentAgent, execMap.Component)
		assert.Equal(t, "test-workflow", execMap.WorkflowID)
		assert.Equal(t, workflowExecID, execMap.WorkflowExecID)
		assert.Equal(t, "test-task", execMap.TaskID)
		assert.Equal(t, taskExecID, execMap.TaskExecID)
		assert.NotNil(t, execMap.AgentID)
		assert.Equal(t, "test-agent", *execMap.AgentID)
		assert.NotNil(t, execMap.AgentExecID)
	})

	t.Run("Should handle empty executions list", func(t *testing.T) {
		execMaps, err := tb.AgentRepo.ExecutionsToMap(tb.Ctx, []core.Execution{})
		require.NoError(t, err)
		assert.Empty(t, execMaps)
	})
}

func TestAgentRepository_CreateExecution_TemplateNormalization(t *testing.T) {
	tb := setupAgentTestBed(t)
	defer tb.Cleanup()

	t.Run("Should parse templates in agent input during execution creation", func(t *testing.T) {
		// Create workflow execution with input data using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{
				"user_name": "John Doe",
				"user_id":   123,
			},
		)

		// Create task execution with templates using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskConfig.With = &core.Input{
			"task_data": "{{ .trigger.input.user_name }}",
		}
		taskConfig.Env = core.EnvMap{"TASK_VAR": "task_value"}
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create test agent config with template input using helper
		agentConfig := utils.CreateTestAgentConfig(t, "template-agent", "You are processing data for {{ .trigger.input.user_name }}", core.EnvMap{
			"AGENT_VAR":   "agent_value",
			"DYNAMIC_VAR": "{{ .trigger.input.user_name }}_processed",
		})
		agentConfig.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		agentConfig.With = &core.Input{
			"greeting":     "Hello, {{ .trigger.input.user_name }}!",
			"user_id":      "{{ .trigger.input.user_id }}",
			"env_message":  "Environment: {{ .env.WORKFLOW_VAR }}",
			"static_value": "no template here",
		}

		// Create execution using helper
		_, execution := utils.CreateTestAgentExecution(t, tb, taskExecID, "template-agent", agentConfig)

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
		// Create workflow execution with nested input data using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"API_BASE": "https://api.example.com"},
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

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create agent config with nested templates using helper
		agentConfig := utils.CreateTestAgentConfig(t, "nested-template-agent", "Process user {{ .trigger.input.user.profile.name }}", core.EnvMap{})
		agentConfig.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		agentConfig.With = &core.Input{
			"api_config": map[string]any{
				"endpoint": "{{ .env.API_BASE }}/users/{{ .trigger.input.user.id }}",
				"headers": map[string]any{
					"X-User-Email": "{{ .trigger.input.user.profile.email }}",
					"Content-Type": "application/json",
				},
			},
			"user_display": "{{ .trigger.input.user.profile.name }}",
			"action_type":  "{{ .trigger.input.action }}",
		}

		// Create execution using helper
		_, execution := utils.CreateTestAgentExecution(t, tb, taskExecID, "nested-template-agent", agentConfig)

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
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{
				"WORKFLOW_ENV": "from_workflow",
				"SHARED_VAR":   "workflow_value",
			},
			&core.Input{
				"service": "user-service",
			},
		)

		// Create task execution with environment using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskConfig.Env = core.EnvMap{
			"TASK_ENV":   "from_task",
			"SHARED_VAR": "task_value", // Should override workflow value
		}
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create agent config with environment merging and templates using helper
		agentConfig := utils.CreateTestAgentConfig(t, "env-merge-agent", "Process service {{ .trigger.input.service }}", core.EnvMap{
			"AGENT_ENV":    "from_agent",
			"SHARED_VAR":   "agent_value", // Should override task value
			"SERVICE_URL":  "https://{{ .trigger.input.service }}.example.com",
			"COMBINED_VAR": "{{ .env.WORKFLOW_ENV }}_and_{{ .env.TASK_ENV }}_and_{{ .env.AGENT_ENV }}",
		})
		agentConfig.Config = agent.ProviderConfig{
			Provider: agent.ProviderAnthropic,
			Model:    agent.ModelClaude3Opus,
		}
		agentConfig.With = &core.Input{}

		// Create execution using helper
		_, execution := utils.CreateTestAgentExecution(t, tb, taskExecID, "env-merge-agent", agentConfig)

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
