package repo

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func setupToolRepoTestBed(t *testing.T) *utils.IntegrationTestBed {
	t.Helper()
	componentsToWatch := []core.ComponentType{
		core.ComponentWorkflow,
		core.ComponentTask,
		core.ComponentAgent,
		core.ComponentTool,
	}
	return utils.SetupIntegrationTestBed(t, utils.DefaultTestTimeout, componentsToWatch)
}

func createTestToolWorkflowExecution(
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

func createTestToolTaskExecution(
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

func createTestToolExecution(
	t *testing.T,
	tb *utils.IntegrationTestBed,
	workflowExecID core.ID,
	taskExecID core.ID,
	toolID string,
	toolConfig *tool.Config,
) (core.ID, *tool.Execution) {
	t.Helper()

	toolExecID := core.MustNewID()
	toolMetadata := &pb.ToolMetadata{
		WorkflowId:     "test-workflow",
		WorkflowExecId: string(workflowExecID),
		TaskId:         "test-task",
		TaskExecId:     string(taskExecID),
		ToolId:         toolID,
		ToolExecId:     string(toolExecID),
		Time:           timestamppb.Now(),
		Source:         "test",
	}

	err := toolConfig.SetCWD(tb.StateDir)
	require.NoError(t, err)

	execution, err := tb.ToolRepo.CreateExecution(tb.Ctx, toolMetadata, toolConfig)
	require.NoError(t, err)
	require.NotNil(t, execution)

	return toolExecID, execution
}

func TestToolRepository_CreateExecution(t *testing.T) {
	tb := setupToolRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should create tool execution successfully", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestToolWorkflowExecution(
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
		taskExecID := createTestToolTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create tool config
		toolConfig := &tool.Config{
			ID:          "code-formatter",
			Description: "A tool for formatting code",
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"code": map[string]any{
							"type":        "string",
							"description": "The code to format",
						},
						"language": map[string]any{
							"type":        "string",
							"description": "The programming language",
						},
					},
					"required": []string{"code"},
				},
			},
			OutputSchema: &schema.OutputSchema{
				Schema: schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"formatted_code": map[string]any{
							"type":        "string",
							"description": "The formatted code",
						},
					},
					"required": []string{"formatted_code"},
				},
			},
			With: &core.Input{
				"indent_size": 2,
				"use_tabs":    false,
			},
			Env: core.EnvMap{"TOOL_VAR": "tool_value"},
		}

		// Create tool execution
		toolExecID, execution := createTestToolExecution(t, tb, workflowExecID, taskExecID, "code-formatter", toolConfig)

		// Verify execution properties
		assert.Equal(t, toolExecID, execution.GetID())
		assert.Equal(t, "code-formatter", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
		assert.Equal(t, workflowExecID, execution.GetWorkflowExecID())
		assert.Equal(t, taskExecID, execution.TaskExecID)
		assert.NotNil(t, execution.GetEnv())
		assert.Equal(t, "tool_value", execution.GetEnv().Prop("TOOL_VAR"))
		assert.Equal(t, "task_value", execution.GetEnv().Prop("TASK_VAR"))
		assert.Equal(t, "workflow_value", execution.GetEnv().Prop("WORKFLOW_VAR"))
		assert.NotNil(t, execution.GetInput())
		assert.Equal(t, 2, execution.GetInput().Prop("indent_size"))
		assert.Equal(t, false, execution.GetInput().Prop("use_tabs"))
	})

	t.Run("Should handle execution creation with execute script", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestToolWorkflowExecution(
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
		taskExecID := createTestToolTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create tool config with execute script
		toolConfig := &tool.Config{
			ID:          "script-runner",
			Description: "A tool that runs scripts",
			Execute:     "./script.ts",
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"script": map[string]any{
						"type":        "string",
						"description": "The script to run",
					},
					"required": []string{"script"},
				},
			},
			With: &core.Input{
				"script": "console.log('Hello, World!');",
			},
			Env: core.EnvMap{},
		}

		// Create tool execution
		toolExecID, execution := createTestToolExecution(t, tb, workflowExecID, taskExecID, "script-runner", toolConfig)

		// Verify execution properties
		assert.Equal(t, toolExecID, execution.GetID())
		assert.Equal(t, "script-runner", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
		assert.NotNil(t, execution.GetInput())
		assert.Equal(t, "console.log('Hello, World!');", execution.GetInput().Prop("script"))
	})
}

