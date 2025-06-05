package worker

import (
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/llm"
	utils "github.com/compozy/compozy/test/integration/helper"
)

// CreateTestAgentProviderConfig creates an agent.ProviderConfig for tests
func CreateTestAgentProviderConfig() llm.ProviderConfig {
	raw := utils.GetTestProviderConfig()
	return llm.ProviderConfig{
		Provider:    raw.Provider,
		Model:       raw.Model,
		APIKey:      raw.APIKey,
		APIURL:      raw.APIURL,
		Temperature: raw.Temperature,
		MaxTokens:   raw.MaxTokens,
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
