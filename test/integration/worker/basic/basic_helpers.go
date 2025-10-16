package basic

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"

	"strings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/toolenv/builder"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
	"github.com/compozy/compozy/test/integration/worker/helpers"
)

// normalizeForCompare makes integration string comparisons robust to incidental whitespace
func normalizeForCompare(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	var b strings.Builder
	prevEmpty := false
	for line := range strings.SplitSeq(s, "\n") {
		empty := strings.TrimSpace(line) == ""
		if empty && prevEmpty {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
		prevEmpty = empty
	}
	return b.String()
}

// executeWorkflowAndGetState executes a real workflow and retrieves state from database
func executeWorkflowAndGetState(
	t *testing.T,
	fixture *helpers.TestFixture,
	_ *helpers.DatabaseHelper,
) *workflow.State {
	ctx := context.Background()
	taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
	defer cleanup()

	// Setup Temporal test environment
	testSuite := &testsuite.WorkflowTestSuite{}
	temporalHelper := helpers.NewTemporalHelper(t, testSuite, "test-task-queue")
	defer temporalHelper.Cleanup(t)

	// Create Redis helper for config store
	redisHelper := helpers.NewRedisHelper(t)
	defer redisHelper.Cleanup(t)

	// Create real activities (mock the repositories they need)
	activities := createTestActivities(t, taskRepo, workflowRepo, fixture)

	// Register workflow and activities
	temporalHelper.RegisterWorkflow(worker.CompozyWorkflow)
	registerTestActivities(temporalHelper, activities)

	// Generate workflow execution ID
	workflowExecID := core.MustNewID()

	// Convert fixture input to proper type
	var workflowInput *core.Input
	if fixture.Input != nil {
		input := core.Input(fixture.Input)
		workflowInput = &input
	}

	// Create workflow input for Temporal execution
	temporalInput := worker.WorkflowInput{
		WorkflowID:     fixture.Workflow.ID,
		WorkflowExecID: workflowExecID,
		Input:          workflowInput,
		InitialTaskID:  findInitialTaskID(fixture),
	}

	// Execute real workflow through Temporal test environment
	temporalHelper.ExecuteWorkflowSync(worker.CompozyWorkflow, temporalInput)

	// Verify workflow completed successfully
	require.True(t, temporalHelper.IsWorkflowCompleted(), "Workflow should complete")
	err := temporalHelper.GetWorkflowError()
	// Check for error based on expected status from fixture, not workflow ID naming
	if fixture.Expected.WorkflowState.Status != "FAILED" {
		require.NoError(t, err, "Workflow should complete without error")
	} else {
		require.Error(t, err, "Workflow should fail as expected")
	}

	// Retrieve final workflow state from database
	finalState, err := workflowRepo.GetState(ctx, workflowExecID)
	require.NoError(t, err, "Failed to retrieve final workflow state")

	return finalState
}

// createTestActivities creates activity instances for testing
func createTestActivities(
	t *testing.T,
	taskRepo task.Repository,
	workflowRepo workflow.Repository,
	fixture *helpers.TestFixture,
) *worker.Activities {
	ctx := context.Background()
	// For testing, we need to create a minimal project config and workflow configs
	// These would normally come from the real system setup
	projectConfig := createTestProjectConfig(t)
	workflows := createTestWorkflowConfigs(fixture)

	// Create a test config store
	configStore := createTestConfigStore()

	// Create a mock runtime manager for testing (we don't need actual tool execution)
	mockRuntime := createMockRuntime(t)

	// Create template engine for tests
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)

	// Create memory manager for tests - use nil for now as it's not needed for most tests
	var memoryManager *memory.Manager

	store := resources.NewMemoryResourceStore()
	require.NoError(t, projectConfig.IndexToResourceStore(ctx, store))
	for _, wfCfg := range workflows {
		require.NoError(t, wfCfg.IndexToResourceStore(ctx, projectConfig.Name, store))
	}
	toolEnv, err := builder.Build(projectConfig, workflows, workflowRepo, taskRepo, store)
	require.NoError(t, err)

	// Create test activities with real repositories
	acts, err := worker.NewActivities(
		ctx,
		projectConfig,
		workflows,
		workflowRepo,
		taskRepo,
		nil,
		mockRuntime,
		configStore,
		nil, // signalDispatcher - not needed for basic test
		nil, // redisCache - not needed for basic test
		memoryManager,
		templateEngine,
		toolEnv,
	)
	require.NoError(t, err)
	return acts
}

