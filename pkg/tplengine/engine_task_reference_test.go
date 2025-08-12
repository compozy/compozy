package tplengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTaskReferences(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected []string
	}{
		{
			name:     "single task reference",
			template: "{{ .tasks.clothing.output.save_data.clothing }}",
			expected: []string{"clothing"},
		},
		{
			name:     "multiple task references",
			template: "{{ .tasks.task1.output }} and {{ .tasks.task2.status }}",
			expected: []string{"task1", "task2"},
		},
		{
			name:     "nested task reference",
			template: "{{ .tasks.parent.children.child1.output }}",
			expected: []string{"parent"},
		},
		{
			name:     "no task references",
			template: "{{ .input.city }}",
			expected: []string{},
		},
		{
			name:     "complex task reference",
			template: "{{ if .tasks.validate.output }}{{ .tasks.process.result }}{{ end }}",
			expected: []string{"validate", "process"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTaskReferences(tt.template)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAreAllTasksAvailable(t *testing.T) {
	tasksMap := map[string]any{
		"task1": map[string]any{"output": "data1"},
		"task2": map[string]any{"output": "data2"},
	}
	t.Run("all tasks available", func(t *testing.T) {
		taskIDs := []string{"task1", "task2"}
		assert.True(t, areAllTasksAvailable(taskIDs, tasksMap))
	})
	t.Run("some tasks missing", func(t *testing.T) {
		taskIDs := []string{"task1", "task3"}
		assert.False(t, areAllTasksAvailable(taskIDs, tasksMap))
	})
	t.Run("empty task list", func(t *testing.T) {
		taskIDs := []string{}
		assert.True(t, areAllTasksAvailable(taskIDs, tasksMap))
	})
}

func TestParseMapWithFilter_TaskReferences(t *testing.T) {
	engine := NewEngine(FormatText)
	t.Run("should defer evaluation when referenced task not available", func(t *testing.T) {
		// Template references task "clothing" which doesn't exist yet
		template := "{{ .tasks.clothing.output.save_data.clothing }}"
		data := map[string]any{
			"tasks": map[string]any{
				"weather": map[string]any{"output": "sunny"},
				// "clothing" task is missing!
			},
		}
		result, err := engine.ParseMapWithFilter(template, data, nil)
		require.NoError(t, err)
		// Should keep the template string as-is since clothing task is not available
		assert.Equal(t, template, result)
	})
	t.Run("should evaluate when all referenced tasks are available", func(t *testing.T) {
		// Template references task "clothing" which now exists
		template := "{{ .tasks.clothing.output.save_data.clothing }}"
		data := map[string]any{
			"tasks": map[string]any{
				"clothing": map[string]any{
					"output": map[string]any{
						"save_data": map[string]any{
							"clothing": []string{"jacket", "umbrella"},
						},
					},
				},
			},
		}
		result, err := engine.ParseMapWithFilter(template, data, nil)
		require.NoError(t, err)
		// Should evaluate the template since clothing task is available
		// The template engine returns the actual array, not a string representation
		expected := []string{"jacket", "umbrella"}
		assert.Equal(t, expected, result)
	})
	t.Run("should handle multiple task references correctly", func(t *testing.T) {
		// Template references both task1 and task2
		template := "{{ .tasks.task1.output }} - {{ .tasks.task2.status }}"
		// Only task1 is available
		data := map[string]any{
			"tasks": map[string]any{
				"task1": map[string]any{"output": "result1"},
				// task2 is missing!
			},
		}
		result, err := engine.ParseMapWithFilter(template, data, nil)
		require.NoError(t, err)
		// Should keep the template as-is since not all tasks are available
		assert.Equal(t, template, result)
	})
	t.Run("should handle map with task references in values", func(t *testing.T) {
		// Map with a value containing task reference
		value := map[string]any{
			"items": "{{ .tasks.clothing.output.save_data.clothing }}",
			"city":  "{{ .input.city }}",
		}
		data := map[string]any{
			"input": map[string]any{"city": "Paris"},
			"tasks": map[string]any{
				"weather": map[string]any{"output": "sunny"},
				// "clothing" task is missing!
			},
		}
		result, err := engine.ParseMapWithFilter(value, data, nil)
		require.NoError(t, err)
		resultMap := result.(map[string]any)
		// Task reference should be deferred
		assert.Equal(t, "{{ .tasks.clothing.output.save_data.clothing }}", resultMap["items"])
		// Input reference should be evaluated
		assert.Equal(t, "Paris", resultMap["city"])
	})
	t.Run("should handle nested structures correctly", func(t *testing.T) {
		// Complex nested structure like in weather workflow
		value := map[string]any{
			"tasks": []any{
				map[string]any{
					"id":    "prepare_content",
					"items": "{{ .tasks.clothing.output.save_data.clothing }}",
					"with": map[string]any{
						"city": "{{ .input.city }}",
					},
				},
			},
		}
		data := map[string]any{
			"input": map[string]any{"city": "London"},
			"tasks": map[string]any{
				// No clothing task yet
			},
		}
		result, err := engine.ParseMapWithFilter(value, data, nil)
		require.NoError(t, err)
		resultMap := result.(map[string]any)
		tasksList := resultMap["tasks"].([]any)
		firstTask := tasksList[0].(map[string]any)
		// Task reference should be deferred
		assert.Equal(t, "{{ .tasks.clothing.output.save_data.clothing }}", firstTask["items"])
		// Input reference should be evaluated
		withMap := firstTask["with"].(map[string]any)
		assert.Equal(t, "London", withMap["city"])
	})
}
