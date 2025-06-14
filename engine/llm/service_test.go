package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
)

func TestNewService(t *testing.T) {
	t.Run("Should create service with clean architecture", func(t *testing.T) {
		runtimeMgr := &runtime.Manager{}
		agentConfig := createTestAgentConfig()
		actionConfig := createTestActionConfig()

		service, err := NewService(runtimeMgr, agentConfig, actionConfig, nil)

		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.NotNil(t, service.orchestrator)
		assert.NotNil(t, service.config)
	})

	t.Run("Should handle MCP configurations", func(t *testing.T) {
		runtimeMgr := &runtime.Manager{}
		agentConfig := createTestAgentConfig()
		actionConfig := createTestActionConfig()
		mcpConfigs := []mcp.Config{
			{
				ID:        "test-mcp",
				URL:       "http://localhost:3000",
				Transport: "sse",
			},
		}

		service, err := NewService(runtimeMgr, agentConfig, actionConfig, mcpConfigs)

		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.NotNil(t, service.orchestrator)
		assert.NotNil(t, service.config)
	})
}

func TestService_InvalidateToolsCache(t *testing.T) {
	t.Run("Should handle cache invalidation", func(t *testing.T) {
		runtimeMgr := &runtime.Manager{}
		agentConfig := createTestAgentConfig()
		actionConfig := createTestActionConfig()

		service, err := NewService(runtimeMgr, agentConfig, actionConfig, nil)
		require.NoError(t, err)

		// Should not panic
		service.InvalidateToolsCache()
	})
}

func TestService_Close(t *testing.T) {
	t.Run("Should close without error", func(t *testing.T) {
		runtimeMgr := &runtime.Manager{}
		agentConfig := createTestAgentConfig()
		actionConfig := createTestActionConfig()

		service, err := NewService(runtimeMgr, agentConfig, actionConfig, nil)
		require.NoError(t, err)

		err = service.Close()
		assert.NoError(t, err)
	})
}

// Helper functions for testing
func createTestAgentConfig() *agent.Config {
	return &agent.Config{
		ID:           "test-agent",
		Instructions: "Test instructions",
		Tools:        []tool.Config{},
		MCPs:         []mcp.Config{},
		Config: core.ProviderConfig{
			Provider: "test",
			Model:    "test-model",
		},
	}
}

func createTestActionConfig() *agent.ActionConfig {
	input := core.NewInput(map[string]any{"test": "value"})
	return &agent.ActionConfig{
		ID:     "test-action",
		Prompt: "Test prompt",
		With:   &input,
	}
}