// registerTestActivities registers all necessary activities with Temporal test environment
func registerTestActivities(temporalHelper *helpers.TemporalHelper, activities *worker.Activities) {
	temporalHelper.RegisterActivity(activities.GetWorkflowData)
	temporalHelper.RegisterActivity(activities.TriggerWorkflow)
	temporalHelper.RegisterActivity(activities.UpdateWorkflowState)
	temporalHelper.RegisterActivity(activities.CompleteWorkflow)
	temporalHelper.RegisterActivity(activities.ExecuteBasicTask)

	// Register activities with specific names as per worker setup
	env := temporalHelper.GetEnvironment()
	env.RegisterActivityWithOptions(
		activities.LoadTaskConfigActivity,
		activity.RegisterOptions{Name: tkacts.LoadTaskConfigLabel},
	)
	env.RegisterActivityWithOptions(
		activities.LoadBatchConfigsActivity,
		activity.RegisterOptions{Name: tkacts.LoadBatchConfigsLabel},
	)
	env.RegisterActivityWithOptions(
		activities.LoadCompositeConfigsActivity,
		activity.RegisterOptions{Name: tkacts.LoadCompositeConfigsLabel},
	)
}

// findInitialTaskID finds the initial task ID from fixture
func findInitialTaskID(fixture *helpers.TestFixture) string {
	if len(fixture.Workflow.Tasks) > 0 {
		return fixture.Workflow.Tasks[0].ID
	}
	return ""
}

// createTestProjectConfig creates a minimal project config for testing
func createTestProjectConfig(t *testing.T) *project.Config {
	cfg := &project.Config{Name: "test-project"}
	if err := cfg.SetCWD(t.TempDir()); err != nil {
		t.Fatalf("failed to set project CWD: %v", err)
	}
	return cfg
}

// createTestWorkflowConfigs creates workflow configs for testing
func createTestWorkflowConfigs(fixture *helpers.TestFixture) []*workflow.Config {
	// For integration tests focused on workflow orchestration,
	// we'll use agent-based tasks instead of tool-based tasks to avoid runtime complexity

	// Update tasks to use agent configuration for simpler testing
	tasks := make([]task.Config, len(fixture.Workflow.Tasks))
	for i := range fixture.Workflow.Tasks {
		tasks[i] = fixture.Workflow.Tasks[i]
		if tasks[i].Type == task.TaskTypeBasic {
			providerConfig := core.NewProviderConfig(core.ProviderMock, "test-model", "")
			// Use a minimal agent configuration for integration testing
			tasks[i].Agent = &agent.Config{
				ID:           "test-agent",
				Model:        agent.Model{Config: *providerConfig},
				Instructions: "Test agent for integration testing",
				With:         tasks[i].With, // Copy task's with field to agent
				Actions: []*agent.ActionConfig{
					{
						ID:     "process_message",
						Prompt: "Process a message for testing",
						InputSchema: &schema.Schema{
							"type": "object",
							"properties": map[string]any{
								"message": map[string]any{"type": "string"},
								"value":   map[string]any{"type": "number"},
							},
						},
					},
					{
						ID:     "process_with_error",
						Prompt: "Process with error for testing",
						InputSchema: &schema.Schema{
							"type": "object",
							"properties": map[string]any{
								"message":     map[string]any{"type": "string"},
								"should_fail": map[string]any{"type": "boolean"},
							},
						},
					},
					{
						ID:     "prepare_data",
						Prompt: "Prepare data for testing",
						InputSchema: &schema.Schema{
							"type": "object",
							"properties": map[string]any{
								"initial_value": map[string]any{"type": "number"},
							},
						},
					},
					{
						ID:     "process_data",
						Prompt: "Process data for testing",
						InputSchema: &schema.Schema{
							"type": "object",
							"properties": map[string]any{
								"multiplier": map[string]any{"type": "number"},
							},
						},
					},
					{
						ID:     "handle_error",
						Prompt: "Handle error for testing",
						InputSchema: &schema.Schema{
							"type": "object",
							"properties": map[string]any{
								"recovery_message": map[string]any{"type": "string"},
							},
						},
					},
				},
			}
			// Remove any tool configuration
			tasks[i].Tool = nil
		}
	}

	// Convert fixture workflow to workflow config
	workflowConfig := &workflow.Config{
		ID:    fixture.Workflow.ID,
		Tasks: tasks,
	}
	return []*workflow.Config{workflowConfig}
}

