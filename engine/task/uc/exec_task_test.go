package uc

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/workflow"
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
