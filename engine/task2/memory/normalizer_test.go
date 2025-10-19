package memory

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
)

func TestNormalizer_Normalize(t *testing.T) {
	templateEngine := &tplengine.TemplateEngine{}
	normalizer := NewNormalizer(t.Context(), templateEngine)

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}
		err := normalizer.Normalize(t.Context(), nil, ctx)
		assert.NoError(t, err)
	})

	t.Run("Should apply base normalization to valid config", func(t *testing.T) {
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeMemory,
				ID:   "test-memory-task",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}
		err := normalizer.Normalize(t.Context(), config, ctx)
		assert.NoError(t, err)
	})

	t.Run("Should normalize memory task with operation", func(t *testing.T) {
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeMemory,
				ID:   "test-memory-task",
			},
			MemoryTask: task.MemoryTask{
				Operation:   task.MemoryOpRead,
				MemoryRef:   "test-memory",
				KeyTemplate: "user:{{.user_id}}",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"user_id": "123",
			},
		}
		err := normalizer.Normalize(t.Context(), config, ctx)
		assert.NoError(t, err)
	})

	t.Run("Should normalize memory task with write operation and payload", func(t *testing.T) {
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeMemory,
				ID:   "test-memory-write",
			},
			MemoryTask: task.MemoryTask{
				Operation:   task.MemoryOpWrite,
				MemoryRef:   "test-memory",
				KeyTemplate: "data:{{.key}}",
				Payload: map[string]any{
					"value":     "test-value",
					"timestamp": "{{.timestamp}}",
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"key":       "test-key",
				"timestamp": "2024-01-01T00:00:00Z",
			},
		}
		err := normalizer.Normalize(t.Context(), config, ctx)
		assert.NoError(t, err)
	})

	t.Run("Should normalize memory task with flush config", func(t *testing.T) {
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeMemory,
				ID:   "test-memory-flush",
			},
			MemoryTask: task.MemoryTask{
				Operation: task.MemoryOpFlush,
				MemoryRef: "test-memory",
				FlushConfig: &task.FlushConfig{
					Strategy: "lru",
					MaxKeys:  100,
					DryRun:   true,
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}
		err := normalizer.Normalize(t.Context(), config, ctx)
		assert.NoError(t, err)
	})

	t.Run("Should normalize memory task with environment variables", func(t *testing.T) {
		envMap := core.EnvMap{
			"MEMORY_KEY": "env-key-value",
		}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeMemory,
				ID:   "test-memory-env",
				Env:  &envMap,
			},
			MemoryTask: task.MemoryTask{
				Operation:   task.MemoryOpRead,
				MemoryRef:   "test-memory",
				KeyTemplate: "user:{{.user_id}}",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"user_id": "123",
				"env":     &envMap,
			},
			MergedEnv: &envMap,
		}
		err := normalizer.Normalize(t.Context(), config, ctx)
		assert.NoError(t, err)
	})

	t.Run("Should handle empty context variables", func(t *testing.T) {
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeMemory,
				ID:   "test-memory-empty-ctx",
			},
			MemoryTask: task.MemoryTask{
				Operation: task.MemoryOpHealth,
				MemoryRef: "test-memory",
				HealthConfig: &task.HealthConfig{
					IncludeStats:      true,
					CheckConnectivity: true,
				},
			},
		}
		ctx := &shared.NormalizationContext{} // No variables initialized
		err := normalizer.Normalize(t.Context(), config, ctx)
		assert.NoError(t, err)
	})
}
