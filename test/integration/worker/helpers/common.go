package helpers

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	coreruntime "github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
)

// TestConfigStore implements services.ConfigStore for testing
type TestConfigStore struct {
	mu       sync.RWMutex
	data     map[string]*task.Config
	metadata map[string][]byte
}

func NewTestConfigStore() *TestConfigStore {
	return &TestConfigStore{
		data:     make(map[string]*task.Config),
		metadata: make(map[string][]byte),
	}
}

func (s *TestConfigStore) Save(_ context.Context, key string, config *task.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = config
	return nil
}

func (s *TestConfigStore) Get(_ context.Context, key string) (*task.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	config, exists := s.data[key]
	if !exists {
		return nil, fmt.Errorf("config not found for taskExecID %s", key)
	}
	return config, nil
}

func (s *TestConfigStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *TestConfigStore) SaveMetadata(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.metadata == nil {
		s.metadata = make(map[string][]byte)
	}
	s.metadata[key] = data
	return nil
}

func (s *TestConfigStore) GetMetadata(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, exists := s.metadata[key]
	if !exists {
		return nil, fmt.Errorf("metadata not found for key %s", key)
	}
	return data, nil
}

func (s *TestConfigStore) DeleteMetadata(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.metadata != nil {
		delete(s.metadata, key)
	}
	return nil
}

func (s *TestConfigStore) Close() error {
	s.data = make(map[string]*task.Config)
	s.metadata = make(map[string][]byte)
	return nil
}

// CreateMockRuntime creates a mock runtime manager for integration tests
func CreateMockRuntime(t *testing.T) *coreruntime.Manager {
	ctx := t.Context()
	rtManager, err := coreruntime.NewRuntimeManager(ctx, "/tmp", coreruntime.WithTestConfig())
	require.NoError(t, err, "failed to create mock runtime manager")
	return rtManager
}

// CreateTestProjectConfig creates a minimal project config for testing
func CreateTestProjectConfig(_ *TestFixture, projectName string) *project.Config {
	cwd, err := core.CWDFromPath("/tmp/test-project")
	if err != nil {
		// Fallback to current directory if path creation fails
		if cwd, err = core.CWDFromPath(""); err != nil {
			// If even current directory fails, use nil (should not happen in tests)
			cwd = nil
		}
	}

	return &project.Config{
		Name: projectName,
		CWD:  cwd,
	}
}

// CreateTestConfigManager creates a test config manager
func CreateTestConfigManager(configStore *TestConfigStore) *services.ConfigManager {
	return services.NewConfigManager(configStore, nil)
}

// FindInitialTaskID finds the initial task ID from fixture
func FindInitialTaskID(fixture *TestFixture) string {
	if len(fixture.Workflow.Tasks) > 0 {
		return fixture.Workflow.Tasks[0].ID
	}
	return ""
}

// RegisterCommonActivities registers common activities with Temporal test environment
func RegisterCommonActivities(env *testsuite.TestWorkflowEnvironment, activities *worker.Activities) {
	env.RegisterActivity(activities.GetWorkflowData)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.CompleteWorkflow)
	env.RegisterActivity(activities.ExecuteBasicTask)
	env.RegisterActivity(activities.ExecuteRouterTask)
	env.RegisterActivity(activities.CreateCollectionState)
	env.RegisterActivity(activities.CreateParallelState)
	env.RegisterActivity(activities.CreateCompositeState)
	env.RegisterActivity(activities.ListChildStates)
	env.RegisterActivity(activities.ExecuteSubtask)
	env.RegisterActivity(activities.GetCollectionResponse)
	env.RegisterActivity(activities.GetParallelResponse)
	env.RegisterActivity(activities.GetCompositeResponse)

	// Register activities with specific names as per worker setup
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

// CreateBasicAgentConfig creates a basic agent configuration for testing
func CreateBasicAgentConfig() *agent.Config {
	return &agent.Config{
		ID:           "test-agent",
		Config:       core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"},
		Instructions: "Test agent for integration testing",
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
}

// CreateCollectionAgentConfig creates a collection-specific agent configuration for testing
func CreateCollectionAgentConfig() *agent.Config {
	return &agent.Config{
		ID:           "test-collection-agent",
		Config:       core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"},
		Instructions: "Test agent for collection workflow integration testing",
		Actions: []*agent.ActionConfig{
			{
				ID:     "process_item",
				Prompt: "Process a single collection item",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"item_name":  map[string]any{"type": "string"},
						"item_value": map[string]any{"type": "number"},
						"multiplier": map[string]any{"type": "number"},
					},
				},
			},
			{
				ID:     "process_parallel_item",
				Prompt: "Process a parallel collection item",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"task_id":    map[string]any{"type": "string"},
						"priority":   map[string]any{"type": "string"},
						"timeout_ms": map[string]any{"type": "number"},
						"start_time": map[string]any{"type": "string"},
					},
				},
			},
			{
				ID:     "process_with_failure",
				Prompt: "Process item that may fail",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"item_id":     map[string]any{"type": "string"},
						"should_fail": map[string]any{"type": "boolean"},
						"value":       map[string]any{"type": "number"},
					},
				},
			},
			{
				ID:     "aggregate_results",
				Prompt: "Aggregate collection results",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"child_results": map[string]any{"type": "array"},
						"total_items":   map[string]any{"type": "number"},
					},
				},
			},
			{
				ID:     "handle_empty_collection",
				Prompt: "Handle empty collection case",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"collection_size": map[string]any{"type": "number"},
					},
				},
			},
		},
	}
}

