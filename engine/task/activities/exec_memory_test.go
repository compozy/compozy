package activities

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/mocks"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
)

func TestExecuteMemory_Factory(t *testing.T) {
	t.Run("Should create memory normalizer without error", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		envMerger := task2core.NewEnvMerger()
		factory, err := task2.NewFactory(&task2.FactoryConfig{
			TemplateEngine: templateEngine,
			EnvMerger:      envMerger,
		})
		require.NoError(t, err)

		// Act
		normalizer, err := factory.CreateNormalizer(task.TaskTypeMemory)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, normalizer)
	})

	t.Run("Should create memory response handler without error", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		envMerger := task2core.NewEnvMerger()
		factory, err := task2.NewFactory(&task2.FactoryConfig{
			TemplateEngine: templateEngine,
			EnvMerger:      envMerger,
		})
		require.NoError(t, err)

		// Act
		handler, err := factory.CreateResponseHandler(context.Background(), task.TaskTypeMemory)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, handler)
		assert.Equal(t, task.TaskTypeMemory, handler.Type())
	})
}

func TestExecuteMemory_BasicOperations(t *testing.T) {
	t.Run("Should execute memory write operation", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow"
		input := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecIDs[workflowID],
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeMemory,
				},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpWrite,
					MemoryRef:   "test_memory",
					KeyTemplate: "test:{{.workflow.id}}",
					Payload: map[string]any{
						"content": "Hello, Memory!",
						"metadata": map[string]any{
							"timestamp": "2024-01-01T00:00:00Z",
						},
					},
				},
			},
			MergedInput: &core.Input{
				"user_id": "test-user",
			},
		}

		// Act
		response, err := activity.Run(ctx, input)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, core.StatusSuccess, response.State.Status)
		assert.NotNil(t, response.State.Output)
		assert.Equal(t, true, (*response.State.Output)["success"])
	})

	t.Run("Should execute memory read operation", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow-read"
		workflowExecID := workflowExecIDs[workflowID]

		// First write some data
		writeInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeMemory,
				},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpWrite,
					MemoryRef:   "test_memory",
					KeyTemplate: "test:{{.workflow.id}}",
					Payload: map[string]any{
						"content": "Test message for read",
					},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		_, err := activity.Run(ctx, writeInput)
		require.NoError(t, err)

		// Now read it back
		readInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeMemory,
				},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpRead,
					MemoryRef:   "test_memory",
					KeyTemplate: "test:{{.workflow.id}}",
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}

		// Act
		response, err := activity.Run(ctx, readInput)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.NotNil(t, response.State.Output)
		assert.NotNil(t, (*response.State.Output)["messages"])
		messages := (*response.State.Output)["messages"]
		assert.NotNil(t, messages)
	})
}

func TestExecuteMemory_AllOperations(t *testing.T) {
	// Non-stateful operations that don't require side effect validation
	operations := []struct {
		name      string
		operation task.MemoryOpType
		config    map[string]any
		validate  func(t *testing.T, response *task.MainTaskResponse)
	}{
		{
			name:      "flush",
			operation: task.MemoryOpFlush,
			config: map[string]any{
				"flush_config": &task.FlushConfig{
					DryRun: true,
					Force:  false,
				},
			},
			validate: func(t *testing.T, response *task.MainTaskResponse) {
				assert.Equal(t, core.StatusSuccess, response.State.Status)
				assert.Equal(t, true, (*response.State.Output)["success"])
				assert.Equal(t, true, (*response.State.Output)["dry_run"])
			},
		},
		{
			name:      "health",
			operation: task.MemoryOpHealth,
			config: map[string]any{
				"health_config": &task.HealthConfig{
					IncludeStats: true,
				},
			},
			validate: func(t *testing.T, response *task.MainTaskResponse) {
				assert.Equal(t, core.StatusSuccess, response.State.Status)
				assert.NotNil(t, (*response.State.Output)["healthy"])
			},
		},
	}

	for _, tc := range operations {
		t.Run("Should execute "+tc.name+" operation", func(t *testing.T) {
			// Arrange
			activity, workflowExecIDs := createTestMemoryActivity(t)
			workflowID := "test-workflow"
			workflowExecID := workflowExecIDs[workflowID]
			config := &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeMemory,
				},
				MemoryTask: task.MemoryTask{
					Operation:   tc.operation,
					MemoryRef:   "test_memory",
					KeyTemplate: "test:{{.workflow.id}}",
				},
			}

			// Add operation-specific config
			for k, v := range tc.config {
				switch k {
				case "flush_config":
					config.FlushConfig = v.(*task.FlushConfig)
				case "health_config":
					config.HealthConfig = v.(*task.HealthConfig)
				}
			}

			input := &ExecuteMemoryInput{
				WorkflowID:     workflowID,
				WorkflowExecID: workflowExecID,
				TaskConfig:     config,
				MergedInput:    &core.Input{"user_id": "test-user"},
			}

			// Act
			response, err := activity.Run(context.Background(), input)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, response)
			tc.validate(t, response)
		})
	}
}

