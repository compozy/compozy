package normalizer

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigNormalizer_NormalizeParallelTask(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize parallel task with sub-tasks containing templates", func(t *testing.T) {
		parallelTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "process_data_parallel",
				Type: task.TaskTypeParallel,
				With: &core.Input{
					"raw_data": "sample data",
					"content":  "This is a great product! I love it.",
				},
				Env: &core.EnvMap{
					"PARALLEL_TIMEOUT": "5m",
				},
				Timeout: "5m",
			},
			ParallelTask: task.ParallelTask{
				Strategy:   task.StrategyWaitAll,
				MaxWorkers: 4,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "sentiment_analysis",
						Type: task.TaskTypeBasic,
						With: &core.Input{
							"text": "{{ .workflow.input.content }}",
						},
					},
					BasicTask: task.BasicTask{
						Action: "analyze_sentiment",
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "extract_keywords",
						Type: task.TaskTypeBasic,
						With: &core.Input{
							"text":         "{{ .workflow.input.content }}",
							"max_keywords": 10,
						},
					},
					BasicTask: task.BasicTask{
						Action: "extract",
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: "exec-123",
			Input: &core.Input{
				"content":  "This is great!",
				"raw_data": "sample",
			},
		}

		workflowConfig := &workflow.Config{
			ID:    "test-workflow",
			Tasks: []task.Config{*parallelTaskConfig},
		}

		// Check what templates look like before normalization
		t.Logf("Before normalization - subTask1 text: %v", (*parallelTaskConfig.Tasks[0].With)["text"])
		t.Logf("Before normalization - subTask2 text: %v", (*parallelTaskConfig.Tasks[1].With)["text"])

		// Normalize the parallel task
		err := normalizer.NormalizeTask(workflowState, workflowConfig, parallelTaskConfig)
		require.NoError(t, err)

		// Check what templates look like after normalization
		t.Logf("After normalization - subTask1 text: %v", (*parallelTaskConfig.Tasks[0].With)["text"])
		t.Logf("After normalization - subTask2 text: %v", (*parallelTaskConfig.Tasks[1].With)["text"])

		// Check that sub-task templates were resolved
		subTask1 := parallelTaskConfig.Tasks[0]
		assert.Equal(t, "This is great!", (*subTask1.With)["text"])

		subTask2 := parallelTaskConfig.Tasks[1]
		assert.Equal(t, "This is great!", (*subTask2.With)["text"])
		// Fix type assertion for max_keywords - it might be converted to float64 by JSON unmarshaling
		maxKeywords := (*subTask2.With)["max_keywords"]
		switch v := maxKeywords.(type) {
		case int:
			assert.Equal(t, 10, v)
		case float64:
			assert.Equal(t, float64(10), v)
		case string:
			assert.Equal(t, "10", v)
		default:
			t.Errorf("max_keywords has unexpected type: %T", maxKeywords)
		}
	})

	t.Run("Should handle sub-task normalization that references parent parallel task context", func(t *testing.T) {
		// Test what happens when we try to normalize individual sub-tasks
		// that might need parent parallel task context
		parallelTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "batch_processor",
				Type: task.TaskTypeParallel,
				With: &core.Input{
					"batch_id":   "batch-123",
					"batch_size": 10,
				},
			},
			ParallelTask: task.ParallelTask{
				Strategy: task.StrategyWaitAll,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "process_item_1",
						Type: task.TaskTypeBasic,
						With: &core.Input{
							"item_id":   "item-1",
							"parent_id": "{{ .parent.id }}",             // Should reference parallel task
							"batch_id":  "{{ .parent.input.batch_id }}", // Should reference parent input
						},
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: "exec-123",
		}

		workflowConfig := &workflow.Config{
			ID:    "test-workflow",
			Tasks: []task.Config{*parallelTaskConfig},
		}

		// This tests the current behavior - parallel task normalization should work
		err := normalizer.NormalizeTask(workflowState, workflowConfig, parallelTaskConfig)
		require.NoError(t, err)

		// Check that basic fields are handled correctly
		subTask := parallelTaskConfig.Tasks[0]
		assert.Equal(t, "item-1", (*subTask.With)["item_id"])

		// These templates should remain unresolved since there's no parent context
		// established for sub-tasks yet (this is what we need to potentially fix)
		t.Logf("parent_id after normalization: %v", (*subTask.With)["parent_id"])
		t.Logf("batch_id after normalization: %v", (*subTask.With)["batch_id"])
	})

	t.Run("Should support nested output access for parallel tasks", func(t *testing.T) {
		// Test task that references nested parallel task outputs
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "aggregator_task",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					// Access nested sub-task outputs using tasks.parallel_task.output.subtask_id.output format
					"sentiment_result": "{{ .tasks.process_data_parallel.output.sentiment_analysis.output.sentiment }}",
					"keywords_result":  "{{ .tasks.process_data_parallel.output.extract_keywords.output.keywords }}",
					"analysis_score":   "{{ .tasks.process_data_parallel.output.sentiment_analysis.output.confidence }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "aggregate",
			},
		}

		// Create parent task exec ID for reference
		parentExecID := core.MustNewID() // guarantees uniqueness for every test run

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: "exec-123",
			Tasks: map[string]*task.State{
				"process_data_parallel": {
					TaskID:        "process_data_parallel",
					TaskExecID:    parentExecID,
					ExecutionType: task.ExecutionParallel,
				},
				"sentiment_analysis": {
					TaskID:        "sentiment_analysis",
					TaskExecID:    core.ID("sentiment_analysis_exec"),
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parentExecID,
					Output: &core.Output{
						"sentiment":  "positive",
						"confidence": 0.95,
					},
				},
				"extract_keywords": {
					TaskID:        "extract_keywords",
					TaskExecID:    core.ID("extract_keywords_exec"),
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parentExecID,
					Output: &core.Output{
						"keywords": []string{"great", "product", "love"},
						"count":    3,
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "process_data_parallel",
						Type: task.TaskTypeParallel,
					},
				},
				*taskConfig,
			},
		}

		err := normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
		require.NoError(t, err)

		// Verify nested output access works
		assert.Equal(t, "positive", (*taskConfig.With)["sentiment_result"])
		assert.Equal(
			t,
			[]string{"great", "product", "love"},
			(*taskConfig.With)["keywords_result"],
		)
		assert.Equal(t, "0.95", (*taskConfig.With)["analysis_score"])
	})

	t.Run("Should demonstrate complete parallel task workflow with nested access", func(t *testing.T) {
		// This test demonstrates a complete workflow:
		// 1. A parallel task that processes data in sub-tasks
		// 2. An aggregator task that accesses the nested outputs
		// 3. A final task that uses the aggregated results

		// Define the parallel task configuration (would contain sub-tasks)
		parallelTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel_processor",
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				Strategy: task.StrategyWaitAll,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "sentiment_analysis",
						Type: task.TaskTypeBasic,
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "keyword_extraction",
						Type: task.TaskTypeBasic,
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "performance_monitor",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}

		// Define an aggregator task that uses nested output access
		aggregatorConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "aggregate_results",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"sentiment":   "{{ .tasks.parallel_processor.output.sentiment_analysis.output.sentiment }}",
					"keywords":    "{{ .tasks.parallel_processor.output.keyword_extraction.output.keywords }}",
					"confidence":  "{{ .tasks.parallel_processor.output.sentiment_analysis.output.confidence }}",
					"duration":    "{{ .tasks.parallel_processor.output.performance_monitor.output.duration }}",
					"full_result": "{{ .tasks.parallel_processor.output.sentiment_analysis.output }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "aggregate",
			},
		}

		// Define a final task that uses the aggregated results
		finalTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "generate_report",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"aggregated_data": "{{ .tasks.aggregate_results.output.summary }}",
					"total_keywords":  "{{ len .tasks.parallel_processor.output.keyword_extraction.output.keywords }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "generate_report",
			},
		}

		// Create parent task exec ID for parallel processor
		parallelProcessorExecID := core.ID("parallel_processor_exec")

		workflowState := &workflow.State{
			WorkflowID:     "analysis-workflow",
			WorkflowExecID: "exec-analysis",
			Tasks: map[string]*task.State{
				"parallel_processor": {
					TaskID:        "parallel_processor",
					TaskExecID:    parallelProcessorExecID,
					ExecutionType: task.ExecutionParallel,
				},
				"sentiment_analysis": {
					TaskID:        "sentiment_analysis",
					TaskExecID:    core.ID("sentiment_analysis_exec"),
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parallelProcessorExecID,
					Output: &core.Output{
						"sentiment":  "positive",
						"confidence": 0.92,
						"details":    "High confidence positive sentiment detected",
					},
				},
				"keyword_extraction": {
					TaskID:        "keyword_extraction",
					TaskExecID:    core.ID("keyword_extraction_exec"),
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parallelProcessorExecID,
					Output: &core.Output{
						"keywords": []string{"excellent", "quality", "recommend", "satisfied"},
						"count":    4,
					},
				},
				"performance_monitor": {
					TaskID:        "performance_monitor",
					TaskExecID:    core.ID("performance_monitor_exec"),
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parallelProcessorExecID,
					Output: &core.Output{
						"duration":    "2.3s",
						"memory_used": "45MB",
					},
				},
				"aggregate_results": {
					TaskID:     "aggregate_results",
					TaskExecID: core.ID("aggregate_results_exec"),
					Output: &core.Output{
						"summary": map[string]any{
							"sentiment":       "positive",
							"keyword_count":   4,
							"confidence":      0.92,
							"processing_time": "2.3s",
						},
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "analysis-workflow",
			Tasks: []task.Config{
				*parallelTaskConfig,
				*aggregatorConfig,
				*finalTaskConfig,
			},
		}

		// Test normalization of the aggregator task (accessing nested outputs)
		err := normalizer.NormalizeTask(workflowState, workflowConfig, aggregatorConfig)
		require.NoError(t, err)

		// Verify the aggregator task can access nested outputs
		assert.Equal(t, "positive", (*aggregatorConfig.With)["sentiment"])
		assert.Equal(
			t,
			[]string{"excellent", "quality", "recommend", "satisfied"},
			(*aggregatorConfig.With)["keywords"],
		)
		assert.Equal(t, "0.92", (*aggregatorConfig.With)["confidence"])
		assert.Equal(t, "2.3s", (*aggregatorConfig.With)["duration"])

		// Verify it can access the full output object
		fullResult := (*aggregatorConfig.With)["full_result"].(map[string]any)
		assert.Equal(t, "positive", fullResult["sentiment"])
		assert.Equal(t, "High confidence positive sentiment detected", fullResult["details"])

		// Test normalization of the final task (accessing both nested and regular outputs)
		err = normalizer.NormalizeTask(workflowState, workflowConfig, finalTaskConfig)
		require.NoError(t, err)

		// Verify the final task can access both aggregated and nested outputs
		expectedSummary := map[string]any{
			"sentiment":       "positive",
			"keyword_count":   4, // Keep original type from the mock data
			"confidence":      0.92,
			"processing_time": "2.3s",
		}
		assert.Equal(t, expectedSummary, (*finalTaskConfig.With)["aggregated_data"])
		assert.Equal(t, "4", (*finalTaskConfig.With)["total_keywords"]) // len() function results are now strings
	})
}

