package task

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("Should create minimal basic task configuration", func(t *testing.T) {
		cfg, err := New(t.Context(), "test-task",
			WithAgent(&agent.Config{ID: "test-agent"}),
		)
		require.NoError(t, err)
		assert.Equal(t, "test-task", cfg.ID)
		assert.Equal(t, enginetask.TaskTypeBasic, cfg.Type)
		assert.NotNil(t, cfg.Agent)
	})

	t.Run("Should create full basic task configuration with all options", func(t *testing.T) {
		cfg, err := New(t.Context(), "test-task",
			WithAgent(&agent.Config{ID: "test-agent"}),
			WithTimeout("30s"),
			WithRetries(3),
			WithSleep("1s"),
		)
		require.NoError(t, err)
		assert.Equal(t, "test-task", cfg.ID)
		assert.Equal(t, "30s", cfg.Timeout)
		assert.Equal(t, 3, cfg.Retries)
		assert.Equal(t, "1s", cfg.Sleep)
	})

	t.Run("Should return error for nil context", func(t *testing.T) {
		_, err := New(nil, "test-task")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context is required")
	})

	t.Run("Should return error for empty ID", func(t *testing.T) {
		_, err := New(t.Context(), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task id is invalid")
	})

	t.Run("Should return error for invalid timeout", func(t *testing.T) {
		_, err := New(t.Context(), "test-task", WithTimeout("invalid"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timeout duration")
	})

	t.Run("Should deep copy configuration", func(t *testing.T) {
		cfg1, err := New(t.Context(), "test-task",
			WithAgent(&agent.Config{ID: "test-agent"}),
		)
		require.NoError(t, err)

		cfg2, err := New(t.Context(), "test-task",
			WithAgent(&agent.Config{ID: "test-agent"}),
		)
		require.NoError(t, err)

		cfg1.ID = "modified"
		assert.Equal(t, "test-task", cfg2.ID)
	})
}

func TestNewRouter(t *testing.T) {
	t.Run("Should create minimal router task configuration", func(t *testing.T) {
		routes := map[string]any{
			"true":  "task-a",
			"false": "task-b",
		}
		cfg, err := NewRouter(t.Context(), "router-task",
			WithCondition("input.approved"),
			WithRoutes(routes),
		)
		require.NoError(t, err)
		assert.Equal(t, "router-task", cfg.ID)
		assert.Equal(t, enginetask.TaskTypeRouter, cfg.Type)
		assert.Equal(t, "input.approved", cfg.Condition)
		assert.Len(t, cfg.Routes, 2)
	})

	t.Run("Should return error for nil context", func(t *testing.T) {
		_, err := NewRouter(nil, "router-task")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context is required")
	})

	t.Run("Should return error for empty condition", func(t *testing.T) {
		routes := map[string]any{"true": "task-a"}
		_, err := NewRouter(t.Context(), "router-task",
			WithCondition(""),
			WithRoutes(routes),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "condition")
	})

	t.Run("Should return error for empty routes", func(t *testing.T) {
		_, err := NewRouter(t.Context(), "router-task",
			WithCondition("input.approved"),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one route must be defined")
	})
}

func TestNewParallel(t *testing.T) {
	t.Run("Should create minimal parallel task configuration", func(t *testing.T) {
		tasks := []enginetask.Config{
			{BaseConfig: enginetask.BaseConfig{ID: "task-1", Type: enginetask.TaskTypeBasic}},
			{BaseConfig: enginetask.BaseConfig{ID: "task-2", Type: enginetask.TaskTypeBasic}},
		}
		cfg, err := NewParallel(t.Context(), "parallel-task", tasks)
		require.NoError(t, err)
		assert.Equal(t, "parallel-task", cfg.ID)
		assert.Equal(t, enginetask.TaskTypeParallel, cfg.Type)
		assert.Len(t, cfg.Tasks, 2)
		assert.Equal(t, enginetask.StrategyWaitAll, cfg.Strategy)
	})

	t.Run("Should create parallel task with custom strategy", func(t *testing.T) {
		tasks := []enginetask.Config{
			{BaseConfig: enginetask.BaseConfig{ID: "task-1", Type: enginetask.TaskTypeBasic}},
			{BaseConfig: enginetask.BaseConfig{ID: "task-2", Type: enginetask.TaskTypeBasic}},
		}
		cfg, err := NewParallel(t.Context(), "parallel-task", tasks,
			WithStrategy(enginetask.StrategyFailFast),
			WithMaxWorkers(5),
		)
		require.NoError(t, err)
		assert.Equal(t, enginetask.StrategyFailFast, cfg.Strategy)
		assert.Equal(t, 5, cfg.MaxWorkers)
	})

	t.Run("Should return error for less than 2 tasks", func(t *testing.T) {
		tasks := []enginetask.Config{
			{BaseConfig: enginetask.BaseConfig{ID: "task-1", Type: enginetask.TaskTypeBasic}},
		}
		_, err := NewParallel(t.Context(), "parallel-task", tasks)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2 tasks")
	})

	t.Run("Should return error for task with empty ID", func(t *testing.T) {
		tasks := []enginetask.Config{
			{BaseConfig: enginetask.BaseConfig{ID: "task-1", Type: enginetask.TaskTypeBasic}},
			{BaseConfig: enginetask.BaseConfig{ID: "", Type: enginetask.TaskTypeBasic}},
		}
		_, err := NewParallel(t.Context(), "parallel-task", tasks)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing an id")
	})

	t.Run("Should return error for invalid strategy", func(t *testing.T) {
		tasks := []enginetask.Config{
			{BaseConfig: enginetask.BaseConfig{ID: "task-1", Type: enginetask.TaskTypeBasic}},
			{BaseConfig: enginetask.BaseConfig{ID: "task-2", Type: enginetask.TaskTypeBasic}},
		}
		_, err := NewParallel(t.Context(), "parallel-task", tasks,
			WithStrategy("invalid-strategy"),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid parallel strategy")
	})
}

func TestNewCollection(t *testing.T) {
	t.Run("Should create minimal collection task configuration", func(t *testing.T) {
		taskTemplate := &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				ID:   "item-task",
				Type: enginetask.TaskTypeBasic,
			},
		}
		cfg, err := NewCollection(t.Context(), "collection-task", "items",
			WithTask(taskTemplate),
		)
		require.NoError(t, err)
		assert.Equal(t, "collection-task", cfg.ID)
		assert.Equal(t, enginetask.TaskTypeCollection, cfg.Type)
		assert.Equal(t, "items", cfg.Items)
		assert.NotNil(t, cfg.Task)
	})

	t.Run("Should create collection task with options", func(t *testing.T) {
		taskTemplate := &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				ID:   "item-task",
				Type: enginetask.TaskTypeBasic,
			},
		}
		cfg, err := NewCollection(t.Context(), "collection-task", "workflow.data",
			WithTask(taskTemplate),
			WithMode(enginetask.CollectionModeSequential),
			WithBatch(10),
		)
		require.NoError(t, err)
		assert.Equal(t, "workflow.data", cfg.Items)
		assert.Equal(t, enginetask.CollectionModeSequential, cfg.Mode)
		assert.Equal(t, 10, cfg.Batch)
	})

	t.Run("Should return error for empty items", func(t *testing.T) {
		taskTemplate := &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{ID: "item-task", Type: enginetask.TaskTypeBasic},
		}
		_, err := NewCollection(t.Context(), "collection-task", "",
			WithTask(taskTemplate),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "items")
	})

	t.Run("Should return error for nil task template", func(t *testing.T) {
		_, err := NewCollection(t.Context(), "collection-task", "items")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task template")
	})

	t.Run("Should return error for task template with empty ID", func(t *testing.T) {
		taskTemplate := &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{ID: "", Type: enginetask.TaskTypeBasic},
		}
		_, err := NewCollection(t.Context(), "collection-task", "items",
			WithTask(taskTemplate),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task template must have an id")
	})
}

