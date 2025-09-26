package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
)

type orchestrator struct {
	cfg      Config
	settings settings

	memory  MemoryManager
	builder RequestBuilder
	client  ClientManager
	loop    *conversationLoop
}

//nolint:gocritic // Config must be passed by value to ensure defensive copy semantics.
func New(cfg Config) (Orchestrator, error) {
	if cfg.PromptBuilder == nil {
		return nil, core.NewError(
			fmt.Errorf("prompt builder cannot be nil"),
			ErrCodeInvalidConfig,
			map[string]any{"field": "PromptBuilder"},
		)
	}
	if cfg.ToolRegistry == nil {
		return nil, core.NewError(
			fmt.Errorf("tool registry cannot be nil"),
			ErrCodeInvalidConfig,
			map[string]any{"field": "ToolRegistry"},
		)
	}
	if cfg.LLMFactory == nil {
		return nil, core.NewError(
			fmt.Errorf("llm factory cannot be nil"),
			ErrCodeInvalidConfig,
			map[string]any{"field": "LLMFactory"},
		)
	}
	cfgCopy := cfg
	settings := buildSettings(&cfgCopy)
	memoryManager := NewMemoryManager(cfgCopy.MemoryProvider, cfgCopy.MemorySync, cfgCopy.AsyncHook)
	reqBuilder := NewRequestBuilder(cfgCopy.PromptBuilder, cfgCopy.ToolRegistry, memoryManager)
	toolExec := NewToolExecutor(cfgCopy.ToolRegistry, &settings)
	responseHandler := NewResponseHandler(&settings)
	llmInvoker := NewLLMInvoker(&settings)
	conv := newConversationLoop(&settings, toolExec, responseHandler, llmInvoker, memoryManager)
	return &orchestrator{
		cfg:      cfgCopy,
		settings: settings,
		memory:   memoryManager,
		builder:  reqBuilder,
		client:   NewClientManager(),
		loop:     conv,
	}, nil
}

func (o *orchestrator) Execute(ctx context.Context, request Request) (*core.Output, error) {
	if err := validateRequest(ctx, request); err != nil {
		return nil, err
	}
	memoryCtx := o.memory.Prepare(ctx, request)
	llmReq, err := o.builder.Build(ctx, request, memoryCtx)
	if err != nil {
		return nil, err
	}
	client, err := o.client.Create(ctx, request, o.cfg.LLMFactory)
	if err != nil {
		return nil, err
	}
	defer o.client.Close(ctx, client)
	state := newLoopState(&o.settings, memoryCtx, request.Action)
	output, response, err := o.loop.Run(ctx, client, &llmReq, request, state)
	if err != nil {
		return nil, err
	}
	if o.loop.memory == nil {
		o.memory.StoreAsync(ctx, memoryCtx, response, llmReq.Messages, request)
	}
	return output, nil
}

func (o *orchestrator) Close() error {
	if o.cfg.ToolRegistry != nil {
		return o.cfg.ToolRegistry.Close()
	}
	return nil
}

func validateRequest(ctx context.Context, request Request) error {
	if request.Agent == nil {
		return NewValidationError(fmt.Errorf("agent config is required"), "agent", nil)
	}
	if request.Action == nil {
		return NewValidationError(fmt.Errorf("action config is required"), "action", nil)
	}
	if strings.TrimSpace(request.Agent.ID) == "" {
		return NewValidationError(fmt.Errorf("agent ID is required"), "agent.id", nil)
	}
	if strings.TrimSpace(request.Action.ID) == "" {
		return NewValidationError(fmt.Errorf("action ID is required"), "action.id", nil)
	}
	if strings.TrimSpace(request.Agent.Instructions) == "" {
		return NewValidationError(fmt.Errorf("agent instructions are required"), "agent.instructions", nil)
	}
	if strings.TrimSpace(request.Action.Prompt) == "" {
		return NewValidationError(fmt.Errorf("action prompt is required"), "action.prompt", nil)
	}
	if request.Action.InputSchema != nil {
		if err := request.Action.ValidateInput(ctx, request.Action.GetInput()); err != nil {
			return NewValidationError(
				fmt.Errorf("input validation failed: %w", err),
				"action.input",
				request.Action.GetInput(),
			)
		}
	}
	return nil
}
