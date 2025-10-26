package aggregate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/aggregate"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	tkhelpers "github.com/compozy/compozy/test/integration/tasks/helpers"
)

// TestAggregateConfigInheritance validates that aggregate tasks properly inherit
// CWD and FilePath when used as child tasks in parent task configurations
func TestAggregateConfigInheritance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}
	t.Parallel()

	t.Run("Should inherit CWD and FilePath as child task", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create aggregate task config without explicit CWD/FilePath
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-aggregate-child",
				Type: task.TaskTypeAggregate,
				With: &core.Input{
					"data_source": "{{ .parent_results }}",
					"format":      "json",
				},
				// No CWD/FilePath - will be inherited by parent normalizer
			},
			BasicTask: task.BasicTask{
				Action: "summarize_results",
			},
		}

		// Simulate inheritance by parent normalizer
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/parent/aggregate/directory"},
				FilePath: "configs/parent_aggregation.yaml",
			},
		}

		// Apply inheritance like a parent normalizer would
		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer to test normalization with inherited context
		normalizer := aggregate.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"parent_results": []map[string]any{
					{"value": 10, "status": "success"},
					{"value": 20, "status": "success"},
					{"value": 30, "status": "failed"},
				},
			},
		}

		// Normalize the aggregate task
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err, "Aggregate normalization should succeed")

		// Verify aggregate task inherited context
		require.NotNil(t, taskConfig.CWD, "Aggregate task should have inherited CWD")
		assert.Equal(t, "/parent/aggregate/directory", taskConfig.CWD.Path,
			"Aggregate task should inherit parent CWD")
		assert.Equal(t, "configs/parent_aggregation.yaml", taskConfig.FilePath,
			"Aggregate task should inherit parent FilePath")

		// Verify template processing worked correctly
		assert.Equal(t, "summarize_results", taskConfig.Action,
			"Aggregate action should be preserved")
	})

	t.Run("Should preserve explicit CWD and FilePath", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create aggregate task with explicit CWD/FilePath
		explicitCWD := &core.PathCWD{Path: "/explicit/aggregate/path"}
		explicitFilePath := "explicit_aggregate.yaml"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-aggregate-explicit",
				Type:     task.TaskTypeAggregate,
				CWD:      explicitCWD,      // Explicit CWD
				FilePath: explicitFilePath, // Explicit FilePath
				With: &core.Input{
					"aggregation_type": "mean",
					"output_format":    "summary",
				},
			},
			BasicTask: task.BasicTask{
				Action: "custom_aggregation",
			},
		}

		// Try to apply inheritance (should not override existing values)
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/parent/path"},
				FilePath: "parent.yaml",
			},
		}

		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer
		normalizer := aggregate.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"data_set": "test_data",
			},
		}
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err)

		// Verify aggregate task preserved its explicit values
		require.NotNil(t, taskConfig.CWD)
		assert.Equal(t, "/explicit/aggregate/path", taskConfig.CWD.Path,
			"Aggregate task should preserve explicit CWD")
		assert.Equal(t, "explicit_aggregate.yaml", taskConfig.FilePath,
			"Aggregate task should preserve explicit FilePath")
		assert.Equal(t, "custom_aggregation", taskConfig.Action,
			"Aggregate action should be preserved")
	})

	t.Run("Should handle aggregate task with templated configuration", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create aggregate task with templated configuration
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-templated-aggregate",
				Type: task.TaskTypeAggregate,
				With: &core.Input{
					"source_data":      "{{ .data_source }}",
					"aggregation_type": "{{ .agg_type }}",
					"result_format":    "{{ .output_format }}",
					"filters": map[string]any{
						"status": "{{ .filter_status }}",
						"range":  "{{ .date_range }}",
					},
				},
				Outputs: &core.Input{
					"result":    "{{ .output.summary }}",
					"stats":     "{{ .output.statistics }}",
					"timestamp": "{{ .current_time }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "{{ .aggregation_method }}",
			},
		}

		// Apply inheritance
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/templated/aggregate/dir"},
				FilePath: "templated_agg.yaml",
			},
		}

		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer
		normalizer := aggregate.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize with complex template context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"aggregation_method": "calculate_metrics",
				"data_source":        "user_activity_logs",
				"agg_type":           "statistical",
				"output_format":      "json_detailed",
				"filter_status":      "active",
				"date_range":         "last_7_days",
				"current_time":       "2023-07-04T10:00:00Z",
			},
		}
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err)

		// Verify inheritance and template processing
		require.NotNil(t, taskConfig.CWD)
		assert.Equal(t, "/templated/aggregate/dir", taskConfig.CWD.Path,
			"Aggregate task should inherit CWD for templated configuration")

		// Verify template processing worked correctly
		assert.Equal(t, "calculate_metrics", taskConfig.Action,
			"Aggregate action should be processed with template variables")

		// Templates are processed by BaseNormalizer
		assert.NotNil(t, taskConfig.With, "With field should be preserved")
		assert.Equal(t, "user_activity_logs", (*taskConfig.With)["source_data"],
			"Template variables should be processed in With fields")
		assert.Equal(t, "statistical", (*taskConfig.With)["aggregation_type"],
			"Template variables should be processed")
		assert.Equal(t, "json_detailed", (*taskConfig.With)["result_format"],
			"Template variables should be processed")

		// Verify nested template processing
		filters, ok := (*taskConfig.With)["filters"].(map[string]any)
		require.True(t, ok, "Filters should be processed as map")
		assert.Equal(t, "active", filters["status"],
			"Nested template variables should be processed")
		assert.Equal(t, "last_7_days", filters["range"],
			"Nested template variables should be processed")
	})

	t.Run("Should handle aggregate task with output transformations", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create aggregate task with output transformations
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-output-aggregate",
				Type: task.TaskTypeAggregate,
				With: &core.Input{
					"analytics_data": "{{ .raw_data }}",
				},
				Outputs: &core.Input{
					"processed_count": "{{ .output.total_processed }}",
					"success_rate":    "{{ .output.success_percentage }}",
					"failure_summary": "{{ .output.failures | length }}",
					"report": map[string]any{
						"generated_at": "{{ .timestamp }}",
						"summary":      "{{ .output.summary }}",
					},
				},
			},
			BasicTask: task.BasicTask{
				Action: "process_analytics",
			},
		}

		// Apply inheritance
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/analytics/output/dir"},
				FilePath: "analytics.yaml",
			},
		}

		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer
		normalizer := aggregate.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize with analytics context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"raw_data": []map[string]any{
					{"id": 1, "status": "success", "duration": 100},
					{"id": 2, "status": "success", "duration": 150},
					{"id": 3, "status": "failed", "duration": 50},
				},
				"timestamp": "2023-07-04T10:30:00Z",
			},
		}
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err)

		// Verify inheritance and output configuration
		require.NotNil(t, taskConfig.CWD)
		assert.Equal(t, "/analytics/output/dir", taskConfig.CWD.Path,
			"Aggregate task should inherit CWD for analytics processing")
		assert.Equal(t, "analytics.yaml", taskConfig.FilePath,
			"Aggregate task should inherit FilePath")

		// Verify output configuration is properly structured
		require.NotNil(t, taskConfig.Outputs, "Outputs should be configured")
		assert.Contains(t, taskConfig.Outputs.AsMap(), "processed_count",
			"Output transformations should be preserved")
		assert.Contains(t, taskConfig.Outputs.AsMap(), "success_rate",
			"Output transformations should be preserved")

		// Verify nested output structure
		reportOutput, ok := (*taskConfig.Outputs)["report"].(map[string]any)
		require.True(t, ok, "Report output should be processed as map")
		assert.Contains(t, reportOutput, "generated_at",
			"Nested output templates should be preserved")
	})

	t.Run("Should handle aggregate task with minimal configuration", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create aggregate task with minimal configuration
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-minimal-aggregate",
				Type: task.TaskTypeAggregate,
				// No With or Outputs - minimal configuration
			},
			BasicTask: task.BasicTask{
				Action: "basic_sum",
			},
		}

		// Apply inheritance
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/minimal/aggregate/dir"},
				FilePath: "minimal.yaml",
			},
		}

		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer
		normalizer := aggregate.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"context": "minimal",
			},
		}
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err)

		// Verify inheritance works with minimal configuration
		require.NotNil(t, taskConfig.CWD)
		assert.Equal(t, "/minimal/aggregate/dir", taskConfig.CWD.Path,
			"Aggregate task should inherit CWD even with minimal configuration")
		assert.Equal(t, "minimal.yaml", taskConfig.FilePath,
			"Aggregate task should inherit FilePath")
		assert.Equal(t, "basic_sum", taskConfig.Action,
			"Aggregate action should be preserved")
	})
}
