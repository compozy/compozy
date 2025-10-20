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
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/runtime/toolenv"
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
	ctx := t.Context()
	taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
	defer cleanup()
	temporalHelper := newTestTemporalHelper(t)
	defer temporalHelper.Cleanup(t)
	redisHelper := helpers.NewRedisHelper(t)
	defer redisHelper.Cleanup(t)
	activities := createTestActivities(t, taskRepo, workflowRepo, fixture)
	registerWorkflowAndActivities(temporalHelper, activities)
	workflowExecID := core.MustNewID()
	input := buildTemporalInput(fixture, workflowExecID)
	runWorkflowAndAssert(t, temporalHelper, input, fixture.Expected.WorkflowState.Status)
	return fetchFinalWorkflowState(ctx, t, workflowRepo, workflowExecID)
}

func newTestTemporalHelper(t *testing.T) *helpers.TemporalHelper {
	testSuite := &testsuite.WorkflowTestSuite{}
	return helpers.NewTemporalHelper(t, testSuite, "test-task-queue")
}

func registerWorkflowAndActivities(helper *helpers.TemporalHelper, activities *worker.Activities) {
	helper.RegisterWorkflow(worker.CompozyWorkflow)
	registerTestActivities(helper, activities)
}

func buildTemporalInput(fixture *helpers.TestFixture, execID core.ID) worker.WorkflowInput {
	var workflowInput *core.Input
	if fixture.Input != nil {
		input := core.Input(fixture.Input)
		workflowInput = &input
	}
	return worker.WorkflowInput{
		WorkflowID:     fixture.Workflow.ID,
		WorkflowExecID: execID,
		Input:          workflowInput,
		InitialTaskID:  findInitialTaskID(fixture),
	}
}

func runWorkflowAndAssert(
	t *testing.T,
	helper *helpers.TemporalHelper,
	input worker.WorkflowInput,
	expectedStatus string,
) {
	helper.ExecuteWorkflowSync(worker.CompozyWorkflow, input)
	require.True(t, helper.IsWorkflowCompleted(), "Workflow should complete")
	err := helper.GetWorkflowError()
	if expectedStatus != "FAILED" {
		require.NoError(t, err, "Workflow should complete without error")
		return
	}
	require.Error(t, err, "Workflow should fail as expected")
}

func fetchFinalWorkflowState(
	ctx context.Context,
	t *testing.T,
	repo workflow.Repository,
	execID core.ID,
) *workflow.State {
	state, err := repo.GetState(ctx, execID)
	require.NoError(t, err, "Failed to retrieve final workflow state")
	return state
}

// createTestActivities creates activity instances for testing
func createTestActivities(
	t *testing.T,
	taskRepo task.Repository,
	workflowRepo workflow.Repository,
	fixture *helpers.TestFixture,
) *worker.Activities {
	ctx := t.Context()
	projectConfig := createTestProjectConfig(t)
	workflows := createTestWorkflowConfigs(fixture)
	configStore := createTestConfigStore()
	mockRuntime := createMockRuntime(t)
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	toolEnv := buildTestToolEnvironment(
		ctx,
		t,
		projectConfig,
		workflows,
		workflowRepo,
		taskRepo,
	)
	return newTestActivities(
		ctx,
		t,
		projectConfig,
		workflows,
		workflowRepo,
		taskRepo,
		mockRuntime,
		configStore,
		templateEngine,
		toolEnv,
	)
}

func buildTestToolEnvironment(
	ctx context.Context,
	t *testing.T,
	projectConfig *project.Config,
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) toolenv.Environment {
	store := resources.NewMemoryResourceStore()
	require.NoError(t, projectConfig.IndexToResourceStore(ctx, store))
	for _, wfCfg := range workflows {
		require.NoError(t, wfCfg.IndexToResourceStore(ctx, projectConfig.Name, store))
	}
	env, err := builder.Build(projectConfig, workflows, workflowRepo, taskRepo, store)
	require.NoError(t, err)
	return env
}

