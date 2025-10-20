package core

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testConfigStore implements ConfigStore interface for testing
type testConfigStore struct {
	mu       sync.RWMutex
	configs  map[string]*task.Config
	metadata map[string][]byte
}

func newTestConfigStore() *testConfigStore {
	return &testConfigStore{
		configs:  make(map[string]*task.Config),
		metadata: make(map[string][]byte),
	}
}

func (s *testConfigStore) Save(_ context.Context, taskExecID string, config *task.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs[taskExecID] = config
	return nil
}

func (s *testConfigStore) Get(_ context.Context, taskExecID string) (*task.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	config, exists := s.configs[taskExecID]
	if !exists {
		return nil, fmt.Errorf("config not found for taskExecID %s", taskExecID)
	}
	return config, nil
}

func (s *testConfigStore) Delete(_ context.Context, taskExecID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.configs, taskExecID)
	return nil
}

func (s *testConfigStore) SaveMetadata(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[key] = data
	return nil
}

func (s *testConfigStore) GetMetadata(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, exists := s.metadata[key]
	if !exists {
		return nil, fmt.Errorf("metadata not found for key %s", key)
	}
	return data, nil
}

func (s *testConfigStore) DeleteMetadata(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.metadata, key)
	return nil
}

func (s *testConfigStore) Close() error {
	return nil
}

func createValidTaskConfig(id string, taskType task.Type) *task.Config {
	cwd, _ := core.CWDFromPath(".")
	return &task.Config{
		BaseConfig: task.BaseConfig{
			ID:   id,
			Type: taskType,
			CWD:  cwd,
		},
	}
}

func TestTaskConfigRepository_Interface_Compliance(t *testing.T) {
	t.Run("Should implement TaskConfigRepository interface", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		assert.NotNil(t, repo)
	})
}

func TestTaskConfigRepository_Constructor(t *testing.T) {
	t.Run("Should create repository with valid dependencies", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")

		repo := NewTaskConfigRepository(testStore, cwd)

		assert.NotNil(t, repo)
		assert.Equal(t, testStore, repo.configStore)
		assert.Equal(t, cwd, repo.cwd)
	})
}

func TestTaskConfigRepository_StoreParallelMetadata(t *testing.T) {
	t.Run("Should store parallel metadata successfully", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()
		metadata := &ParallelTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []*task.Config{createValidTaskConfig("child1", task.TaskTypeBasic)},
			Strategy:      "wait_all",
			MaxWorkers:    4,
		}

		err := repo.StoreParallelMetadata(t.Context(), parentStateID, metadata)

		assert.NoError(t, err)
	})

	t.Run("Should return error for empty parent state ID", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		metadata := &ParallelTaskMetadata{}

		err := repo.StoreParallelMetadata(t.Context(), "", metadata)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent state ID cannot be empty")
	})

	t.Run("Should return error for nil metadata", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()

		err := repo.StoreParallelMetadata(t.Context(), parentStateID, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parallel metadata cannot be nil")
	})
}

func TestTaskConfigRepository_LoadParallelMetadata(t *testing.T) {
	t.Run("Should load parallel metadata successfully", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()
		originalMetadata := &ParallelTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []*task.Config{createValidTaskConfig("child1", task.TaskTypeBasic)},
			Strategy:      "wait_all",
			MaxWorkers:    4,
		}

		// Store the metadata first
		err := repo.StoreParallelMetadata(t.Context(), parentStateID, originalMetadata)
		require.NoError(t, err)

		// Load it back
		result, err := repo.LoadParallelMetadata(t.Context(), parentStateID)

		assert.NoError(t, err)

		// Type assert the result
		parallelResult, ok := result.(*ParallelTaskMetadata)
		require.True(t, ok, "Result should be *ParallelTaskMetadata")
		assert.Equal(t, originalMetadata.ParentStateID, parallelResult.ParentStateID)
		assert.Equal(t, originalMetadata.Strategy, parallelResult.Strategy)
		assert.Equal(t, originalMetadata.MaxWorkers, parallelResult.MaxWorkers)
	})

	t.Run("Should return error for empty parent state ID", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		result, err := repo.LoadParallelMetadata(t.Context(), "")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "parent state ID cannot be empty")
	})

	t.Run("Should return error for non-existent metadata", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()

		result, err := repo.LoadParallelMetadata(t.Context(), parentStateID)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get parallel task metadata")
	})
}

