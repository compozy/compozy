package shared_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// TestTaskContextInheritance provides comprehensive testing for context inheritance
// across all task types in the engine/task2 system
func TestTaskContextInheritance(t *testing.T) {
	testCases := []struct {
		name           string
		taskType       task.Type
		parentCWD      string
		parentFilePath string
		childCWD       string
		childFilePath  string
		expectInherit  bool
		description    string
	}{
		{
			name:           "InheritTaskConfig inherits CWD when child has none (parallel type)",
			taskType:       task.TaskTypeParallel,
			parentCWD:      "/custom/parallel/path",
			parentFilePath: "parallel.yaml",
			childCWD:       "",
			childFilePath:  "",
			expectInherit:  true,
			description:    "InheritTaskConfig should copy parent CWD and FilePath to child",
		},
		{
			name:           "InheritTaskConfig inherits FilePath when child has none (composite type)",
			taskType:       task.TaskTypeComposite,
			parentCWD:      "/work/composite/dir",
			parentFilePath: "composite.yml",
			childCWD:       "",
			childFilePath:  "",
			expectInherit:  true,
			description:    "InheritTaskConfig should copy parent CWD and FilePath to child",
		},
		{
			name:           "InheritTaskConfig copies context from parent to child (router type)",
			taskType:       task.TaskTypeRouter,
			parentCWD:      "/router/base/path",
			parentFilePath: "routes.yaml",
			childCWD:       "",
			childFilePath:  "",
			expectInherit:  true,
			description:    "InheritTaskConfig should copy parent context to child task",
		},
		{
			name:           "InheritTaskConfig copies context from parent to child (wait type)",
			taskType:       task.TaskTypeWait,
			parentCWD:      "/wait/context/dir",
			parentFilePath: "processor.yaml",
			childCWD:       "",
			childFilePath:  "",
			expectInherit:  true,
			description:    "InheritTaskConfig should copy parent context to child task",
		},
		{
			name:           "InheritTaskConfig copies context from parent to child (collection type)",
			taskType:       task.TaskTypeCollection,
			parentCWD:      "/collection/base/dir",
			parentFilePath: "items.yaml",
			childCWD:       "",
			childFilePath:  "",
			expectInherit:  true,
			description:    "InheritTaskConfig should copy parent context to child task",
		},
		{
			name:           "InheritTaskConfig copies context from parent to child (aggregate type)",
			taskType:       task.TaskTypeAggregate,
			parentCWD:      "/agg/parent/path",
			parentFilePath: "agg.yaml",
			childCWD:       "",
			childFilePath:  "",
			expectInherit:  true,
			description:    "InheritTaskConfig should copy parent context to child task",
		},
		{
			name:           "InheritTaskConfig copies context from parent to child (signal type)",
			taskType:       task.TaskTypeSignal,
			parentCWD:      "/signal/parent/dir",
			parentFilePath: "signal.yaml",
			childCWD:       "",
			childFilePath:  "",
			expectInherit:  true,
			description:    "InheritTaskConfig should copy parent context to child task",
		},
		{
			name:           "Child CWD is preserved when explicitly set",
			taskType:       task.TaskTypeParallel,
			parentCWD:      "/parent/path",
			parentFilePath: "parent.yaml",
			childCWD:       "/explicit/child/path",
			childFilePath:  "",
			expectInherit:  false,
			description:    "Child task with explicit CWD should not inherit parent CWD",
		},
		{
			name:           "Child FilePath is preserved when explicitly set",
			taskType:       task.TaskTypeComposite,
			parentCWD:      "/parent/dir",
			parentFilePath: "parent.yml",
			childCWD:       "",
			childFilePath:  "explicit_child.yaml",
			expectInherit:  false,
			description:    "Child task with explicit FilePath should not inherit parent FilePath",
		},
		{
			name:           "Both CWD and FilePath are preserved when explicitly set",
			taskType:       task.TaskTypeCollection,
			parentCWD:      "/parent/base",
			parentFilePath: "parent.config",
			childCWD:       "/child/explicit",
			childFilePath:  "child.config",
			expectInherit:  false,
			description:    "Child task with explicit CWD and FilePath should not inherit from parent",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create parent task with CWD and FilePath
			parentTask := &task.Config{
				BaseConfig: task.BaseConfig{
					ID:       "parent-task",
					Type:     tc.taskType,
					FilePath: tc.parentFilePath,
				},
			}

			// Set parent CWD if provided
			if tc.parentCWD != "" {
				parentTask.CWD = &core.PathCWD{Path: tc.parentCWD}
			}

			// Create child task with or without explicit CWD/FilePath
			childTask := &task.Config{
				BaseConfig: task.BaseConfig{
					ID:       "child-task",
					Type:     task.TaskTypeBasic,
					FilePath: tc.childFilePath,
				},
			}

			// Set child CWD if provided
			if tc.childCWD != "" {
				childTask.CWD = &core.PathCWD{Path: tc.childCWD}
			}

			// Apply inheritance logic using the shared utility function
			shared.InheritTaskConfig(childTask, parentTask)

			// Verify inheritance behavior based on expectation
			if tc.expectInherit {
				// Child should inherit parent context when not explicitly set
				if tc.childCWD == "" && tc.parentCWD != "" {
					require.NotNil(t, childTask.CWD, "Child task should inherit parent CWD")
					assert.Equal(t, tc.parentCWD, childTask.CWD.Path,
						"Child task should inherit parent CWD: %s", tc.description)
				}
				if tc.childFilePath == "" && tc.parentFilePath != "" {
					assert.Equal(t, tc.parentFilePath, childTask.FilePath,
						"Child task should inherit parent FilePath: %s", tc.description)
				}
			} else {
				// Child should preserve its explicit settings
				if tc.childCWD != "" {
					require.NotNil(t, childTask.CWD, "Child task should preserve explicit CWD")
					assert.Equal(t, tc.childCWD, childTask.CWD.Path,
						"Child task should preserve explicit CWD: %s", tc.description)
				}
				if tc.childFilePath != "" {
					assert.Equal(t, tc.childFilePath, childTask.FilePath,
						"Child task should preserve explicit FilePath: %s", tc.description)
				}
			}
		})
	}
}

