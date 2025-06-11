package utils

import (
	"fmt"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	wf "github.com/compozy/compozy/engine/workflow"
	testhelpers "github.com/compozy/compozy/test/helpers"
)

// -----
// Workflow Test Builder
// -----

// WorkflowTestBuilder provides a fluent interface for building test workflow configurations
type WorkflowTestBuilder struct {
	workflowID  string
	version     string
	description string
	tasks       []task.Config
	agents      []agent.Config
	tools       []tool.Config
	envVars     map[string]string
	t           *testing.T
}

// NewWorkflowTestBuilder creates a new workflow test builder
func NewWorkflowTestBuilder(t *testing.T, workflowID string) *WorkflowTestBuilder {
	return &WorkflowTestBuilder{
		workflowID:  workflowID,
		version:     "1.0.0",
		description: "Test workflow for worker testing",
		tasks:       []task.Config{},
		agents:      []agent.Config{},
		tools:       []tool.Config{},
		envVars:     map[string]string{"TEST_MODE": "true"},
		t:           t,
	}
}

// WithVersion sets the workflow version
func (b *WorkflowTestBuilder) WithVersion(version string) *WorkflowTestBuilder {
	b.version = version
	return b
}

// WithDescription sets the workflow description
func (b *WorkflowTestBuilder) WithDescription(description string) *WorkflowTestBuilder {
	b.description = description
	return b
}

// WithEnvVar adds an environment variable
func (b *WorkflowTestBuilder) WithEnvVar(key, value string) *WorkflowTestBuilder {
	b.envVars[key] = value
	return b
}

// -----
// Task Building Methods
// -----

// WithBasicTask adds a basic task to the workflow
func (b *WorkflowTestBuilder) WithBasicTask(taskID, actionID, actionPrompt string) *WorkflowTestBuilder {
	agentConfig := testhelpers.CreateTestAgentConfigWithAction(
		taskID+"-agent",
		"You are a test assistant. Process the given request.",
		actionID,
		actionPrompt,
	)

	taskConfig := task.Config{
		BaseConfig: task.BaseConfig{
			ID:    taskID,
			Type:  task.TaskTypeBasic,
			Agent: agentConfig,
			With: &core.Input{
				"message": "Test message for " + taskID,
			},
		},
		BasicTask: task.BasicTask{
			Action: actionID,
		},
	}

	b.tasks = append(b.tasks, taskConfig)
	b.agents = append(b.agents, *agentConfig)
	return b
}

// WithRouterTask adds a router task to the workflow
func (b *WorkflowTestBuilder) WithRouterTask(taskID, condition string, routes map[string]string) *WorkflowTestBuilder {
	taskConfig := task.Config{
		BaseConfig: task.BaseConfig{
			ID:   taskID,
			Type: task.TaskTypeRouter,
			With: &core.Input{
				"condition": condition,
			},
		},
		RouterTask: task.RouterTask{
			Condition: condition,
		},
	}

	// Add success transitions for each route
	if len(routes) > 0 {
		for _, nextTaskID := range routes {
			// In a real scenario, you'd set up proper routing logic
			// For testing, we'll use the first route as the default success transition
			if taskConfig.OnSuccess == nil {
				taskConfig.OnSuccess = &core.SuccessTransition{
					Next: &nextTaskID,
				}
				break
			}
		}
	}

	b.tasks = append(b.tasks, taskConfig)
	return b
}

// WithParallelTask adds a parallel task to the workflow
func (b *WorkflowTestBuilder) WithParallelTask(taskID, strategy string, subtaskIDs []string) *WorkflowTestBuilder {
	// Create subtasks
	for _, subtaskID := range subtaskIDs {
		subtask := task.Config{
			BaseConfig: task.BaseConfig{
				ID:   subtaskID,
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"data": "Test data for " + subtaskID,
				},
			},
		}
		b.tasks = append(b.tasks, subtask)
	}

	// Create parallel task configuration
	taskConfig := task.Config{
		BaseConfig: task.BaseConfig{
			ID:   taskID,
			Type: task.TaskTypeParallel,
			With: &core.Input{
				"strategy": strategy,
			},
		},
		// Note: ParallelTask config structure varies, using base config for now
	}

	b.tasks = append(b.tasks, taskConfig)
	return b
}

// WithCollectionTask adds a collection task to the workflow
func (b *WorkflowTestBuilder) WithCollectionTask(
	taskID string,
	items []string,
	filterExpr string,
) *WorkflowTestBuilder {
	taskConfig := task.Config{
		BaseConfig: task.BaseConfig{
			ID:   taskID,
			Type: task.TaskTypeCollection,
			With: &core.Input{
				"items":  items,
				"filter": filterExpr,
			},
		},
		// Note: Collection task config structure varies, using base config for now
	}

	b.tasks = append(b.tasks, taskConfig)
	return b
}