func TestExecuteMemory_StatefulOperations(t *testing.T) {
	t.Run("Should execute append operation and verify data was added", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow-append"
		workflowExecID := workflowExecIDs[workflowID]
		keyTemplate := "test:{{.workflow.id}}"

		// 1. Write initial data
		writeInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpWrite,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
					Payload:     map[string]any{"content": "initial message"},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		_, err := activity.Run(ctx, writeInput)
		require.NoError(t, err, "Setup: write operation failed")

		// 2. Act: Execute the append operation
		appendInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpAppend,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
					Payload:     map[string]any{"content": "appended message"},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		response, err := activity.Run(ctx, appendInput)

		// 3. Assert: Check append response
		assert.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, core.StatusSuccess, response.State.Status)
		assert.Equal(t, true, (*response.State.Output)["success"])

		// 4. Verify: Read the data and confirm both messages are present
		readInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpRead,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		readResponse, err := activity.Run(ctx, readInput)
		require.NoError(t, err)
		require.NotNil(t, readResponse.State.Output)
		messages := (*readResponse.State.Output)["messages"]
		require.NotNil(t, messages, "messages field should not be nil")
		// Messages field is actually []llm.Message, not []any
		assert.Len(t, messages, 2, "Should have both initial and appended messages")
	})

	t.Run("Should execute delete operation and remove data", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow-delete"
		workflowExecID := workflowExecIDs[workflowID]
		keyTemplate := "test:{{.workflow.id}}"

		// 1. Write data first
		writeInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpWrite,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
					Payload:     map[string]any{"content": "data to be deleted"},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		_, err := activity.Run(ctx, writeInput)
		require.NoError(t, err, "Setup: write operation failed")

		// 2. Act: Execute the delete operation
		deleteInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpDelete,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		response, err := activity.Run(ctx, deleteInput)

		// 3. Assert: Check delete response
		assert.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, core.StatusSuccess, response.State.Status)
		assert.Equal(t, true, (*response.State.Output)["success"])

		// 4. Verify: Try to read the data and confirm it's gone
		readInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpRead,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		readResponse, err := activity.Run(ctx, readInput)
		require.NoError(t, err)
		require.NotNil(t, readResponse.State.Output)
		messages := (*readResponse.State.Output)["messages"]
		require.NotNil(t, messages, "messages field should not be nil")
		// Messages field is actually []llm.Message, not []any
		assert.Len(t, messages, 0, "Data should have been deleted, but was found")
	})

	t.Run("Should execute clear operation and remove all data", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow-clear"
		workflowExecID := workflowExecIDs[workflowID]
		keyTemplate := "test:{{.workflow.id}}"

		// 1. Write some data first
		writeInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpWrite,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
					Payload:     map[string]any{"content": "data to be cleared"},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		_, err := activity.Run(ctx, writeInput)
		require.NoError(t, err, "Setup: write operation failed")

		// 2. Act: Execute the clear operation
		clearInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpClear,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
					ClearConfig: &task.ClearConfig{
						Confirm: true,
						Backup:  false,
					},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		response, err := activity.Run(ctx, clearInput)

		// 3. Assert: Check clear response
		assert.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, core.StatusSuccess, response.State.Status)
		assert.Equal(t, true, (*response.State.Output)["success"])

		// 4. Verify: Try to read the data and confirm it's gone
		readInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpRead,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		readResponse, err := activity.Run(ctx, readInput)
		require.NoError(t, err)
		require.NotNil(t, readResponse.State.Output)
		messages := (*readResponse.State.Output)["messages"]
		require.NotNil(t, messages, "messages field should not be nil")
		// Messages field is actually []llm.Message, not []any
		assert.Len(t, messages, 0, "Data should have been cleared, but was found")
	})

	t.Run("Should execute stats operation and return meaningful statistics", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow-stats"
		workflowExecID := workflowExecIDs[workflowID]
		keyTemplate := "test:{{.workflow.id}}"

		// 1. Write some data first to get meaningful stats
		writeInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpWrite,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
					Payload:     map[string]any{"content": "test data for stats"},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		_, err := activity.Run(ctx, writeInput)
		require.NoError(t, err, "Setup: write operation failed")

		// 2. Act: Execute the stats operation
		statsInput := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpStats,
					MemoryRef:   "test_memory",
					KeyTemplate: keyTemplate,
					StatsConfig: &task.StatsConfig{
						IncludeContent: true,
					},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		response, err := activity.Run(ctx, statsInput)

		// 3. Assert: Check stats response
		assert.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, core.StatusSuccess, response.State.Status)

		// 4. Verify: Check that meaningful statistics are returned
		output := (*response.State.Output)
		assert.NotNil(t, output["message_count"])
		messageCount, ok := output["message_count"].(int)
		if ok {
			assert.Greater(t, messageCount, 0, "Should have at least one message in stats")
		}
	})
}