func TestNewWait(t *testing.T) {
	t.Run("Should create minimal wait task configuration", func(t *testing.T) {
		cfg, err := NewWait(t.Context(), "wait-task", "user-approval")
		require.NoError(t, err)
		assert.Equal(t, "wait-task", cfg.ID)
		assert.Equal(t, enginetask.TaskTypeWait, cfg.Type)
		assert.Equal(t, "user-approval", cfg.WaitFor)
	})

	t.Run("Should create wait task with timeout", func(t *testing.T) {
		cfg, err := NewWait(t.Context(), "wait-task", "payment-complete",
			WithTimeout("30s"),
		)
		require.NoError(t, err)
		assert.Equal(t, "payment-complete", cfg.WaitFor)
		assert.Equal(t, "30s", cfg.Timeout)
	})

	t.Run("Should return error for empty wait_for", func(t *testing.T) {
		_, err := NewWait(t.Context(), "wait-task", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "wait_for")
	})
}

func TestNewSignal(t *testing.T) {
	t.Run("Should create minimal signal task configuration", func(t *testing.T) {
		cfg, err := NewSignal(t.Context(), "signal-task", "process-complete")
		require.NoError(t, err)
		assert.Equal(t, "signal-task", cfg.ID)
		assert.Equal(t, enginetask.TaskTypeSignal, cfg.Type)
		assert.NotNil(t, cfg.Signal)
		assert.Equal(t, "process-complete", cfg.Signal.ID)
	})

	t.Run("Should create signal task with payload", func(t *testing.T) {
		payload := map[string]any{
			"status": "success",
			"data":   "result",
		}
		cfg, err := NewSignal(t.Context(), "signal-task", "task-done",
			WithPayload(payload),
		)
		require.NoError(t, err)
		assert.Equal(t, "task-done", cfg.Signal.ID)
		assert.Equal(t, payload, cfg.Payload)
	})

	t.Run("Should return error for empty signal ID", func(t *testing.T) {
		_, err := NewSignal(t.Context(), "signal-task", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signal id")
	})
}

func TestNewMemory(t *testing.T) {
	t.Run("Should create minimal memory task configuration", func(t *testing.T) {
		cfg, err := NewMemory(t.Context(), "memory-task", enginetask.MemoryOpRead)
		require.NoError(t, err)
		assert.Equal(t, "memory-task", cfg.ID)
		assert.Equal(t, enginetask.TaskTypeMemory, cfg.Type)
		assert.Equal(t, enginetask.MemoryOpRead, cfg.Operation)
	})

	t.Run("Should create memory task with all operations", func(t *testing.T) {
		operations := []enginetask.MemoryOpType{
			enginetask.MemoryOpRead,
			enginetask.MemoryOpWrite,
			enginetask.MemoryOpAppend,
			enginetask.MemoryOpDelete,
			enginetask.MemoryOpClear,
			enginetask.MemoryOpFlush,
			enginetask.MemoryOpHealth,
			enginetask.MemoryOpStats,
		}

		for _, op := range operations {
			cfg, err := NewMemory(t.Context(), "memory-task", op)
			require.NoError(t, err)
			assert.Equal(t, op, cfg.Operation)
		}
	})

	t.Run("Should create memory task with memory reference", func(t *testing.T) {
		cfg, err := NewMemory(t.Context(), "memory-task", enginetask.MemoryOpWrite,
			WithMemoryRef("session-data"),
			WithKeyTemplate("user-{{.user_id}}"),
		)
		require.NoError(t, err)
		assert.Equal(t, "session-data", cfg.MemoryRef)
		assert.Equal(t, "user-{{.user_id}}", cfg.KeyTemplate)
	})

	t.Run("Should return error for invalid operation", func(t *testing.T) {
		_, err := NewMemory(t.Context(), "memory-task", "invalid-op")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid memory operation")
	})
}

func TestDeepCopyBehavior(t *testing.T) {
	t.Run("Should deep copy config to prevent external mutation", func(t *testing.T) {
		originalTasks := []enginetask.Config{
			{BaseConfig: enginetask.BaseConfig{ID: "task-1", Type: enginetask.TaskTypeBasic}},
			{BaseConfig: enginetask.BaseConfig{ID: "task-2", Type: enginetask.TaskTypeBasic}},
		}

		cfg1, err := NewParallel(t.Context(), "parallel-1", originalTasks)
		require.NoError(t, err)

		cfg1.Tasks[0].ID = "modified"

		cfg2, err := NewParallel(t.Context(), "parallel-2", originalTasks)
		require.NoError(t, err)

		assert.NotEqual(t, cfg1.Tasks[0].ID, cfg2.Tasks[0].ID)
	})
}

func TestWhitespaceTrimming(t *testing.T) {
	t.Run("Should trim whitespace from IDs", func(t *testing.T) {
		cfg, err := New(t.Context(), "  test-task  ",
			WithAgent(&agent.Config{ID: "test-agent"}),
		)
		require.NoError(t, err)
		assert.Equal(t, "test-task", cfg.ID)
	})

	t.Run("Should trim whitespace from condition", func(t *testing.T) {
		routes := map[string]any{"true": "task-a"}
		cfg, err := NewRouter(t.Context(), "router-task",
			WithCondition("  input.approved  "),
			WithRoutes(routes),
		)
		require.NoError(t, err)
		assert.Equal(t, "input.approved", cfg.Condition)
	})

	t.Run("Should trim whitespace from items", func(t *testing.T) {
		taskTemplate := &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{ID: "item-task", Type: enginetask.TaskTypeBasic},
		}
		cfg, err := NewCollection(t.Context(), "collection-task", "  workflow.data  ",
			WithTask(taskTemplate),
		)
		require.NoError(t, err)
		assert.Equal(t, "workflow.data", cfg.Items)
	})
}

