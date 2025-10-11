package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticPromptBuilder struct{}

type staticTemplateState struct {
	prompt string
}

type systemRendererStub struct{}

//nolint:gocritic // Test stub matches orchestrator PromptBuilder interface signature.
func (staticPromptBuilder) Build(_ context.Context, input PromptBuildInput) (PromptBuildResult, error) {
	if input.Action == nil {
		return PromptBuildResult{}, fmt.Errorf("action config is required")
	}
	prompt := input.Action.Prompt
	return PromptBuildResult{
		Prompt:   prompt,
		Format:   llmadapter.DefaultOutputFormat(),
		Template: staticTemplateState{prompt: prompt},
		Context:  PromptDynamicContext{},
	}, nil
}

func (s staticTemplateState) Render(context.Context, PromptDynamicContext) (string, error) {
	return s.prompt, nil
}

func (systemRendererStub) Render(ctx context.Context, instructions string) (string, error) {
	return composeSystemPromptFallback(ctx, instructions), nil
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

func (f stubLLMFactory) BuildRoute(
	cfg *enginecore.ProviderConfig,
	fallbacks ...*enginecore.ProviderConfig,
) (*llmadapter.ProviderRoute, error) {
	if cfg == nil {
		return nil, fmt.Errorf("provider config must not be nil")
	}
	registry := llmadapter.NewProviderRegistry()
	stub := &testProvider{name: cfg.Provider, client: f.client}
	if err := registry.Register(stub); err != nil {
		return nil, err
	}
	return registry.BuildRoute(cfg, fallbacks...)
}

func (f stubLLMFactory) Capabilities(name enginecore.ProviderName) (llmadapter.ProviderCapabilities, error) {
	if name == enginecore.ProviderOpenAI || name == enginecore.ProviderXAI {
		return llmadapter.ProviderCapabilities{StructuredOutput: true, Streaming: true}, nil
	}
	return llmadapter.ProviderCapabilities{}, nil
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

type testProvider struct {
	name   enginecore.ProviderName
	client llmadapter.LLMClient
}

func (p *testProvider) Name() enginecore.ProviderName { return p.name }

func (p *testProvider) Capabilities() llmadapter.ProviderCapabilities {
	return llmadapter.ProviderCapabilities{}
}

func (p *testProvider) NewClient(context.Context, *enginecore.ProviderConfig) (llmadapter.LLMClient, error) {
	return p.client, nil
}

func TestOrchestrator_Execute(t *testing.T) {
	t.Run("Should return final content when LLM responds without tool calls", func(t *testing.T) {
		client := &stubLLMClient{response: &llmadapter.LLMResponse{Content: "hello"}}
		orc, err := New(Config{
			ToolRegistry:         noopToolRegistry{},
			PromptBuilder:        staticPromptBuilder{},
			SystemPromptRenderer: systemRendererStub{},
			LLMFactory:           stubLLMFactory{client: client},
			MemoryProvider:       nil,
			MemorySync:           nil,
			Timeout:              0,
			MaxConcurrentTools:   1,
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

type closableRegistry struct{ closeErr error }

func (c closableRegistry) Find(_ context.Context, _ string) (RegistryTool, bool) { return nil, false }
func (c closableRegistry) ListAll(_ context.Context) ([]RegistryTool, error)     { return nil, nil }
func (c closableRegistry) Close() error                                          { return c.closeErr }

type promptNoop struct{}

type systemNoop struct{}

//nolint:gocritic // Test stub matches orchestrator PromptBuilder interface signature.
func (promptNoop) Build(_ context.Context, input PromptBuildInput) (PromptBuildResult, error) {
	if input.Action == nil {
		return PromptBuildResult{}, errors.New("action is required")
	}
	return PromptBuildResult{
		Prompt:   input.Action.Prompt,
		Format:   llmadapter.DefaultOutputFormat(),
		Template: noopTemplateState(input.Action.Prompt),
		Context:  PromptDynamicContext{},
	}, nil
}

type noopTemplateState string

func (s noopTemplateState) Render(context.Context, PromptDynamicContext) (string, error) {
	return string(s), nil
}

func (systemNoop) Render(ctx context.Context, instructions string) (string, error) {
	return composeSystemPromptFallback(ctx, instructions), nil
}

func TestOrchestrator_Close_ErrorPropagation(t *testing.T) {
	t.Run("Should propagate error from registry Close", func(t *testing.T) {
		factory, err := llmadapter.NewDefaultFactory(context.Background())
		require.NoError(t, err)
		orc, err := New(Config{
			ToolRegistry:         closableRegistry{closeErr: errors.New("bye")},
			PromptBuilder:        promptNoop{},
			SystemPromptRenderer: systemNoop{},
			LLMFactory:           factory,
		})
		require.NoError(t, err)
		assert.Error(t, orc.Close())
	})
}

func TestOrchestrator_Close_NoRegistry(t *testing.T) {
	t.Run("Should return nil when registry Close succeeds", func(t *testing.T) {
		factory, err := llmadapter.NewDefaultFactory(context.Background())
		require.NoError(t, err)
		orc, err := New(Config{
			ToolRegistry:         closableRegistry{closeErr: nil},
			PromptBuilder:        promptNoop{},
			SystemPromptRenderer: systemNoop{},
			LLMFactory:           factory,
		})
		require.NoError(t, err)
		assert.NoError(t, orc.Close())
	})
}