func TestTaskConfigRepository_StoreCollectionMetadata(t *testing.T) {
	t.Run("Should store collection metadata successfully", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()
		metadata := &CollectionTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []*task.Config{createValidTaskConfig("child1", task.TaskTypeBasic)},
			Strategy:      "wait_all",
			MaxWorkers:    10,
			ItemCount:     5,
			SkippedCount:  1,
			Mode:          "parallel",
			BatchSize:     2,
		}

		err := repo.StoreCollectionMetadata(t.Context(), parentStateID, metadata)

		assert.NoError(t, err)
	})

	t.Run("Should return error for empty parent state ID", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		metadata := &CollectionTaskMetadata{}

		err := repo.StoreCollectionMetadata(t.Context(), "", metadata)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent state ID cannot be empty")
	})

	t.Run("Should return error for nil metadata", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()

		err := repo.StoreCollectionMetadata(t.Context(), parentStateID, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection metadata cannot be nil")
	})
}

func TestTaskConfigRepository_LoadCollectionMetadata(t *testing.T) {
	t.Run("Should load collection metadata successfully", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()
		originalMetadata := &CollectionTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []*task.Config{createValidTaskConfig("child1", task.TaskTypeBasic)},
			Strategy:      "wait_all",
			MaxWorkers:    10,
			ItemCount:     5,
			SkippedCount:  1,
			Mode:          "parallel",
			BatchSize:     2,
		}

		// Store the metadata first
		err := repo.StoreCollectionMetadata(t.Context(), parentStateID, originalMetadata)
		require.NoError(t, err)

		// Load it back
		result, err := repo.LoadCollectionMetadata(t.Context(), parentStateID)

		assert.NoError(t, err)

		// Type assert the result
		collectionResult, ok := result.(*CollectionTaskMetadata)
		require.True(t, ok, "Result should be *CollectionTaskMetadata")
		assert.Equal(t, originalMetadata.ParentStateID, collectionResult.ParentStateID)
		assert.Equal(t, originalMetadata.ItemCount, collectionResult.ItemCount)
		assert.Equal(t, originalMetadata.Mode, collectionResult.Mode)
		assert.Equal(t, originalMetadata.BatchSize, collectionResult.BatchSize)
		assert.Equal(t, originalMetadata.Strategy, collectionResult.Strategy)
		assert.Equal(t, originalMetadata.MaxWorkers, collectionResult.MaxWorkers)
	})

	t.Run("Should return error for empty parent state ID", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		result, err := repo.LoadCollectionMetadata(t.Context(), "")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "parent state ID cannot be empty")
	})

	t.Run("Should return error for non-existent metadata", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()

		result, err := repo.LoadCollectionMetadata(t.Context(), parentStateID)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get collection task metadata")
	})
}

func TestTaskConfigRepository_StoreCompositeMetadata(t *testing.T) {
	t.Run("Should store composite metadata successfully", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()
		metadata := &CompositeTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []*task.Config{createValidTaskConfig("child1", task.TaskTypeBasic)},
			Strategy:      "wait_all",
			MaxWorkers:    1,
		}

		err := repo.StoreCompositeMetadata(t.Context(), parentStateID, metadata)

		assert.NoError(t, err)
	})

	t.Run("Should return error for empty parent state ID", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		metadata := &CompositeTaskMetadata{}

		err := repo.StoreCompositeMetadata(t.Context(), "", metadata)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent state ID cannot be empty")
	})

	t.Run("Should return error for nil metadata", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()

		err := repo.StoreCompositeMetadata(t.Context(), parentStateID, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "composite metadata cannot be nil")
	})
}

func TestTaskConfigRepository_LoadCompositeMetadata(t *testing.T) {
	t.Run("Should load composite metadata successfully", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()
		originalMetadata := &CompositeTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []*task.Config{createValidTaskConfig("child1", task.TaskTypeBasic)},
			Strategy:      "wait_all",
			MaxWorkers:    1,
		}

		// Store the metadata first
		err := repo.StoreCompositeMetadata(t.Context(), parentStateID, originalMetadata)
		require.NoError(t, err)

		// Load it back
		result, err := repo.LoadCompositeMetadata(t.Context(), parentStateID)

		assert.NoError(t, err)

		// Type assert the result
		compositeResult, ok := result.(*CompositeTaskMetadata)
		require.True(t, ok, "Result should be *CompositeTaskMetadata")
		assert.Equal(t, originalMetadata.ParentStateID, compositeResult.ParentStateID)
		assert.Equal(t, originalMetadata.Strategy, compositeResult.Strategy)
		assert.Equal(t, originalMetadata.MaxWorkers, compositeResult.MaxWorkers)
	})

	t.Run("Should return error for empty parent state ID", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		result, err := repo.LoadCompositeMetadata(t.Context(), "")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "parent state ID cannot be empty")
	})

	t.Run("Should return error for non-existent metadata", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()

		result, err := repo.LoadCompositeMetadata(t.Context(), parentStateID)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get composite task metadata")
	})
}

