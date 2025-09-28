package uc

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowedMCPIDs(t *testing.T) {
	t.Run("Should return union of agent and workflow MCP IDs, lowercased and deduped", func(t *testing.T) {
		exec := &ExecuteTask{}
		ag := &agent.Config{LLMProperties: agent.LLMProperties{MCPs: []mcp.Config{
			{ID: "FileSystem"},
			{ID: "github"},
		}}}
		wf := &workflow.Config{MCPs: []mcp.Config{
			{ID: "filesystem"},
			{ID: "Search"},
		}}
		ids := exec.allowedMCPIDs(ag, &ExecuteTaskInput{WorkflowConfig: wf})
		require.NotNil(t, ids)
		// Expect: filesystem, github, search (lowercased, deduped)
		assert.ElementsMatch(t, []string{"filesystem", "github", "search"}, ids)
	})

	t.Run("Should return nil when neither agent nor workflow declares MCPs", func(t *testing.T) {
		exec := &ExecuteTask{}
		agentCfg := &agent.Config{
			Model: agent.Model{Config: core.ProviderConfig{Provider: core.ProviderMock, Model: "mock-model"}},
		}
		ids := exec.allowedMCPIDs(agentCfg, &ExecuteTaskInput{WorkflowConfig: &workflow.Config{}})
		assert.Nil(t, ids)
	})
	t.Run("Should trim spaces and normalize casing", func(t *testing.T) {
		exec := &ExecuteTask{}
		ag := &agent.Config{LLMProperties: agent.LLMProperties{MCPs: []mcp.Config{{ID: "  FileSystem  "}}}}
		wf := &workflow.Config{MCPs: []mcp.Config{{ID: "FILESYSTEM"}}}
		ids := exec.allowedMCPIDs(ag, &ExecuteTaskInput{WorkflowConfig: wf})
		require.NotNil(t, ids)
		assert.ElementsMatch(t, []string{"filesystem"}, ids)
	})
}

func TestNormalizeProviderConfigWithEnv(t *testing.T) {
	t.Run("ShouldResolveTemplatesWithProjectEnv", func(t *testing.T) {
		exec := &ExecuteTask{templateEngine: tplengine.NewEngine(tplengine.FormatJSON)}
		providerCfg := core.ProviderConfig{
			Provider: core.ProviderGroq,
			Model:    "llama-3",
			APIKey:   "{{ .env.GROQ_API_KEY }}",
		}
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "direct-agent", Type: task.TaskTypeBasic}}
		projCfg := &project.Config{Name: "sync"}
		projCfg.SetEnv(core.EnvMap{"GROQ_API_KEY": "test-secret"})
		input := &ExecuteTaskInput{TaskConfig: taskCfg, ProjectConfig: projCfg}
		require.NoError(t, exec.normalizeProviderConfigWithEnv(context.Background(), &providerCfg, input))
		assert.Equal(t, "test-secret", providerCfg.APIKey)
		require.NotNil(t, taskCfg.Env)
		assert.Equal(t, "test-secret", (*taskCfg.Env)["GROQ_API_KEY"])
	})
}