func TestExecuteMemory_ErrorHandling(t *testing.T) {
	t.Run("Should fail with missing memory_ref", func(t *testing.T) {
		// Arrange
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow"
		workflowExecID := workflowExecIDs[workflowID]
		input := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeMemory,
				},
				MemoryTask: task.MemoryTask{
					Operation: task.MemoryOpWrite,
					// memory_ref missing
					KeyTemplate: "test:key",
					Payload:     map[string]any{"content": "test"},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}

		// Act
		_, err := activity.Run(context.Background(), input)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory configuration error")
	})

	t.Run("Should fail with missing key_template", func(t *testing.T) {
		// Arrange
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow"
		workflowExecID := workflowExecIDs[workflowID]
		input := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeMemory,
				},
				MemoryTask: task.MemoryTask{
					Operation: task.MemoryOpWrite,
					MemoryRef: "test_memory",
					// key_template missing
					Payload: map[string]any{"content": "test"},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}

		// Act
		_, err := activity.Run(context.Background(), input)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key_template")
	})

	t.Run("Should fail with invalid operation", func(t *testing.T) {
		// Arrange
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow"
		workflowExecID := workflowExecIDs[workflowID]
		input := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeMemory,
				},
				MemoryTask: task.MemoryTask{
					Operation:   "invalid_operation",
					MemoryRef:   "test_memory",
					KeyTemplate: "test:key",
					Payload:     map[string]any{"content": "test"},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}

		// Act
		_, err := activity.Run(context.Background(), input)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported memory operation")
	})

	t.Run("Should fail with nil task_config", func(t *testing.T) {
		// Arrange
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow"
		workflowExecID := workflowExecIDs[workflowID]
		input := &ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     nil, // nil config
			MergedInput:    &core.Input{"user_id": "test-user"},
		}

		// Act
		_, err := activity.Run(context.Background(), input)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task_config is required")
	})
}

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
		return nil, nil // Return nil instead of error for this test
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
		return nil, nil // Return nil instead of error for this test
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

// setupTestRedis creates a test Redis instance using miniredis
func setupTestRedis(t *testing.T) *redis.Client {
	t.Helper()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(func() { mr.Close() })

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		DB:   0,
	})
	t.Cleanup(func() { _ = redisClient.Close() })

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = redisClient.Ping(ctx).Err()
	require.NoError(t, err)

	return redisClient
}

// setupLockManager creates a lock manager for testing
func setupLockManager(t *testing.T, redisClient *redis.Client) cache.LockManager {
	t.Helper()

	lockManager, err := cache.NewRedisLockManager(redisClient)
	require.NoError(t, err)

	return lockManager
}

// setupTestConfigRegistry creates and configures a test config registry
func setupTestConfigRegistry(t *testing.T) *autoload.ConfigRegistry {
	t.Helper()

	configRegistry := autoload.NewConfigRegistry()

	// Add test memory config
	testMemoryConfig := &memory.Config{
		Resource:    "memory",
		ID:          "test_memory",
		Type:        memcore.TokenBasedMemory,
		Description: "Test memory for integration tests",
		MaxTokens:   4000,
		MaxMessages: 100,
		Persistence: memcore.PersistenceConfig{
			Type: memcore.RedisPersistence,
			TTL:  "24h",
		},
		Flushing: &memcore.FlushingStrategyConfig{
			Type:               memcore.SimpleFIFOFlushing,
			SummarizeThreshold: 0.8,
		},
	}

	err := testMemoryConfig.Validate()
	require.NoError(t, err)

	err = configRegistry.Register(testMemoryConfig, "test")
	require.NoError(t, err)

	return configRegistry
}