func TestTaskConfigRepository_SaveTaskConfig(t *testing.T) {
	t.Run("Should save task config successfully", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		taskExecID := "task_exec_123"
		config := createValidTaskConfig("task1", task.TaskTypeBasic)

		err := repo.SaveTaskConfig(t.Context(), taskExecID, config)

		assert.NoError(t, err)
	})

	t.Run("Should return error for empty task execution ID", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		config := createValidTaskConfig("task1", task.TaskTypeBasic)

		err := repo.SaveTaskConfig(t.Context(), "", config)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task execution ID cannot be empty")
	})

	t.Run("Should return error for nil config", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		err := repo.SaveTaskConfig(t.Context(), "task_exec_123", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task config cannot be nil")
	})
}

func TestTaskConfigRepository_GetTaskConfig(t *testing.T) {
	t.Run("Should get task config successfully", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		taskExecID := "task_exec_123"
		originalConfig := createValidTaskConfig("task1", task.TaskTypeBasic)

		// Save the config first
		err := repo.SaveTaskConfig(t.Context(), taskExecID, originalConfig)
		require.NoError(t, err)

		// Get it back
		result, err := repo.GetTaskConfig(t.Context(), taskExecID)

		assert.NoError(t, err)
		assert.Equal(t, originalConfig.ID, result.ID)
		assert.Equal(t, originalConfig.Type, result.Type)
	})

	t.Run("Should return error for empty task execution ID", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		result, err := repo.GetTaskConfig(t.Context(), "")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "task execution ID cannot be empty")
	})

	t.Run("Should return error for non-existent config", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		taskExecID := "non_existent_task"

		result, err := repo.GetTaskConfig(t.Context(), taskExecID)

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestTaskConfigRepository_DeleteTaskConfig(t *testing.T) {
	t.Run("Should delete task config successfully", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		taskExecID := "task_exec_123"
		config := createValidTaskConfig("task1", task.TaskTypeBasic)

		// Save the config first
		err := repo.SaveTaskConfig(t.Context(), taskExecID, config)
		require.NoError(t, err)

		// Delete it
		err = repo.DeleteTaskConfig(t.Context(), taskExecID)

		assert.NoError(t, err)

		// Verify it's gone
		result, err := repo.GetTaskConfig(t.Context(), taskExecID)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should return error for empty task execution ID", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		err := repo.DeleteTaskConfig(t.Context(), "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task execution ID cannot be empty")
	})
}

func TestTaskConfigRepository_ExtractParallelStrategy(t *testing.T) {
	t.Run("Should extract strategy from map format", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		input := core.Input{
			"parallel_config": map[string]any{
				"strategy":    "fail_fast",
				"max_workers": 8,
			},
		}

		parentState := &task.State{Input: &input}

		strategy, err := repo.ExtractParallelStrategy(t.Context(), parentState)

		assert.NoError(t, err)
		assert.Equal(t, task.StrategyFailFast, strategy)
	})

	t.Run("Should extract strategy from JSON string format", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		input := core.Input{
			"parallel_config": `{"strategy": "best_effort", "max_workers": 4}`,
		}

		parentState := &task.State{Input: &input}

		strategy, err := repo.ExtractParallelStrategy(t.Context(), parentState)

		assert.NoError(t, err)
		assert.Equal(t, task.StrategyBestEffort, strategy)
	})

	t.Run("Should return default strategy for nil state", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		strategy, err := repo.ExtractParallelStrategy(t.Context(), nil)

		assert.NoError(t, err)
		assert.Equal(t, task.StrategyWaitAll, strategy)
	})

	t.Run("Should return default strategy for nil input", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentState := &task.State{Input: nil}

		strategy, err := repo.ExtractParallelStrategy(t.Context(), parentState)

		assert.NoError(t, err)
		assert.Equal(t, task.StrategyWaitAll, strategy)
	})

	t.Run("Should return default strategy for invalid strategy", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		input := core.Input{
			"parallel_config": map[string]any{
				"strategy": "invalid_strategy",
			},
		}

		parentState := &task.State{Input: &input}

		strategy, err := repo.ExtractParallelStrategy(t.Context(), parentState)

		assert.NoError(t, err)
		assert.Equal(t, task.StrategyWaitAll, strategy)
	})
}

