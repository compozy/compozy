package orchestrator

import (
	"context"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
)

type middlewareChain struct {
	items []Middleware
}

func newMiddlewareChain(items []Middleware) middlewareChain {
	if len(items) == 0 {
		return middlewareChain{}
	}
	clean := make([]Middleware, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		clean = append(clean, item)
	}
	return middlewareChain{items: clean}
}

func (c middlewareChain) beforeLLMRequest(
	ctx context.Context,
	loopCtx *LoopContext,
	req *llmadapter.LLMRequest,
) error {
	for _, mw := range c.items {
		if err := mw.BeforeLLMRequest(ctx, loopCtx, req); err != nil {
			return err
		}
	}
	return nil
}

func (c middlewareChain) afterLLMResponse(
	ctx context.Context,
	loopCtx *LoopContext,
	resp *llmadapter.LLMResponse,
) error {
	for _, mw := range c.items {
		if err := mw.AfterLLMResponse(ctx, loopCtx, resp); err != nil {
			return err
		}
	}
	return nil
}

func (c middlewareChain) beforeToolExecution(
	ctx context.Context,
	loopCtx *LoopContext,
	call *llmadapter.ToolCall,
) error {
	for _, mw := range c.items {
		if err := mw.BeforeToolExecution(ctx, loopCtx, call); err != nil {
			return err
		}
	}
	return nil
}

func (c middlewareChain) afterToolExecution(
	ctx context.Context,
	loopCtx *LoopContext,
	call *llmadapter.ToolCall,
	result *llmadapter.ToolResult,
) error {
	for _, mw := range c.items {
		if err := mw.AfterToolExecution(ctx, loopCtx, call, result); err != nil {
			return err
		}
	}
	return nil
}

func (c middlewareChain) beforeFinalize(ctx context.Context, loopCtx *LoopContext) error {
	for _, mw := range c.items {
		if err := mw.BeforeFinalize(ctx, loopCtx); err != nil {
			return err
		}
	}
	return nil
}
