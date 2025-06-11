package worker

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/e2e/utils"
	testhelpers "github.com/compozy/compozy/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----
// Templating Test Fixtures
// -----

// TemplatingTestFixture holds test data for templating validation
type TemplatingTestFixture struct {
	WorkflowID     string
	Description    string
	InputData      *core.Input
	ExpectedOutput map[string]any
	ValidationFunc func(t *testing.T, finalState *workflow.State)
}

// CreateTemplatingFixture creates a test fixture for templating validation
func CreateTemplatingFixture(testName string, description string) *TemplatingTestFixture {
	return &TemplatingTestFixture{
		WorkflowID:  testName + "-workflow",
		Description: description,
		InputData: &core.Input{
			"name":      "John",
			"city":      "San Francisco",
			"count":     3,
			"timestamp": "2025-06-10T12:00:00Z",
		},
		ExpectedOutput: make(map[string]any),
	}
}

// SetupTemplatingTest creates a worker test helper configured for templating testing
func SetupTemplatingTest(
	t *testing.T,
	fixture *TemplatingTestFixture,
	taskConfigs ...TaskConfig,
) *utils.WorkerTestHelper {
	// Set up project directory with tools
	projectDir := setupToolProjectDirectory(t)

	// Create base test configuration
	baseBuilder := testhelpers.NewTestConfigBuilder(t).
		WithTestID(fixture.WorkflowID).
		WithProjectDir(projectDir)

	// Add tasks based on configuration - only support tool tasks for now
	for _, taskConfig := range taskConfigs {
		baseBuilder = baseBuilder.WithToolTask(taskConfig.ID, taskConfig.ToolConfig, taskConfig.InputData)
	}

	// Build base config
	baseConfig := baseBuilder.Build(t)
	baseConfig.WorkflowConfig.ID = fixture.WorkflowID
	baseConfig.ProjectConfig.SetCWD(projectDir)

	// Add task transitions and outputs if specified
	for _, taskConfig := range taskConfigs {
		if taskConfig.NextTask != "" {
			for j := range baseConfig.WorkflowConfig.Tasks {
				if baseConfig.WorkflowConfig.Tasks[j].ID == taskConfig.ID {
					nextTaskID := taskConfig.NextTask
					baseConfig.WorkflowConfig.Tasks[j].OnSuccess = &core.SuccessTransition{
						Next: &nextTaskID,
					}
					break
				}
			}
		}

		// Add outputs configuration
		if taskConfig.Outputs != nil {
			for j := range baseConfig.WorkflowConfig.Tasks {
				if baseConfig.WorkflowConfig.Tasks[j].ID == taskConfig.ID {
					outputs := core.Input(taskConfig.Outputs)
					baseConfig.WorkflowConfig.Tasks[j].Outputs = &outputs
					break
				}
			}
		}
	}

	// Create worker test configuration
	builder := utils.NewWorkerTestBuilder(t).
		WithTaskQueue("templating-test-queue").
		WithActivityTimeout(testhelpers.DefaultTestTimeout)

	config := builder.Build(t)
	config.ContainerTestConfig = baseConfig

	return utils.NewWorkerTestHelper(t, config)
}

// TaskConfig holds configuration for test tasks
type TaskConfig struct {
	ID         string
	Type       string
	ToolConfig *tool.Config
	InputData  *core.Input
	Outputs    map[string]any
	NextTask   string
}

// -----
// Input Templating Tests
// -----

