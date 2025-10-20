package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	coreruntime "github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/toolenv/builder"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
)

// CreateMockRuntime creates a mock runtime manager for integration tests
func CreateMockRuntime(t *testing.T) coreruntime.Runtime {
	ctx := t.Context()
	config := coreruntime.TestConfig()
	factory := coreruntime.NewDefaultFactory("/tmp")
	rtManager, err := factory.CreateRuntime(ctx, config)
	require.NoError(t, err, "failed to create mock runtime manager")
	return rtManager
}

// NoopUsageMetrics implements usage.Metrics for tests.
type NoopUsageMetrics struct{}

var _ usage.Metrics = (*NoopUsageMetrics)(nil)

func (NoopUsageMetrics) RecordSuccess(
	context.Context,
	core.ComponentType,
	string,
	string,
	int,
	int,
	time.Duration,
) {
}

func (NoopUsageMetrics) RecordFailure(
	context.Context,
	core.ComponentType,
	string,
	string,
	time.Duration,
) {
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

// CreateTestConfigManager removed - ConfigManager has been replaced by task2.Factory

// FindInitialTaskID finds the initial task ID from fixture
func FindInitialTaskID(fixture *TestFixture) string {
	if len(fixture.Workflow.Tasks) > 0 {
		return fixture.Workflow.Tasks[0].ID
	}
	return ""
}

// names expected by the worker (e.g., the task-config and update-child-state labels).
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
	env.RegisterActivityWithOptions(
		activities.UpdateChildState,
		activity.RegisterOptions{Name: tkacts.UpdateChildStateLabel},
	)
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
	env.RegisterActivityWithOptions(
		activities.LoadCollectionConfigsActivity,
		activity.RegisterOptions{Name: tkacts.LoadCollectionConfigsLabel},
	)
}

// CreateBasicAgentConfig creates a basic agent configuration for testing.
// It composes the base agent with a reusable set of action templates.
func CreateBasicAgentConfig() *agent.Config {
	return &agent.Config{
		ID:           "test-agent",
		Model:        agent.Model{Config: core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"}},
		Instructions: "Test agent for integration testing",
		Actions:      basicAgentActions(),
	}
}

// basicAgentActions returns the action suite used by basic agent fixtures.
// Keeping it separate ensures the constructor remains concise.
func basicAgentActions() []*agent.ActionConfig {
	return []*agent.ActionConfig{
		createProcessMessageAction(),
		createProcessWithErrorAction(),
		createPrepareDataAction(),
		createProcessDataAction(),
		createHandleErrorAction(),
	}
}

// createProcessMessageAction defines the simple message processing action for tests.
// It includes optional message templating to exercise conditional rendering.
func createProcessMessageAction() *agent.ActionConfig {
	return &agent.ActionConfig{
		ID:     "process_message",
		Prompt: "Process a message for testing. {{ if .input.message }}Message: {{ .input.message }}{{ end }}",
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"value":   map[string]any{"type": "number"},
			},
		},
	}
}

// createProcessWithErrorAction defines an action path that can intentionally fail.
// It lets integration tests verify error propagation paths.
func createProcessWithErrorAction() *agent.ActionConfig {
	return &agent.ActionConfig{
		ID:     "process_with_error",
		Prompt: "Process with error for testing",
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"message":     map[string]any{"type": "string"},
				"should_fail": map[string]any{"type": "boolean"},
			},
		},
	}
}

// createPrepareDataAction describes the preprocessing action used by the basic agent.
// It primes numeric values before the process_data step executes.
func createPrepareDataAction() *agent.ActionConfig {
	return &agent.ActionConfig{
		ID:     "prepare_data",
		Prompt: "Prepare data for testing",
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"initial_value": map[string]any{"type": "number"},
			},
		},
	}
}

// createProcessDataAction defines the primary processing step with multiplier support.
// It drives calculations that downstream assertions validate.
func createProcessDataAction() *agent.ActionConfig {
	return &agent.ActionConfig{
		ID:     "process_data",
		Prompt: "Process data for testing",
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"multiplier": map[string]any{"type": "number"},
			},
		},
	}
}

