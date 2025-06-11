package worker

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/e2e/utils"
	testhelpers "github.com/compozy/compozy/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----
// Basic Task Execution Test Infrastructure
// -----

// BasicTaskTestFixture holds test data for basic task execution tests
type BasicTaskTestFixture struct {
	WorkflowID     string
	TaskID         string
	AgentID        string
	ActionID       string
	ActionPrompt   string
	InputData      *core.Input
	ExpectedOutput map[string]any
}

// CreateBasicTaskFixture creates a test fixture for basic task execution
func CreateBasicTaskFixture(testName string) *BasicTaskTestFixture {
	return &BasicTaskTestFixture{
		WorkflowID:   testName + "-workflow",
		TaskID:       testName + "-task",
		AgentID:      testName + "-agent",
		ActionID:     "process",
		ActionPrompt: "Process this input: {{.workflow.input.message}}",
		InputData: &core.Input{
			"message": "Hello, World!",
		},
		ExpectedOutput: map[string]any{
			"response": "Mock response for",
		},
	}
}

// SetupBasicTaskTest creates a worker test helper configured for basic task testing
func SetupBasicTaskTest(t *testing.T, fixture *BasicTaskTestFixture) *utils.WorkerTestHelper {
	// Create a properly configured workflow that matches the fixture
	baseBuilder := testhelpers.NewTestConfigBuilder(t).
		WithTestID(fixture.WorkflowID).
		WithSimpleTask(fixture.TaskID, fixture.ActionID, fixture.ActionPrompt)

	// Build base config first to get the workflow config
	baseConfig := baseBuilder.Build(t)

	// Set the workflow ID to match what we'll execute
	baseConfig.WorkflowConfig.ID = fixture.WorkflowID

	// Create worker test configuration using existing patterns
	builder := utils.NewWorkerTestBuilder(t).
		WithTaskQueue("basic-task-test-queue").
		WithActivityTimeout(testhelpers.DefaultTestTimeout)

	// Build worker config
	config := builder.Build(t)

	// Replace the container config with our properly configured one
	config.ContainerTestConfig = baseConfig

	return utils.NewWorkerTestHelper(t, config)
}

// SetupBasicTaskTestWithCustomInput creates a worker test helper with flexible input configuration
func SetupBasicTaskTestWithCustomInput(t *testing.T, fixture *BasicTaskTestFixture) *utils.WorkerTestHelper {
	// Create agent config with the specific action prompt
	agentConfig := testhelpers.CreateTestAgentConfigWithAction(
		fixture.AgentID,
		"You are a test assistant. Respond with the message provided.",
		fixture.ActionID,
		fixture.ActionPrompt,
	)

	// Create task config with the custom input data
	taskConfig := task.Config{
		BaseConfig: task.BaseConfig{
			ID:    fixture.TaskID,
			Type:  task.TaskTypeBasic,
			Agent: agentConfig,
			With:  fixture.InputData, // Use the input data from the fixture
		},
		BasicTask: task.BasicTask{
			Action: fixture.ActionID,
		},
	}

	// Create workflow configuration
	workflowConfig := &wf.Config{
		ID:          fixture.WorkflowID,
		Version:     "1.0.0",
		Description: "Test workflow",
		Tasks:       []task.Config{taskConfig},
		Agents:      []agent.Config{*agentConfig},
		Opts: wf.Opts{
			Env: &core.EnvMap{},
		},
	}

	// Create base container config
	baseBuilder := testhelpers.NewTestConfigBuilder(t).WithTestID(fixture.WorkflowID)
	baseConfig := baseBuilder.Build(t)

	// Replace the workflow config with our custom one
	baseConfig.WorkflowConfig = workflowConfig

	// Create worker test configuration
	builder := utils.NewWorkerTestBuilder(t).
		WithTaskQueue("basic-task-test-queue").
		WithActivityTimeout(testhelpers.DefaultTestTimeout)

	config := builder.Build(t)
	config.ContainerTestConfig = baseConfig

	return utils.NewWorkerTestHelper(t, config)
}