func TestTaskConfigRepository_ValidateStrategy(t *testing.T) {
	testStore := newTestConfigStore()
	defer testStore.Close()
	cwd, _ := core.CWDFromPath(".")
	repo := NewTaskConfigRepository(testStore, cwd)

	testCases := []struct {
		name        string
		strategy    string
		expected    task.ParallelStrategy
		shouldError bool
	}{
		{"Should validate wait_all strategy", "wait_all", task.StrategyWaitAll, false},
		{"Should validate fail_fast strategy", "fail_fast", task.StrategyFailFast, false},
		{"Should validate best_effort strategy", "best_effort", task.StrategyBestEffort, false},
		{"Should validate race strategy", "race", task.StrategyRace, false},
		{"Should reject invalid strategy", "invalid", "", true},
		{"Should reject empty strategy", "", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := repo.ValidateStrategy(tc.strategy)

			if tc.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid parallel strategy")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestTaskConfigRepository_CalculateMaxWorkers(t *testing.T) {
	testStore := newTestConfigStore()
	defer testStore.Close()
	cwd, _ := core.CWDFromPath(".")
	repo := NewTaskConfigRepository(testStore, cwd)

	testCases := []struct {
		name       string
		taskType   task.Type
		maxWorkers int
		expected   int
	}{
		{"Should default collection workers to 10", task.TaskTypeCollection, 0, 10},
		{"Should use provided collection workers", task.TaskTypeCollection, 20, 20},
		{"Should default parallel workers to 4", task.TaskTypeParallel, 0, 4},
		{"Should use provided parallel workers", task.TaskTypeParallel, 8, 8},
		{"Should limit composite to 1 worker", task.TaskTypeComposite, 10, 1},
		{"Should limit basic to 1 worker", task.TaskTypeBasic, 5, 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := repo.CalculateMaxWorkers(tc.taskType, tc.maxWorkers)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTaskConfigRepository_CWDPropagation(t *testing.T) {
	t.Run("Should propagate CWD to child configs without CWD", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath("/test/path")
		repo := NewTaskConfigRepository(testStore, cwd)

		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child1",
				Type: task.TaskTypeBasic,
				CWD:  nil, // No CWD set
			},
		}

		childConfigs := []*task.Config{childConfig}

		repo.propagateCWDToChildren(childConfigs)

		assert.Equal(t, cwd, childConfig.CWD)
	})

	t.Run("Should not override existing CWD in child configs", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath("/test/path")
		repo := NewTaskConfigRepository(testStore, cwd)

		existingCWD, _ := core.CWDFromPath("/existing/path")
		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child1",
				Type: task.TaskTypeBasic,
				CWD:  existingCWD,
			},
		}

		childConfigs := []*task.Config{childConfig}

		repo.propagateCWDToChildren(childConfigs)

		assert.Equal(t, existingCWD, childConfig.CWD)
		assert.NotEqual(t, cwd, childConfig.CWD)
	})

	t.Run("Should handle nil CWD in repository", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		repo := NewTaskConfigRepository(testStore, nil)

		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child1",
				Type: task.TaskTypeBasic,
				CWD:  nil,
			},
		}

		childConfigs := []*task.Config{childConfig}

		repo.propagateCWDToChildren(childConfigs)

		assert.Nil(t, childConfig.CWD)
	})
}

func TestTaskConfigRepository_KeyGeneration(t *testing.T) {
	testStore := newTestConfigStore()
	defer testStore.Close()
	cwd, _ := core.CWDFromPath(".")
	repo := NewTaskConfigRepository(testStore, cwd)

	parentStateID := core.ID("test-state-123")

	t.Run("Should generate consistent parallel metadata key", func(t *testing.T) {
		key := repo.buildParallelMetadataKey(parentStateID)
		expected := "parallel_metadata:test-state-123"
		assert.Equal(t, expected, key)
	})

	t.Run("Should generate consistent collection metadata key", func(t *testing.T) {
		key := repo.buildCollectionMetadataKey(parentStateID)
		expected := "collection_metadata:test-state-123"
		assert.Equal(t, expected, key)
	})

	t.Run("Should generate consistent composite metadata key", func(t *testing.T) {
		key := repo.buildCompositeMetadataKey(parentStateID)
		expected := "composite_metadata:test-state-123"
		assert.Equal(t, expected, key)
	})
}

