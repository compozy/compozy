package orchestrator

import (
	"context"

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

func (c *clientManager) Create(
	ctx context.Context,
	request Request,
	factory llmadapter.Factory,
) (llmadapter.LLMClient, error) {
	if factory == nil {
		factory = llmadapter.NewDefaultFactory()
	}
	client, err := factory.CreateClient(ctx, &request.Agent.Model.Config)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMCreation, map[string]any{
			"provider": request.Agent.Model.Config.Provider,
			"model":    request.Agent.Model.Config.Model,
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