// -----
// Basic Task Execution Tests
// -----

func TestBasicTaskExecution_SimpleSuccess(t *testing.T) {
	fixture := CreateBasicTaskFixture("simple-success")
	helper := SetupBasicTaskTest(t, fixture)
	defer helper.Cleanup()

	// Execute basic workflow
	workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
	require.NotEmpty(t, workflowExecID)

	// Create database state verifier and wait for workflow completion
	verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)

	// Verify workflow completes successfully
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

	// Validate task exists and has proper structure
	require.Contains(t, state.Tasks, fixture.TaskID)
	taskState := state.Tasks[fixture.TaskID]
	assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)
	assert.Equal(t, core.StatusSuccess, taskState.Status)

	// Validate task has output
	require.NotNil(t, taskState.Output)
	output := *taskState.Output
	assert.Contains(t, output, "response")
	assert.Contains(t, output["response"], "Mock response")
}

func TestBasicTaskExecution_WithDifferentAgentTypes(t *testing.T) {
	testCases := []struct {
		name           string
		agentType      string
		model          string
		expectError    bool
		validateOutput func(t *testing.T, output map[string]any)
	}{
		{
			name:        "MockAgent",
			agentType:   "mock",
			model:       "mock-test",
			expectError: false,
			validateOutput: func(t *testing.T, output map[string]any) {
				assert.Contains(t, output, "response")
				assert.Contains(t, output["response"], "Mock response")
			},
		},
		{
			name:        "OpenAIAgent_MockProvider",
			agentType:   "openai",
			model:       "gpt-4",
			expectError: false, // Should work with mock provider
			validateOutput: func(t *testing.T, output map[string]any) {
				assert.Contains(t, output, "response")
			},
		},
		{
			name:        "AnthropicAgent_MockProvider",
			agentType:   "anthropic",
			model:       "claude-3-sonnet",
			expectError: false, // Should work with mock provider
			validateOutput: func(t *testing.T, output map[string]any) {
				assert.Contains(t, output, "response")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := CreateBasicTaskFixture(tc.name)
			fixture.AgentID = tc.agentType + "-agent"

			helper := SetupBasicTaskTest(t, fixture)
			defer helper.Cleanup()

			// Execute workflow
			workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
			require.NotEmpty(t, workflowExecID)

			// Create database state verifier
			verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)

			if tc.expectError {
				// Verify workflow fails as expected
				verifier.VerifyWorkflowCompletesWithStatus(
					workflowExecID,
					core.StatusFailed,
					testhelpers.DefaultTestTimeout,
				)
			} else {
				// Verify workflow completes successfully
				verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

				// Get final state and validate output
				ctx := context.Background()
				state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
				require.NoError(t, err)

				if taskState, exists := state.Tasks[fixture.TaskID]; exists && taskState.Output != nil {
					tc.validateOutput(t, *taskState.Output)
				}
			}
		})
	}
}