// createHandleErrorAction registers the fallback action for recovery scenarios.
// It ensures tests can exercise error-handling logic in workflows.
func createHandleErrorAction() *agent.ActionConfig {
	return &agent.ActionConfig{
		ID:     "handle_error",
		Prompt: "Handle error for testing",
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"recovery_message": map[string]any{"type": "string"},
			},
		},
	}
}

// CreateCollectionAgentConfig creates a collection-specific agent configuration for testing
func CreateCollectionAgentConfig() *agent.Config {
	return &agent.Config{
		ID:           "test-collection-agent",
		Model:        agent.Model{Config: core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"}},
		Instructions: "Test agent for collection workflow integration testing",
		Actions:      createCollectionAgentActions(),
	}
}

// createCollectionAgentActions creates the action configurations for collection agent
func createCollectionAgentActions() []*agent.ActionConfig {
	return []*agent.ActionConfig{
		createProcessItemAction(),
		createProcessParallelItemAction(),
		createProcessWithFailureAction(),
		createAnalyzeActivityAction(),
		createProcessCityAction(),
		createAggregateResultsAction(),
		createHandleEmptyCollectionAction(),
	}
}

func createProcessItemAction() *agent.ActionConfig {
	return &agent.ActionConfig{
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
		OutputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"result":          map[string]any{"type": "string"},
				"processed_value": map[string]any{"type": "number"},
			},
			"required": []string{"result", "processed_value"},
		},
	}
}

func createProcessParallelItemAction() *agent.ActionConfig {
	return &agent.ActionConfig{
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
	}
}

func createProcessWithFailureAction() *agent.ActionConfig {
	return &agent.ActionConfig{
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
	}
}

func createAnalyzeActivityAction() *agent.ActionConfig {
	return &agent.ActionConfig{
		ID:     "analyze_activity",
		Prompt: "Analyze a single activity",
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"activity_name": map[string]any{"type": "string"},
			},
		},
		OutputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"analysis": map[string]any{"type": "string"},
				"rating":   map[string]any{"type": "number"},
			},
			"required": []string{"analysis", "rating"},
		},
	}
}

func createProcessCityAction() *agent.ActionConfig {
	return &agent.ActionConfig{
		ID:     "process_city",
		Prompt: "Process city data",
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"city_name":     map[string]any{"type": "string"},
				"city_position": map[string]any{"type": "number"},
			},
		},
		OutputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"weather":    map[string]any{"type": "string"},
				"population": map[string]any{"type": "number"},
			},
			"required": []string{"weather", "population"},
		},
	}
}

func createAggregateResultsAction() *agent.ActionConfig {
	return &agent.ActionConfig{
		ID:     "aggregate_results",
		Prompt: "Aggregate collection results",
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"child_results": map[string]any{"type": "array"},
				"total_items":   map[string]any{"type": "number"},
			},
		},
	}
}

func createHandleEmptyCollectionAction() *agent.ActionConfig {
	return &agent.ActionConfig{
		ID:     "handle_empty_collection",
		Prompt: "Handle empty collection case",
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"collection_size": map[string]any{"type": "number"},
			},
		},
	}
}

