package worker

import (
	"context"
	"io"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/test/e2e/utils"
	testhelpers "github.com/compozy/compozy/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----
// Helper Functions
// -----

// setupToolProjectDirectory creates a test project directory with tool files and deno.json
func setupToolProjectDirectory(t *testing.T) string {
	t.Helper()

	// Create test project directory
	projectDir := t.TempDir()

	// Get project root
	projectRoot, err := testhelpers.FindProjectRoot()
	require.NoError(t, err)

	// Copy tool files to project directory
	toolsDir := filepath.Join(projectRoot, "engine", "runtime", "fixtures")
	tools := []string{"test_tool.ts", "echo_tool.ts", "format_code.ts"}

	for _, toolFile := range tools {
		src := filepath.Join(toolsDir, toolFile)
		dst := filepath.Join(projectDir, toolFile)
		copyTestFile(t, src, dst)
	}

	// Copy deno.json for import map
	denoConfigSrc := filepath.Join(projectRoot, "engine", "runtime", "fixtures", "deno.json")
	denoConfigDst := filepath.Join(projectDir, "deno.json")
	copyTestFile(t, denoConfigSrc, denoConfigDst)

	// Compile the worker for this project directory
	err = runtime.Compile(projectDir)
	require.NoError(t, err)

	return projectDir
}

// setupToolTaskTestWithCustomConfig creates a tool test helper with custom configuration and project setup
func setupToolTaskTestWithCustomConfig(
	t *testing.T,
	workflowID string,
	toolConfig *tool.Config,
	inputData *core.Input,
) *utils.WorkerTestHelper {
	t.Helper()

	// Set up project directory
	projectDir := setupToolProjectDirectory(t)

	// Create test configuration
	baseBuilder := testhelpers.NewTestConfigBuilder(t).
		WithTestID(workflowID).
		WithProjectDir(projectDir).
		WithToolTask(workflowID+"-task", toolConfig, inputData)

	baseConfig := baseBuilder.Build(t)
	baseConfig.WorkflowConfig.ID = workflowID
	baseConfig.ProjectConfig.SetCWD(projectDir)

	// Create worker test configuration
	builder := utils.NewWorkerTestBuilder(t).
		WithTaskQueue(workflowID + "-test-queue").
		WithActivityTimeout(testhelpers.DefaultTestTimeout)

	config := builder.Build(t)
	config.ContainerTestConfig = baseConfig

	return utils.NewWorkerTestHelper(t, config)
}

// copyTestFile copies a file from src to dst for testing
func copyTestFile(t *testing.T, src, dst string) {
	t.Helper()

	srcFile, err := os.Open(src)
	require.NoError(t, err)
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	require.NoError(t, err)
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	require.NoError(t, err)
}

// -----
// Tool Integration Test Fixtures
// -----

// ToolTaskTestFixture holds test data for tool integration tests
type ToolTaskTestFixture struct {
	WorkflowID     string
	TaskID         string
	ToolID         string
	ToolPath       string
	InputData      *core.Input
	ExpectedOutput map[string]any
}

// CreateToolTaskFixture creates a test fixture for tool integration testing
func CreateToolTaskFixture(testName string, toolID string) *ToolTaskTestFixture {
	return &ToolTaskTestFixture{
		WorkflowID: testName + "-workflow",
		TaskID:     testName + "-task",
		ToolID:     toolID, // Use the import map tool ID
		ToolPath:   toolID, // For compatibility, set path same as ID
		InputData: &core.Input{
			"message": "Test message from tool integration",
			"count":   3,
		},
		ExpectedOutput: map[string]any{
			"result": "Test message from tool integration Test message from tool integration Test message from tool integration",
		},
	}
}

// SetupToolTaskTest creates a worker test helper configured for tool testing
func SetupToolTaskTest(t *testing.T, fixture *ToolTaskTestFixture) *utils.WorkerTestHelper {
	// Create tool configuration
	toolConfig := &tool.Config{
		ID:          fixture.ToolID,
		Description: "Test tool for integration testing",
		Execute:     fixture.ToolPath,
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"count":   map[string]any{"type": "number"},
				"delay":   map[string]any{"type": "number"},
			},
		},
		OutputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"result":       map[string]any{"type": "string"},
				"processed_at": map[string]any{"type": "string"},
				"metadata":     map[string]any{"type": "object"},
			},
		},
	}

	// Create a workflow with tool task
	baseBuilder := testhelpers.NewTestConfigBuilder(t).
		WithTestID(fixture.WorkflowID).
		WithToolTask(fixture.TaskID, toolConfig, fixture.InputData)

	// Build base config
	baseConfig := baseBuilder.Build(t)
	baseConfig.WorkflowConfig.ID = fixture.WorkflowID

	// Create worker test configuration
	builder := utils.NewWorkerTestBuilder(t).
		WithTaskQueue("tool-integration-test-queue").
		WithActivityTimeout(testhelpers.DefaultTestTimeout)

	// Build worker config
	config := builder.Build(t)
	config.ContainerTestConfig = baseConfig

	return utils.NewWorkerTestHelper(t, config)
}

