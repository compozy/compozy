package utils

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/worker"
	testhelpers "github.com/compozy/compozy/test/helpers"
	"go.temporal.io/sdk/testsuite"
)

// -----
// Worker Test Configuration
// -----

// WorkerTestConfig extends ContainerTestConfig with worker-specific options
type WorkerTestConfig struct {
	*testhelpers.ContainerTestConfig
	Worker          *worker.Worker
	TemporalTestEnv *testsuite.TestWorkflowEnvironment
	TaskQueue       string
	MaxWorkers      int
	WorkerTimeout   time.Duration
	ActivityTimeout time.Duration
	WorkflowTimeout time.Duration
	RuntimeManager  *runtime.Manager
	TestSuite       *testsuite.WorkflowTestSuite
}

// Cleanup extends the base cleanup with worker-specific cleanup
func (w *WorkerTestConfig) Cleanup(t *testing.T) {
	t.Cleanup(func() {
		if w.Worker != nil {
			ctx, cancel := context.WithTimeout(context.Background(), w.WorkerTimeout)
			defer cancel()
			w.Worker.Stop(ctx)
		}
		if w.TemporalTestEnv != nil {
			w.TemporalTestEnv.AssertExpectations(t)
		}
		// Call base cleanup
		w.ContainerTestConfig.Cleanup(t)
	})
}

// -----
// Worker Test Builder
// -----

// WorkerTestBuilder provides a fluent interface for building worker test configurations
type WorkerTestBuilder struct {
	*testhelpers.TestConfigBuilder
	taskQueue       string
	maxWorkers      int
	workerTimeout   time.Duration
	activityTimeout time.Duration
	workflowTimeout time.Duration
	executionTypes  []task.ExecutionType
}

// NewWorkerTestBuilder creates a new worker test configuration builder
func NewWorkerTestBuilder(t *testing.T) *WorkerTestBuilder {
	baseBuilder := testhelpers.NewTestConfigBuilder(t)
	return &WorkerTestBuilder{
		TestConfigBuilder: baseBuilder,
		taskQueue:         "test-worker-queue",
		maxWorkers:        1,
		workerTimeout:     30 * time.Second,
		activityTimeout:   10 * time.Second,
		workflowTimeout:   60 * time.Second,
		executionTypes:    []task.ExecutionType{},
	}
}

// WithTaskQueue sets a custom task queue name
func (b *WorkerTestBuilder) WithTaskQueue(taskQueue string) *WorkerTestBuilder {
	b.taskQueue = taskQueue
	return b
}

// WithMaxWorkers sets the maximum number of workers
func (b *WorkerTestBuilder) WithMaxWorkers(maxWorkers int) *WorkerTestBuilder {
	b.maxWorkers = maxWorkers
	return b
}

// WithWorkerTimeout sets the worker timeout
func (b *WorkerTestBuilder) WithWorkerTimeout(timeout time.Duration) *WorkerTestBuilder {
	b.workerTimeout = timeout
	return b
}

// WithActivityTimeout sets the activity timeout
func (b *WorkerTestBuilder) WithActivityTimeout(timeout time.Duration) *WorkerTestBuilder {
	b.activityTimeout = timeout
	return b
}

// WithWorkflowTimeout sets the workflow timeout
func (b *WorkerTestBuilder) WithWorkflowTimeout(timeout time.Duration) *WorkerTestBuilder {
	b.workflowTimeout = timeout
	return b
}

// WithBasicTask adds a basic task execution test scenario
func (b *WorkerTestBuilder) WithBasicTask(_, _, _ string) *WorkerTestBuilder {
	// Just record the execution type - actual task configuration is handled by TestConfigBuilder
	b.executionTypes = append(b.executionTypes, task.ExecutionBasic)
	return b
}

// WithRouterTask adds a router task execution test scenario
func (b *WorkerTestBuilder) WithRouterTask(_ string, _ map[string]string) *WorkerTestBuilder {
	// Add router-specific task configuration
	b.executionTypes = append(b.executionTypes, task.ExecutionRouter)
	return b
}

// WithParallelTask adds a parallel task execution test scenario
func (b *WorkerTestBuilder) WithParallelTask(_ string, _ string, _ []string) *WorkerTestBuilder {
	// Add parallel-specific task configuration
	b.executionTypes = append(b.executionTypes, task.ExecutionParallel)
	return b
}

// WithCollectionTask adds a collection task execution test scenario
func (b *WorkerTestBuilder) WithCollectionTask(_ string, _ []string, _ string) *WorkerTestBuilder {
	// Add collection-specific task configuration
	b.executionTypes = append(b.executionTypes, task.ExecutionCollection)
	return b
}

// Build creates the final WorkerTestConfig
func (b *WorkerTestBuilder) Build(t *testing.T) *WorkerTestConfig {
	// Build base configuration
	baseConfig := b.TestConfigBuilder.Build(t)

	// Create test suite
	testSuite := &testsuite.WorkflowTestSuite{}

	// Create temporal test environment with deterministic configuration
	testEnv := testSuite.NewTestWorkflowEnvironment()

	// Configure test environment timeouts and deterministic settings
	b.configureTestEnvironment(testEnv)

	// Create runtime manager
	runtimeManager, err := runtime.NewRuntimeManager(
		baseConfig.ProjectConfig.GetCWD().PathStr(),
		runtime.WithTestConfig(),
	)
	if err != nil {
		t.Fatalf("Failed to create runtime manager: %v", err)
	}

	return &WorkerTestConfig{
		ContainerTestConfig: baseConfig,
		TemporalTestEnv:     testEnv,
		TaskQueue:           b.taskQueue,
		MaxWorkers:          b.maxWorkers,
		WorkerTimeout:       b.workerTimeout,
		ActivityTimeout:     b.activityTimeout,
		WorkflowTimeout:     b.workflowTimeout,
		RuntimeManager:      runtimeManager,
		TestSuite:           testSuite,
	}
}

// configureTestEnvironment sets up deterministic testing configuration
func (b *WorkerTestBuilder) configureTestEnvironment(env *testsuite.TestWorkflowEnvironment) {
	// Set workflow timeout for deterministic testing
	env.SetTestTimeout(b.workflowTimeout)

	// Configure for deterministic execution
	// TestWorkflowEnvironment automatically provides:
	// - Deterministic time through mock clock
	// - Controlled workflow execution order
	// - Replay testing capabilities
	// - Predictable random number generation
}

// -----
// Convenience Functions
// -----

// CreateWorkerTestConfig creates a basic worker test configuration
func CreateWorkerTestConfig(t *testing.T) *WorkerTestConfig {
	return NewWorkerTestBuilder(t).
		WithBasicTask("test-task", "test-action", "Process this message: {{.parent.input.message}}").
		Build(t)
}

// CreateWorkerTestConfigWithExecutionType creates a worker test config for specific execution type
func CreateWorkerTestConfigWithExecutionType(t *testing.T, execType task.ExecutionType) *WorkerTestConfig {
	builder := NewWorkerTestBuilder(t)

	switch execType {
	case task.ExecutionBasic:
		builder.WithBasicTask("basic-task", "basic-action", "Process basic task: {{.parent.input.data}}")
	case task.ExecutionRouter:
		builder.WithRouterTask("router-task", map[string]string{
			"route1": "Process route 1",
			"route2": "Process route 2",
		})
	case task.ExecutionParallel:
		builder.WithParallelTask("parallel-task", "wait_all", []string{"sub-task-1", "sub-task-2"})
	case task.ExecutionCollection:
		builder.WithCollectionTask("collection-task", []string{"item1", "item2", "item3"}, "")
	}

	return builder.Build(t)
}