func TestTemplating_WorkflowInputTemplating(t *testing.T) {
	t.Run("SimpleWorkflowInputReference", func(t *testing.T) {
		fixture := CreateTemplatingFixture("workflow-input", "Test workflow input templating")

		// Create tool that echoes input to validate templating
		toolConfig := &tool.Config{
			ID:          "echo-tool",
			Description: "Echo tool for input templating validation",
			Execute:     "echo-tool",
		}

		taskConfig := TaskConfig{
			ID:         "input-echo-task",
			Type:       "tool",
			ToolConfig: toolConfig,
			InputData: &core.Input{
				"workflow_name": "{{ .workflow.input.name }}",
				"workflow_city": "{{ .workflow.input.city }}",
				"static_value":  "constant",
			},
		}

		helper := SetupTemplatingTest(t, fixture, taskConfig)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
		require.NotEmpty(t, workflowExecID)

		// Verify workflow completion
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)
		verifier.VerifyTaskStateEventually(
			workflowExecID,
			"input-echo-task",
			core.StatusSuccess,
			testhelpers.DefaultTestTimeout,
		)

		// Get final state and validate templating
		ctx := context.Background()
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		taskState := state.Tasks["input-echo-task"]
		require.NotNil(t, taskState.Output)
		output := *taskState.Output

		// Verify input was properly templated
		echo := output["echo"].(map[string]any)
		assert.Equal(t, "John", echo["workflow_name"])
		assert.Equal(t, "San Francisco", echo["workflow_city"])
		assert.Equal(t, "constant", echo["static_value"])
	})

	t.Run("ComplexWorkflowInputTemplating", func(t *testing.T) {
		fixture := CreateTemplatingFixture("complex-input", "Test complex workflow input templating")

		toolConfig := &tool.Config{
			ID:          "test-tool",
			Description: "Test tool for complex input templating",
			Execute:     "test-tool",
		}

		taskConfig := TaskConfig{
			ID:         "complexinputtask",
			Type:       "tool",
			ToolConfig: toolConfig,
			InputData: &core.Input{
				"message": "Hello {{ .workflow.input.name }} from {{ .workflow.input.city }}",
				"count":   "{{ .workflow.input.count }}",
			},
		}

		helper := SetupTemplatingTest(t, fixture, taskConfig)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
		require.NotEmpty(t, workflowExecID)

		// Verify completion and validate templating
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		ctx := context.Background()
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		taskState := state.Tasks["complexinputtask"]
		require.NotNil(t, taskState.Output)
		output := *taskState.Output

		// Verify complex message templating - test tool should process the templated input
		assert.Equal(t, "Hello John from San Francisco", output["result"])
	})
}

func TestTemplating_TaskOutputTemplating(t *testing.T) {
	t.Run("SingleTaskOutputReference", func(t *testing.T) {
		fixture := CreateTemplatingFixture("task-output", "Test task output templating")

		// First task generates output
		firstTool := &tool.Config{
			ID:          "test-tool",
			Description: "First tool that generates output",
			Execute:     "test-tool",
		}

		// Second task references first task's output
		secondTool := &tool.Config{
			ID:          "echo-tool",
			Description: "Second tool that echoes first task output",
			Execute:     "echo-tool",
		}

		taskConfigs := []TaskConfig{
			{
				ID:         "first-task",
				Type:       "tool",
				ToolConfig: firstTool,
				InputData: &core.Input{
					"message": "Generated data",
					"count":   2,
				},
				NextTask: "second-task",
			},
			{
				ID:         "second-task",
				Type:       "tool",
				ToolConfig: secondTool,
				InputData: &core.Input{
					"previous_result": "{{ .tasks.first-task.output.result }}",
					"previous_meta":   "{{ .tasks.first-task.output.metadata }}",
				},
			},
		}

		helper := SetupTemplatingTest(t, fixture, taskConfigs...)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
		require.NotEmpty(t, workflowExecID)

		// Verify completion
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)
		verifier.VerifyTaskStateEventually(
			workflowExecID,
			"first-task",
			core.StatusSuccess,
			testhelpers.DefaultTestTimeout,
		)
		verifier.VerifyTaskStateEventually(
			workflowExecID,
			"second-task",
			core.StatusSuccess,
			testhelpers.DefaultTestTimeout,
		)

		// Validate task output templating
		ctx := context.Background()
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		firstTask := state.Tasks["first-task"]
		secondTask := state.Tasks["second-task"]

		require.NotNil(t, firstTask.Output)
		require.NotNil(t, secondTask.Output)

		firstOutput := *firstTask.Output
		secondOutput := *secondTask.Output

		// Verify second task received first task's output
		secondEcho := secondOutput["echo"].(map[string]any)
		assert.Equal(t, firstOutput["result"], secondEcho["previous_result"])
		assert.Equal(t, firstOutput["metadata"], secondEcho["previous_meta"])
	})

	t.Run("TaskOutputWithSprigFunctions", func(t *testing.T) {
		fixture := CreateTemplatingFixture("sprig-functions", "Test task output with Sprig functions")

		// First task generates JSON output
		firstTool := &tool.Config{
			ID:          "test-tool",
			Description: "Tool that generates structured output",
			Execute:     "test-tool",
		}

		// Second task uses Sprig functions to process output
		secondTool := &tool.Config{
			ID:          "echo-tool",
			Description: "Tool that processes output with Sprig functions",
			Execute:     "echo-tool",
		}

		taskConfigs := []TaskConfig{
			{
				ID:         "datatask",
				Type:       "tool",
				ToolConfig: firstTool,
				InputData: &core.Input{
					"message": "test data",
					"count":   3,
				},
				NextTask: "processtask",
			},
			{
				ID:         "processtask",
				Type:       "tool",
				ToolConfig: secondTool,
				InputData: &core.Input{
					"json_output":  "{{ .tasks.datatask.output | toJson }}",
					"result_upper": "{{ .tasks.datatask.output.result | upper }}",
					"timestamp":    "{{ now }}",
				},
			},
		}

		helper := SetupTemplatingTest(t, fixture, taskConfigs...)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
		require.NotEmpty(t, workflowExecID)

		// Verify completion
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		ctx := context.Background()
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		processTask := state.Tasks["processtask"]
		require.NotNil(t, processTask.Output)

		processOutput := *processTask.Output
		processEcho := processOutput["echo"].(map[string]any)

		// Verify Sprig functions worked
		jsonOutputMap := processEcho["json_output"].(map[string]any)
		assert.Contains(t, jsonOutputMap["result"], "test data test data test data")
		assert.Equal(t, "TEST DATA TEST DATA TEST DATA", processEcho["result_upper"])
		assert.NotEmpty(t, processEcho["timestamp"])
	})
}