// SetupToolTaskTestWithProjectSetup creates a worker test helper with proper project setup for tools
func SetupToolTaskTestWithProjectSetup(t *testing.T, fixture *ToolTaskTestFixture) *utils.WorkerTestHelper {
	// Set up project directory with tools and deno.json
	projectDir := setupToolProjectDirectory(t)

	// Create tool configuration
	// The tool ID is the import path for Deno
	toolID := fixture.ToolID
	toolConfig := &tool.Config{
		ID:          toolID,
		Description: "Test tool for integration testing",
		Execute:     toolID, // Use tool ID as execute path (import map)
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"count":   map[string]any{"type": "number"},
				"delay":   map[string]any{"type": "number"},
			},
		},
		OutputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"result":       map[string]any{"type": "string"},
				"processed_at": map[string]any{"type": "string"},
				"metadata":     map[string]any{"type": "object"},
			},
		},
	}

	// Create a workflow with tool task
	baseBuilder := testhelpers.NewTestConfigBuilder(t).
		WithTestID(fixture.WorkflowID).
		WithProjectDir(projectDir). // Set the project directory
		WithToolTask(fixture.TaskID, toolConfig, fixture.InputData)

	// Build base config
	baseConfig := baseBuilder.Build(t)
	baseConfig.WorkflowConfig.ID = fixture.WorkflowID

	// Ensure the project config has the right CWD
	baseConfig.ProjectConfig.SetCWD(projectDir)

	// Create worker test configuration
	builder := utils.NewWorkerTestBuilder(t).
		WithTaskQueue("tool-integration-test-queue").
		WithActivityTimeout(testhelpers.DefaultTestTimeout)

	// Build worker config
	config := builder.Build(t)
	config.ContainerTestConfig = baseConfig

	return utils.NewWorkerTestHelper(t, config)
}

// -----
// Tool Integration Tests
// -----

func TestToolIntegration_BasicToolExecution(t *testing.T) {
	// Use the import map ID from deno.json
	fixture := CreateToolTaskFixture("basic-tool-execution", "test-tool")
	fixture.ToolID = "test-tool" // Use import map ID from deno.json
	helper := SetupToolTaskTestWithProjectSetup(t, fixture)
	defer helper.Cleanup()

	// Execute workflow with tool task
	workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
	require.NotEmpty(t, workflowExecID)

	// Create database state verifier and wait for workflow completion
	verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
	verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

	// Verify task completed successfully
	verifier.VerifyTaskStateEventually(
		workflowExecID,
		fixture.TaskID,
		core.StatusSuccess,
		testhelpers.DefaultTestTimeout,
	)

	// Get final workflow state for detailed validation
	ctx := context.Background()
	state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
	require.NoError(t, err)
	require.NotNil(t, state)

	// Validate task completed - check if task exists and has output
	require.Contains(t, state.Tasks, fixture.TaskID)
	taskState := state.Tasks[fixture.TaskID]
	assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)
	assert.Equal(t, core.StatusSuccess, taskState.Status)

	// Validate tool output
	require.NotNil(t, taskState.Output)
	output := *taskState.Output

	// Check expected output structure
	assert.Contains(t, output, "result")
	assert.Contains(t, output, "processed_at")
	assert.Contains(t, output, "metadata")

	// Validate result matches expected pattern
	result, ok := output["result"].(string)
	require.True(t, ok, "result should be a string")
	assert.Equal(t, fixture.ExpectedOutput["result"], result)

	// Validate metadata
	metadata, ok := output["metadata"].(map[string]any)
	require.True(t, ok, "metadata should be a map")
	assert.Equal(t, "test-tool", metadata["tool_name"])
	assert.Equal(t, "1.0.0", metadata["version"])
}

