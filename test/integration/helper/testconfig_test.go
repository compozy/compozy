package utils

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTestProviderConfig(t *testing.T) {
	t.Run("Should return standardized test provider config", func(t *testing.T) {
		config := GetTestProviderConfig()
		require.NotNil(t, config)

		assert.Equal(t, llm.ProviderMock, config.Provider)
		assert.Equal(t, llm.ModelMockTest, config.Model)
		assert.Equal(t, "http://localhost", config.APIURL)
		assert.Equal(t, float32(0.0), config.Temperature)
		assert.Equal(t, int32(50), config.MaxTokens)
		assert.NotEmpty(t, config.APIKey)
	})
}

func TestCreateTestAgentProviderConfig(t *testing.T) {
	t.Run("Should create valid agent provider config", func(t *testing.T) {
		config := CreateTestAgentProviderConfig()

		assert.Equal(t, llm.ProviderMock, config.Provider)
		assert.Equal(t, llm.ModelMockTest, config.Model)
		assert.Equal(t, "http://localhost", config.APIURL)
		assert.Equal(t, float32(0.0), config.Temperature)
		assert.Equal(t, int32(50), config.MaxTokens)
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
		assert.Equal(t, llm.ProviderMock, config.Config.Provider)
		assert.Equal(t, llm.ModelMockTest, config.Config.Model)
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