func TestTemplating_OutputFormattingAndTransformation(t *testing.T) {
	t.Run("TaskOutputsMapping", func(t *testing.T) {
		fixture := CreateTemplatingFixture("output-mapping", "Test task outputs mapping")

		toolConfig := &tool.Config{
			ID:          "test-tool",
			Description: "Tool that generates data for output mapping",
			Execute:     "test-tool",
		}

		taskConfig := TaskConfig{
			ID:         "mappingtask",
			Type:       "tool",
			ToolConfig: toolConfig,
			InputData: &core.Input{
				"message": "Original message",
				"count":   1,
			},
			Outputs: map[string]any{
				"transformed_result": "{{ .output.result | upper }}",
				"processed_at":       "{{ .output.processed_at }}",
				"workflow_context": map[string]any{
					"input_name": "{{ .workflow.input.name }}",
					"input_city": "{{ .workflow.input.city }}",
					"result":     "{{ .output.result }}",
				},
				"metadata_summary": map[string]any{
					"tool_name": "{{ .output.metadata.tool_name }}",
					"version":   "{{ .output.metadata.version }}",
					"timestamp": "{{ now }}",
				},
			},
		}

		helper := SetupTemplatingTest(t, fixture, taskConfig)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
		require.NotEmpty(t, workflowExecID)

		// Verify completion
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		ctx := context.Background()
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		taskState := state.Tasks["mappingtask"]
		require.NotNil(t, taskState.Output)

		taskOutput := *taskState.Output

		// Verify output transformations
		assert.Equal(t, "ORIGINAL MESSAGE", taskOutput["transformed_result"])
		assert.NotEmpty(t, taskOutput["processed_at"])

		// Verify nested output mappings
		workflowContext := taskOutput["workflow_context"].(map[string]any)
		assert.Equal(t, "John", workflowContext["input_name"])
		assert.Equal(t, "San Francisco", workflowContext["input_city"])
		assert.Equal(t, "Original message", workflowContext["result"])

		metadataSummary := taskOutput["metadata_summary"].(map[string]any)
		assert.Equal(t, "test-tool", metadataSummary["tool_name"])
		assert.Equal(t, "1.0.0", metadataSummary["version"])
		assert.NotEmpty(t, metadataSummary["timestamp"])
	})

	t.Run("ConditionalOutputTemplating", func(t *testing.T) {
		fixture := CreateTemplatingFixture("conditional-output", "Test conditional output templating")

		toolConfig := &tool.Config{
			ID:          "test-tool",
			Description: "Tool for conditional templating test",
			Execute:     "test-tool",
		}

		taskConfig := TaskConfig{
			ID:         "conditionaltask",
			Type:       "tool",
			ToolConfig: toolConfig,
			InputData: &core.Input{
				"message": "condition test",
				"count":   2,
			},
			Outputs: map[string]any{
				"status":         "{{ if eq .output.result \"condition test condition test\" }}success{{ else }}failure{{ end }}",
				"message_length": "{{ len .output.result }}",
				"has_metadata":   "{{ if .output.metadata }}true{{ else }}false{{ end }}",
				"tool_info":      "{{ .output.metadata.tool_name | default \"unknown\" }}",
			},
		}

		helper := SetupTemplatingTest(t, fixture, taskConfig)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
		require.NotEmpty(t, workflowExecID)

		// Verify completion
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		ctx := context.Background()
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		taskState := state.Tasks["conditionaltask"]
		require.NotNil(t, taskState.Output)

		taskOutput := *taskState.Output

		// Verify conditional templating
		assert.Equal(t, "success", taskOutput["status"])
		assert.Equal(t, "29", taskOutput["message_length"]) // "condition test condition test" = 29 chars
		assert.Equal(t, "true", taskOutput["has_metadata"])
		assert.Equal(t, "test-tool", taskOutput["tool_info"])
	})
}