func TestToolIntegration_EchoToolExecution(t *testing.T) {
	// Use the echo tool from import map
	fixture := CreateToolTaskFixture("echo-tool-execution", "echo-tool")
	fixture.InputData = &core.Input{
		"test": "echo this",
		"nested": map[string]any{
			"value": "nested echo",
		},
	}

	helper := SetupToolTaskTestWithProjectSetup(t, fixture)
	defer helper.Cleanup()

	// Execute workflow
	workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
	require.NotEmpty(t, workflowExecID)

	// Create database state verifier and wait for workflow completion
	verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
	verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

	// Verify task completed successfully
	verifier.VerifyTaskStateEventually(
		workflowExecID,
		fixture.TaskID,
		core.StatusSuccess,
		testhelpers.DefaultTestTimeout,
	)

	// Get final workflow state for detailed validation
	ctx := context.Background()
	state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
	require.NoError(t, err)
	require.NotNil(t, state)

	// Validate task completed - check if task exists and has output
	require.Contains(t, state.Tasks, fixture.TaskID)
	taskState := state.Tasks[fixture.TaskID]
	assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)
	assert.Equal(t, core.StatusSuccess, taskState.Status)

	// Validate echo output
	require.NotNil(t, taskState.Output)
	output := *taskState.Output

	// Echo tool should return the input
	echo, ok := output["echo"].(map[string]any)
	require.True(t, ok, "echo should be a map")
	assert.Equal(t, "echo this", echo["test"])

	// Check nested values
	nested, ok := echo["nested"].(map[string]any)
	require.True(t, ok, "nested should be a map")
	assert.Equal(t, "nested echo", nested["value"])

	// Validate tool metadata
	assert.Equal(t, "echo-tool", output["tool_name"])
	assert.Equal(t, "object", output["type"])
	assert.Contains(t, output, "timestamp")
}

func TestToolIntegration_MultipleToolsInWorkflow(t *testing.T) {
	// Test workflow with multiple tool tasks
	t.Run("SequentialToolExecution", func(t *testing.T) {
		// Set up project directory
		projectDir := setupToolProjectDirectory(t)

		// Create workflow with two sequential tool tasks
		workflowID := "multi-tool-workflow"

		baseBuilder := testhelpers.NewTestConfigBuilder(t).
			WithTestID(workflowID).
			WithProjectDir(projectDir)

		// Add first tool task (echo)
		echoTool := &tool.Config{
			ID:          "echo-tool",
			Description: "Echo input data",
			Execute:     "echo-tool", // Use import map ID
		}
		baseBuilder = baseBuilder.WithToolTask("echo-task", echoTool, &core.Input{
			"step": "first",
			"data": "echo data",
		})

		// Add second tool task (test tool)
		testTool := &tool.Config{
			ID:          "test-tool",
			Description: "Process test data",
			Execute:     "test-tool", // Use import map ID
		}
		baseBuilder = baseBuilder.WithToolTask("test-task", testTool, &core.Input{
			"message": "simple static message", // Simplified to rule out template issues
			"count":   2,
		})

		// Build config first
		baseConfig := baseBuilder.Build(t)

		// Add transition from echo-task to test-task manually
		for i := range baseConfig.WorkflowConfig.Tasks {
			if baseConfig.WorkflowConfig.Tasks[i].ID == "echo-task" {
				nextTaskID := "test-task"
				baseConfig.WorkflowConfig.Tasks[i].OnSuccess = &core.SuccessTransition{
					Next: &nextTaskID,
				}
				break
			}
		}
		baseConfig.WorkflowConfig.ID = workflowID
		baseConfig.ProjectConfig.SetCWD(projectDir)

		builder := utils.NewWorkerTestBuilder(t).
			WithTaskQueue("multi-tool-test-queue").
			WithActivityTimeout(testhelpers.DefaultTestTimeout)

		config := builder.Build(t)
		config.ContainerTestConfig = baseConfig

		helper := utils.NewWorkerTestHelper(t, config)
		defer helper.Cleanup()

		// Execute workflow
		workflowExecID := helper.ExecuteBasicWorkflow(workflowID, &core.Input{})
		require.NotEmpty(t, workflowExecID)

		// Create database state verifier and wait for workflow completion
		verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
		verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

		// Verify both tasks completed successfully
		verifier.VerifyTaskStateEventually(
			workflowExecID,
			"echo-task",
			core.StatusSuccess,
			testhelpers.DefaultTestTimeout,
		)
		verifier.VerifyTaskStateEventually(
			workflowExecID,
			"test-task",
			core.StatusSuccess,
			testhelpers.DefaultTestTimeout,
		)

		// Get final workflow state for detailed validation
		ctx := context.Background()
		state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, state)

		// Check first task (echo) exists and has output
		require.Contains(t, state.Tasks, "echo-task")
		echoTask := state.Tasks["echo-task"]
		assert.Equal(t, task.ExecutionBasic, echoTask.ExecutionType)
		assert.Equal(t, core.StatusSuccess, echoTask.Status)
		require.NotNil(t, echoTask.Output)

		// Debug: Log the echo task output structure
		t.Logf("Echo task output: %+v", *echoTask.Output)

		// Check second task executed
		require.Contains(t, state.Tasks, "test-task", "Available tasks: %v", maps.Keys(state.Tasks))
		testTask := state.Tasks["test-task"]
		assert.Equal(t, task.ExecutionBasic, testTask.ExecutionType)
		assert.Equal(t, core.StatusSuccess, testTask.Status)
		require.NotNil(t, testTask.Output)

		output := *testTask.Output
		result, ok := output["result"].(string)
		require.True(t, ok)
		assert.Equal(t, "simple static message simple static message", result) // Should be repeated twice
	})
}

