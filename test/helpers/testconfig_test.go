package utils

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTestProviderConfig(t *testing.T) {
	t.Run("Should return standardized test provider config", func(t *testing.T) {
		config := GetTestProviderConfig()
		require.NotNil(t, config)

		assert.Equal(t, core.ProviderMock, config.Provider)
		assert.Equal(t, "mock-test", config.Model)
		assert.Equal(t, "http://localhost", config.APIURL)
		assert.Equal(t, float64(0.0), config.Temperature)
		assert.Equal(t, int32(50), config.MaxTokens)
		assert.NotEmpty(t, config.APIKey)
	})
}

func TestCreateTestAgentProviderConfig(t *testing.T) {
	t.Run("Should create valid agent provider config", func(t *testing.T) {
		config := CreateTestAgentProviderConfig()

		assert.Equal(t, core.ProviderMock, config.Provider)
		assert.Equal(t, "mock-test", config.Model)
		assert.Equal(t, "http://localhost", config.APIURL)
		assert.Equal(t, float64(0.0), config.Params.Temperature)
		assert.Equal(t, int32(50), config.Params.MaxTokens)
		assert.NotEmpty(t, config.APIKey)
	})
}

func TestCreateTestAgentConfig(t *testing.T) {
	t.Run("Should create complete agent config for testing", func(t *testing.T) {
		id := "test-agent"
		instructions := "You are a test assistant"

		config := CreateTestAgentConfig(id, instructions)
		require.NotNil(t, config)

		assert.Equal(t, id, config.ID)
		assert.Equal(t, instructions, config.Instructions)
		assert.Equal(t, core.ProviderMock, config.Config.Provider)
		assert.Equal(t, "mock-test", config.Config.Model)
	})
}

func TestCreateTestAgentConfigWithAction(t *testing.T) {
	t.Run("Should create agent config with action", func(t *testing.T) {
		id := "test-agent"
		instructions := "You are a test assistant"
		actionID := "test-action"
		actionPrompt := "Process this: {{.input}}"

		config := CreateTestAgentConfigWithAction(id, instructions, actionID, actionPrompt)
		require.NotNil(t, config)

		assert.Equal(t, id, config.ID)
		assert.Equal(t, instructions, config.Instructions)
		require.Len(t, config.Actions, 1)

		action := config.Actions[0]
		assert.Equal(t, actionID, action.ID)
		assert.Equal(t, actionPrompt, action.Prompt)
	})
}

func TestCreateTestAgentConfigWithActions(t *testing.T) {
	t.Run("Should create agent config with multiple actions", func(t *testing.T) {
		id := "test-agent"
		instructions := "You are a test assistant"
		actions := map[string]string{
			"action-1": "Do task 1: {{.input1}}",
			"action-2": "Do task 2: {{.input2}}",
		}

		config := CreateTestAgentConfigWithActions(id, instructions, actions)
		require.NotNil(t, config)

		assert.Equal(t, id, config.ID)
		assert.Equal(t, instructions, config.Instructions)
		require.Len(t, config.Actions, 2)

		// Find actions by ID
		actionMap := make(map[string]*agent.ActionConfig)
		for _, action := range config.Actions {
			actionMap[action.ID] = action
		}

		assert.Contains(t, actionMap, "action-1")
		assert.Contains(t, actionMap, "action-2")
		assert.Equal(t, "Do task 1: {{.input1}}", actionMap["action-1"].Prompt)
		assert.Equal(t, "Do task 2: {{.input2}}", actionMap["action-2"].Prompt)
	})
}

func TestCommonUtilities(t *testing.T) {
	t.Run("Should create string pointer", func(t *testing.T) {
		value := "test"
		ptr := StringPtr(value)
		require.NotNil(t, ptr)
		assert.Equal(t, value, *ptr)
	})

	t.Run("Should create ID pointer", func(t *testing.T) {
		value := "test-id"
		ptr := IDPtr(value)
		require.NotNil(t, ptr)
		assert.Equal(t, core.ID(value), *ptr)
	})

	t.Run("Should generate unique test ID", func(t *testing.T) {
		id1 := GenerateUniqueTestID("test")

		// Add a small delay to ensure different timestamps
		time.Sleep(1 * time.Millisecond)

		id2 := GenerateUniqueTestID("test")

		assert.NotEqual(t, id1, id2)
		assert.Contains(t, id1, "test-test-")
		assert.Contains(t, id2, "test-test-")
	})

	t.Run("Should get environment variable or default", func(t *testing.T) {
		// Test with non-existent variable
		result := GetTestEnvOrDefault("NON_EXISTENT_VAR", "default")
		assert.Equal(t, "default", result)

		// Set an environment variable for testing
		t.Setenv("TEST_VAR", "custom_value")
		result = GetTestEnvOrDefault("TEST_VAR", "default")
		assert.Equal(t, "custom_value", result)
	})
}
