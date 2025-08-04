package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/tool"
)

// mockRuntime is a mock implementation of runtime.Runtime for testing
type mockRuntime struct{}

func (m *mockRuntime) ExecuteTool(
	_ context.Context,
	_ string,
	_ core.ID,
	_ *core.Input,
	_ *core.Input,
	_ core.EnvMap,
) (*core.Output, error) {
	return &core.Output{}, nil
}

func (m *mockRuntime) ExecuteToolWithTimeout(
	_ context.Context,
	_ string,
	_ core.ID,
	_ *core.Input,
	_ *core.Input,
	_ core.EnvMap,
	_ time.Duration,
) (*core.Output, error) {
	return &core.Output{}, nil
}

func (m *mockRuntime) GetGlobalTimeout() time.Duration {
	return 60 * time.Second
}

func TestNewService(t *testing.T) {
	t.Run("Should create service with proper timeout configuration", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		customTimeout := 45 * time.Second

		service, err := NewService(context.Background(), runtimeMgr, agentConfig, WithTimeout(customTimeout))

		require.NoError(t, err)
		assert.Equal(t, customTimeout, service.config.Timeout)
		assert.True(t, service.config.EnableStructuredOutput) // Default should be true
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