// CreateParallelAgentConfig creates a parallel-specific agent configuration for testing.
// It provides reusable action templates for parallel workflow fixtures.
func CreateParallelAgentConfig() *agent.Config {
	return &agent.Config{
		ID:           "test-parallel-agent",
		Model:        agent.Model{Config: core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"}},
		Instructions: "Test agent for parallel workflow integration testing",
		Actions:      parallelAgentActions(),
	}
}

// parallelAgentActions returns the action list exercised by parallel workflows.
// Splitting this logic keeps the constructor lean and readable.
func parallelAgentActions() []*agent.ActionConfig {
	return []*agent.ActionConfig{
		createProcessParallelTaskAction(),
		createSynchronizeResultsAction(),
		createHandleParallelFailureAction(),
		createAggregateParallelOutputsAction(),
		createExecuteConcurrentTaskAction(),
	}
}

// createProcessParallelTaskAction defines the base parallel task execution action.
// It includes duration and value fields used across integration tests.
func createProcessParallelTaskAction() *agent.ActionConfig {
	return &agent.ActionConfig{
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
	}
}

// createSynchronizeResultsAction captures the fan-in synchronization step.
// It allows tests to assert aggregation of child outputs.
func createSynchronizeResultsAction() *agent.ActionConfig {
	return &agent.ActionConfig{
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
	}
}

// createHandleParallelFailureAction models failure handling in parallel flows.
// It includes fields for error type control in tests.
func createHandleParallelFailureAction() *agent.ActionConfig {
	return &agent.ActionConfig{
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
	}
}

// createAggregateParallelOutputsAction defines the aggregation action for parallel workflows.
// It validates that result collation handles success/failure counts and timings.
func createAggregateParallelOutputsAction() *agent.ActionConfig {
	return &agent.ActionConfig{
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
	}
}

// createExecuteConcurrentTaskAction defines per-task execution metadata for concurrent mode.
// It ensures workflows can emit detailed telemetry for each parallel run.
func createExecuteConcurrentTaskAction() *agent.ActionConfig {
	return &agent.ActionConfig{
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
	}
}

// CreateTestActivities returns a *worker.Activities preconfigured for tests.
// It builds a project config and workflow configs from the provided fixture and agentConfig,
// initializes a JSON template engine, and constructs an Activities instance wired to the given
// repos, runtime, and config store.
func CreateTestActivities(
	t *testing.T,
	taskRepo task.Repository,
	workflowRepo workflow.Repository,
	fixture *TestFixture,
	runtime coreruntime.Runtime,
	configStore *services.TestConfigStore,
	projectName string,
	agentConfig *agent.Config,
) *worker.Activities {
	projectConfig := CreateTestProjectConfig(fixture, projectName)
	workflows := createTestWorkflowConfigs(fixture, agentConfig)

	// Create template engine for tests
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)

	// Create memory manager for tests - use nil for now as it's not needed for most tests
	var memoryManager *memory.Manager

	ctx := t.Context()
	mgr := config.NewManager(ctx, config.NewService())
	_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	require.NoError(t, err)
	log := logger.NewForTests()
	ctx = logger.ContextWithLogger(config.ContextWithManager(ctx, mgr), log)
	store := resources.NewMemoryResourceStore()
	require.NoError(t, projectConfig.IndexToResourceStore(ctx, store))
	for _, wfCfg := range workflows {
		require.NoError(t, wfCfg.IndexToResourceStore(ctx, projectConfig.Name, store))
	}
	toolEnv, err := builder.Build(projectConfig, workflows, workflowRepo, taskRepo, store)
	require.NoError(t, err)

	acts, err := worker.NewActivities(
		ctx,
		projectConfig,
		workflows,
		workflowRepo,
		taskRepo,
		&NoopUsageMetrics{},
		providermetrics.Nop(),
		runtime,
		configStore,
		nil, // signalDispatcher - not needed for test
		nil, // redisCache - not needed for test
		memoryManager,
		templateEngine,
		toolEnv,
	)
	require.NoError(t, err)
	return acts
}

// applyAgentToTask sets the agent configuration for taskConfig, clears any tool reference,
// and assigns the provided working directory. It mutates the supplied taskConfig in-place.
func applyAgentToTask(taskConfig *task.Config, agentConfig *agent.Config, cwd *core.PathCWD) {
	taskConfig.Agent = agentConfig
	taskConfig.Tool = nil
	taskConfig.CWD = cwd
}

// applyAgentToBasicTasks applies agentConfig and working directory to each basic task in the provided slice.
// It mutates the task.Config elements in-place; tasks whose Type is not TaskTypeBasic are left unchanged.
func applyAgentToBasicTasks(tasks []task.Config, agentConfig *agent.Config, cwd *core.PathCWD) {
	for i := range tasks {
		if tasks[i].Type == task.TaskTypeBasic {
			applyAgentToTask(&tasks[i], agentConfig, cwd)
		}
	}
}