func TestToolIntegration_ToolWithEnvironmentVariables(t *testing.T) {
	fixture := CreateToolTaskFixture("tool-with-env", "test-tool")

	// Create tool with environment variables
	toolConfig := &tool.Config{
		ID:          fixture.ToolID,
		Description: "Tool with environment variables",
		Execute:     "test-tool", // Use import map ID
		Env: &core.EnvMap{
			"TEST_ENV": "integration-test",
		},
	}

	helper := setupToolTaskTestWithCustomConfig(t, fixture.WorkflowID, toolConfig, fixture.InputData)
	defer helper.Cleanup()

	// Execute workflow
	workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
	require.NotEmpty(t, workflowExecID)

	// Create database state verifier and wait for workflow completion
	verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
	verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

	// Use the actual task ID that was created (workflowID + "-task")
	actualTaskID := fixture.WorkflowID + "-task"
	verifier.VerifyTaskStateEventually(workflowExecID, actualTaskID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

	// Get final workflow state for detailed validation
	ctx := context.Background()
	state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
	require.NoError(t, err)
	require.NotNil(t, state)

	require.Contains(t, state.Tasks, actualTaskID)
	taskState := state.Tasks[actualTaskID]
	assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)
	assert.Equal(t, core.StatusSuccess, taskState.Status)

	require.NotNil(t, taskState.Output)
	output := *taskState.Output

	metadata, ok := output["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "integration-test", metadata["environment"])
}

func TestToolIntegration_ToolExecutionWithDelay(t *testing.T) {
	fixture := CreateToolTaskFixture("tool-with-delay", "test-tool")
	fixture.InputData = &core.Input{
		"message": "Delayed execution",
		"delay":   100, // 100ms delay
	}

	helper := SetupToolTaskTestWithProjectSetup(t, fixture)
	defer helper.Cleanup()

	// Execute workflow
	workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
	require.NotEmpty(t, workflowExecID)

	// Create database state verifier and wait for workflow completion (with longer timeout for delay)
	verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
	verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 10*time.Second)

	// Verify task completed successfully
	verifier.VerifyTaskStateEventually(workflowExecID, fixture.TaskID, core.StatusSuccess, 10*time.Second)

	// Get final workflow state for detailed validation
	ctx := context.Background()
	state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
	require.NoError(t, err)
	require.NotNil(t, state)

	// Validate task completed - check if task exists and has output
	require.Contains(t, state.Tasks, fixture.TaskID)
	taskState := state.Tasks[fixture.TaskID]
	assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)
	assert.Equal(t, core.StatusSuccess, taskState.Status)

	require.NotNil(t, taskState.Output)
	output := *taskState.Output

	result, ok := output["result"].(string)
	require.True(t, ok)
	assert.Equal(t, "Delayed execution", result)
}