// TestInheritTaskConfig_EdgeCases tests edge cases and error conditions
func TestInheritTaskConfig_EdgeCases(t *testing.T) {
	t.Run("Should handle nil parent config gracefully", func(t *testing.T) {
		childTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-task",
				Type: task.TaskTypeBasic,
			},
		}

		// This should not panic or cause errors
		shared.InheritTaskConfig(childTask, nil)

		// Child task should remain unchanged
		assert.Nil(t, childTask.CWD, "Child CWD should remain nil with nil parent")
		assert.Empty(t, childTask.FilePath, "Child FilePath should remain empty with nil parent")
	})

	t.Run("Should handle nil child config gracefully", func(_ *testing.T) {
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "parent-task",
				Type:     task.TaskTypeParallel,
				FilePath: "parent.yaml",
				CWD:      &core.PathCWD{Path: "/parent/path"},
			},
		}

		// This should not panic or cause errors
		shared.InheritTaskConfig(nil, parentTask)
		// No assertions needed - just ensuring no panic
	})

	t.Run("Should handle both nil configs gracefully", func(_ *testing.T) {
		// This should not panic or cause errors
		shared.InheritTaskConfig(nil, nil)
		// No assertions needed - just ensuring no panic
	})

	t.Run("Should handle parent with nil CWD", func(t *testing.T) {
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "parent-task",
				Type:     task.TaskTypeParallel,
				FilePath: "parent.yaml",
				// CWD is nil
			},
		}

		childTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-task",
				Type: task.TaskTypeBasic,
			},
		}

		shared.InheritTaskConfig(childTask, parentTask)

		// Child should inherit FilePath but CWD should remain nil
		assert.Nil(t, childTask.CWD, "Child CWD should remain nil when parent CWD is nil")
		assert.Equal(t, "parent.yaml", childTask.FilePath, "Child should inherit parent FilePath")
	})

	t.Run("Should handle parent with empty FilePath", func(t *testing.T) {
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeParallel,
				// FilePath is empty
				CWD: &core.PathCWD{Path: "/parent/path"},
			},
		}

		childTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-task",
				Type: task.TaskTypeBasic,
			},
		}

		shared.InheritTaskConfig(childTask, parentTask)

		// Child should inherit CWD but FilePath should remain empty
		require.NotNil(t, childTask.CWD, "Child should inherit parent CWD")
		assert.Equal(t, "/parent/path", childTask.CWD.Path, "Child should inherit parent CWD")
		assert.Empty(t, childTask.FilePath, "Child FilePath should remain empty when parent FilePath is empty")
	})
}

