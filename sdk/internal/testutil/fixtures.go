package testutil

import (
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// NewTestModel returns a minimal provider configuration suitable for use in tests.
func NewTestModel(provider, model string) *core.ProviderConfig {
	trimmedProvider := strings.TrimSpace(provider)
	if trimmedProvider == "" {
		trimmedProvider = string(core.ProviderOpenAI)
	}
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		trimmedModel = "gpt-4o-mini"
	}
	return &core.ProviderConfig{
		Provider: core.ProviderName(strings.ToLower(trimmedProvider)),
		Model:    trimmedModel,
		APIKey:   "{{ .env.TEST_API_KEY }}",
	}
}

// NewTestAgent creates an agent configuration with default instructions and inline model config.
func NewTestAgent(id string) *agent.Config {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		trimmedID = "test-agent"
	}
	model := NewTestModel(string(core.ProviderOpenAI), "gpt-4o-mini")
	cfg := &agent.Config{
		ID:           trimmedID,
		Instructions: "You are a reliable assistant used for automated tests.",
		Model: agent.Model{
			Config: *model,
		},
	}
	_ = cfg.SetCWD(repoRoot)
	return cfg
}

// NewTestWorkflow constructs a workflow configuration with a single basic task referencing a test agent.
func NewTestWorkflow(id string) *workflow.Config {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		trimmedID = "test-workflow"
	}
	agentCfg := NewTestAgent(trimmedID + "-agent")
	withInput := make(core.Input)
	taskCfg := task.Config{
		BaseConfig: task.BaseConfig{
			ID:    trimmedID + "-task",
			Type:  task.TaskTypeBasic,
			Agent: agentCfg,
			With:  &withInput,
		},
		BasicTask: task.BasicTask{Prompt: "Summarize the workflow input."},
	}
	if cwd, err := core.CWDFromPath(repoRoot); err == nil {
		taskCfg.CWD = cwd
	}
	wf := &workflow.Config{
		ID:      trimmedID,
		Version: "1.0.0",
		Agents:  []agent.Config{*agentCfg},
		Tasks:   []task.Config{taskCfg},
	}
	if cwd, err := core.CWDFromPath(repoRoot); err == nil {
		wf.CWD = cwd
	}
	return wf
}
