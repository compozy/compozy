package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
)

type errFactory struct{ err error }

func (e errFactory) CreateClient(context.Context, *enginecore.ProviderConfig) (llmadapter.LLMClient, error) {
	return nil, e.err
}

type errClient struct{}

func (e *errClient) GenerateContent(context.Context, *llmadapter.LLMRequest) (*llmadapter.LLMResponse, error) {
	return nil, nil
}
func (e *errClient) Close() error { return assert.AnError }

func TestClientManager_CreateAndClose(t *testing.T) {
	cm := NewClientManager()
	ag := &agent.Config{Model: agent.Model{Config: enginecore.ProviderConfig{Provider: "openai", Model: "gpt-4o"}}}
	act := &agent.ActionConfig{ID: "a", Prompt: "p"}
	t.Run("Should wrap factory error", func(t *testing.T) {
		_, err := cm.Create(context.Background(), Request{Agent: ag, Action: act}, errFactory{err: assert.AnError})
		assert.Error(t, err)
	})
	t.Run("Should log and ignore client close error", func(t *testing.T) {
		assert.NotPanics(t, func() { cm.Close(context.Background(), &errClient{}) })
	})
}