func TestTemplating_ComplexTemplatingScenarios(t *testing.T) {
	t.Run("MultiTaskOutputChaining", func(t *testing.T) {
		fixture := CreateTemplatingFixture("multi-task-chain", "Test multi-task output chaining")

		// Three tasks that chain outputs together
		taskConfigs := []TaskConfig{
			{
				ID:         "taska",
				Type:       "tool",
				ToolConfig: &tool.Config{ID: "test-tool", Execute: "test-tool"},
				InputData: &core.Input{
					"message": "Task A: {{ .workflow.input.name }}",
					"count":   1,
				},
				NextTask: "taskb",
				Outputs: map[string]any{
					"processed_name": "{{ .output.result }}",
					"source":         "taska",
				},
			},
			{
				ID:         "taskb",
				Type:       "tool",
				ToolConfig: &tool.Config{ID: "test-tool", Execute: "test-tool"},
				InputData: &core.Input{
					"message": "Task B: {{ .tasks.taska.output.processed_name }}",
					"count":   1,
				},
				NextTask: "taskc",
				Outputs: map[string]any{
					"combined_result": "{{ .output.result }}",
					"chain_info": map[string]any{
						"from_a": "{{ .tasks.taska.output.processed_name }}",
						"from_b": "{{ .output.result }}",
					},
				},
			},
			{
				ID:         "taskc",
				Type:       "tool",
				ToolConfig: &tool.Config{ID: "echo-tool", Execute: "echo-tool"},
				InputData: &core.Input{
					"final_result":   "{{ .tasks.taskb.output.combined_result }}",
					"chain_summary":  "{{ .tasks.taskb.output.chain_info | toJson }}",
					"original_input": "{{ .workflow.input.name }}",
				},
				Outputs: map[string]any{
					"complete_chain": map[string]any{
						"start":  "{{ .workflow.input.name }}",
						"task_a": "{{ .tasks.taska.output.processed_name }}",
						"task_b": "{{ .tasks.taskb.output.combined_result }}",
						"final":  "{{ .output.echo.final_result }}",
					},
				},
			},
		}

		helper := SetupTemplatingTest(t, fixture, taskConfigs...)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
		require.NotEmpty(t, workflowExecID)

		// Verify completion
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		ctx := context.Background()
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		// Verify the complete chain
		taskC := state.Tasks["taskc"]
		require.NotNil(t, taskC.Output)

		finalOutput := *taskC.Output
		completeChain := finalOutput["complete_chain"].(map[string]any)

		assert.Equal(t, "John", completeChain["start"])
		assert.Equal(t, "Task A: John", completeChain["task_a"])
		assert.Equal(t, "Task B: Task A: John", completeChain["task_b"])
		assert.Equal(t, "Task B: Task A: John", completeChain["final"])
	})

	t.Run("TemplatingErrorHandling", func(t *testing.T) {
		t.Skip("Skipping error handling test - will be covered in error scenario tests")
		// This test would be moved to the error handling test suite
		// where we test invalid template syntax, missing references, etc.
	})
}

