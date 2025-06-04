package utils

import (
	"github.com/compozy/compozy/engine/agent"
)

// TestProviderConfig holds the standardized test provider configuration
type TestProviderConfig struct {
	Provider    agent.ProviderName
	Model       agent.ModelName
	APIKey      string
	APIURL      string
	Temperature float32
	MaxTokens   int32
}

// GetTestProviderConfig returns a standardized test provider configuration
// Uses Mock provider for deterministic testing without API calls
func GetTestProviderConfig() *TestProviderConfig {
	return &TestProviderConfig{
		Provider:    agent.ProviderMock,
		Model:       agent.ModelMockTest, // Mock model for testing
		APIKey:      "test-api-key",      // Not used by mock provider
		APIURL:      "http://localhost",  // Not used by mock provider
		Temperature: 0.0,                 // Deterministic for testing
		MaxTokens:   50,                  // Small tokens for fast tests
	}
}

// CreateTestAgentProviderConfig creates an agent.ProviderConfig for tests
func CreateTestAgentProviderConfig() agent.ProviderConfig {
	testConfig := GetTestProviderConfig()
	return agent.ProviderConfig{
		Provider:    testConfig.Provider,
		Model:       testConfig.Model,
		APIKey:      testConfig.APIKey,
		APIURL:      testConfig.APIURL,
		Temperature: testConfig.Temperature,
		MaxTokens:   testConfig.MaxTokens,
	}
}

// CreateTestAgentConfig creates a complete agent config for testing
func CreateTestAgentConfig(id, instructions string) *agent.Config {
	return &agent.Config{
		ID:           id,
		Instructions: instructions,
		Config:       CreateTestAgentProviderConfig(),
	}
}

// CreateTestAgentConfigWithAction creates an agent config with a specific action
func CreateTestAgentConfigWithAction(id, instructions, actionID, actionPrompt string) *agent.Config {
	return &agent.Config{
		ID:           id,
		Instructions: instructions,
		Config:       CreateTestAgentProviderConfig(),
		Actions: []*agent.ActionConfig{
			{
				ID:     actionID,
				Prompt: actionPrompt,
			},
		},
	}
}

// CreateTestAgentConfigWithActions creates an agent config with multiple actions
func CreateTestAgentConfigWithActions(id, instructions string, actions map[string]string) *agent.Config {
	actionConfigs := make([]*agent.ActionConfig, 0, len(actions))
	for actionID, actionPrompt := range actions {
		actionConfigs = append(actionConfigs, &agent.ActionConfig{
			ID:     actionID,
			Prompt: actionPrompt,
		})
	}

	return &agent.Config{
		ID:           id,
		Instructions: instructions,
		Config:       CreateTestAgentProviderConfig(),
		Actions:      actionConfigs,
	}
}