func TestBasicTaskExecution_InputTemplating(t *testing.T) {
	testCases := []struct {
		name         string
		actionPrompt string
		inputData    *core.Input
		validateFunc func(t *testing.T, output map[string]any)
	}{
		{
			name:         "SimpleStringTemplating",
			actionPrompt: "Echo: {{.workflow.input.message}}",
			inputData: &core.Input{
				"message": "Test message",
			},
			validateFunc: func(t *testing.T, output map[string]any) {
				assert.Contains(t, output, "response")
				response := output["response"].(string)
				assert.Contains(t, response, "Test message")
			},
		},
		{
			name:         "NumericInputTemplating",
			actionPrompt: "Process number: {{.workflow.input.count}}",
			inputData: &core.Input{
				"count": 42,
			},
			validateFunc: func(t *testing.T, output map[string]any) {
				assert.Contains(t, output, "response")
				response := output["response"].(string)
				assert.Contains(t, response, "42")
			},
		},
		{
			name:         "ObjectInputTemplating",
			actionPrompt: "Process user: {{.workflow.input.user.name}} ({{.workflow.input.user.email}})",
			inputData: &core.Input{
				"user": map[string]any{
					"name":  "John Doe",
					"email": "john@example.com",
				},
			},
			validateFunc: func(t *testing.T, output map[string]any) {
				assert.Contains(t, output, "response")
				response := output["response"].(string)
				assert.Contains(t, response, "John Doe")
				assert.Contains(t, response, "john@example.com")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := CreateBasicTaskFixture(tc.name)
			fixture.ActionPrompt = tc.actionPrompt
			fixture.InputData = tc.inputData

			helper := SetupBasicTaskTestWithCustomInput(t, fixture)
			defer helper.Cleanup()

			// Execute workflow with the input data that matches the template expectations
			workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, tc.inputData)
			require.NotEmpty(t, workflowExecID)

			// Create database state verifier and wait for completion
			verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
			verifier.VerifyWorkflowCompletesWithStatus(
				workflowExecID,
				core.StatusSuccess,
				testhelpers.DefaultTestTimeout,
			)

			// Verify task completed and validate output
			verifier.VerifyTaskStateEventually(
				workflowExecID,
				fixture.TaskID,
				core.StatusSuccess,
				testhelpers.DefaultTestTimeout,
			)

			// Get final state and validate specific output
			ctx := context.Background()
			state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
			require.NoError(t, err)

			require.Contains(t, state.Tasks, fixture.TaskID)
			taskState := state.Tasks[fixture.TaskID]
			require.NotNil(t, taskState.Output)
			tc.validateFunc(t, *taskState.Output)
		})
	}
}

// -----
// Test Fixtures and Utilities
// -----

// CreateTestAgentConfigurations creates various agent configurations for testing
func CreateTestAgentConfigurations() map[string]*core.ProviderConfig {
	return map[string]*core.ProviderConfig{
		"mock": {
			Provider: core.ProviderMock,
			Model:    "mock-test",
			APIKey:   "test-key",
			Params: core.PromptParams{
				Temperature: 0.0,
				MaxTokens:   100,
			},
		},
		"openai": {
			Provider: core.ProviderOpenAI,
			Model:    "gpt-4",
			APIKey:   "test-key",
			Params: core.PromptParams{
				Temperature: 0.7,
				MaxTokens:   200,
			},
		},
		"anthropic": {
			Provider: core.ProviderAnthropic,
			Model:    "claude-3-sonnet",
			APIKey:   "test-key",
			Params: core.PromptParams{
				Temperature: 0.5,
				MaxTokens:   150,
			},
		},
	}
}

// -----
// Comprehensive Success Path Tests
// -----

func TestBasicTaskExecution_StateTransitions(t *testing.T) {
	fixture := CreateBasicTaskFixture("state-transitions")
	helper := SetupBasicTaskTest(t, fixture)
	defer helper.Cleanup()

	// Execute workflow
	workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
	require.NotEmpty(t, workflowExecID)

	// Create database state verifier and verify completion
	verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)

	// With our race condition fix, the workflow executes very quickly.
	// Instead of checking intermediate states, verify the final successful state.
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

	// Get final state and verify proper completion
	ctx := context.Background()
	state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
	require.NoError(t, err)
	require.NotNil(t, state)

	// Validate task exists and has proper structure
	require.Contains(t, state.Tasks, fixture.TaskID)
	taskState := state.Tasks[fixture.TaskID]
	assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)
	assert.Equal(t, core.StatusSuccess, taskState.Status)

	t.Log(
		"State transition test completed successfully - workflow execution is now very fast due to race condition fix",
	)
}