// createTestConfigStore creates a test config store
func createTestConfigStore() services.ConfigStore {
	// For testing, create a simple in-memory config store that implements the interface
	return &testConfigStore{
		data:     make(map[string]*task.Config),
		metadata: make(map[string][]byte),
	}
}

// createTestConfigManager removed - ConfigManager has been replaced by task2.Factory

// Verification functions that check actual database state

func verifyBasicTaskExecution(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying basic task execution from database state")

	// Find all basic tasks
	var basicTasks []*task.State
	for _, taskState := range result.Tasks {
		if taskState.ExecutionType == task.ExecutionBasic {
			basicTasks = append(basicTasks, taskState)
		}
	}

	require.NotEmpty(t, basicTasks, "Should have at least one basic task")

	// Debug: Print actual output for integration test development
	for _, basicTask := range basicTasks {
		t.Logf("DEBUG: Task %s actual output: %+v", basicTask.TaskID, basicTask.Output)
	}

	// Verify each basic task
	for _, basicTask := range basicTasks {
		assert.Equal(t, core.StatusSuccess, basicTask.Status, "Basic task %s should be successful", basicTask.TaskID)
		require.NotNil(t, basicTask.Output, "Basic task %s should have outputs", basicTask.TaskID)

		// Verify specific outputs match fixture expectations
		for _, expectedTask := range fixture.Expected.TaskStates {
			if expectedTask.Name == basicTask.TaskID && expectedTask.Output != nil {
				for key, expectedValue := range expectedTask.Output {
					actualValue, ok := (*basicTask.Output)[key]
					assert.True(t, ok, "Output key %s should exist in task %s", key, basicTask.TaskID)

					// Handle type conversion for JSON deserialization
					if expectedStr, ok := expectedValue.(string); ok {
						if actualStr, ok := actualValue.(string); ok {
							assert.Equal(
								t,
								normalizeForCompare(expectedStr),
								normalizeForCompare(actualStr),
								"Output value mismatch for key %s in task %s",
								key,
								basicTask.TaskID,
							)
							continue
						}
					}
					if expectedInt, ok := expectedValue.(int); ok {
						if actualFloat, ok := actualValue.(float64); ok {
							assert.Equal(
								t,
								float64(expectedInt),
								actualFloat,
								"Output value mismatch for key %s in task %s",
								key,
								basicTask.TaskID,
							)
						} else {
							assert.Equal(t, expectedValue, actualValue, "Output value mismatch for key %s in task %s", key, basicTask.TaskID)
						}
					} else {
						assert.Equal(t, expectedValue, actualValue, "Output value mismatch for key %s in task %s", key, basicTask.TaskID)
					}
				}
			}
		}
	}
}

func verifyBasicTaskInputs(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying basic task inputs from database state")

	for _, taskState := range result.Tasks {
		if taskState.ExecutionType == task.ExecutionBasic {
			// Verify task has proper input data
			for _, expectedTask := range fixture.Expected.TaskStates {
				if expectedTask.Name == taskState.TaskID && expectedTask.Inputs != nil {
					require.NotNil(t, taskState.Input, "Task should have input data")
					for key, expectedValue := range expectedTask.Inputs {
						actualValue, ok := (*taskState.Input)[key]
						assert.True(t, ok, "Input key %s should exist", key)

						// Handle type conversion for JSON deserialization
						if expectedInt, ok := expectedValue.(int); ok {
							if actualFloat, ok := actualValue.(float64); ok {
								assert.Equal(
									t,
									float64(expectedInt),
									actualFloat,
									"Input value mismatch for key %s",
									key,
								)
							} else {
								assert.Equal(t, expectedValue, actualValue, "Input value mismatch for key %s", key)
							}
						} else {
							assert.Equal(t, expectedValue, actualValue, "Input value mismatch for key %s", key)
						}
					}
				}
			}
		}
	}
}

