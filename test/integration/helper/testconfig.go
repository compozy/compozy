package utils

import (
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"go.temporal.io/sdk/testsuite"
)

// TestProviderConfig holds the standardized test provider configuration
type TestProviderConfig struct {
	Provider    core.ProviderName
	Model       string
	APIKey      string
	APIURL      string
	Temperature float64
	MaxTokens   int32
}

// GetTestProviderConfig returns a standardized test provider configuration
// Uses Mock provider for deterministic testing without API calls
func GetTestProviderConfig() *TestProviderConfig {
	return &TestProviderConfig{
		Provider:    core.ProviderMock,
		Model:       "mock-test",        // Mock model for testing
		APIKey:      "test-api-key",     // Not used by mock provider
		APIURL:      "http://localhost", // Not used by mock provider
		Temperature: 0.0,                // Deterministic for testing
		MaxTokens:   50,                 // Small tokens for fast tests
	}
}

// CreateTestAgentProviderConfig creates an agent.ProviderConfig for tests
func CreateTestAgentProviderConfig() core.ProviderConfig {
	testConfig := GetTestProviderConfig()
	return core.ProviderConfig{
		Provider: testConfig.Provider,
		Model:    testConfig.Model,
		APIKey:   testConfig.APIKey,
		APIURL:   testConfig.APIURL,
		Params: core.PromptParams{
			Temperature: testConfig.Temperature,
			MaxTokens:   testConfig.MaxTokens,
		},
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

func SetupWorkflowEnvironment(env *testsuite.TestWorkflowEnvironment, config *ContainerTestConfig) {
	runtime, err := runtime.NewRuntimeManager(config.ProjectConfig.GetCWD().PathStr(), runtime.WithTestConfig())
	if err != nil {
		panic(err)
	}
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*workflow.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
		runtime,
	)
	env.RegisterWorkflow(worker.CompozyWorkflow)
	env.RegisterActivity(activities.GetWorkflowData)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.ExecuteBasicTask)
	env.RegisterActivity(activities.CompleteWorkflow)
}
