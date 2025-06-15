package utils

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/worker"
	wf "github.com/compozy/compozy/engine/workflow"
	"go.temporal.io/sdk/testsuite"
)

// WithTestID sets a custom test ID
func (b *TestConfigBuilder) WithTestID(testID string) *TestConfigBuilder {
	b.testID = testID
	return b
}

// WithDescription sets a custom description
func (b *TestConfigBuilder) WithDescription(description string) *TestConfigBuilder {
	b.description = description
	return b
}

// WithEnvVar adds an environment variable
func (b *TestConfigBuilder) WithEnvVar(key, value string) *TestConfigBuilder {
	if b.envVars == nil {
		b.envVars = make(map[string]string)
	}
	b.envVars[key] = value
	return b
}

// WithProjectDir sets a custom project directory
func (b *TestConfigBuilder) WithProjectDir(dir string) *TestConfigBuilder {
	b.projectDir = dir
	return b
}

// WithSimpleTask adds a simple task with an agent
func (b *TestConfigBuilder) WithSimpleTask(taskID, actionID, actionPrompt string) *TestConfigBuilder {
	agentConfig := CreateTestAgentConfigWithAction(
		fmt.Sprintf("%s-agent", taskID),
		"You are a test assistant. Respond with the message provided.",
		actionID,
		actionPrompt,
	)

	taskConfig := task.Config{
		BaseConfig: task.BaseConfig{
			ID:    taskID,
			Type:  task.TaskTypeBasic,
			Agent: agentConfig,
			With: &core.Input{
				"message": "Hello, World!",
			},
		},
		BasicTask: task.BasicTask{
			Action: actionID,
		},
	}

	b.workflowTasks = append(b.workflowTasks, taskConfig)
	b.agents = append(b.agents, *agentConfig)
	return b
}

// WithCollectionTask adds a collection task with an agent
func (b *TestConfigBuilder) WithCollectionTask(
	taskID, actionID, actionPrompt, items, filter, itemVar, indexVar string,
	mode task.CollectionMode,
	batch int,
	strategy task.ParallelStrategy,
) *TestConfigBuilder {
	agentConfig := CreateTestAgentConfigWithAction(
		fmt.Sprintf("%s-agent", taskID),
		"You are a test assistant. Process the provided item and return a structured response.",
		actionID,
		actionPrompt,
	)

	collectionConfig := task.CollectionConfig{
		Items:    items,
		Filter:   filter,
		ItemVar:  itemVar,
		IndexVar: indexVar,
		Mode:     mode,
		Batch:    batch,
	}

	// Create task template for collection processing
	taskTemplate := &task.Config{
		BaseConfig: task.BaseConfig{
			ID:    "{{ .item_var }}-task",
			Type:  task.TaskTypeBasic,
			Agent: agentConfig,
		},
		BasicTask: task.BasicTask{
			Action: actionID,
		},
	}

	taskConfig := task.Config{
		BaseConfig: task.BaseConfig{
			ID:    taskID,
			Type:  task.TaskTypeCollection,
			Agent: agentConfig,
		},
		CollectionConfig: collectionConfig,
		ParallelTask: task.ParallelTask{
			Strategy: strategy,
			Task:     taskTemplate,
		},
	}

	b.workflowTasks = append(b.workflowTasks, taskConfig)
	b.agents = append(b.agents, *agentConfig)
	return b
}

// WithMultiStepTasks adds multiple tasks in sequence for pause/resume testing
func (b *TestConfigBuilder) WithMultiStepTasks(stepCount int) *TestConfigBuilder {
	actions := make(map[string]string)
	tasks := make([]task.Config, 0, stepCount)

	// Create actions for each step
	for i := 1; i <= stepCount; i++ {
		actionID := fmt.Sprintf("action-%d", i)
		actions[actionID] = fmt.Sprintf("Process step %d: {{.parent.input.step}}", i)
	}

	agentConfig := CreateTestAgentConfigWithActions(
		"test-agent",
		"You are a test assistant. Respond with the message provided.",
		actions,
	)

	// Create tasks with transitions
	for i := 1; i <= stepCount; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		actionID := fmt.Sprintf("action-%d", i)

		baseConfig := task.BaseConfig{
			ID:    taskID,
			Type:  task.TaskTypeBasic,
			Agent: agentConfig,
			With: &core.Input{
				"step": fmt.Sprintf("%d", i),
			},
		}

		// Add transition to next task (except for the last task)
		if i < stepCount {
			baseConfig.OnSuccess = &core.SuccessTransition{
				Next: StringPtr(fmt.Sprintf("task-%d", i+1)),
			}
		}

		taskConfig := task.Config{
			BaseConfig: baseConfig,
			BasicTask: task.BasicTask{
				Action: actionID,
			},
		}

		tasks = append(tasks, taskConfig)
	}

	b.workflowTasks = tasks
	b.agents = []agent.Config{*agentConfig}
	return b
}