func TestToolRepository_LoadExecution(t *testing.T) {
	tb := setupToolRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load existing execution", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestToolWorkflowExecution(
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
		taskExecID := createTestToolTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create tool config
		toolConfig := &tool.Config{
			ID:          "code-formatter",
			Description: "A tool for formatting code",
			With: &core.Input{
				"indent_size": 2,
			},
			Env: core.EnvMap{"TOOL_VAR": "tool_value"},
		}

		// Create tool execution
		toolExecID, createdExecution := createTestToolExecution(t, tb, workflowExecID, taskExecID, "code-formatter", toolConfig)

		// Load the execution
		loadedExecution, err := tb.ToolRepo.GetExecution(tb.Ctx, toolExecID)
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
		nonExistentToolExecID := core.MustNewID()

		execution, err := tb.ToolRepo.GetExecution(tb.Ctx, nonExistentToolExecID)
		assert.Error(t, err)
		assert.Nil(t, execution)
	})
}

func TestToolRepository_LoadExecutionsMapByWorkflowExecID(t *testing.T) {
	tb := setupToolRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load executions JSON for existing workflow execution", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestToolWorkflowExecution(
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
		taskExecID := createTestToolTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create multiple tool executions for the same workflow execution
		toolConfig1 := &tool.Config{
			ID:          "tool-1",
			Description: "First tool",
			With: &core.Input{
				"param1": "value1",
			},
			Env: core.EnvMap{},
		}
		toolExecID1, _ := createTestToolExecution(t, tb, workflowExecID, taskExecID, "tool-1", toolConfig1)

		toolConfig2 := &tool.Config{
			ID:          "tool-2",
			Description: "Second tool",
			With: &core.Input{
				"param2": "value2",
			},
			Env: core.EnvMap{},
		}
		toolExecID2, _ := createTestToolExecution(t, tb, workflowExecID, taskExecID, "tool-2", toolConfig2)

		// Load executions JSON
		executionsJSON, err := tb.ToolRepo.LoadExecutionsMapByWorkflowExecID(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, executionsJSON)

		// Verify we have both executions
		assert.Len(t, executionsJSON, 2)

		// Verify execution data
		exec1, exists := executionsJSON[toolExecID1].(map[core.ID]any)
		assert.True(t, exists)
		assert.Equal(t, "tool-1", exec1[core.ID("tool_id")])
		assert.Equal(t, toolExecID1, exec1[core.ID("tool_exec_id")])
		assert.Equal(t, core.StatusPending, exec1[core.ID("status")])

		exec2, exists := executionsJSON[toolExecID2].(map[core.ID]any)
		assert.True(t, exists)
		assert.Equal(t, "tool-2", exec2[core.ID("tool_id")])
		assert.Equal(t, toolExecID2, exec2[core.ID("tool_exec_id")])
		assert.Equal(t, core.StatusPending, exec2[core.ID("status")])
	})

	t.Run("Should return empty map for workflow execution with no tools", func(t *testing.T) {
		nonExistentWorkflowExecID := core.MustNewID()

		executionsJSON, err := tb.ToolRepo.LoadExecutionsMapByWorkflowExecID(tb.Ctx, nonExistentWorkflowExecID)
		require.NoError(t, err)
		assert.Empty(t, executionsJSON)
	})
}