// setupMemoryManager creates a memory manager for testing
func setupMemoryManager(
	t *testing.T,
	redisClient *redis.Client,
	lockManager cache.LockManager,
	configRegistry *autoload.ConfigRegistry,
) *memory.Manager {
	t.Helper()

	log := logger.NewForTests()
	templateEngine := tplengine.NewEngine(tplengine.FormatText)
	mockClient := &mocks.Client{}
	privacyManager := privacy.NewManager()

	memoryManager, err := memory.NewManager(&memory.ManagerOptions{
		ResourceRegistry:  configRegistry,
		TplEngine:         templateEngine,
		BaseLockManager:   lockManager,
		BaseRedisClient:   redisClient,
		TemporalClient:    mockClient,
		TemporalTaskQueue: "test-memory-queue",
		PrivacyManager:    privacyManager,
		Logger:            log,
	})
	require.NoError(t, err)

	return memoryManager
}

// setupTask2Factory creates a Task2 factory for testing
func setupTask2Factory(t *testing.T, workflowRepo *store.WorkflowRepo, taskRepo *store.TaskRepo) task2.Factory {
	t.Helper()

	templateEngine := tplengine.NewEngine(tplengine.FormatText)
	envMerger := task2core.NewEnvMerger()

	task2Factory, err := task2.NewFactory(&task2.FactoryConfig{
		TemplateEngine: templateEngine,
		EnvMerger:      envMerger,
		WorkflowRepo:   workflowRepo,
		TaskRepo:       taskRepo,
	})
	require.NoError(t, err)

	return task2Factory
}

// setupTestWorkflows creates test workflow configs
func setupTestWorkflows() []*workflow.Config {
	return []*workflow.Config{
		{ID: "test-workflow"},
		{ID: "test-workflow-read"},
		{ID: "test-workflow-append"},
		{ID: "test-workflow-delete"},
		{ID: "test-workflow-clear"},
		{ID: "test-workflow-stats"},
	}
}

// setupWorkflowStates creates workflow states in the database for testing
func setupWorkflowStates(
	ctx context.Context,
	t *testing.T,
	workflowRepo *store.WorkflowRepo,
	workflows []*workflow.Config,
) map[string]core.ID {
	t.Helper()

	workflowExecIDs := make(map[string]core.ID)

	for _, wf := range workflows {
		execID := core.MustNewID()
		workflowExecIDs[wf.ID] = execID

		state := &workflow.State{
			WorkflowID:     wf.ID,
			WorkflowExecID: execID,
			Input:          &core.Input{},
			Status:         core.StatusRunning,
		}

		err := workflowRepo.UpsertState(ctx, state)
		require.NoError(t, err, "Failed to create workflow state for %s", wf.ID)
	}

	return workflowExecIDs
}

// createTestMemoryActivity creates a test ExecuteMemory activity with all required dependencies
func createTestMemoryActivity(t *testing.T) (*ExecuteMemory, map[string]core.ID) {
	t.Helper()
	ctx := context.Background()

	// Setup Redis
	redisClient := setupTestRedis(t)

	// Setup lock manager
	lockManager := setupLockManager(t, redisClient)

	// Setup config registry
	configRegistry := setupTestConfigRegistry(t)

	// Setup real repositories instead of mocks
	taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
	t.Cleanup(cleanup)

	// Setup memory manager
	memoryManager := setupMemoryManager(t, redisClient, lockManager, configRegistry)

	// Setup Task2 factory
	task2Factory := setupTask2Factory(t, workflowRepo, taskRepo)

	// Setup config store
	configStore := newTestConfigStore()

	// Setup test workflows
	testWorkflows := setupTestWorkflows()

	// Create workflow states in the database and get their exec IDs
	workflowExecIDs := setupWorkflowStates(ctx, t, workflowRepo, testWorkflows)

	// Create test project config
	projectConfig := &project.Config{
		Name: "test-project",
		CWD:  nil, // CWD not needed for tests
	}

	// Create template engine
	templateEngine := tplengine.NewEngine(tplengine.FormatText)

	// Create activity
	activity, err := NewExecuteMemory(
		testWorkflows,
		workflowRepo,
		taskRepo,
		configStore,
		memoryManager,
		nil, // pathCWD not needed for tests
		templateEngine,
		projectConfig,
		task2Factory,
	)
	require.NoError(t, err)

	return activity, workflowExecIDs
}
