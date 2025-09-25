package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticPromptBuilder struct{}

func (staticPromptBuilder) Build(_ context.Context, action *agent.ActionConfig) (string, error) {
	return action.Prompt, nil
}

func (staticPromptBuilder) EnhanceForStructuredOutput(
	_ context.Context,
	prompt string,
	_ *schema.Schema,
	_ bool,
) string {
	return prompt
}

func (staticPromptBuilder) ShouldUseStructuredOutput(
	_ string,
	_ *agent.ActionConfig,
	_ []tool.Config,
) bool {
	return false
}

type noopToolRegistry struct{}

func (noopToolRegistry) Find(context.Context, string) (RegistryTool, bool) {
	return nil, false
}

func (noopToolRegistry) ListAll(context.Context) ([]RegistryTool, error) {
	return nil, nil
}

func (noopToolRegistry) Close() error { return nil }

type stubLLMFactory struct{ client llmadapter.LLMClient }

func (f stubLLMFactory) CreateClient(
	_ context.Context,
	_ *enginecore.ProviderConfig,
) (llmadapter.LLMClient, error) {
	return f.client, nil
}

type stubLLMClient struct {
	response *llmadapter.LLMResponse
}

func (s *stubLLMClient) GenerateContent(
	_ context.Context,
	_ *llmadapter.LLMRequest,
) (*llmadapter.LLMResponse, error) {
	return s.response, nil
}

func (s *stubLLMClient) Close() error { return nil }

func TestOrchestrator_Execute(t *testing.T) {
	t.Run("Should return final content when LLM responds without tool calls", func(t *testing.T) {
		client := &stubLLMClient{response: &llmadapter.LLMResponse{Content: "hello"}}
		orc, err := New(Config{
			ToolRegistry:       noopToolRegistry{},
			PromptBuilder:      staticPromptBuilder{},
			LLMFactory:         stubLLMFactory{client: client},
			MemoryProvider:     nil,
			MemorySync:         nil,
			Timeout:            0,
			MaxConcurrentTools: 1,
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = orc.Close() })

		ag := &agent.Config{
			ID:           "agent-1",
			Instructions: "instruct",
			Model:        agent.Model{Config: enginecore.ProviderConfig{Provider: "openai", Model: "gpt"}},
		}
		action := &agent.ActionConfig{ID: "action-1", Prompt: "Say hi"}

		result, err := orc.Execute(context.Background(), Request{Agent: ag, Action: action})
		require.NoError(t, err)

		assert.Equal(t, "hello", (*result)["response"])
	})
}
