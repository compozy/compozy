package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/pkg/logger"
)

type ClientManager interface {
	Create(ctx context.Context, request Request, factory llmadapter.Factory) (llmadapter.LLMClient, error)
	Close(ctx context.Context, client llmadapter.LLMClient)
}

type clientManager struct{}

func NewClientManager() ClientManager {
	return &clientManager{}
}

//nolint:gocritic // Request is passed by value to preserve immutability guarantees for downstream components.
func (c *clientManager) Create(
	ctx context.Context,
	request Request,
	factory llmadapter.Factory,
) (llmadapter.LLMClient, error) {
	if factory == nil {
		var err error
		factory, err = llmadapter.NewDefaultFactory(ctx)
		if err != nil {
			return nil, NewLLMError(
				fmt.Errorf("failed to build default LLM factory: %w", err),
				ErrCodeLLMCreation,
				map[string]any{"reason": "factory_initialization"},
			)
		}
	}
	if request.Agent == nil {
		return nil, NewLLMError(fmt.Errorf("agent configuration is nil"), ErrCodeLLMCreation, map[string]any{
			"reason": "nil agent config",
		})
	}
	cfg := &request.Agent.Model.Config
	client, err := factory.CreateClient(ctx, cfg)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMCreation, map[string]any{
			"provider": cfg.Provider,
			"model":    cfg.Model,
		})
	}
	return client, nil
}

func (c *clientManager) Close(ctx context.Context, client llmadapter.LLMClient) {
	if client == nil {
		return
	}
	if err := client.Close(); err != nil {
		logger.FromContext(ctx).Error("Failed to close LLM client", "error", core.RedactError(err))
	}
}