// WithLongRunningTask adds a task with sleep for cancellation testing
func (b *TestConfigBuilder) WithLongRunningTask(duration string) *TestConfigBuilder {
	agentConfig := CreateTestAgentConfigWithAction(
		"slow-agent",
		"You are a slow test assistant. Take your time to process.",
		"long-action",
		"Process for duration: {{.parent.input.duration}}. Think deeply.",
	)

	taskConfig := task.Config{
		BaseConfig: task.BaseConfig{
			ID:    "long-task",
			Type:  task.TaskTypeBasic,
			Agent: agentConfig,
			With: &core.Input{
				"duration": duration,
			},
			Sleep: "2s", // Add sleep to simulate long-running task
		},
		BasicTask: task.BasicTask{
			Action: "long-action",
		},
	}

	b.workflowTasks = []task.Config{taskConfig}
	b.agents = []agent.Config{*agentConfig}
	return b
}

// WithToolTask adds a task that executes a tool
func (b *TestConfigBuilder) WithToolTask(taskID string, toolConfig *tool.Config, input *core.Input) *TestConfigBuilder {
	taskConfig := task.Config{
		BaseConfig: task.BaseConfig{
			ID:   taskID,
			Type: task.TaskTypeBasic,
			Tool: toolConfig,
			With: input,
		},
		BasicTask: task.BasicTask{
			// Tool tasks don't need an action
		},
	}

	b.workflowTasks = append(b.workflowTasks, taskConfig)
	b.tools = append(b.tools, *toolConfig)
	return b
}

// Build creates the final ContainerTestConfig
func (b *TestConfigBuilder) Build(t *testing.T) *ContainerTestConfig {
	// If no tasks were explicitly added, add a default simple task
	if len(b.workflowTasks) == 0 {
		b.WithSimpleTask("test-task", "test-action", "Process this message: {{.parent.input.message}}")
	}

	// Build environment variables
	envMap := make(core.EnvMap)
	for k, v := range b.envVars {
		envMap[k] = v
	}
	env := &envMap

	// Create workflow configuration
	workflowConfig := &wf.Config{
		ID:          fmt.Sprintf("%s-workflow", b.testID),
		Version:     "1.0.0",
		Description: b.description,
		Tasks:       b.workflowTasks,
		Agents:      b.agents,
		Tools:       b.tools,
		Opts: wf.Opts{
			Env: env,
		},
	}

	// Create project configuration
	projectConfig := &project.Config{
		Name:    "test-project",
		Version: "1.0.0",
	}
	if err := projectConfig.SetCWD(b.projectDir); err != nil {
		t.Fatalf("Failed to set project CWD: %v", err)
	}

	// Create repositories
	workflowRepo := store.NewWorkflowRepo(b.dbPool)
	taskRepo := store.NewTaskRepo(b.dbPool)

	return &ContainerTestConfig{
		WorkflowConfig:   workflowConfig,
		ProjectConfig:    projectConfig,
		WorkflowRepo:     workflowRepo,
		TaskRepo:         taskRepo,
		DB:               b.dbPool,
		ExpectedDuration: DefaultTestTimeout,
		testID:           b.testID,
	}
}

// -----
// Convenience Functions (Backwards Compatibility)
// -----

// CreateContainerTestConfig creates a test configuration using the builder (backwards compatible)
func CreateContainerTestConfig(t *testing.T) *ContainerTestConfig {
	return NewTestConfigBuilder(t).Build(t)
}

// CreateContainerTestConfigForMultiTask creates a container test configuration for multi-task workflows
func CreateContainerTestConfigForMultiTask(t *testing.T, workflowConfig *wf.Config) *ContainerTestConfig {
	config := NewTestConfigBuilder(t).Build(t)
	config.WorkflowConfig = workflowConfig
	return config
}