func TestErrorAccumulation(t *testing.T) {
	t.Run("Should accumulate multiple validation errors", func(t *testing.T) {
		tasks := []enginetask.Config{
			{BaseConfig: enginetask.BaseConfig{ID: "", Type: enginetask.TaskTypeBasic}},
		}
		_, err := NewParallel(t.Context(), "", tasks)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task id is invalid")
		assert.Contains(t, err.Error(), "at least 2 tasks")
	})
}

func TestTransitionConfiguration(t *testing.T) {
	t.Run("Should configure success and error transitions", func(t *testing.T) {
		nextTask := "next-task"
		errorHandler := "error-handler"
		cfg, err := New(t.Context(), "test-task",
			WithAgent(&agent.Config{ID: "test-agent"}),
			WithOnSuccess(&core.SuccessTransition{
				Next: &nextTask,
			}),
			WithOnError(&core.ErrorTransition{
				Next: &errorHandler,
			}),
			WithRetries(3),
		)
		require.NoError(t, err)
		assert.NotNil(t, cfg.OnSuccess)
		assert.Equal(t, "next-task", *cfg.OnSuccess.Next)
		assert.NotNil(t, cfg.OnError)
		assert.Equal(t, "error-handler", *cfg.OnError.Next)
		assert.Equal(t, 3, cfg.Retries)
	})
}