func TestTaskConfigRepository_IntegrationTests(t *testing.T) {
	t.Run("Should complete full parallel metadata cycle", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()
		originalMetadata := &ParallelTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []*task.Config{createValidTaskConfig("child1", task.TaskTypeBasic)},
			Strategy:      "wait_all",
			MaxWorkers:    4,
		}

		// Store phase
		err := repo.StoreParallelMetadata(t.Context(), parentStateID, originalMetadata)
		require.NoError(t, err)

		// Load phase
		loadedMetadata, err := repo.LoadParallelMetadata(t.Context(), parentStateID)
		require.NoError(t, err)

		// Verify data integrity - type assert first
		parallelLoadedMetadata, ok := loadedMetadata.(*ParallelTaskMetadata)
		require.True(t, ok, "Loaded metadata should be *ParallelTaskMetadata")
		assert.Equal(t, originalMetadata.ParentStateID, parallelLoadedMetadata.ParentStateID)
		assert.Equal(t, originalMetadata.Strategy, parallelLoadedMetadata.Strategy)
		assert.Equal(t, originalMetadata.MaxWorkers, parallelLoadedMetadata.MaxWorkers)
		assert.Len(t, parallelLoadedMetadata.ChildConfigs, 1)
	})

	t.Run("Should complete full task config CRUD cycle", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		taskExecID := "task_exec_integration_test"
		originalConfig := createValidTaskConfig("integration_task", task.TaskTypeBasic)

		// Save phase
		err := repo.SaveTaskConfig(t.Context(), taskExecID, originalConfig)
		require.NoError(t, err)

		// Get phase
		loadedConfig, err := repo.GetTaskConfig(t.Context(), taskExecID)
		require.NoError(t, err)
		assert.Equal(t, originalConfig.ID, loadedConfig.ID)
		assert.Equal(t, originalConfig.Type, loadedConfig.Type)

		// Delete phase
		err = repo.DeleteTaskConfig(t.Context(), taskExecID)
		require.NoError(t, err)

		// Verify deletion
		result, err := repo.GetTaskConfig(t.Context(), taskExecID)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should complete full collection metadata cycle", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()
		originalMetadata := &CollectionTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []*task.Config{createValidTaskConfig("child1", task.TaskTypeBasic)},
			Strategy:      "best_effort",
			MaxWorkers:    5,
			ItemCount:     10,
			SkippedCount:  2,
			Mode:          "sequential",
			BatchSize:     3,
		}

		// Store and load
		err := repo.StoreCollectionMetadata(t.Context(), parentStateID, originalMetadata)
		require.NoError(t, err)

		loadedMetadata, err := repo.LoadCollectionMetadata(t.Context(), parentStateID)
		require.NoError(t, err)

		// Verify data integrity
		collectionLoadedMetadata, ok := loadedMetadata.(*CollectionTaskMetadata)
		require.True(t, ok, "Loaded metadata should be *CollectionTaskMetadata")
		assert.Equal(t, originalMetadata.ParentStateID, collectionLoadedMetadata.ParentStateID)
		assert.Equal(t, originalMetadata.Strategy, collectionLoadedMetadata.Strategy)
		assert.Equal(t, originalMetadata.MaxWorkers, collectionLoadedMetadata.MaxWorkers)
		assert.Equal(t, originalMetadata.ItemCount, collectionLoadedMetadata.ItemCount)
		assert.Equal(t, originalMetadata.Mode, collectionLoadedMetadata.Mode)
		assert.Equal(t, originalMetadata.BatchSize, collectionLoadedMetadata.BatchSize)
	})

	t.Run("Should complete full composite metadata cycle", func(t *testing.T) {
		testStore := newTestConfigStore()
		defer testStore.Close()
		cwd, _ := core.CWDFromPath(".")
		repo := NewTaskConfigRepository(testStore, cwd)

		parentStateID := core.MustNewID()
		originalMetadata := &CompositeTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []*task.Config{createValidTaskConfig("child1", task.TaskTypeBasic)},
			Strategy:      "race",
			MaxWorkers:    1,
		}

		// Store and load
		err := repo.StoreCompositeMetadata(t.Context(), parentStateID, originalMetadata)
		require.NoError(t, err)

		loadedMetadata, err := repo.LoadCompositeMetadata(t.Context(), parentStateID)
		require.NoError(t, err)

		// Verify data integrity
		compositeLoadedMetadata, ok := loadedMetadata.(*CompositeTaskMetadata)
		require.True(t, ok, "Loaded metadata should be *CompositeTaskMetadata")
		assert.Equal(t, originalMetadata.ParentStateID, compositeLoadedMetadata.ParentStateID)
		assert.Equal(t, originalMetadata.Strategy, compositeLoadedMetadata.Strategy)
		assert.Equal(t, originalMetadata.MaxWorkers, compositeLoadedMetadata.MaxWorkers)
	})
}