func verifyBasicErrorHandling(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying error handling from database state")

	var hasFailedTask bool

	for _, taskState := range result.Tasks {
		if taskState.Status == core.StatusFailed {
			hasFailedTask = true
			assert.NotNil(t, taskState.Error, "Failed task should have error information")
		}
	}

	// For error scenario fixtures, verify we have the expected pattern
	if fixture.Expected.WorkflowState.Status == "FAILED" {
		assert.True(t, hasFailedTask, "Workflows expected to fail should have at least one failed task")
		// Note: Error handler doesn't run for validation/execution failures (realistic behavior)
		assert.Equal(
			t,
			core.StatusFailed,
			result.Status,
			"Workflow status should be FAILED as expected",
		)
	}
}

func verifyBasicTaskTransitions(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying task transitions from database state")

	if len(result.Tasks) < 2 {
		t.Log("Skipping transition verification - single task workflow")
		return
	}

	// Convert tasks to slice for ordering
	var tasks []*task.State
	for _, taskState := range result.Tasks {
		tasks = append(tasks, taskState)
	}

	// Sort by creation time using standard library
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})

	// Verify execution order
	for i := 1; i < len(tasks); i++ {
		prevTask := tasks[i-1]
		currentTask := tasks[i]

		assert.True(t,
			currentTask.CreatedAt.After(prevTask.UpdatedAt) || currentTask.CreatedAt.Equal(prevTask.UpdatedAt),
			"Task %s should start after task %s completes", currentTask.TaskID, prevTask.TaskID)
	}
}

func verifyFinalTaskBehavior(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying final task behavior from database state")

	var finalTasks []*task.State
	for _, taskState := range result.Tasks {
		// Check if this task is marked as final in the fixture
		for i := range fixture.Workflow.Tasks {
			taskConfig := &fixture.Workflow.Tasks[i]
			if taskConfig.ID == taskState.TaskID && taskConfig.Final {
				finalTasks = append(finalTasks, taskState)
			}
		}
	}

	if len(finalTasks) > 0 {
		t.Logf("Found %d final tasks", len(finalTasks))
		for _, finalTask := range finalTasks {
			assert.Equal(t, core.StatusSuccess, finalTask.Status, "Final task should complete successfully")
			assert.NotNil(t, finalTask.Output, "Final task should have output")
		}
	}
}

// testConfigStore implements services.ConfigStore for testing
type testConfigStore struct {
	mu       sync.RWMutex
	data     map[string]*task.Config
	metadata map[string][]byte
}

func (s *testConfigStore) Save(_ context.Context, key string, config *task.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = config
	return nil
}

func (s *testConfigStore) Get(_ context.Context, key string) (*task.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	config, exists := s.data[key]
	if !exists {
		return nil, fmt.Errorf("config not found for taskExecID %s", key)
	}
	return config, nil
}

func (s *testConfigStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *testConfigStore) SaveMetadata(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.metadata == nil {
		s.metadata = make(map[string][]byte)
	}
	s.metadata[key] = data
	return nil
}

func (s *testConfigStore) GetMetadata(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, exists := s.metadata[key]
	if !exists {
		return nil, fmt.Errorf("metadata not found for key %s", key)
	}
	return data, nil
}

func (s *testConfigStore) DeleteMetadata(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.metadata != nil {
		delete(s.metadata, key)
	}
	return nil
}

func (s *testConfigStore) Close() error {
	s.data = make(map[string]*task.Config)
	s.metadata = make(map[string][]byte)
	return nil
}

// createMockRuntime creates a mock runtime manager for integration tests
// This focuses on testing workflow orchestration without actual tool execution
func createMockRuntime(t *testing.T) runtime.Runtime {
	// Create a test runtime manager that won't be used since we're using agents
	// We need to provide something to satisfy the interface
	ctx := t.Context()
	config := runtime.TestConfig()
	factory := runtime.NewDefaultFactory("/tmp")
	rtManager, err := factory.CreateRuntime(ctx, config)
	require.NoError(t, err, "failed to create mock runtime manager")
	return rtManager
}