func newTestActivities(
	ctx context.Context,
	t *testing.T,
	projectConfig *project.Config,
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime runtime.Runtime,
	configStore services.ConfigStore,
	templateEngine *tplengine.TemplateEngine,
	toolEnv toolenv.Environment,
) *worker.Activities {
	acts, err := worker.NewActivities(
		ctx,
		projectConfig,
		workflows,
		workflowRepo,
		taskRepo,
		&helpers.NoopUsageMetrics{},
		providermetrics.Nop(),
		runtime,
		configStore,
		nil,
		nil,
		nil,
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
	tasks := cloneFixtureTasks(fixture.Workflow.Tasks)
	applyAgentDefaults(tasks)
	workflowConfig := &workflow.Config{ID: fixture.Workflow.ID, Tasks: tasks}
	return []*workflow.Config{workflowConfig}
}

func cloneFixtureTasks(source []task.Config) []task.Config {
	cloned := make([]task.Config, len(source))
	copy(cloned, source)
	return cloned
}

func applyAgentDefaults(tasks []task.Config) {
	for i := range tasks {
		if tasks[i].Type != task.TaskTypeBasic {
			continue
		}
		tasks[i].Agent = newTestAgentConfig(&tasks[i])
		tasks[i].Tool = nil
	}
}

func newTestAgentConfig(taskCfg *task.Config) *agent.Config {
	providerConfig := core.NewProviderConfig(core.ProviderMock, "test-model", "")
	return &agent.Config{
		ID:           "test-agent",
		Model:        agent.Model{Config: *providerConfig},
		Instructions: "Test agent for integration testing",
		With:         taskCfg.With,
		Actions: []*agent.ActionConfig{
			newActionConfig("process_message", "Process a message for testing", map[string]any{
				"message": map[string]any{"type": "string"},
				"value":   map[string]any{"type": "number"},
			}),
			newActionConfig("process_with_error", "Process with error for testing", map[string]any{
				"message":     map[string]any{"type": "string"},
				"should_fail": map[string]any{"type": "boolean"},
			}),
			newActionConfig("prepare_data", "Prepare data for testing", map[string]any{
				"initial_value": map[string]any{"type": "number"},
			}),
			newActionConfig("process_data", "Process data for testing", map[string]any{
				"multiplier": map[string]any{"type": "number"},
			}),
			newActionConfig("handle_error", "Handle error for testing", map[string]any{
				"recovery_message": map[string]any{"type": "string"},
			}),
		},
	}
}

func newActionConfig(id, prompt string, properties map[string]any) *agent.ActionConfig {
	return &agent.ActionConfig{
		ID:     id,
		Prompt: prompt,
		InputSchema: &schema.Schema{
			"type":       "object",
			"properties": properties,
		},
	}
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
	basicTasks := collectBasicTaskStates(result)
	require.NotEmpty(t, basicTasks, "Should have at least one basic task")
	logTaskOutputs(t, basicTasks)
	for _, taskState := range basicTasks {
		assert.Equal(t, core.StatusSuccess, taskState.Status, "Basic task %s should be successful", taskState.TaskID)
		require.NotNil(t, taskState.Output, "Basic task %s should have outputs", taskState.TaskID)
		expected := findExpectedTaskState(fixture, taskState.TaskID)
		if expected == nil || expected.Output == nil {
			continue
		}
		compareTaskOutputs(t, taskState.TaskID, *taskState.Output, expected.Output)
	}
}

func collectBasicTaskStates(result *workflow.State) []*task.State {
	var tasks []*task.State
	for _, taskState := range result.Tasks {
		if taskState.ExecutionType == task.ExecutionBasic {
			tasks = append(tasks, taskState)
		}
	}
	return tasks
}

func logTaskOutputs(t *testing.T, tasks []*task.State) {
	for _, basicTask := range tasks {
		t.Logf("DEBUG: Task %s actual output: %+v", basicTask.TaskID, basicTask.Output)
	}
}

func findExpectedTaskState(fixture *helpers.TestFixture, taskID string) *helpers.TaskStateExpectation {
	for i := range fixture.Expected.TaskStates {
		expected := &fixture.Expected.TaskStates[i]
		if expected.Name == taskID {
			return expected
		}
	}
	return nil
}

func compareTaskOutputs(
	t *testing.T,
	taskID string,
	actual core.Output,
	expected map[string]any,
) {
	for key, expectedValue := range expected {
		value, ok := actual[key]
		assert.True(t, ok, "Output key %s should exist in task %s", key, taskID)
		assertJSONEquivalent(t, expectedValue, value, "Output value mismatch for key %s in task %s", key, taskID)
	}
}

func assertJSONEquivalent(t *testing.T, expected any, actual any, msg string, args ...any) {
	t.Helper()
	message := fmt.Sprintf(msg, args...)
	switch ev := expected.(type) {
	case string:
		av, ok := actual.(string)
		if ok {
			assert.Equal(t, normalizeForCompare(ev), normalizeForCompare(av), message)
			return
		}
	case int:
		if av, ok := actual.(float64); ok {
			assert.Equal(t, float64(ev), av, message)
			return
		}
	}
	assert.Equal(t, expected, actual, message)
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