// CreateParallelAgentConfig creates a parallel-specific agent configuration for testing
func CreateParallelAgentConfig() *agent.Config {
	return &agent.Config{
		ID:           "test-parallel-agent",
		Config:       core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"},
		Instructions: "Test agent for parallel workflow integration testing",
		Actions: []*agent.ActionConfig{
			{
				ID:     "process_parallel_task",
				Prompt: "Process a task in parallel execution",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"task_name": map[string]any{"type": "string"},
						"duration":  map[string]any{"type": "number"},
						"value":     map[string]any{"type": "string"},
					},
				},
			},
			{
				ID:     "synchronize_results",
				Prompt: "Synchronize parallel task results",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"child_results": map[string]any{"type": "array"},
						"strategy":      map[string]any{"type": "string"},
						"total_tasks":   map[string]any{"type": "number"},
					},
				},
			},
			{
				ID:     "handle_parallel_failure",
				Prompt: "Handle failure in parallel execution",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"task_id":     map[string]any{"type": "string"},
						"error_type":  map[string]any{"type": "string"},
						"should_fail": map[string]any{"type": "boolean"},
					},
				},
			},
			{
				ID:     "aggregate_parallel_outputs",
				Prompt: "Aggregate outputs from parallel tasks",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"outputs":        map[string]any{"type": "array"},
						"success_count":  map[string]any{"type": "number"},
						"failed_count":   map[string]any{"type": "number"},
						"execution_time": map[string]any{"type": "number"},
					},
				},
			},
			{
				ID:     "execute_concurrent_task",
				Prompt: "Execute a single task in concurrent mode",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"task_index":   map[string]any{"type": "number"},
						"start_time":   map[string]any{"type": "string"},
						"max_duration": map[string]any{"type": "number"},
						"input_data":   map[string]any{"type": "object"},
					},
				},
			},
		},
	}
}

// CreateTestActivities creates activity instances for testing
func CreateTestActivities(
	_ *testing.T,
	taskRepo *store.TaskRepo,
	workflowRepo *store.WorkflowRepo,
	fixture *TestFixture,
	runtime *coreruntime.Manager,
	configStore *TestConfigStore,
	projectName string,
	agentConfig *agent.Config,
) *worker.Activities {
	projectConfig := CreateTestProjectConfig(fixture, projectName)
	workflows := createTestWorkflowConfigs(fixture, agentConfig)
	configManager := CreateTestConfigManager(configStore)

	return worker.NewActivities(
		projectConfig,
		workflows,
		workflowRepo,
		taskRepo,
		runtime,
		configStore,
		nil, // signalDispatcher - not needed for test
		configManager,
		nil, // redisCache - not needed for test
		nil, // memoryManager - not needed for test
		nil, // templateEngine - not needed for test
	)
}

// createTestWorkflowConfigs creates workflow configs for testing with the provided agent config
func createTestWorkflowConfigs(fixture *TestFixture, agentConfig *agent.Config) []*workflow.Config {
	// Create CWD for tasks
	cwd, err := core.CWDFromPath("/tmp/test-project")
	if err != nil {
		// Fallback to current directory if path creation fails
		if cwd, err = core.CWDFromPath(""); err != nil {
			// If even current directory fails, use nil (should not happen in tests)
			cwd = nil
		}
	}

	tasks := make([]task.Config, len(fixture.Workflow.Tasks))
	for i := range fixture.Workflow.Tasks {
		t := fixture.Workflow.Tasks[i]
		tasks[i] = t
		// Apply agent configuration and CWD to appropriate task types
		if t.Type == task.TaskTypeBasic || t.Type == task.TaskTypeCollection {
			tasks[i].Agent = agentConfig
			tasks[i].Tool = nil // Remove any tool configuration
			tasks[i].CWD = cwd  // Set CWD for task validation

			// For collection tasks, also apply agent to the child task template
			if t.Type == task.TaskTypeCollection && t.Task != nil {
				tasks[i].Task.Agent = agentConfig
				tasks[i].Task.Tool = nil
				tasks[i].Task.CWD = cwd
			}
		}

		// For parallel tasks, apply agent to all child tasks
		if t.Type == task.TaskTypeParallel {
			for j := range tasks[i].Tasks {
				if tasks[i].Tasks[j].Type == task.TaskTypeBasic {
					tasks[i].Tasks[j].Agent = agentConfig
					tasks[i].Tasks[j].Tool = nil
					tasks[i].Tasks[j].CWD = cwd
				}
			}
		}
	}

	workflowConfig := &workflow.Config{
		ID:    fixture.Workflow.ID,
		Tasks: tasks,
	}
	return []*workflow.Config{workflowConfig}
}