// TestInheritTaskConfig_NestedScenarios tests complex nested inheritance scenarios
func TestInheritTaskConfig_NestedScenarios(t *testing.T) {
	t.Run("Should handle three-level inheritance chain", func(t *testing.T) {
		// Level 1: Root task
		rootTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "root-task",
				Type:     task.TaskTypeComposite,
				FilePath: "root.yaml",
				CWD:      &core.PathCWD{Path: "/root/directory"},
			},
		}

		// Level 2: Intermediate task (inherits from root)
		intermediateTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "intermediate-task",
				Type: task.TaskTypeParallel,
			},
		}

		// Level 3: Leaf task (inherits from intermediate)
		leafTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "leaf-task",
				Type: task.TaskTypeBasic,
			},
		}

		// Apply inheritance chain
		shared.InheritTaskConfig(intermediateTask, rootTask)
		shared.InheritTaskConfig(leafTask, intermediateTask)

		// Verify inheritance propagates through the chain
		require.NotNil(t, intermediateTask.CWD, "Intermediate task should inherit root CWD")
		assert.Equal(t, "/root/directory", intermediateTask.CWD.Path, "Intermediate task should inherit root CWD")
		assert.Equal(t, "root.yaml", intermediateTask.FilePath, "Intermediate task should inherit root FilePath")

		require.NotNil(t, leafTask.CWD, "Leaf task should inherit CWD through chain")
		assert.Equal(t, "/root/directory", leafTask.CWD.Path, "Leaf task should inherit CWD through chain")
		assert.Equal(t, "root.yaml", leafTask.FilePath, "Leaf task should inherit FilePath through chain")
	})

	t.Run("Should handle partial inheritance in chain", func(t *testing.T) {
		// Level 1: Root task with only CWD
		rootTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "root-task",
				Type: task.TaskTypeComposite,
				// No FilePath
				CWD: &core.PathCWD{Path: "/root/only/cwd"},
			},
		}

		// Level 2: Intermediate task with only FilePath
		intermediateTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "intermediate-task",
				Type:     task.TaskTypeParallel,
				FilePath: "intermediate.yaml",
			},
			// No CWD
		}

		// Level 3: Leaf task (should inherit both)
		leafTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "leaf-task",
				Type: task.TaskTypeBasic,
			},
		}

		// Apply inheritance chain
		shared.InheritTaskConfig(intermediateTask, rootTask)
		shared.InheritTaskConfig(leafTask, intermediateTask)

		// Intermediate should inherit CWD from root, keep its own FilePath
		require.NotNil(t, intermediateTask.CWD, "Intermediate task should inherit root CWD")
		assert.Equal(t, "/root/only/cwd", intermediateTask.CWD.Path, "Intermediate task should inherit root CWD")
		assert.Equal(t, "intermediate.yaml", intermediateTask.FilePath, "Intermediate task should keep its FilePath")

		// Leaf should inherit both from intermediate
		require.NotNil(t, leafTask.CWD, "Leaf task should inherit CWD")
		assert.Equal(t, "/root/only/cwd", leafTask.CWD.Path, "Leaf task should inherit CWD through chain")
		assert.Equal(t, "intermediate.yaml", leafTask.FilePath, "Leaf task should inherit FilePath from intermediate")
	})
}

// TestInheritTaskConfig_ConcurrentSafety tests thread safety of inheritance function
func TestInheritTaskConfig_ConcurrentSafety(t *testing.T) {
	t.Run("Should handle concurrent inheritance operations safely", func(t *testing.T) {
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "parent-task",
				Type:     task.TaskTypeParallel,
				FilePath: "concurrent.yaml",
				CWD:      &core.PathCWD{Path: "/concurrent/test"},
			},
		}

		// Create multiple child tasks
		numChildren := 10
		childTasks := make([]*task.Config, numChildren)
		for i := 0; i < numChildren; i++ {
			childTasks[i] = &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   fmt.Sprintf("child-task-%d", i),
					Type: task.TaskTypeBasic,
				},
			}
		}

		// Apply inheritance concurrently
		done := make(chan bool, numChildren)
		for i := 0; i < numChildren; i++ {
			go func(childTask *task.Config) {
				defer func() { done <- true }()
				shared.InheritTaskConfig(childTask, parentTask)
			}(childTasks[i])
		}

		// Wait for all goroutines to complete
		for i := 0; i < numChildren; i++ {
			<-done
		}

		// Verify all children inherited correctly
		for i, childTask := range childTasks {
			require.NotNil(t, childTask.CWD, "Child task %d should inherit parent CWD", i)
			assert.Equal(t, "/concurrent/test", childTask.CWD.Path, "Child task %d should inherit parent CWD", i)
			assert.Equal(t, "concurrent.yaml", childTask.FilePath, "Child task %d should inherit parent FilePath", i)
		}
	})
}
