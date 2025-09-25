package orchestrator

import (
	"context"
	"fmt"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		var coreErr *enginecore.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeLLMCreation, coreErr.Code)
		assert.Equal(t, "openai", fmt.Sprint(coreErr.Details["provider"]))
		assert.Equal(t, "gpt-4o", fmt.Sprint(coreErr.Details["model"]))
	})
	t.Run("Should fail when agent configuration missing", func(t *testing.T) {
		_, err := cm.Create(context.Background(), Request{Action: act}, errFactory{err: assert.AnError})
		require.Error(t, err)
		var coreErr *enginecore.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeLLMCreation, coreErr.Code)
		assert.Equal(t, "nil agent config", coreErr.Details["reason"])
	})
	t.Run("Should log and ignore client close error", func(t *testing.T) {
		assert.NotPanics(t, func() { cm.Close(context.Background(), &errClient{}) })
	})
}
