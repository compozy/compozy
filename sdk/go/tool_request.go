package aghsdk

import (
	"context"
	"encoding/json"
	"strings"
)

// ToolRequest is passed to a typed tool handler.
type ToolRequest[TInput any] struct {
	Input    TInput
	RawInput json.RawMessage
	Context  ExtensionContext
	Host     *HostAPI
	Session  ExtensionSession
	ToolID   ToolID
	Handler  string

	invocationID string
}

// ToolHandlerFunc handles one typed tool request.
type ToolHandlerFunc[TInput any] func(context.Context, ToolRequest[TInput]) (ToolResult, error)

type rawToolRequest struct {
	input        json.RawMessage
	context      ExtensionContext
	toolID       ToolID
	handler      string
	invocationID string
}

type clarifyAskWireParams struct {
	InvocationID string   `json:"invocation_id"`
	Question     string   `json:"question"`
	Choices      []string `json:"choices,omitempty"`
}

// AskClarification blocks this tool call until an operator answers or the daemon timeout expires.
func (r ToolRequest[TInput]) AskClarification(
	ctx context.Context,
	question ClarifyQuestion,
) (ClarifyAnswer, error) {
	if r.Host == nil {
		return ClarifyAnswer{}, NewNotInitializedError()
	}
	invocationID := strings.TrimSpace(r.invocationID)
	if invocationID == "" {
		return ClarifyAnswer{}, NewInvalidParamsError("tool invocation context is required", nil)
	}
	var answer ClarifyAnswer
	err := r.Host.Request(ctx, HostAPIMethodClarifyAsk, clarifyAskWireParams{
		InvocationID: invocationID,
		Question:     question.Question,
		Choices:      append([]string(nil), question.Choices...),
	}, &answer)
	if err != nil {
		return ClarifyAnswer{}, err
	}
	return answer, nil
}
