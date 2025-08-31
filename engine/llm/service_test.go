package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
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

// testFactory implements llmadapter.Factory for injecting a test client
type testFactory struct{ client llmadapter.LLMClient }

func (f testFactory) CreateClient(_ *core.ProviderConfig) (llmadapter.LLMClient, error) {
	return f.client, nil
}

// testClient wraps TestAdapter to satisfy LLMClient (adds Close)
type testClient struct{ *llmadapter.TestAdapter }

func (c testClient) Close() error { return nil }

func TestService_GenerateContent_DirectPrompt(t *testing.T) {
	t.Run("Should handle direct prompt without actionID", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		agentConfig.Instructions = "You are a helpful test agent"

		// Inject a test LLM client
		ta := llmadapter.NewTestAdapter()
		ta.SetResponse("Test response from direct prompt")
		service, err := NewService(
			context.Background(),
			runtimeMgr,
			agentConfig,
			WithLLMFactory(testFactory{client: testClient{ta}}),
		)
		require.NoError(t, err)

		out, err := service.GenerateContent(context.Background(), agentConfig, &core.Input{}, "", "Analyze this text")
		require.NoError(t, err)
		require.NotNil(t, out)
		// Direct prompt returns text response parsed into {"response": ...}
		assert.Equal(t, "Test response from direct prompt", (*out)["response"])
	})

	t.Run("Should error when both actionID and prompt are empty", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		agentConfig.Instructions = "Test agent"

		service, err := NewService(
			context.Background(),
			runtimeMgr,
			agentConfig,
			WithLLMFactory(testFactory{client: testClient{llmadapter.NewTestAdapter()}}),
		)
		require.NoError(t, err)

		_, err = service.GenerateContent(context.Background(), agentConfig, &core.Input{}, "", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "either actionID or directPrompt must be provided")
	})

	t.Run("Should work with actionID for backward compatibility", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		agentConfig.Instructions = "You are a helpful test agent"
		agentConfig.Actions = []*agent.ActionConfig{
			{ID: "analyze", Prompt: "Analyze input: {{ .input.text }}"},
		}

		ta := llmadapter.NewTestAdapter()
		ta.SetResponse(`{"ok":true}`)
		service, err := NewService(
			context.Background(),
			runtimeMgr,
			agentConfig,
			WithLLMFactory(testFactory{client: testClient{ta}}),
		)
		require.NoError(t, err)

		with := core.Input{"text": "hello"}
		out, err := service.GenerateContent(context.Background(), agentConfig, &with, "analyze", "")
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, true, (*out)["ok"])
	})

	t.Run("Should support combined action and prompt for enhanced context", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		agentConfig.Instructions = "You are a helpful test agent"
		agentConfig.Actions = []*agent.ActionConfig{
			{ID: "analyze", Prompt: "Analyze the data"},
		}

		ta := llmadapter.NewTestAdapter()
		ta.SetResponse(`{"enhanced":true, "focused":true}`)
		service, err := NewService(
			context.Background(),
			runtimeMgr,
			agentConfig,
			WithLLMFactory(testFactory{client: testClient{ta}}),
		)
		require.NoError(t, err)

		with := core.Input{"data": "test data"}
		// Provide both action and prompt for enhanced context
		out, err := service.GenerateContent(
			context.Background(),
			agentConfig,
			&with,
			"analyze",
			"Focus on security implications",
		)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, true, (*out)["enhanced"])
		assert.Equal(t, true, (*out)["focused"])
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
