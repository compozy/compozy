package memory

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
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task2"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
)

func TestExecuteMemoryIntegration(t *testing.T) {
	t.Run("Should execute memory write operation", func(t *testing.T) {
		ctx := context.Background()
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow"
		input := &activities.ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecIDs[workflowID],
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpWrite,
					MemoryRef:   "test_memory",
					KeyTemplate: "test:{{.workflow.id}}",
					Payload:     map[string]any{"content": "Hello, Memory!"},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		response, err := activity.Run(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, core.StatusSuccess, response.State.Status)
	})

	t.Run("Should execute memory read operation", func(t *testing.T) {
		ctx := context.Background()
		activity, workflowExecIDs := createTestMemoryActivity(t)
		workflowID := "test-workflow-read"
		workflowExecID := workflowExecIDs[workflowID]
		writeInput := &activities.ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpWrite,
					MemoryRef:   "test_memory",
					KeyTemplate: "test:{{.workflow.id}}",
					Payload:     map[string]any{"content": "Test message"},
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		_, err := activity.Run(ctx, writeInput)
		require.NoError(t, err)
		readInput := &activities.ExecuteMemoryInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpRead,
					MemoryRef:   "test_memory",
					KeyTemplate: "test:{{.workflow.id}}",
				},
			},
			MergedInput: &core.Input{"user_id": "test-user"},
		}
		response, err := activity.Run(ctx, readInput)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.NotNil(t, response.State.Output)
	})
}

// The helpers below are adapted from the original unit tests to build a full integration stack.

func createTestMemoryActivity(t *testing.T) (*activities.ExecuteMemory, map[string]core.ID) {
	t.Helper()
	ctx := context.Background()
	redisClient := setupRedisClient(t)
	lockManager := setupLockManager(t, redisClient)
	configRegistry := setupTestConfigRegistry(t)
	taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
	t.Cleanup(cleanup)
	memoryManager := setupMemoryManager(t, redisClient, lockManager, configRegistry)
	task2Factory := setupTask2Factory(t, workflowRepo, taskRepo)
	configStore := newTestConfigStore()
	testWorkflows := setupTestWorkflows()
	workflowExecIDs := setupWorkflowStates(ctx, t, workflowRepo, testWorkflows)
	projectConfig := &project.Config{Name: "test-project", CWD: nil}
	templateEngine := tplengine.NewEngine(tplengine.FormatText)
	activity, err := activities.NewExecuteMemory(
		testWorkflows,
		workflowRepo,
		taskRepo,
		configStore,
		memoryManager,
		nil,
		templateEngine,
		projectConfig,
		task2Factory,
	)
	require.NoError(t, err)
	return activity, workflowExecIDs
}

func setupRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(func() { mr.Close() })
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DB: 0})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.Ping(ctx).Err())
	return client
}

func setupLockManager(t *testing.T, redisClient *redis.Client) cache.LockManager {
	t.Helper()
	lockManager, err := cache.NewRedisLockManager(redisClient)
	require.NoError(t, err)
	return lockManager
}

func setupTestConfigRegistry(t *testing.T) *autoload.ConfigRegistry {
	t.Helper()
	configRegistry := autoload.NewConfigRegistry()
	testMemoryConfig := &memory.Config{
		Resource:    "memory",
		ID:          "test_memory",
		Type:        memcore.TokenBasedMemory,
		Description: "Test memory for integration tests",
		MaxTokens:   4000,
		MaxMessages: 100,
		Persistence: memcore.PersistenceConfig{Type: memcore.RedisPersistence, TTL: "24h"},
		Flushing:    &memcore.FlushingStrategyConfig{Type: memcore.SimpleFIFOFlushing, SummarizeThreshold: 0.8},
	}
	require.NoError(t, testMemoryConfig.Validate(t.Context()))
	require.NoError(t, configRegistry.Register(testMemoryConfig, "test"))
	return configRegistry
}