// ExecuteWorkflowAndGetState executes a real Temporal workflow and retrieves final state from database
func ExecuteWorkflowAndGetState(
	t *testing.T,
	fixture *TestFixture,
	dbHelper *DatabaseHelper,
	projectName string,
	agentConfig *agent.Config,
) *workflow.State {
	ctx := context.Background()

	// Create test suite and worker
	testSuite := testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Create repositories and runtime
	taskRepo := store.NewTaskRepo(dbHelper.GetPool())
	workflowRepo := store.NewWorkflowRepo(dbHelper.GetPool())
	configStore := NewTestConfigStore()
	runtime := CreateMockRuntime(t)

	// Ensure proper cleanup of resources
	defer func() {
		if err := configStore.Close(); err != nil {
			t.Logf("Warning: failed to close config store: %v", err)
		}
	}()

	// Register activities with test activities
	activities := CreateTestActivities(
		t,
		taskRepo,
		workflowRepo,
		fixture,
		runtime,
		configStore,
		projectName,
		agentConfig,
	)
	RegisterCommonActivities(env, activities)

	// Prepare workflow input
	workflowExecID := core.MustNewID()

	var workflowInput *core.Input
	if fixture.Input != nil {
		input := core.Input(fixture.Input)
		workflowInput = &input
	}

	temporalInput := worker.WorkflowInput{
		WorkflowID:     fixture.Workflow.ID,
		WorkflowExecID: workflowExecID,
		Input:          workflowInput,
		InitialTaskID:  FindInitialTaskID(fixture),
	}

	// Execute workflow through Temporal
	env.ExecuteWorkflow(worker.CompozyWorkflow, temporalInput)

	// Verify workflow completed successfully
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Retrieve final state from database
	finalState, err := workflowRepo.GetState(ctx, workflowExecID)
	require.NoError(t, err, "Failed to retrieve final workflow state")

	return finalState
}

// FindTasksByExecutionType finds all tasks of a specific execution type
func FindTasksByExecutionType(result *workflow.State, executionType task.ExecutionType) []*task.State {
	var tasks []*task.State
	for _, taskState := range result.Tasks {
		if taskState.ExecutionType == executionType {
			tasks = append(tasks, taskState)
		}
	}
	return tasks
}

// FindChildTasks finds all child tasks for a given parent task
func FindChildTasks(result *workflow.State, parentTaskExecID core.ID) []*task.State {
	var childTasks []*task.State
	for _, taskState := range result.Tasks {
		if taskState.ParentStateID != nil && *taskState.ParentStateID == parentTaskExecID {
			childTasks = append(childTasks, taskState)
		}
	}
	return childTasks
}

// FindParentTask finds the first task of a specific execution type (useful for collection, composite, etc.)
func FindParentTask(result *workflow.State, executionType task.ExecutionType) *task.State {
	for _, taskState := range result.Tasks {
		if taskState.ExecutionType == executionType {
			return taskState
		}
	}
	return nil
}

// VerifyTaskStatus verifies that a task has the expected status
func VerifyTaskStatus(t *testing.T, taskState *task.State, expectedStatus string, taskDescription string) {
	assert.Equal(
		t,
		expectedStatus,
		string(taskState.Status),
		"%s should have status %s",
		taskDescription,
		expectedStatus,
	)
}

// VerifyTaskHasOutput verifies that a task has output data
func VerifyTaskHasOutput(t *testing.T, taskState *task.State, taskDescription string) {
	require.NotNil(t, taskState.Output, "%s should have outputs", taskDescription)
}

// VerifyChildTaskCount verifies the expected number of child tasks
func VerifyChildTaskCount(
	t *testing.T,
	result *workflow.State,
	parentTaskExecID core.ID,
	expectedCount int,
	taskDescription string,
) {
	childTasks := FindChildTasks(result, parentTaskExecID)
	assert.Equal(t, expectedCount, len(childTasks), "%s should have %d child tasks", taskDescription, expectedCount)
}