// CreateContainerTestConfigForCancellation creates a container test configuration for cancellation workflows
func CreateContainerTestConfigForCancellation(t *testing.T, workflowConfig *wf.Config) *ContainerTestConfig {
	config := NewTestConfigBuilder(t).Build(t)
	config.WorkflowConfig = workflowConfig
	return config
}

// CreatePauseableWorkflowConfig creates a workflow config with multiple tasks for pause/resume testing
func CreatePauseableWorkflowConfig(t *testing.T) *wf.Config {
	return NewTestConfigBuilder(t).
		WithTestID(GenerateUniqueTestID("pauseable")).
		WithMultiStepTasks(3).
		Build(t).WorkflowConfig
}

func CreateCancellableWorkflowConfig(t *testing.T) *wf.Config {
	return NewTestConfigBuilder(t).
		WithTestID(GenerateUniqueTestID("cancellable")).
		WithLongRunningTask("10s").
		Build(t).WorkflowConfig
}

// SetupWorkflowEnvironment sets up the Temporal workflow environment for testing
func SetupWorkflowEnvironment(t *testing.T, env *testsuite.TestWorkflowEnvironment, config *ContainerTestConfig) {
	// Configure test environment for deterministic testing
	ConfigureTemporalTestEnvironment(env)

	// Setup in-memory Redis cache for testing
	mr := miniredis.RunT(t)
	cacheConfig := &cache.Config{
		Host:     mr.Host(),
		Port:     mr.Port(),
		Password: "redis_secret",
		DB:       5, // Use a different DB for tests
	}

	ctx := context.Background()
	redisCache, err := cache.SetupCache(ctx, cacheConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to setup Redis cache for tests: %v", err))
	}

	// Ensure Redis cache is properly closed when test finishes
	t.Cleanup(func() {
		redisCache.Close()
	})

	configStore := services.NewRedisConfigStore(redisCache.Redis, 1*time.Hour)
	runtime, err := runtime.NewRuntimeManager(config.ProjectConfig.GetCWD().PathStr(), runtime.WithTestConfig())
	if err != nil {
		panic(err)
	}
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*wf.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
		runtime,
		configStore,
		services.NewMockSignalDispatcher(),
	)

	// Register workflows
	env.RegisterWorkflow(worker.CompozyWorkflow)

	// Register all worker activities for comprehensive testing
	RegisterWorkerActivities(env, activities)
}

// ConfigureTemporalTestEnvironment sets up deterministic testing configuration
func ConfigureTemporalTestEnvironment(env *testsuite.TestWorkflowEnvironment) {
	// Set default test timeout for deterministic testing
	env.SetTestTimeout(DefaultTestTimeout)

	// Configure test environment for deterministic behavior
	// TestWorkflowEnvironment is already deterministic by design
	// - Uses mock clock for time-based operations
	// - Provides controlled execution order
	// - Enables replay testing capabilities
}

// RegisterWorkerActivities registers all worker activities for testing
func RegisterWorkerActivities(env *testsuite.TestWorkflowEnvironment, activities *worker.Activities) {
	// Core workflow activities
	env.RegisterActivity(activities.GetWorkflowData)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.CompleteWorkflow)

	// Task execution activities
	env.RegisterActivity(activities.ExecuteBasicTask)
	env.RegisterActivity(activities.ExecuteRouterTask)
	env.RegisterActivity(activities.ExecuteSubtask)
	env.RegisterActivity(activities.CreateParallelState)
	env.RegisterActivity(activities.GetParallelResponse)
	env.RegisterActivity(activities.CreateCollectionState)
	env.RegisterActivity(activities.GetCollectionResponse)
	env.RegisterActivity(activities.GetProgress)
	env.RegisterActivity(activities.UpdateParentStatus)
	env.RegisterActivity(activities.ListChildStates)
}

// -----
// Advanced Temporal Test Configuration
// -----

// SetupTemporalTestEnvironmentWithTimeout creates a configured test environment with custom timeout
func SetupTemporalTestEnvironmentWithTimeout(timeout time.Duration) *testsuite.TestWorkflowEnvironment {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.SetTestTimeout(timeout)
	return env
}

// SetupDeterministicTestEnvironment creates a test environment optimized for deterministic testing
func SetupDeterministicTestEnvironment(t *testing.T, config *ContainerTestConfig) *testsuite.TestWorkflowEnvironment {
	env := SetupTemporalTestEnvironmentWithTimeout(DefaultTestTimeout)
	SetupWorkflowEnvironment(t, env, config)
	return env
}
