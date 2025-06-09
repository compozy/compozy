package normalizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

func TestContextBuilder_BuildCollectionContext(t *testing.T) {
	contextBuilder := NewContextBuilder()

	tests := []struct {
		name          string
		workflowState *workflow.State
		taskConfig    *task.Config
		expected      map[string]any
		notContains   []string
	}{
		{
			name: "Should build context with workflow input and output",
			workflowState: &workflow.State{
				Input: &core.Input{
					"param1": "value1",
					"param2": 42,
				},
				Output: &core.Output{
					"result": "success",
					"count":  3,
				},
			},
			taskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID: "test-task",
				},
			},
			expected: map[string]any{
				"input": core.Input{
					"param1": "value1",
					"param2": 42,
				},
				"output": core.Output{
					"result": "success",
					"count":  3,
				},
			},
		},
		{
			name:          "Should build context with task 'with' parameters",
			workflowState: &workflow.State{},
			taskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID: "test-task",
					With: &core.Input{
						"customParam": "customValue",
						"iterations":  5,
					},
				},
			},
			expected: map[string]any{
				"customParam": "customValue",
				"iterations":  5,
			},
		},
		{
			name: "Should handle nil workflow input/output",
			workflowState: &workflow.State{
				Input:  nil,
				Output: nil,
			},
			taskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID: "test-task",
				},
			},
			expected:    map[string]any{},
			notContains: []string{"input", "output"},
		},
		{
			name: "Should merge workflow state and task config",
			workflowState: &workflow.State{
				Input: &core.Input{
					"globalParam": "globalValue",
				},
				Output: &core.Output{
					"globalResult": "done",
				},
			},
			taskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID: "test-task",
					With: &core.Input{
						"taskParam": "taskValue",
						"override":  "fromTask",
					},
				},
			},
			expected: map[string]any{
				"input": core.Input{
					"globalParam": "globalValue",
				},
				"output": core.Output{
					"globalResult": "done",
				},
				"taskParam": "taskValue",
				"override":  "fromTask",
			},
		},
		{
			name: "Should handle empty task config",
			workflowState: &workflow.State{
				Input: &core.Input{
					"param": "value",
				},
			},
			taskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-task",
					With: nil,
				},
			},
			expected: map[string]any{
				"input": core.Input{
					"param": "value",
				},
			},
			notContains: []string{"output"},
		},
		{
			name: "Should include tasks context with task states and outputs",
			workflowState: &workflow.State{
				WorkflowID: "workflow1",
				Input:      &core.Input{"city": "New York"},
				Tasks: map[string]*task.State{
					"weather": {
						TaskID: "weather",
						Status: core.StatusSuccess,
						Output: &core.Output{
							"temperature": 25,
							"humidity":    60,
							"weather":     "sunny",
						},
					},
					"activities": {
						TaskID: "activities",
						Status: core.StatusSuccess,
						Output: &core.Output{
							"activities": []string{"walking", "sightseeing", "outdoor dining"},
						},
					},
				},
			},
			taskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID: "activity_analysis",
				},
			},
			expected: map[string]any{
				"workflow": map[string]any{
					"id":     "workflow1",
					"input":  &core.Input{"city": "New York"},
					"output": (*core.Output)(nil),
				},
				"tasks": map[string]any{
					"weather": map[string]any{
						"id":    "weather",
						"input": (*core.Input)(nil),
						"output": core.Output{
							"temperature": 25,
							"humidity":    60,
							"weather":     "sunny",
						},
					},
					"activities": map[string]any{
						"id":    "activities",
						"input": (*core.Input)(nil),
						"output": core.Output{
							"activities": []string{"walking", "sightseeing", "outdoor dining"},
						},
					},
				},
				"input": core.Input{"city": "New York"},
			},
			notContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contextBuilder.BuildCollectionContext(tt.workflowState, tt.taskConfig)

			require.NotNil(t, result)

			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, result[key])
			}

			for _, key := range tt.notContains {
				assert.NotContains(t, result, key)
			}
		})
	}
}