func setupMemoryManager(
	t *testing.T,
	redisClient *redis.Client,
	lockManager cache.LockManager,
	configRegistry *autoload.ConfigRegistry,
) *memory.Manager {
	t.Helper()
	templateEngine := tplengine.NewEngine(tplengine.FormatText)
	privacyManager := privacy.NewManager()
	memoryManager, err := memory.NewManager(&memory.ManagerOptions{
		ResourceRegistry:  configRegistry,
		TplEngine:         templateEngine,
		BaseLockManager:   lockManager,
		BaseRedisClient:   redisClient,
		TemporalClient:    &mocks.Client{},
		TemporalTaskQueue: "test-memory-queue",
		PrivacyManager:    privacyManager,
	})
	require.NoError(t, err)
	return memoryManager
}

func setupTask2Factory(t *testing.T, workflowRepo workflow.Repository, taskRepo task.Repository) task2.Factory {
	t.Helper()
	templateEngine := tplengine.NewEngine(tplengine.FormatText)
	envMerger := task2core.NewEnvMerger()
	factory, err := task2.NewFactory(&task2.FactoryConfig{
		TemplateEngine: templateEngine,
		EnvMerger:      envMerger,
		WorkflowRepo:   workflowRepo,
		TaskRepo:       taskRepo,
	})
	require.NoError(t, err)
	return factory
}

func setupTestWorkflows() []*workflow.Config {
	return []*workflow.Config{
		{
			ID: "test-workflow",
			Tasks: []task.Config{{
				BaseConfig: task.BaseConfig{ID: "memory", Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpWrite,
					MemoryRef:   "test_memory",
					KeyTemplate: "test:{{.workflow.id}}",
				},
			}},
		},
		{
			ID: "test-workflow-read",
			Tasks: []task.Config{{
				BaseConfig: task.BaseConfig{ID: "memory", Type: task.TaskTypeMemory},
				MemoryTask: task.MemoryTask{
					Operation:   task.MemoryOpRead,
					MemoryRef:   "test_memory",
					KeyTemplate: "test:{{.workflow.id}}",
				},
			}},
		},
	}
}

// Minimal config store for tests
type testConfigStore struct {
	mu       sync.RWMutex
	configs  map[string]*task.Config
	metadata map[string][]byte
}

func newTestConfigStore() *testConfigStore {
	return &testConfigStore{configs: make(map[string]*task.Config), metadata: make(map[string][]byte)}
}

func (s *testConfigStore) Save(_ context.Context, taskExecID string, config *task.Config) error {
	s.mu.Lock()
	s.configs[taskExecID] = config
	s.mu.Unlock()
	return nil
}

func (s *testConfigStore) Get(_ context.Context, taskExecID string) (*task.Config, error) {
	s.mu.RLock()
	cfg := s.configs[taskExecID]
	s.mu.RUnlock()
	if cfg == nil {
		return nil, nil
	}
	return cfg, nil
}

func (s *testConfigStore) Delete(_ context.Context, taskExecID string) error {
	s.mu.Lock()
	delete(s.configs, taskExecID)
	s.mu.Unlock()
	return nil
}

func (s *testConfigStore) SaveMetadata(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	s.metadata[key] = data
	s.mu.Unlock()
	return nil
}

func (s *testConfigStore) GetMetadata(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	data := s.metadata[key]
	s.mu.RUnlock()
	if data == nil {
		return nil, nil
	}
	return data, nil
}

func (s *testConfigStore) DeleteMetadata(_ context.Context, key string) error {
	s.mu.Lock()
	delete(s.metadata, key)
	s.mu.Unlock()
	return nil
}

func (s *testConfigStore) Close() error { return nil }

// Create workflow states in DB
func setupWorkflowStates(
	ctx context.Context,
	t *testing.T,
	workflowRepo workflow.Repository,
	workflows []*workflow.Config,
) map[string]core.ID {
	t.Helper()
	ids := make(map[string]core.ID)
	for _, wf := range workflows {
		execID := core.MustNewID()
		ids[wf.ID] = execID
		state := &workflow.State{
			WorkflowID:     wf.ID,
			WorkflowExecID: execID,
			Input:          &core.Input{},
			Status:         core.StatusRunning,
		}
		require.NoError(t, workflowRepo.UpsertState(ctx, state))
	}
	return ids
}