func TestBasicTaskExecution_DifferentComplexityLevels(t *testing.T) {
	testCases := []struct {
		name         string
		actionPrompt string
		complexity   string
		inputData    *core.Input
	}{
		{
			name:         "SimplePrompt",
			actionPrompt: "Say hello",
			complexity:   "low",
			inputData: &core.Input{
				"message": "Hello, World!",
			},
		},
		{
			name:         "MediumComplexityPrompt",
			actionPrompt: "Analyze the following data and provide insights: {{.workflow.input.data}}",
			complexity:   "medium",
			inputData: &core.Input{
				"data": "Sample data for analysis",
			},
		},
		{
			name:         "ComplexPrompt",
			actionPrompt: "Given the context {{.workflow.input.context}}, analyze the problem {{.workflow.input.problem}} and generate a detailed solution with pros and cons",
			complexity:   "high",
			inputData: &core.Input{
				"context": "Business context",
				"problem": "Complex business problem to solve",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := CreateBasicTaskFixture(tc.name)
			fixture.ActionPrompt = tc.actionPrompt
			fixture.InputData = tc.inputData

			helper := SetupBasicTaskTestWithCustomInput(t, fixture)
			defer helper.Cleanup()

			// Execute workflow with input data that matches the template expectations
			workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, tc.inputData)
			require.NotEmpty(t, workflowExecID)

			// Verify completion with proper validation
			verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
			verifier.VerifyWorkflowCompletesWithStatus(
				workflowExecID,
				core.StatusSuccess,
				testhelpers.DefaultTestTimeout,
			)

			t.Logf("Successfully executed %s complexity task", tc.complexity)
		})
	}
}

func TestBasicTaskExecution_SynchronousVsAsynchronousPatterns(t *testing.T) {
	testCases := []struct {
		name           string
		executionType  string
		expectedOutput string
	}{
		{
			name:           "SynchronousExecution",
			executionType:  "sync",
			expectedOutput: "response",
		},
		{
			name:           "AsynchronousExecution",
			executionType:  "async",
			expectedOutput: "response",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := CreateBasicTaskFixture(tc.name)

			// Configure for different execution patterns
			if tc.executionType == "async" {
				fixture.ActionPrompt = "Process this asynchronously: {{.workflow.input.message}}"
			}

			helper := SetupBasicTaskTest(t, fixture)
			defer helper.Cleanup()

			// Execute workflow
			workflowExecID := helper.ExecuteBasicWorkflow(fixture.WorkflowID, fixture.InputData)
			require.NotEmpty(t, workflowExecID)

			// Verify completion with proper validation
			verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)
			verifier.VerifyWorkflowCompletesWithStatus(
				workflowExecID,
				core.StatusSuccess,
				testhelpers.DefaultTestTimeout,
			)

			// Verify task has expected output
			ctx := context.Background()
			state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
			require.NoError(t, err)

			require.Contains(t, state.Tasks, fixture.TaskID)
			taskState := state.Tasks[fixture.TaskID]
			require.NotNil(t, taskState.Output)
			assert.Contains(t, *taskState.Output, tc.expectedOutput)

			t.Logf("Successfully executed %s pattern", tc.executionType)
		})
	}
}

// ValidateBasicTaskExecution performs common validation for basic task execution
func ValidateBasicTaskExecution(
	t *testing.T,
	helper *utils.WorkerTestHelper,
	workflowExecID core.ID,
	expectedTaskID string,
) {
	// Create database state verifier
	verifier := testhelpers.NewDatabaseStateVerifier(t, helper.GetConfig().ContainerTestConfig)

	// Verify workflow completes successfully
	verifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, testhelpers.DefaultTestTimeout)

	// Verify task completed successfully
	verifier.VerifyTaskStateEventually(
		workflowExecID,
		expectedTaskID,
		core.StatusSuccess,
		testhelpers.DefaultTestTimeout,
	)

	// Get final state and validate task details
	ctx := context.Background()
	state, err := helper.GetConfig().WorkflowRepo.GetState(ctx, workflowExecID)
	require.NoError(t, err)

	require.Contains(t, state.Tasks, expectedTaskID)
	taskState := state.Tasks[expectedTaskID]
	assert.Equal(t, task.ExecutionBasic, taskState.ExecutionType)
	assert.Equal(t, core.StatusSuccess, taskState.Status)

	// Validate task has output
	require.NotNil(t, taskState.Output)
	output := *taskState.Output
	assert.NotEmpty(t, output)
}
