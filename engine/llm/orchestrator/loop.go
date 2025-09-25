package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	roleAssistant = "assistant"
	roleTool      = "tool"
)

type conversationLoop struct {
	cfg       settings
	tools     ToolExecutor
	responses ResponseHandler
	invoker   LLMInvoker
}

func newConversationLoop(
	cfg *settings,
	tools ToolExecutor,
	responses ResponseHandler,
	invoker LLMInvoker,
) *conversationLoop {
	if cfg == nil {
		cfg = &settings{}
	}
	return &conversationLoop{cfg: *cfg, tools: tools, responses: responses, invoker: invoker}
}

func (l *conversationLoop) Run(
	ctx context.Context,
	client llmadapter.LLMClient,
	llmReq *llmadapter.LLMRequest,
	request Request,
	state *loopState,
) (*core.Output, *llmadapter.LLMResponse, error) {
	for iter := 0; iter < l.cfg.maxToolIterations; iter++ {
		l.logLoopStart(ctx, request, llmReq, iter)

		response, err := l.invoker.Invoke(ctx, client, llmReq, request)
		if err != nil {
			return nil, nil, err
		}

		if len(response.ToolCalls) == 0 {
			output, cont, err := l.responses.HandleNoToolCalls(ctx, response, request, llmReq, state)
			if err != nil {
				return nil, nil, err
			}
			if cont {
				continue
			}
			return output, response, nil
		}

		llmReq.Messages = append(
			llmReq.Messages,
			llmadapter.Message{Role: roleAssistant, ToolCalls: response.ToolCalls},
		)

		toolResults, err := l.tools.Execute(ctx, response.ToolCalls)
		if err != nil {
			return nil, nil, err
		}

		if err := l.tools.UpdateBudgets(ctx, toolResults, state); err != nil {
			return nil, nil, err
		}

		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{Role: roleTool, ToolResults: toolResults})

		if l.cfg.enableProgressTracking {
			fingerprint := buildIterationFingerprint(response.ToolCalls, toolResults)
			if state.detectNoProgress(l.cfg.noProgressThreshold, fingerprint) {
				return nil, nil, fmt.Errorf("%w: %d consecutive iterations", ErrNoProgress, state.noProgressCount)
			}
		}
	}

	return nil, nil, fmt.Errorf("max tool iterations reached without final response")
}

func (l *conversationLoop) logLoopStart(ctx context.Context, request Request, llmReq *llmadapter.LLMRequest, iter int) {
	logger.FromContext(ctx).Debug(
		"Generating LLM response",
		"agent_id", request.Agent.ID,
		"action_id", request.Action.ID,
		"messages_count", len(llmReq.Messages),
		"tools_count", len(llmReq.Tools),
		"iteration", iter,
	)
}
