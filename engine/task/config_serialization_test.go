package task

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func Test_ConfigSerializationWithPublicFields(t *testing.T) {
	t.Run("Should serialize and deserialize FilePath and CWD correctly with JSON", func(t *testing.T) {
		// Create a config with all fields set
		CWD, err := core.CWDFromPath("/test/working/directory")
		require.NoError(t, err)

		originalConfig := &Config{
			BaseConfig: BaseConfig{
				ID:       "test-task",
				Type:     TaskTypeBasic,
				FilePath: "/test/config/file.yaml",
				CWD:      CWD,
				Config: core.GlobalOpts{
					StartToCloseTimeout: "5m",
				},
				With: &core.Input{
					"param1": "value1",
				},
			},
			BasicTask: BasicTask{
				Action: "test-action",
			},
		}

		// Serialize to JSON
		jsonData, err := json.Marshal(originalConfig)
		require.NoError(t, err)

		// Deserialize from JSON
		var deserializedConfig Config
		err = json.Unmarshal(jsonData, &deserializedConfig)
		require.NoError(t, err)

		// Verify all fields are preserved
		assert.Equal(t, originalConfig.ID, deserializedConfig.ID)
		assert.Equal(t, originalConfig.Type, deserializedConfig.Type)
		assert.Equal(t, originalConfig.FilePath, deserializedConfig.FilePath)
		assert.NotNil(t, deserializedConfig.CWD)
		assert.Equal(t, originalConfig.CWD.PathStr(), deserializedConfig.CWD.PathStr())
		assert.Equal(t, originalConfig.Action, deserializedConfig.Action)
		assert.Equal(t, originalConfig.Config.StartToCloseTimeout, deserializedConfig.Config.StartToCloseTimeout)
		assert.Equal(t, (*originalConfig.With)["param1"], (*deserializedConfig.With)["param1"])
	})

	t.Run("Should serialize and deserialize FilePath and CWD correctly with YAML", func(t *testing.T) {
		// Create a config with all fields set
		CWD, err := core.CWDFromPath("/test/working/directory")
		require.NoError(t, err)

		originalConfig := &Config{
			BaseConfig: BaseConfig{
				ID:       "test-task",
				Type:     TaskTypeBasic,
				FilePath: "/test/config/file.yaml",
				CWD:      CWD,
				Config: core.GlobalOpts{
					StartToCloseTimeout: "5m",
				},
				With: &core.Input{
					"param1": "value1",
				},
			},
			BasicTask: BasicTask{
				Action: "test-action",
			},
		}

		// Serialize to YAML
		yamlData, err := yaml.Marshal(originalConfig)
		require.NoError(t, err)

		// Deserialize from YAML
		var deserializedConfig Config
		err = yaml.Unmarshal(yamlData, &deserializedConfig)
		require.NoError(t, err)

		// Verify all fields are preserved
		assert.Equal(t, originalConfig.ID, deserializedConfig.ID)
		assert.Equal(t, originalConfig.Type, deserializedConfig.Type)
		assert.Equal(t, originalConfig.FilePath, deserializedConfig.FilePath)
		assert.NotNil(t, deserializedConfig.CWD)
		assert.Equal(t, originalConfig.CWD.PathStr(), deserializedConfig.CWD.PathStr())
		assert.Equal(t, originalConfig.Action, deserializedConfig.Action)
		assert.Equal(t, originalConfig.Config.StartToCloseTimeout, deserializedConfig.Config.StartToCloseTimeout)
		assert.Equal(t, (*originalConfig.With)["param1"], (*deserializedConfig.With)["param1"])
	})

	t.Run("Should handle nil CWD correctly during serialization", func(t *testing.T) {
		originalConfig := &Config{
			BaseConfig: BaseConfig{
				ID:       "test-task",
				Type:     TaskTypeBasic,
				FilePath: "/test/config/file.yaml",
				CWD:      nil, // Explicitly nil
			},
			BasicTask: BasicTask{
				Action: "test-action",
			},
		}

		// Serialize to JSON
		jsonData, err := json.Marshal(originalConfig)
		require.NoError(t, err)

		// Deserialize from JSON
		var deserializedConfig Config
		err = json.Unmarshal(jsonData, &deserializedConfig)
		require.NoError(t, err)

		// Verify CWD is still nil
		assert.Nil(t, deserializedConfig.CWD)
		assert.Equal(t, originalConfig.FilePath, deserializedConfig.FilePath)
	})

	t.Run("Should work correctly with Redis-like serialization scenario", func(t *testing.T) {
		// Create a complex config similar to what would be stored in Redis
		CWD, err := core.CWDFromPath("/project/root")
		require.NoError(t, err)

		originalConfig := &Config{
			BaseConfig: BaseConfig{
				ID:       "parallel-task",
				Type:     TaskTypeParallel,
				FilePath: "/project/tasks/parallel.yaml",
				CWD:      CWD,
			},
			ParallelTask: ParallelTask{
				Strategy:   StrategyWaitAll,
				MaxWorkers: 5,
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:       "child-1",
							Type:     TaskTypeBasic,
							FilePath: "/project/tasks/child1.yaml",
							CWD:      CWD,
						},
						BasicTask: BasicTask{
							Action: "process",
						},
					},
					{
						BaseConfig: BaseConfig{
							ID:       "child-2",
							Type:     TaskTypeBasic,
							FilePath: "/project/tasks/child2.yaml",
							CWD:      CWD,
						},
						BasicTask: BasicTask{
							Action: "transform",
						},
					},
				},
			},
		}

		// Simulate Redis storage: serialize to JSON
		jsonData, err := json.Marshal(originalConfig)
		require.NoError(t, err)

		// Simulate Redis retrieval: deserialize from JSON
		var retrievedConfig Config
		err = json.Unmarshal(jsonData, &retrievedConfig)
		require.NoError(t, err)

		// Verify parent config
		assert.Equal(t, originalConfig.ID, retrievedConfig.ID)
		assert.Equal(t, originalConfig.Type, retrievedConfig.Type)
		assert.Equal(t, originalConfig.FilePath, retrievedConfig.FilePath)
		assert.NotNil(t, retrievedConfig.CWD)
		assert.Equal(t, originalConfig.CWD.PathStr(), retrievedConfig.CWD.PathStr())

		// Verify child configs
		assert.Len(t, retrievedConfig.Tasks, 2)
		for i, childTask := range retrievedConfig.Tasks {
			assert.Equal(t, originalConfig.Tasks[i].ID, childTask.ID)
			assert.Equal(t, originalConfig.Tasks[i].FilePath, childTask.FilePath)
			assert.NotNil(t, childTask.CWD)
			assert.Equal(t, originalConfig.Tasks[i].CWD.PathStr(), childTask.CWD.PathStr())
			assert.Equal(t, originalConfig.Tasks[i].Action, childTask.Action)
		}
	})
}