// WithTaskTransition adds a transition between tasks
func (b *WorkflowTestBuilder) WithTaskTransition(fromTaskID, toTaskID string, onError bool) *WorkflowTestBuilder {
	// Find the source task and add transition
	for i := range b.tasks {
		if b.tasks[i].ID == fromTaskID {
			if onError {
				b.tasks[i].OnError = &core.ErrorTransition{
					Next: &toTaskID,
				}
			} else {
				b.tasks[i].OnSuccess = &core.SuccessTransition{
					Next: &toTaskID,
				}
			}
			break
		}
	}
	return b
}

// -----
// Agent and Tool Building Methods
// -----

// WithAgent adds a custom agent to the workflow
func (b *WorkflowTestBuilder) WithAgent(agentConfig *agent.Config) *WorkflowTestBuilder {
	b.agents = append(b.agents, *agentConfig)
	return b
}

// WithTool adds a custom tool to the workflow
func (b *WorkflowTestBuilder) WithTool(toolConfig *tool.Config) *WorkflowTestBuilder {
	b.tools = append(b.tools, *toolConfig)
	return b
}

// -----
// Build Methods
// -----

// Build creates the final workflow configuration
func (b *WorkflowTestBuilder) Build() *wf.Config {
	// Build environment variables
	env := &core.EnvMap{}
	for k, v := range b.envVars {
		(*env)[k] = v
	}

	// Create workflow configuration
	workflowConfig := &wf.Config{
		ID:          b.workflowID,
		Version:     b.version,
		Description: b.description,
		Tasks:       b.tasks,
		Agents:      b.agents,
		Tools:       b.tools,
		Opts: wf.Opts{
			Env: env,
		},
	}

	return workflowConfig
}

// BuildWithHelper creates both workflow config and worker test helper
func (b *WorkflowTestBuilder) BuildWithHelper() (*wf.Config, *WorkerTestHelper) {
	workflowConfig := b.Build()

	// Create worker test config
	workerConfig := NewWorkerTestBuilder(b.t).Build(b.t)
	workerConfig.WorkflowConfig = workflowConfig

	// Create worker test helper
	helper := NewWorkerTestHelper(b.t, workerConfig)

	return workflowConfig, helper
}

// -----
// Convenience Methods for Common Patterns
// -----

// CreateBasicWorkflow creates a simple workflow with one basic task
func CreateBasicWorkflow(t *testing.T, workflowID, taskID string) *wf.Config {
	return NewWorkflowTestBuilder(t, workflowID).
		WithBasicTask(taskID, "test-action", "Process test message: {{.parent.input.message}}").
		Build()
}

// CreateMultiTaskWorkflow creates a workflow with multiple connected tasks
func CreateMultiTaskWorkflow(t *testing.T, workflowID string, taskCount int) *wf.Config {
	builder := NewWorkflowTestBuilder(t, workflowID)

	// Add tasks and chain them together
	for i := 1; i <= taskCount; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		actionID := fmt.Sprintf("action-%d", i)
		prompt := fmt.Sprintf("Process step %d: {{.parent.input.step}}", i)

		builder.WithBasicTask(taskID, actionID, prompt)

		// Add transition to next task (except for the last one)
		if i < taskCount {
			nextTaskID := fmt.Sprintf("task-%d", i+1)
			builder.WithTaskTransition(taskID, nextTaskID, false)
		}
	}

	return builder.Build()
}

// CreateParallelWorkflow creates a workflow with parallel task execution
func CreateParallelWorkflow(t *testing.T, workflowID string, strategy string, subtaskCount int) *wf.Config {
	builder := NewWorkflowTestBuilder(t, workflowID)

	// Create subtask IDs
	subtaskIDs := make([]string, 0, subtaskCount)
	for i := 1; i <= subtaskCount; i++ {
		subtaskIDs = append(subtaskIDs, fmt.Sprintf("subtask-%d", i))
	}

	return builder.
		WithParallelTask("parallel-task", strategy, subtaskIDs).
		Build()
}

// CreateCollectionWorkflow creates a workflow with collection task processing
func CreateCollectionWorkflow(t *testing.T, workflowID string, items []string, filter string) *wf.Config {
	return NewWorkflowTestBuilder(t, workflowID).
		WithCollectionTask("collection-task", items, filter).
		Build()
}