func TestToolRepository_CreateExecution_TemplateNormalization(t *testing.T) {
	tb := setupToolRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should parse templates in tool input during execution creation", func(t *testing.T) {
		// Create workflow execution with input data
		workflowExecID := createTestToolWorkflowExecution(
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
		taskExecID := createTestToolTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create test tool config with template input
		toolConfig := &tool.Config{
			ID:          "template-tool",
			Description: "A tool that processes templates for {{ .trigger.input.user_name }}",
			With: &core.Input{
				"greeting":     "Hello, {{ .trigger.input.user_name }}!",
				"user_id":      "{{ .trigger.input.user_id }}",
				"env_message":  "Environment: {{ .env.WORKFLOW_VAR }}",
				"static_value": "no template here",
			},
			Env: core.EnvMap{
				"TOOL_VAR":    "tool_value",
				"DYNAMIC_VAR": "{{ .trigger.input.user_name }}_processed",
			},
		}

		// Create execution
		_, execution := createTestToolExecution(t, tb, workflowExecID, taskExecID, "template-tool", toolConfig)

		// Verify templates were parsed in input
		input := execution.GetInput()
		assert.Equal(t, "Hello, John Doe!", input.Prop("greeting"))
		assert.Equal(t, "123", input.Prop("user_id")) // Numbers become strings in templates
		assert.Equal(t, "Environment: workflow_value", input.Prop("env_message"))
		assert.Equal(t, "no template here", input.Prop("static_value"))

		// Verify templates were parsed in environment
		env := execution.GetEnv()
		assert.Equal(t, "tool_value", env.Prop("TOOL_VAR"))
		assert.Equal(t, "John Doe_processed", env.Prop("DYNAMIC_VAR"))
		assert.Equal(t, "workflow_value", env.Prop("WORKFLOW_VAR")) // From workflow
		assert.Equal(t, "task_value", env.Prop("TASK_VAR"))         // From task
	})

	t.Run("Should handle nested templates in tool configuration", func(t *testing.T) {
		// Create workflow execution with nested input data
		workflowExecID := createTestToolWorkflowExecution(
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
		taskExecID := createTestToolTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create tool config with nested templates
		toolConfig := &tool.Config{
			ID:          "nested-template-tool",
			Description: "Process user {{ .trigger.input.user.profile.name }}",
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
		_, execution := createTestToolExecution(t, tb, workflowExecID, taskExecID, "nested-template-tool", toolConfig)

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
		workflowExecID := createTestToolWorkflowExecution(
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
		taskExecID := createTestToolTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create tool config with environment merging and templates
		toolConfig := &tool.Config{
			ID:          "env-merge-tool",
			Description: "Process service {{ .trigger.input.service }}",
			With:        &core.Input{},
			Env: core.EnvMap{
				"TOOL_ENV":     "from_tool",
				"SHARED_VAR":   "tool_value", // Should override task value
				"SERVICE_URL":  "https://{{ .trigger.input.service }}.example.com",
				"COMBINED_VAR": "{{ .env.WORKFLOW_ENV }}_and_{{ .env.TASK_ENV }}_and_{{ .env.TOOL_ENV }}",
			},
		}

		// Create execution
		_, execution := createTestToolExecution(t, tb, workflowExecID, taskExecID, "env-merge-tool", toolConfig)

		// Verify environment variable merging and template parsing
		env := execution.GetEnv()

		// Workflow env should be present
		assert.Equal(t, "from_workflow", env.Prop("WORKFLOW_ENV"))

		// Task env should be present
		assert.Equal(t, "from_task", env.Prop("TASK_ENV"))

		// Tool env should be present
		assert.Equal(t, "from_tool", env.Prop("TOOL_ENV"))

		// Tool should override task and workflow for shared variables
		assert.Equal(t, "tool_value", env.Prop("SHARED_VAR"))

		// Templates should be parsed
		assert.Equal(t, "https://user-service.example.com", env.Prop("SERVICE_URL"))

		// Complex template combining multiple env vars should work
		combinedVar := env.Prop("COMBINED_VAR")
		assert.Contains(t, combinedVar, "from_workflow")
		assert.Contains(t, combinedVar, "from_task")
		assert.Contains(t, combinedVar, "from_tool")
	})

	t.Run("Should handle tool with package reference templates", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestToolWorkflowExecution(
			t, tb, core.EnvMap{"PACKAGE_VERSION": "1.0.0"},
			&core.Input{
				"package_name": "compozy-tools",
				"registry":     "npm",
			},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		taskExecID := createTestToolTaskExecution(t, tb, workflowExecID, taskConfig)

		// Create tool config with package reference and templates
		toolConfig := &tool.Config{
			ID:          "package-tool",
			Description: "Tool from package {{ .trigger.input.package_name }}",
			With: &core.Input{
				"package_info": map[string]any{
					"name":     "{{ .trigger.input.package_name }}",
					"version":  "{{ .env.PACKAGE_VERSION }}",
					"registry": "{{ .trigger.input.registry }}",
					"url":      "https://{{ .trigger.input.registry }}.com/{{ .trigger.input.package_name }}",
				},
				"install_command": "{{ .trigger.input.registry }} install {{ .trigger.input.package_name }}@{{ .env.PACKAGE_VERSION }}",
			},
			Env: core.EnvMap{
				"PACKAGE_PATH": "/packages/{{ .trigger.input.package_name }}",
			},
		}

		// Create execution
		_, execution := createTestToolExecution(t, tb, workflowExecID, taskExecID, "package-tool", toolConfig)

		// Verify package reference templates were parsed
		input := execution.GetInput()

		packageInfo, ok := input.Prop("package_info").(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "compozy-tools", packageInfo["name"])
		assert.Equal(t, "1.0.0", packageInfo["version"])
		assert.Equal(t, "npm", packageInfo["registry"])
		assert.Equal(t, "https://npm.com/compozy-tools", packageInfo["url"])

		assert.Equal(t, "npm install compozy-tools@1.0.0", input.Prop("install_command"))

		// Verify environment templates
		env := execution.GetEnv()
		assert.Equal(t, "/packages/compozy-tools", env.Prop("PACKAGE_PATH"))
	})
}
