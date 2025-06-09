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