func TestConfigNormalizer_NestedParallelTasks(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize two levels of nested parallel tasks with templates", func(t *testing.T) {
		// Create a deeply nested structure:
		// batch_processor (parallel)
		//   ├── data_processor (parallel)
		//   │   ├── sentiment_analysis (basic)
		//   │   └── keyword_extraction (basic)
		//   └── metadata_processor (basic)

		nestedParallelTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "batch_processor",
				Type: task.TaskTypeParallel,
				With: &core.Input{
					"batch_id":   "batch-456",
					"batch_size": 100,
					"priority":   "high",
				},
			},
			ParallelTask: task.ParallelTask{
				Strategy:   task.StrategyWaitAll,
				MaxWorkers: 2,
			},
			Tasks: []task.Config{
				// First sub-task: another parallel task
				{
					BaseConfig: task.BaseConfig{
						ID:   "data_processor",
						Type: task.TaskTypeParallel,
						With: &core.Input{
							"processor_id":    "dp-{{ .parent.input.batch_id }}",
							"parent_batch":    "{{ .parent.id }}",
							"parent_priority": "{{ .parent.input.priority }}",
							"workflow_id":     "{{ .workflow.id }}",
						},
					},
					ParallelTask: task.ParallelTask{
						Strategy: task.StrategyWaitAll,
					},
					Tasks: []task.Config{
						// Deeply nested basic task 1
						{
							BaseConfig: task.BaseConfig{
								ID:   "sentiment_analysis",
								Type: task.TaskTypeBasic,
								With: &core.Input{
									"text":              "{{ .workflow.input.content }}",
									"processor_parent":  "{{ .parent.id }}",
									"batch_parent":      "{{ .parent.input.parent_batch }}",
									"original_priority": "{{ .parent.input.parent_priority }}",
									"task_chain":        "{{ .workflow.id }}.{{ .parent.input.parent_batch }}.{{ .parent.id }}.sentiment_analysis",
								},
							},
							BasicTask: task.BasicTask{
								Action: "analyze_sentiment",
							},
						},
						// Deeply nested basic task 2
						{
							BaseConfig: task.BaseConfig{
								ID:   "keyword_extraction",
								Type: task.TaskTypeBasic,
								With: &core.Input{
									"text":              "{{ .workflow.input.content }}",
									"processor_parent":  "{{ .parent.id }}",
									"batch_parent":      "{{ .parent.input.parent_batch }}",
									"original_priority": "{{ .parent.input.parent_priority }}",
									"max_keywords":      "{{ .parent.input.parent_batch | len }}",
								},
							},
							BasicTask: task.BasicTask{
								Action: "extract_keywords",
							},
						},
					},
				},
				// Second sub-task: basic task at first nesting level
				{
					BaseConfig: task.BaseConfig{
						ID:   "metadata_processor",
						Type: task.TaskTypeBasic,
						With: &core.Input{
							"batch_info":     "{{ .parent.input.batch_id }}",
							"batch_priority": "{{ .parent.input.priority }}",
							"parent_task":    "{{ .parent.id }}",
							"workflow_ref":   "{{ .workflow.id }}",
						},
					},
					BasicTask: task.BasicTask{
						Action: "process_metadata",
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "nested-workflow",
			WorkflowExecID: "exec-nested-123",
			Input: &core.Input{
				"content": "This is amazing content to analyze!",
			},
		}

		workflowConfig := &workflow.Config{
			ID:    "nested-workflow",
			Tasks: []task.Config{*nestedParallelTaskConfig},
		}

		// Normalize the nested parallel task structure
		err := normalizer.NormalizeTask(workflowState, workflowConfig, nestedParallelTaskConfig)
		require.NoError(t, err)

		// Verify normalization of the outer parallel task
		assert.Equal(t, "batch-456", (*nestedParallelTaskConfig.With)["batch_id"])
		assert.Equal(
			t,
			float64(100),
			(*nestedParallelTaskConfig.With)["batch_size"],
		) // JSON unmarshaling converts to float64

		// Verify normalization of the first sub-task (nested parallel task)
		dataProcessor := &nestedParallelTaskConfig.Tasks[0]
		assert.Equal(t, "dp-batch-456", (*dataProcessor.With)["processor_id"])
		assert.Equal(t, "batch_processor", (*dataProcessor.With)["parent_batch"])
		assert.Equal(t, "high", (*dataProcessor.With)["parent_priority"])
		assert.Equal(t, "nested-workflow", (*dataProcessor.With)["workflow_id"])

		// Verify normalization of deeply nested basic tasks
		sentimentTask := &dataProcessor.Tasks[0]
		assert.Equal(t, "This is amazing content to analyze!", (*sentimentTask.With)["text"])
		assert.Equal(t, "data_processor", (*sentimentTask.With)["processor_parent"])
		assert.Equal(t, "batch_processor", (*sentimentTask.With)["batch_parent"])
		assert.Equal(t, "high", (*sentimentTask.With)["original_priority"])
		assert.Equal(
			t,
			"nested-workflow.batch_processor.data_processor.sentiment_analysis",
			(*sentimentTask.With)["task_chain"],
		)

		keywordTask := &dataProcessor.Tasks[1]
		assert.Equal(t, "This is amazing content to analyze!", (*keywordTask.With)["text"])
		assert.Equal(t, "data_processor", (*keywordTask.With)["processor_parent"])
		assert.Equal(t, "batch_processor", (*keywordTask.With)["batch_parent"])
		assert.Equal(t, "high", (*keywordTask.With)["original_priority"])
		assert.Equal(t, "15", (*keywordTask.With)["max_keywords"])

		// Verify normalization of the second sub-task (basic task)
		metadataProcessor := &nestedParallelTaskConfig.Tasks[1]
		assert.Equal(t, "batch-456", (*metadataProcessor.With)["batch_info"])
		assert.Equal(t, "high", (*metadataProcessor.With)["batch_priority"])
		assert.Equal(t, "batch_processor", (*metadataProcessor.With)["parent_task"])
		assert.Equal(t, "nested-workflow", (*metadataProcessor.With)["workflow_ref"])
	})

	t.Run("Should handle nested parallel task output access correctly", func(t *testing.T) {
		// Test accessing outputs from deeply nested parallel structures
		aggregatorTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "nested_aggregator",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					// Access nested outputs: batch_processor.data_processor.sentiment_analysis
					"deep_sentiment": "{{ .tasks.batch_processor.output.data_processor.output.sentiment_analysis.output.sentiment }}",
					"deep_keywords":  "{{ .tasks.batch_processor.output.data_processor.output.keyword_extraction.output.keywords }}",
					// Access first-level output: batch_processor.metadata_processor
					"metadata": "{{ .tasks.batch_processor.output.metadata_processor.output.metadata }}",
					// Count nested results
					"keyword_count": "{{ len .tasks.batch_processor.output.data_processor.output.keyword_extraction.output.keywords }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "aggregate_nested",
			},
		}

		// Create task exec IDs for nested hierarchy
		batchProcessorExecID := core.ID("batch_processor_exec")
		dataProcessorExecID := core.ID("data_processor_exec")

		workflowState := &workflow.State{
			WorkflowID:     "nested-output-workflow",
			WorkflowExecID: "exec-nested-output",
			Tasks: map[string]*task.State{
				"batch_processor": {
					TaskID:        "batch_processor",
					TaskExecID:    batchProcessorExecID,
					ExecutionType: task.ExecutionParallel,
				},
				"data_processor": {
					TaskID:        "data_processor",
					TaskExecID:    dataProcessorExecID,
					ExecutionType: task.ExecutionParallel,
					ParentStateID: &batchProcessorExecID,
				},
				"sentiment_analysis": {
					TaskID:        "sentiment_analysis",
					TaskExecID:    core.ID("sentiment_analysis_exec"),
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &dataProcessorExecID,
					Output: &core.Output{
						"sentiment":  "very_positive",
						"confidence": 0.98,
					},
				},
				"keyword_extraction": {
					TaskID:        "keyword_extraction",
					TaskExecID:    core.ID("keyword_extraction_exec"),
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &dataProcessorExecID,
					Output: &core.Output{
						"keywords": []string{"amazing", "content", "analyze", "great"},
						"count":    4,
					},
				},
				"metadata_processor": {
					TaskID:        "metadata_processor",
					TaskExecID:    core.ID("metadata_processor_exec"),
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &batchProcessorExecID,
					Output: &core.Output{
						"metadata":     "processed_metadata",
						"process_time": "1.2s",
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "nested-output-workflow",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "batch_processor",
						Type: task.TaskTypeParallel,
					},
				},
				*aggregatorTaskConfig,
			},
		}

		err := normalizer.NormalizeTask(workflowState, workflowConfig, aggregatorTaskConfig)
		require.NoError(t, err)

		// Verify deeply nested output access
		assert.Equal(t, "very_positive", (*aggregatorTaskConfig.With)["deep_sentiment"])
		assert.Equal(
			t,
			[]string{"amazing", "content", "analyze", "great"},
			(*aggregatorTaskConfig.With)["deep_keywords"],
		)
		assert.Equal(t, "processed_metadata", (*aggregatorTaskConfig.With)["metadata"])
		assert.Equal(t, "4", (*aggregatorTaskConfig.With)["keyword_count"])
	})

	t.Run("Should handle template errors in deeply nested structures gracefully", func(t *testing.T) {
		// Test error handling when templates in deeply nested tasks have issues
		nestedTaskWithError := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "error_batch_processor",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "error_data_processor",
						Type: task.TaskTypeParallel,
					},
					Tasks: []task.Config{
						{
							BaseConfig: task.BaseConfig{
								ID:   "error_task",
								Type: task.TaskTypeBasic,
								With: &core.Input{
									// This should cause an error due to invalid template
									"invalid": "{{ .nonexistent.field.value }}",
								},
							},
						},
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "error-workflow",
			WorkflowExecID: "exec-error",
		}

		workflowConfig := &workflow.Config{
			ID:    "error-workflow",
			Tasks: []task.Config{*nestedTaskWithError},
		}

		err := normalizer.NormalizeTask(workflowState, workflowConfig, nestedTaskWithError)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonexistent")
		assert.Contains(t, err.Error(), "failed to normalize sub-task error_task")
	})
}