func TestContextBuilder_BuildCollectionContext_EdgeCases(t *testing.T) {
	contextBuilder := NewContextBuilder()

	t.Run("Should handle nil workflow state gracefully", func(t *testing.T) {
		result := contextBuilder.BuildCollectionContext(nil, &task.Config{})
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("Should handle nil task config gracefully", func(t *testing.T) {
		result := contextBuilder.BuildCollectionContext(&workflow.State{}, nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("Should handle both nil parameters gracefully", func(t *testing.T) {
		result := contextBuilder.BuildCollectionContext(nil, nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})
}

func TestContextBuilder_BuildTaskOutput(t *testing.T) {
	contextBuilder := NewContextBuilder()

	t.Run("Should handle basic task output", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{},
		}

		taskState := &task.State{
			TaskID:        "basic-task",
			ExecutionType: task.ExecutionBasic,
			Output: &core.Output{
				"result": "success",
				"count":  42,
			},
		}

		result := contextBuilder.buildTaskOutput(taskState, ctx)

		expected := core.Output{
			"result": "success",
			"count":  42,
		}
		assert.Equal(t, expected, result)
	})

	t.Run("Should handle nil output for basic task", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{},
		}

		taskState := &task.State{
			TaskID:        "basic-task",
			ExecutionType: task.ExecutionBasic,
			Output:        nil,
		}

		result := contextBuilder.buildTaskOutput(taskState, ctx)
		assert.Nil(t, result)
	})

	t.Run("Should handle parallel execution task with nested outputs", func(t *testing.T) {
		parentExecID := core.ID("parent-exec-123")
		childTaskExecID := core.ID("child-exec-456")

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				Tasks: map[string]*task.State{
					"child-task": {
						TaskID:        "child-task",
						TaskExecID:    childTaskExecID,
						ParentStateID: &parentExecID,
						ExecutionType: task.ExecutionBasic,
						Output: &core.Output{
							"childResult": "child-success",
						},
					},
				},
			},
			ChildrenIndex: map[string][]string{
				string(parentExecID): {"child-task"},
			},
		}

		parentTaskState := &task.State{
			TaskID:        "parent-task",
			TaskExecID:    parentExecID,
			ExecutionType: task.ExecutionParallel,
			Output: &core.Output{
				"parentResult": "parent-success",
			},
		}

		result := contextBuilder.buildTaskOutput(parentTaskState, ctx)

		resultMap := result.(map[string]any)
		assert.Equal(t, core.Output{"parentResult": "parent-success"}, resultMap["output"])

		childOutput := resultMap["child-task"].(map[string]any)
		assert.Equal(t, core.Output{"childResult": "child-success"}, childOutput["output"])
	})

	t.Run("Should handle parallel execution task without children", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{},
			ChildrenIndex: make(map[string][]string),
		}

		taskState := &task.State{
			TaskID:        "parallel-task",
			TaskExecID:    "exec-123",
			ExecutionType: task.ExecutionParallel,
			Output: &core.Output{
				"result": "parallel-success",
			},
		}

		result := contextBuilder.buildTaskOutput(taskState, ctx)

		resultMap := result.(map[string]any)
		assert.Equal(t, core.Output{"result": "parallel-success"}, resultMap["output"])
		assert.Len(t, resultMap, 1) // Only the parent output, no children
	})
}

func TestContextBuilder_BuildChildrenIndex(t *testing.T) {
	contextBuilder := NewContextBuilder()

	t.Run("Should build children index correctly", func(t *testing.T) {
		parentExecID := core.ID("parent-123")
		child1ExecID := core.ID("child1-456")
		child2ExecID := core.ID("child2-789")

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				Tasks: map[string]*task.State{
					"parent": {
						TaskExecID: parentExecID,
					},
					"child1": {
						TaskExecID:    child1ExecID,
						ParentStateID: &parentExecID,
					},
					"child2": {
						TaskExecID:    child2ExecID,
						ParentStateID: &parentExecID,
					},
					"standalone": {
						TaskExecID: "standalone-999",
					},
				},
			},
		}

		contextBuilder.buildChildrenIndex(ctx)

		assert.Len(t, ctx.ChildrenIndex, 1)
		assert.Contains(t, ctx.ChildrenIndex, string(parentExecID))
		assert.ElementsMatch(t, []string{"child1", "child2"}, ctx.ChildrenIndex[string(parentExecID)])
	})

	t.Run("Should handle empty workflow state", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: nil,
		}

		contextBuilder.buildChildrenIndex(ctx)

		assert.NotNil(t, ctx.ChildrenIndex)
		assert.Empty(t, ctx.ChildrenIndex)
	})

	t.Run("Should handle workflow state with no tasks", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				Tasks: nil,
			},
		}

		contextBuilder.buildChildrenIndex(ctx)

		assert.NotNil(t, ctx.ChildrenIndex)
		assert.Empty(t, ctx.ChildrenIndex)
	})
}

func TestContextBuilder_MergeTaskConfig(t *testing.T) {
	contextBuilder := NewContextBuilder()

	t.Run("Should merge task config into context", func(t *testing.T) {
		taskContext := map[string]any{
			"id": "test-task",
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "process",
			},
		}

		contextBuilder.mergeTaskConfig(taskContext, taskConfig)

		assert.Equal(t, "test-task", taskContext["id"])
		assert.Equal(t, string(task.TaskTypeBasic), taskContext["type"])
		assert.Equal(t, "process", taskContext["action"])
	})

	t.Run("Should not override input and output from runtime state", func(t *testing.T) {
		taskContext := map[string]any{
			"id":     "test-task",
			"input":  "runtime-input",
			"output": "runtime-output",
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"config": "value",
				},
			},
		}

		contextBuilder.mergeTaskConfig(taskContext, taskConfig)

		// Runtime input/output should not be overridden
		assert.Equal(t, "runtime-input", taskContext["input"])
		assert.Equal(t, "runtime-output", taskContext["output"])
		// But config fields should be merged
		assert.Equal(t, string(task.TaskTypeBasic), taskContext["type"])
	})
}
