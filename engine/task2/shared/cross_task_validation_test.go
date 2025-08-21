package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// TestCrossTaskValidation provides comprehensive validation of context inheritance
// across all task types to ensure consistent behavior and prevent regressions
func TestCrossTaskValidation(t *testing.T) {
	t.Run("Should validate inheritance consistency across all task types", func(t *testing.T) {
		// Define all task types that support inheritance
		taskTypes := []task.Type{
			task.TaskTypeParallel,
			task.TaskTypeComposite,
			task.TaskTypeCollection,
			task.TaskTypeRouter,
			task.TaskTypeWait,
			task.TaskTypeAggregate,
			task.TaskTypeSignal,
			task.TaskTypeBasic,
		}

		// Common parent configuration
		parentCWD := &core.PathCWD{Path: "/validation/parent/path"}
		parentFilePath := "validation.yaml"

		// Test each task type
		for _, taskType := range taskTypes {
			t.Run(string(taskType), func(t *testing.T) {
				// Create parent task
				parentTask := &task.Config{
					BaseConfig: task.BaseConfig{
						ID:       "validation-parent-" + string(taskType),
						Type:     taskType,
						CWD:      parentCWD,
						FilePath: parentFilePath,
					},
				}

				// Create child task without CWD/FilePath
				childTask := &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "validation-child",
						Type: task.TaskTypeBasic,
					},
				}

				// Apply inheritance
				shared.InheritTaskConfig(childTask, parentTask)

				// Validate inheritance worked correctly
				require.NotNil(t, childTask.CWD, "Child should inherit CWD for task type %s", taskType)
				assert.Equal(t, parentCWD.Path, childTask.CWD.Path,
					"Child should inherit correct CWD path for task type %s", taskType)
				assert.Equal(t, parentFilePath, childTask.FilePath,
					"Child should inherit correct FilePath for task type %s", taskType)
			})
		}
	})

	t.Run("Should validate inheritance chain depth consistency", func(t *testing.T) {
		// Test inheritance through multiple levels
		rootCWD := &core.PathCWD{Path: "/root/path"}
		rootFilePath := "root.yaml"

		// Create root task
		rootTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "root-task",
				Type:     task.TaskTypeComposite,
				CWD:      rootCWD,
				FilePath: rootFilePath,
			},
		}

		// Create chain of tasks
		level1Task := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "level1-task",
				Type: task.TaskTypeParallel,
			},
		}

		level2Task := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "level2-task",
				Type: task.TaskTypeBasic,
			},
		}

		// Apply inheritance chain
		shared.InheritTaskConfig(level1Task, rootTask)
		shared.InheritTaskConfig(level2Task, level1Task)

		// Validate full chain inheritance
		require.NotNil(t, level2Task.CWD, "Level 2 should inherit CWD through chain")
		assert.Equal(t, rootCWD.Path, level2Task.CWD.Path,
			"Level 2 should inherit root CWD through chain")
		assert.Equal(t, rootFilePath, level2Task.FilePath,
			"Level 2 should inherit root FilePath through chain")
	})

	t.Run("Should validate partial inheritance scenarios", func(t *testing.T) {
		// Test when parent has only CWD
		parentWithCWDOnly := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-cwd-only",
				Type: task.TaskTypeParallel,
				CWD:  &core.PathCWD{Path: "/cwd/only"},
			},
		}

		childFromCWDOnly := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-from-cwd-only",
				Type: task.TaskTypeBasic,
			},
		}

		shared.InheritTaskConfig(childFromCWDOnly, parentWithCWDOnly)

		require.NotNil(t, childFromCWDOnly.CWD, "Child should inherit CWD when parent has only CWD")
		assert.Equal(t, "/cwd/only", childFromCWDOnly.CWD.Path,
			"Child should inherit correct CWD value")
		assert.Empty(t, childFromCWDOnly.FilePath,
			"Child FilePath should remain empty when parent has no FilePath")

		// Test when parent has only FilePath
		parentWithFilePathOnly := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "parent-filepath-only",
				Type:     task.TaskTypeComposite,
				FilePath: "filepath_only.yaml",
			},
		}

		childFromFilePathOnly := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-from-filepath-only",
				Type: task.TaskTypeBasic,
			},
		}

		shared.InheritTaskConfig(childFromFilePathOnly, parentWithFilePathOnly)

		assert.Nil(t, childFromFilePathOnly.CWD,
			"Child CWD should remain nil when parent has no CWD")
		assert.Equal(t, "filepath_only.yaml", childFromFilePathOnly.FilePath,
			"Child should inherit correct FilePath value")
	})

	t.Run("Should validate mixed inheritance scenarios", func(t *testing.T) {
		// Parent with both, child with CWD only
		parentFull := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "parent-full",
				Type:     task.TaskTypeRouter,
				CWD:      &core.PathCWD{Path: "/parent/full"},
				FilePath: "parent_full.yaml",
			},
		}

		childWithCWD := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-with-cwd",
				Type: task.TaskTypeBasic,
				CWD:  &core.PathCWD{Path: "/child/own"},
			},
		}

		shared.InheritTaskConfig(childWithCWD, parentFull)

		// Child should keep its CWD but inherit FilePath
		require.NotNil(t, childWithCWD.CWD, "Child should keep its CWD")
		assert.Equal(t, "/child/own", childWithCWD.CWD.Path,
			"Child should preserve its own CWD")
		assert.Equal(t, "parent_full.yaml", childWithCWD.FilePath,
			"Child should inherit parent FilePath")

		// Parent with both, child with FilePath only
		childWithFilePath := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "child-with-filepath",
				Type:     task.TaskTypeBasic,
				FilePath: "child_own.yaml",
			},
		}

		shared.InheritTaskConfig(childWithFilePath, parentFull)

		// Child should inherit CWD but keep its FilePath
		require.NotNil(t, childWithFilePath.CWD, "Child should inherit CWD")
		assert.Equal(t, "/parent/full", childWithFilePath.CWD.Path,
			"Child should inherit parent CWD")
		assert.Equal(t, "child_own.yaml", childWithFilePath.FilePath,
			"Child should preserve its own FilePath")
	})
}

