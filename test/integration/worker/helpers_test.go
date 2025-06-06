package worker

import (
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	utils "github.com/compozy/compozy/test/integration/helper"
	"go.temporal.io/sdk/testsuite"
)

// CreateTestAgentProviderConfig creates an agent.ProviderConfig for tests
func CreateTestAgentProviderConfig() core.ProviderConfig {
	raw := utils.GetTestProviderConfig()
	return core.ProviderConfig{
		Provider: raw.Provider,
		Model:    raw.Model,
		APIKey:   raw.APIKey,
		APIURL:   raw.APIURL,
		Params: core.PromptParams{
			Temperature: float64(raw.Temperature),
			MaxTokens:   raw.MaxTokens,
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
	env.RegisterActivity(activities.DispatchTask)
	env.RegisterActivity(activities.ExecuteBasicTask)
	env.RegisterActivity(activities.CompleteWorkflow)
}
