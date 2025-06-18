package normalizer

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigNormalizer_NormalizeWorkflowOutput(t *testing.T) {
	t.Run("Should transform workflow output using task outputs and input context", func(t *testing.T) {
		normalizer := NewConfigNormalizer()

		// Create workflow state with multiple task outputs
		workflowState := &workflow.State{
			WorkflowID:     "data-processing-workflow",
			WorkflowExecID: core.MustNewID(),
			Input: &core.Input{
				"data_source": "api.example.com",
				"batch_size":  100,
			},
			Tasks: map[string]*task.State{
				"data_fetcher": {
					TaskID: "data_fetcher",
					Status: core.StatusSuccess,
					Output: &core.Output{
						"records_count": 250,
						"data_format":   "json",
						"fetch_time":    "2.5s",
					},
				},
				"data_processor": {
					TaskID: "data_processor",
					Status: core.StatusSuccess,
					Output: &core.Output{
						"processed_records": 245,
						"invalid_records":   5,
						"processing_time":   "5.2s",
					},
				},
				"data_validator": {
					TaskID: "data_validator",
					Status: core.StatusSuccess,
					Output: &core.Output{
						"validation_score": 0.98,
						"errors_found":     2,
					},
				},
			},
		}

		// Create outputs configuration that transforms workflow output
		outputsConfig := &core.Input{
			// Simple field mapping from task outputs
			"total_records": "{{ .tasks.data_fetcher.output.records_count }}",
			"valid_records": "{{ .tasks.data_processor.output.processed_records }}",

			// Computed values combining multiple task outputs
			"processing_summary": "Processed {{ .tasks.data_processor.output.processed_records }} of {{ .tasks.data_fetcher.output.records_count }} records in {{ .tasks.data_processor.output.processing_time }}",

			// Workflow input access
			"source_info": map[string]any{
				"data_source": "{{ .input.data_source }}",
				"batch_size":  "{{ .input.batch_size }}",
			},

			// Complex nested object creation combining task outputs
			"quality_metrics": map[string]any{
				"fetch_time":       "{{ .tasks.data_fetcher.output.fetch_time }}",
				"processing_time":  "{{ .tasks.data_processor.output.processing_time }}",
				"validation_score": "{{ .tasks.data_validator.output.validation_score }}",
				"error_rate":       "0.02",
				"data_format":      "{{ .tasks.data_fetcher.output.data_format }}",
			},

			// Task status access
			"all_tasks_completed": "{{ and (eq .tasks.data_fetcher.status \"SUCCESS\") (eq .tasks.data_processor.status \"SUCCESS\") (eq .tasks.data_validator.status \"SUCCESS\") }}",
		}

		// Execute workflow output transformation
		result, err := normalizer.NormalizeWorkflowOutput(workflowState, outputsConfig)

		// Assertions
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify simple field mapping
		assert.Equal(t, "250", (*result)["total_records"])
		assert.Equal(t, "245", (*result)["valid_records"])

		// Verify computed values
		assert.Equal(t, "Processed 245 of 250 records in 5.2s", (*result)["processing_summary"])

		// Verify workflow input access
		sourceInfo, ok := (*result)["source_info"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "api.example.com", sourceInfo["data_source"])
		assert.Equal(t, "100", sourceInfo["batch_size"])

		// Verify complex nested object creation
		qualityMetrics, ok := (*result)["quality_metrics"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "2.5s", qualityMetrics["fetch_time"])
		assert.Equal(t, "5.2s", qualityMetrics["processing_time"])
		assert.Equal(t, "0.98", qualityMetrics["validation_score"])
		assert.Equal(t, "0.02", qualityMetrics["error_rate"])
		assert.Equal(t, "json", qualityMetrics["data_format"])

		// Verify task status access
		assert.Equal(t, "true", (*result)["all_tasks_completed"])
	})

	t.Run("Should handle task errors in output transformation", func(t *testing.T) {
		normalizer := NewConfigNormalizer()

		workflowState := &workflow.State{
			WorkflowID:     "workflow-with-errors",
			WorkflowExecID: core.MustNewID(),
			Input: &core.Input{
				"retry_count": 3,
			},
			Tasks: map[string]*task.State{
				"success_task": {
					TaskID: "success_task",
					Status: core.StatusSuccess,
					Output: &core.Output{
						"result": "success",
					},
				},
				"failed_task": {
					TaskID: "failed_task",
					Status: core.StatusFailed,
					Error:  core.NewError(nil, "TASK_EXECUTION_FAILED", map[string]any{"code": "500"}),
					Output: &core.Output{
						"partial_result": "incomplete",
					},
				},
			},
		}

		outputsConfig := &core.Input{
			// Access successful task output
			"success_result": "{{ .tasks.success_task.output.result }}",

			// Access failed task error and partial output
			"failure_info": map[string]any{
				"error_code":     "500",
				"error_type":     "TASK_EXECUTION_FAILED",
				"partial_result": "{{ .tasks.failed_task.output.partial_result }}",
				"task_status":    "{{ .tasks.failed_task.status }}",
			},

			// Conditional output based on task status
			"has_failures": "true",
			"retry_count":  "{{ .input.retry_count }}",
		}

		result, err := normalizer.NormalizeWorkflowOutput(workflowState, outputsConfig)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify success task output access
		assert.Equal(t, "success", (*result)["success_result"])

		// Verify failure information access
		failureInfo, ok := (*result)["failure_info"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "500", failureInfo["error_code"])
		assert.Equal(t, "TASK_EXECUTION_FAILED", failureInfo["error_type"])
		assert.Equal(t, "incomplete", failureInfo["partial_result"])
		assert.Equal(t, core.StatusFailed, failureInfo["task_status"])

		// Verify conditional outputs
		assert.Equal(t, "true", (*result)["has_failures"])
		assert.Equal(t, "3", (*result)["retry_count"])
	})

	t.Run("Should return nil when outputs config is nil", func(t *testing.T) {
		normalizer := NewConfigNormalizer()

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"test_task": {
					TaskID: "test_task",
					Status: core.StatusSuccess,
					Output: &core.Output{
						"result": "test",
					},
				},
			},
		}

		result, err := normalizer.NormalizeWorkflowOutput(workflowState, nil)

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should handle workflow without input", func(t *testing.T) {
		normalizer := NewConfigNormalizer()

		workflowState := &workflow.State{
			WorkflowID:     "no-input-workflow",
			WorkflowExecID: core.MustNewID(),
			Input:          nil, // No workflow input
			Tasks: map[string]*task.State{
				"processor": {
					TaskID: "processor",
					Status: core.StatusSuccess,
					Output: &core.Output{
						"result": "processed",
					},
				},
			},
		}

		outputsConfig := &core.Input{
			"task_result": "{{ .tasks.processor.output.result }}",
			// Should handle missing input gracefully
			"workflow_input_exists": "{{ if .input }}true{{ else }}false{{ end }}",
		}

		result, err := normalizer.NormalizeWorkflowOutput(workflowState, outputsConfig)

		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "processed", (*result)["task_result"])
		assert.Equal(t, "false", (*result)["workflow_input_exists"])
	})

	t.Run("Should handle template errors gracefully", func(t *testing.T) {
		normalizer := NewConfigNormalizer()

		workflowState := &workflow.State{
			WorkflowID:     "error-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"test_task": {
					TaskID: "test_task",
					Status: core.StatusSuccess,
					Output: &core.Output{
						"result": "test",
					},
				},
			},
		}

		outputsConfig := &core.Input{
			"valid_field":   "{{ .tasks.test_task.output.result }}",
			"invalid_field": "{{ .tasks.nonexistent_task.output.value }}", // Should cause error
		}

		_, err := normalizer.NormalizeWorkflowOutput(workflowState, outputsConfig)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to transform workflow output field invalid_field")
		assert.Contains(t, err.Error(), "nonexistent_task")
	})

	t.Run("Should handle complex parallel task output structures", func(t *testing.T) {
		normalizer := NewConfigNormalizer()

		workflowState := &workflow.State{
			WorkflowID:     "parallel-workflow",
			WorkflowExecID: core.MustNewID(),
			Input: &core.Input{
				"batch_id": "batch-123",
			},
			Tasks: map[string]*task.State{
				"parallel_processor": {
					TaskID: "parallel_processor",
					Status: core.StatusSuccess,
					Output: &core.Output{
						// Simulating parallel task output structure
						"task_1": map[string]any{
							"output": map[string]any{
								"processed_count": 100,
								"processing_time": "2.1s",
							},
						},
						"task_2": map[string]any{
							"output": map[string]any{
								"processed_count": 150,
								"processing_time": "3.2s",
							},
						},
					},
				},
				"aggregator": {
					TaskID: "aggregator",
					Status: core.StatusSuccess,
					Output: &core.Output{
						"total_processed": 250,
						"avg_time":        "2.65s",
					},
				},
			},
		}

		outputsConfig := &core.Input{
			// Access nested parallel task outputs
			"task1_count": "{{ .tasks.parallel_processor.output.task_1.output.processed_count }}",
			"task2_count": "{{ .tasks.parallel_processor.output.task_2.output.processed_count }}",

			// Access aggregated results
			"total_count":  "{{ .tasks.aggregator.output.total_processed }}",
			"average_time": "{{ .tasks.aggregator.output.avg_time }}",

			// Combine input and parallel task outputs
			"batch_summary": map[string]any{
				"batch_id":      "{{ .input.batch_id }}",
				"total_records": "{{ .tasks.aggregator.output.total_processed }}",
				"sub_tasks": map[string]any{
					"task_1_count": "{{ .tasks.parallel_processor.output.task_1.output.processed_count }}",
					"task_2_count": "{{ .tasks.parallel_processor.output.task_2.output.processed_count }}",
				},
			},
		}

		result, err := normalizer.NormalizeWorkflowOutput(workflowState, outputsConfig)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify parallel task output access
		assert.Equal(t, "100", (*result)["task1_count"])
		assert.Equal(t, "150", (*result)["task2_count"])

		// Verify aggregated results
		assert.Equal(t, "250", (*result)["total_count"])
		assert.Equal(t, "2.65s", (*result)["average_time"])

		// Verify complex nested structure
		batchSummary, ok := (*result)["batch_summary"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "batch-123", batchSummary["batch_id"])
		assert.Equal(t, "250", batchSummary["total_records"])

		subTasks, ok := batchSummary["sub_tasks"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "100", subTasks["task_1_count"])
		assert.Equal(t, "150", subTasks["task_2_count"])
	})

	t.Run("Should handle empty task map", func(t *testing.T) {
		normalizer := NewConfigNormalizer()

		workflowState := &workflow.State{
			WorkflowID:     "empty-workflow",
			WorkflowExecID: core.MustNewID(),
			Input: &core.Input{
				"status": "initialized",
			},
			Tasks: map[string]*task.State{}, // Empty task map
		}

		outputsConfig := &core.Input{
			"workflow_status": "{{ .input.status }}",
			"task_count":      "{{ len .tasks }}",
		}

		result, err := normalizer.NormalizeWorkflowOutput(workflowState, outputsConfig)

		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "initialized", (*result)["workflow_status"])
		assert.Equal(t, "0", (*result)["task_count"])
	})
}