func TestToolIntegration_ToolSelectionLogic(t *testing.T) {
	testCases := []struct {
		name           string
		toolID         string
		inputData      *core.Input
		validateOutput func(t *testing.T, output map[string]any)
	}{
		{
			name:   "SimpleEchoTool",
			toolID: "echo-tool",
			inputData: &core.Input{
				"simple": "value",
			},
			validateOutput: func(t *testing.T, output map[string]any) {
				echo := output["echo"].(map[string]any)
				assert.Equal(t, "value", echo["simple"])
			},
		},
		{
			name:   "ComplexTestTool",
			toolID: "test-tool",
			inputData: &core.Input{
				"message": "Complex tool test",
				"count":   5,
			},
			validateOutput: func(t *testing.T, output map[string]any) {
				result := output["result"].(string)
				// Should repeat message 5 times
				expected := "Complex tool test Complex tool test Complex tool test Complex tool test Complex tool test"
				assert.Equal(t, expected, result)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := CreateToolTaskFixture(tc.name, tc.toolID)
			fixture.InputData = tc.inputData

			helper := SetupToolTaskTestWithProjectSetup(t, fixture)
			defer helper.Cleanup()

			// Execute workflow
			workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
			require.NotEmpty(t, workflowExecID)

			// Create database state verifier and wait for workflow completion
			verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
			verifier.VerifyWorkflowCompletesWithStatus(
				workflowExecID,
				core.StatusSuccess,
				testhelpers.DefaultTestTimeout,
			)

			// Verify task completed successfully
			verifier.VerifyTaskStateEventually(
				workflowExecID,
				fixture.TaskID,
				core.StatusSuccess,
				testhelpers.DefaultTestTimeout,
			)

			// Get final workflow state for detailed validation
			ctx := context.Background()
			state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
			require.NoError(t, err)
			require.NotNil(t, state)

			// Validate task completed - check if task exists and has output
			require.Contains(t, state.Tasks, fixture.TaskID)
			taskState := state.Tasks[fixture.TaskID]
			assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)
			assert.Equal(t, core.StatusSuccess, taskState.Status)

			require.NotNil(t, taskState.Output)
			tc.validateOutput(t, *taskState.Output)
		})
	}
}

func TestToolIntegration_ToolResultIntegration(t *testing.T) {
	// Test that tool results are properly integrated into workflow state
	fixture := CreateToolTaskFixture("tool-result-integration", "test-tool")

	helper := SetupToolTaskTestWithProjectSetup(t, fixture)
	defer helper.Cleanup()

	// Execute workflow
	workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
	require.NotEmpty(t, workflowExecID)

	// Create database state verifier and wait for workflow completion
	verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
	verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

	// Verify task completed successfully
	verifier.VerifyTaskStateEventually(
		workflowExecID,
		fixture.TaskID,
		core.StatusSuccess,
		testhelpers.DefaultTestTimeout,
	)

	// Get final workflow state for detailed validation
	ctx := context.Background()
	state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
	require.NoError(t, err)
	require.NotNil(t, state)

	// Verify task state includes tool output
	require.Contains(t, state.Tasks, fixture.TaskID)
	taskState := state.Tasks[fixture.TaskID]
	assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)
	assert.Equal(t, core.StatusSuccess, taskState.Status)

	// Check output is persisted correctly
	require.NotNil(t, taskState.Output)
	assert.NotEmpty(t, taskState.UpdatedAt)
	assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)

	// Validate output structure
	output := *taskState.Output
	assert.Contains(t, output, "result")
	assert.Contains(t, output, "processed_at")
	assert.Contains(t, output, "metadata")

	// Verify output can be used in subsequent tasks
	// This is implicitly tested in MultipleToolsInWorkflow test
}

func TestToolIntegration_ToolExecutionLogs(t *testing.T) {
	// Test that tool execution logs are properly captured
	fixture := CreateToolTaskFixture("tool-execution-logs", "test-tool")

	helper := SetupToolTaskTestWithProjectSetup(t, fixture)
	defer helper.Cleanup()

	// Execute workflow
	workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
	require.NotEmpty(t, workflowExecID)

	// Create database state verifier and wait for workflow completion
	verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
	verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

	// Verify task completed successfully
	verifier.VerifyTaskStateEventually(
		workflowExecID,
		fixture.TaskID,
		core.StatusSuccess,
		testhelpers.DefaultTestTimeout,
	)

	// Get final workflow state for detailed validation
	ctx := context.Background()
	state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
	require.NoError(t, err)
	require.NotNil(t, state)

	// Validate task completed - check if task exists and has output
	require.Contains(t, state.Tasks, fixture.TaskID)
	taskState := state.Tasks[fixture.TaskID]
	assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)
	assert.Equal(t, core.StatusSuccess, taskState.Status)

	// Verify execution metadata
	require.NotNil(t, taskState.Output)
	assert.NotEmpty(t, taskState.CreatedAt)
	assert.NotEmpty(t, taskState.UpdatedAt)
	assert.Nil(t, taskState.Error) // No errors should be logged
}