func TestTemplating_TemplateValidationAndEdgeCases(t *testing.T) {
	t.Run("EmptyAndNilTemplateValues", func(t *testing.T) {
		fixture := CreateTemplatingFixture("empty-values", "Test empty and nil template values")

		toolConfig := &tool.Config{
			ID:          "echo-tool",
			Description: "Tool for testing empty values",
			Execute:     "echo-tool",
		}

		taskConfig := TaskConfig{
			ID:         "emptytesttask",
			Type:       "tool",
			ToolConfig: toolConfig,
			InputData: &core.Input{
				"empty_string": "",
				"null_value":   nil,
				"zero_count":   0,
			},
			Outputs: map[string]any{
				"empty_check": "{{ .output.echo.empty_string | empty }}",
				"null_check":  "{{ .output.echo.null_value | empty }}",
				"zero_check":  "{{ eq (.output.echo.zero_count | toString) \"0\" }}",
				"has_empty":   "{{ if .output.echo.empty_string }}false{{ else }}true{{ end }}",
			},
		}

		helper := SetupTemplatingTest(t, fixture, taskConfig)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
		require.NotEmpty(t, workflowExecID)

		// Verify completion
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		ctx := context.Background()
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		taskState := state.Tasks["emptytesttask"]
		require.NotNil(t, taskState.Output)

		taskOutput := *taskState.Output

		// Verify empty value handling
		assert.Equal(t, "true", taskOutput["empty_check"])
		assert.Equal(t, "true", taskOutput["null_check"])
		assert.Equal(t, "true", taskOutput["zero_check"])
		assert.Equal(t, "true", taskOutput["has_empty"])
	})

	t.Run("ComplexDataStructureTemplating", func(t *testing.T) {
		fixture := CreateTemplatingFixture("complex-data", "Test complex data structure templating")

		// Input with nested structures
		fixture.InputData = &core.Input{
			"user": map[string]any{
				"name": "Alice",
				"profile": map[string]any{
					"age":      30,
					"location": "Seattle",
					"tags":     []string{"developer", "gopher", "ai"},
				},
			},
			"settings": map[string]any{
				"theme":  "dark",
				"notify": true,
			},
		}

		toolConfig := &tool.Config{
			ID:          "echo-tool",
			Description: "Tool for complex structure testing",
			Execute:     "echo-tool",
		}

		taskConfig := TaskConfig{
			ID:         "complexstructuretask",
			Type:       "tool",
			ToolConfig: toolConfig,
			InputData: &core.Input{
				"user_name": "{{ .workflow.input.user.name }}",
				"user_age":  "{{ .workflow.input.user.profile.age }}",
				"location":  "{{ .workflow.input.user.profile.location }}",
				"first_tag": "{{ index .workflow.input.user.profile.tags 0 }}",
				"tag_count": "{{ len .workflow.input.user.profile.tags }}",
				"theme":     "{{ .workflow.input.settings.theme }}",
			},
			Outputs: map[string]any{
				"user_summary": map[string]any{
					"name":     "{{ .output.echo.user_name }}",
					"age":      "{{ .output.echo.user_age }}",
					"location": "{{ .output.echo.location }}",
					"tags": map[string]any{
						"first": "{{ .output.echo.first_tag }}",
						"count": "{{ .output.echo.tag_count }}",
					},
				},
				"settings_info": map[string]any{
					"theme":        "{{ .output.echo.theme }}",
					"is_dark_mode": "{{ eq .output.echo.theme \"dark\" }}",
				},
			},
		}

		helper := SetupTemplatingTest(t, fixture, taskConfig)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
		require.NotEmpty(t, workflowExecID)

		// Verify completion
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		ctx := context.Background()
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)

		taskState := state.Tasks["complexstructuretask"]
		require.NotNil(t, taskState.Output)

		taskOutput := *taskState.Output
		userSummary := taskOutput["user_summary"].(map[string]any)
		settingsInfo := taskOutput["settings_info"].(map[string]any)

		// Verify complex structure templating
		assert.Equal(t, "Alice", userSummary["name"])
		assert.Equal(t, "30", userSummary["age"])
		assert.Equal(t, "Seattle", userSummary["location"])

		tags := userSummary["tags"].(map[string]any)
		assert.Equal(t, "developer", tags["first"])
		assert.Equal(t, "3", tags["count"])

		assert.Equal(t, "dark", settingsInfo["theme"])
		assert.Equal(t, "true", settingsInfo["is_dark_mode"])
	})
}
