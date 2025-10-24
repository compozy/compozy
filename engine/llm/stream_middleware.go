package llm

import (
	"context"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	orchestratorpkg "github.com/compozy/compozy/engine/llm/orchestrator"
)

type streamMiddleware struct{}

func newStreamMiddleware() orchestratorpkg.Middleware {
	return streamMiddleware{}
}

func (streamMiddleware) BeforeLLMRequest(
	ctx context.Context,
	_ *orchestratorpkg.LoopContext,
	req *llmadapter.LLMRequest,
) error {
	session, ok := streamSessionFromContext(ctx)
	if !ok || session == nil || req == nil {
		return nil
	}
	if req.Options.StreamingHandler == nil {
		req.Options.StreamingHandler = session.streamingHandler
	}
	return nil
}

func (streamMiddleware) AfterLLMResponse(context.Context, *orchestratorpkg.LoopContext, *llmadapter.LLMResponse) error {
	return nil
}

func (streamMiddleware) BeforeToolExecution(
	context.Context,
	*orchestratorpkg.LoopContext,
	*llmadapter.ToolCall,
) error {
	return nil
}

func (streamMiddleware) AfterToolExecution(
	context.Context,
	*orchestratorpkg.LoopContext,
	*llmadapter.ToolCall,
	*llmadapter.ToolResult,
) error {
	return nil
}

func (streamMiddleware) BeforeFinalize(context.Context, *orchestratorpkg.LoopContext) error {
	return nil
}
