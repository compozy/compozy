package task

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectionTaskConfigLoad(t *testing.T) {
	cwd, err := core.CWDFromPath(".")
	require.NoError(t, err)
	fixturePath := filepath.Join("fixtures", "collection_task.yaml")
	
	config, err := Load(cwd, fixturePath)
	require.NoError(t, err)
	require.NotNil(t, config)
	
	// Test basic properties
	assert.Equal(t, "process_user_notifications", config.ID)
	assert.Equal(t, TaskTypeCollection, config.Type)
	assert.Equal(t, "{{ .workflow.input.users }}", config.CollectionTask.Items)
	assert.Equal(t, "{{ and .user.active (not .user.notified) }}", config.CollectionTask.Filter)
	assert.Equal(t, CollectionModeParallel, config.CollectionTask.GetMode())
	assert.Equal(t, 5, config.CollectionTask.GetBatch())
	assert.True(t, config.CollectionTask.ContinueOnError)
	assert.Equal(t, StrategyWaitAll, config.GetStrategy())
	assert.Equal(t, 10, config.GetMaxWorkers())
	assert.Equal(t, "5m", config.Timeout)
	assert.Equal(t, "user", config.CollectionTask.GetItemVar())
	assert.Equal(t, "user_index", config.CollectionTask.GetIndexVar())
	assert.Equal(t, "{{ gt .summary.failed 10 }}", config.CollectionTask.StopCondition)
	
	// Test task template
	assert.NotNil(t, config.Task)
	assert.Equal(t, "notify_user", config.Task.ID)
	assert.Equal(t, TaskTypeBasic, config.Task.Type)
	assert.Equal(t, "send_notification", config.Task.Action)
	assert.NotNil(t, config.Task.Agent)
	assert.Equal(t, "notification_agent", config.Task.Agent.ID)
}

func TestCollectionTaskValidation(t *testing.T) {
	cwd, err := core.CWDFromPath(".")
	require.NoError(t, err)
	fixturePath := filepath.Join("fixtures", "collection_task.yaml")
	
	config, err := Load(cwd, fixturePath)
	require.NoError(t, err)
	
	// Test validation passes
	err = config.Validate()
	assert.NoError(t, err)
}

func TestCollectionTaskExecutionType(t *testing.T) {
	config := &Config{
		BaseConfig: BaseConfig{
			Type: TaskTypeCollection,
		},
	}
	
	execType := config.GetExecType()
	assert.Equal(t, ExecutionCollection, execType)
}

func TestCollectionTaskValidationErrors(t *testing.T) {
	cwd, err := core.CWDFromPath(".")
	require.NoError(t, err)
	
	tests := []struct {
		name     string
		config   *Config
		wantErr  string
	}{
		{
			name: "missing items expression",
			config: &Config{
				BaseConfig: BaseConfig{
					ID:   "test",
					Type: TaskTypeCollection,
					cwd:  cwd,
					Task: &Config{
						BaseConfig: BaseConfig{
							ID:   "template",
							Type: TaskTypeBasic,
							cwd:  cwd,
						},
					},
				},
				CollectionTask: CollectionTask{},
			},
			wantErr: "collection tasks must specify an items expression",
		},
		{
			name: "missing task template",
			config: &Config{
				BaseConfig: BaseConfig{
					ID:   "test",
					Type: TaskTypeCollection,
					cwd:  cwd,
				},
				CollectionTask: CollectionTask{
					Items: "{{ .input.items }}",
				},
			},
			wantErr: "collection tasks must specify a task template",
		},
		{
			name: "invalid mode",
			config: &Config{
				BaseConfig: BaseConfig{
					ID:   "test",
					Type: TaskTypeCollection,
					cwd:  cwd,
					Task: &Config{
						BaseConfig: BaseConfig{
							ID:   "template",
							Type: TaskTypeBasic,
							cwd:  cwd,
						},
					},
				},
				CollectionTask: CollectionTask{
					Items: "{{ .input.items }}",
					Mode:  CollectionMode("invalid"),
				},
			},
			wantErr: "invalid collection mode: invalid",
		},
		{
			name: "same item and index variables",
			config: &Config{
				BaseConfig: BaseConfig{
					ID:   "test",
					Type: TaskTypeCollection,
					cwd:  cwd,
					Task: &Config{
						BaseConfig: BaseConfig{
							ID:   "template",
							Type: TaskTypeBasic,
							cwd:  cwd,
						},
					},
				},
				CollectionTask: CollectionTask{
					Items:    "{{ .input.items }}",
					ItemVar:  "item",
					IndexVar: "item",
				},
			},
			wantErr: "item_var and index_var cannot be the same: item",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCollectionTaskHelperMethods(t *testing.T) {
	config := &Config{
		BaseConfig: BaseConfig{
			Type:       TaskTypeCollection,
			Strategy:   StrategyBestEffort,
			MaxWorkers: 20,
			Timeout:    "10m",
		},
		CollectionTask: CollectionTask{
			Mode:     CollectionModeSequential,
			Batch:    10,
			ItemVar:  "record",
			IndexVar: "idx",
		},
	}
	
	// Test getter methods
	assert.Equal(t, CollectionModeSequential, config.CollectionTask.GetMode())
	assert.Equal(t, 10, config.CollectionTask.GetBatch())
	assert.Equal(t, "record", config.CollectionTask.GetItemVar())
	assert.Equal(t, "idx", config.CollectionTask.GetIndexVar())
	assert.Equal(t, StrategyBestEffort, config.GetStrategy())
	assert.Equal(t, 20, config.GetMaxWorkers())
	
	timeout, err := config.GetTimeout()
	require.NoError(t, err)
	assert.Equal(t, "10m0s", timeout.String())
}

func TestCollectionTaskDefaults(t *testing.T) {
	config := &Config{
		BaseConfig: BaseConfig{
			Type: TaskTypeCollection,
		},
		CollectionTask: CollectionTask{},
	}
	
	// Test default values
	assert.Equal(t, CollectionModeParallel, config.CollectionTask.GetMode())
	assert.Equal(t, 1, config.CollectionTask.GetBatch())
	assert.Equal(t, "item", config.CollectionTask.GetItemVar())
	assert.Equal(t, "index", config.CollectionTask.GetIndexVar())
	assert.Equal(t, StrategyWaitAll, config.GetStrategy())
	assert.Equal(t, 10, config.GetMaxWorkers())
}

func TestCollectionTaskInputValidation(t *testing.T) {
	cwd, err := core.CWDFromPath(".")
	require.NoError(t, err)
	fixturePath := filepath.Join("fixtures", "collection_task.yaml")
	
	config, err := Load(cwd, fixturePath)
	require.NoError(t, err)
	
	// Valid input
	validInput := &core.Input{
		"users": []any{
			map[string]any{
				"id":           "user1",
				"name":         "Alice",
				"email":        "alice@example.com",
				"active":       true,
				"notified":     false,
				"unread_count": 3,
			},
		},
	}
	
	err = config.ValidateInput(context.Background(), validInput)
	assert.NoError(t, err)
	
	// Invalid input - missing required field
	invalidInput := &core.Input{
		"users": []any{
			map[string]any{
				"id":    "user1",
				"name":  "Alice",
				"email": "alice@example.com",
				// missing active and notified fields
			},
		},
	}
	
	err = config.ValidateInput(context.Background(), invalidInput)
	assert.Error(t, err)
}