// selection, and setting the task's CWD.
func configureBasicTask(taskConfig *task.Config, agentConfig *agent.Config, cwd *core.PathCWD) {
	applyAgentToTask(taskConfig, agentConfig, cwd)
}

// configureCollectionTask configures a collection task and its child template by applying
// the provided agent configuration and working directory.
//
// It mutates taskConfig in-place: assigns the agent and cwd to the collection task, and if a
// child task template exists it clones that template
// (to avoid mutating shared fixture state), applies the agent to the clone, and attaches it back
// to taskConfig. If the cloned child is a composite
// task, the agent is propagated to all nested basic child tasks.
func configureCollectionTask(taskConfig *task.Config, agentConfig *agent.Config, cwd *core.PathCWD) {
	applyAgentToTask(taskConfig, agentConfig, cwd)

	// Apply to child task template if it exists
	if taskConfig.Task != nil {
		// Clone the child config to avoid mutating shared fixture state
		child := *taskConfig.Task
		// If the child has its own tasks, clone the slice header to decouple
		if len(child.Tasks) > 0 {
			child.Tasks = append([]task.Config(nil), child.Tasks...)
		}
		taskConfig.Task = &child
		applyAgentToTask(taskConfig.Task, agentConfig, cwd)

		// If child is composite, apply to its nested tasks
		if taskConfig.Task.Type == task.TaskTypeComposite {
			applyAgentToBasicTasks(taskConfig.Task.Tasks, agentConfig, cwd)
		}
	}
}

// configureParallelTask copies the task's child slice to decouple shared state and applies
// the provided agent configuration and working directory to any basic child tasks.
func configureParallelTask(taskConfig *task.Config, agentConfig *agent.Config, cwd *core.PathCWD) {
	if len(taskConfig.Tasks) > 0 {
		cloned := append([]task.Config(nil), taskConfig.Tasks...)
		taskConfig.Tasks = cloned
	}
	applyAgentToBasicTasks(taskConfig.Tasks, agentConfig, cwd)
}

// configureCompositeTask configures a composite task and its child tasks.
//
// configureCompositeTask applies the provided agent configuration and working
// directory to the composite task itself, clones the composite's child task
// slice (to avoid shared/mutated state), and propagates the agent and CWD to
// any basic child tasks contained directly in the composite.
func configureCompositeTask(taskConfig *task.Config, agentConfig *agent.Config, cwd *core.PathCWD) {
	applyAgentToTask(taskConfig, agentConfig, cwd)
	if len(taskConfig.Tasks) > 0 {
		cloned := append([]task.Config(nil), taskConfig.Tasks...)
		taskConfig.Tasks = cloned
	}
	applyAgentToBasicTasks(taskConfig.Tasks, agentConfig, cwd)
}

// createTestWorkflowConfigs builds workflow configurations from the test fixture and applies
// the provided agent configuration to every task (including nested child tasks) according to
// each task's type. It precomputes a working directory (CWD) for tasks with a fallback to the
// current directory and then returns a slice containing a single configured *workflow.Config.
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

		// Apply agent configuration based on task type
		switch t.Type {
		case task.TaskTypeBasic:
			configureBasicTask(&tasks[i], agentConfig, cwd)
		case task.TaskTypeCollection:
			configureCollectionTask(&tasks[i], agentConfig, cwd)
		case task.TaskTypeParallel:
			configureParallelTask(&tasks[i], agentConfig, cwd)
		case task.TaskTypeComposite:
			configureCompositeTask(&tasks[i], agentConfig, cwd)
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
	_ *DatabaseHelper,
	projectName string,
	agentConfig *agent.Config,
) *workflow.State {
	ctx := t.Context()
	taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
	defer cleanup()

	// Create test suite and worker
	testSuite := testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Create repositories and runtime
	configStore := services.NewTestConfigStore(t)
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