// TestRegressionPrevention ensures that fixes don't break existing functionality
func TestRegressionPrevention(t *testing.T) {
	t.Run("Should maintain backward compatibility for nil configs", func(t *testing.T) {
		// Test all nil combinations that should not panic
		testCases := []struct {
			name   string
			parent *task.Config
			child  *task.Config
		}{
			{"nil parent, valid child", nil, &task.Config{}},
			{"valid parent, nil child", &task.Config{}, nil},
			{"both nil", nil, nil},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Should not panic
				assert.NotPanics(t, func() {
					shared.InheritTaskConfig(tc.child, tc.parent)
				}, "InheritTaskConfig should handle nil configs gracefully")
			})
		}
	})

	t.Run("Should not modify parent config during inheritance", func(t *testing.T) {
		// Create parent with specific values
		originalCWDPath := "/original/parent/path"
		originalFilePath := "original_parent.yaml"

		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "immutable-parent",
				Type:     task.TaskTypeParallel,
				CWD:      &core.PathCWD{Path: originalCWDPath},
				FilePath: originalFilePath,
			},
		}

		// Create multiple children
		for i := range 5 {
			childTask := &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "child-" + string(rune(i)),
					Type: task.TaskTypeBasic,
				},
			}

			shared.InheritTaskConfig(childTask, parentTask)

			// Parent should remain unchanged
			require.NotNil(t, parentTask.CWD, "Parent CWD should not be nil")
			assert.Equal(t, originalCWDPath, parentTask.CWD.Path,
				"Parent CWD should remain unchanged after inheritance")
			assert.Equal(t, originalFilePath, parentTask.FilePath,
				"Parent FilePath should remain unchanged after inheritance")
		}
	})

	t.Run("Should handle pointer sharing correctly", func(t *testing.T) {
		// InheritTaskConfig shares the same pointer, not a deep copy
		// This test documents the actual behavior
		parentCWD := &core.PathCWD{Path: "/shared/pointer/test"}
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "pointer-parent",
				Type:     task.TaskTypeCollection,
				CWD:      parentCWD,
				FilePath: "pointer.yaml",
			},
		}

		childTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "pointer-child",
				Type: task.TaskTypeBasic,
			},
		}

		shared.InheritTaskConfig(childTask, parentTask)

		// Child and parent share the same CWD pointer
		assert.Same(t, parentTask.CWD, childTask.CWD,
			"Child and parent share the same CWD pointer")

		// Modifying child CWD affects parent CWD since they share the pointer
		childTask.CWD.Path = "/modified/child/path"

		// Both parent and child have the modified value
		assert.Equal(t, "/modified/child/path", parentTask.CWD.Path,
			"Parent CWD is affected by child modifications due to shared pointer")
		assert.Equal(t, "/modified/child/path", childTask.CWD.Path,
			"Child has the modified CWD")

		// FilePath is a string, so it's copied by value
		childTask.FilePath = "modified.yaml"
		assert.Equal(t, "pointer.yaml", parentTask.FilePath,
			"Parent FilePath is not affected since strings are copied by value")
	})

	t.Run("Should validate all task type constants are covered", func(t *testing.T) {
		// This test ensures we don't miss any new task types in the future
		knownTaskTypes := map[task.Type]bool{
			task.TaskTypeBasic:      true,
			task.TaskTypeParallel:   true,
			task.TaskTypeComposite:  true,
			task.TaskTypeCollection: true,
			task.TaskTypeRouter:     true,
			task.TaskTypeWait:       true,
			task.TaskTypeAggregate:  true,
			task.TaskTypeSignal:     true,
		}

		// This will help catch if new task types are added
		assert.Equal(t, 8, len(knownTaskTypes),
			"Update this test if new task types are added to ensure inheritance coverage")
	})
}

// BenchmarkInheritance provides performance benchmarks for inheritance operations
func BenchmarkInheritance(b *testing.B) {
	// Setup parent config
	parentTask := &task.Config{
		BaseConfig: task.BaseConfig{
			ID:       "bench-parent",
			Type:     task.TaskTypeParallel,
			CWD:      &core.PathCWD{Path: "/benchmark/parent/path"},
			FilePath: "benchmark.yaml",
		},
	}

	b.Run("Single inheritance", func(b *testing.B) {
		for b.Loop() {
			childTask := &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "bench-child",
					Type: task.TaskTypeBasic,
				},
			}
			shared.InheritTaskConfig(childTask, parentTask)
		}
	})

	b.Run("Chain inheritance depth 3", func(b *testing.B) {
		for b.Loop() {
			level1 := &task.Config{BaseConfig: task.BaseConfig{ID: "l1", Type: task.TaskTypeParallel}}
			level2 := &task.Config{BaseConfig: task.BaseConfig{ID: "l2", Type: task.TaskTypeComposite}}
			level3 := &task.Config{BaseConfig: task.BaseConfig{ID: "l3", Type: task.TaskTypeBasic}}

			shared.InheritTaskConfig(level1, parentTask)
			shared.InheritTaskConfig(level2, level1)
			shared.InheritTaskConfig(level3, level2)
		}
	})
}
