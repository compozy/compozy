package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseHandler_JSONModeAndContentErrors(t *testing.T) {
	t.Run("Should continue loop when non-JSON in JSON mode", func(t *testing.T) {
		h := NewResponseHandler(&settings{maxSequentialToolErrors: 2})
		ctx := context.Background()
		req := &llmadapter.LLMRequest{Options: llmadapter.CallOptions{UseJSONMode: true}}
		state := newLoopState(&settings{maxSequentialToolErrors: 2}, nil, nil)
		request := Request{Action: &agent.ActionConfig{}}
		output, cont, err := h.HandleNoToolCalls(ctx, &llmadapter.LLMResponse{Content: "not-json"}, request, req, state)
		require.NoError(t, err)
		assert.Nil(t, output)
		assert.True(t, cont)
		require.GreaterOrEqual(t, len(req.Messages), 2)
		assert.Equal(t, llmadapter.RoleAssistant, req.Messages[len(req.Messages)-2].Role)
		assert.Equal(t, llmadapter.RoleTool, req.Messages[len(req.Messages)-1].Role)
	})
	t.Run("Should error after exceeding JSON-mode budget", func(t *testing.T) {
		h := NewResponseHandler(&settings{maxSequentialToolErrors: 2})
		ctx := context.Background()
		req := &llmadapter.LLMRequest{Options: llmadapter.CallOptions{UseJSONMode: true}}
		state := newLoopState(&settings{maxSequentialToolErrors: 2}, nil, nil)
		request := Request{Action: &agent.ActionConfig{}}
		_, cont, err := h.HandleNoToolCalls(
			ctx,
			&llmadapter.LLMResponse{Content: "still-not-json"},
			request,
			req,
			state,
		)
		require.NoError(t, err)
		assert.True(t, cont)
		_, cont, err = h.HandleNoToolCalls(ctx, &llmadapter.LLMResponse{Content: "still-not-json"}, request, req, state)
		require.False(t, cont)
		require.Error(t, err)
		var coreErr *core.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeBudgetExceeded, coreErr.Code)
	})
}

func TestResponseHandler_ParseContent(t *testing.T) {
	h := NewResponseHandler(&settings{})
	t.Run("Should parse JSON object to output map", func(t *testing.T) {
		ctx := context.Background()
		req := &llmadapter.LLMRequest{}
		state := newLoopState(&settings{}, nil, nil)
		request := Request{Action: &agent.ActionConfig{}}
		output, cont, err := h.HandleNoToolCalls(
			ctx,
			&llmadapter.LLMResponse{Content: `{"k":"v"}`},
			request,
			req,
			state,
		)
		require.NoError(t, err)
		assert.False(t, cont)
		require.NotNil(t, output)
		assert.Equal(t, "v", (*output)["k"])
	})
	t.Run("Should error when JSON not an object", func(t *testing.T) {
		_, err := h.(*responseHandler).parseContent(context.Background(), `[]`, &agent.ActionConfig{})
		require.Error(t, err)
		var coreErr *core.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeInvalidResponse, coreErr.Code)
		require.ErrorContains(t, err, "expected JSON object")
	})
	t.Run("Should error when JSON required but got text", func(t *testing.T) {
		action := &agent.ActionConfig{JSONMode: true}
		_, err := h.(*responseHandler).parseContent(context.Background(), "plain", action)
		require.Error(t, err)
		var coreErr *core.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeInvalidResponse, coreErr.Code)
		require.ErrorContains(t, err, "expected structured JSON output")
	})
}

func TestResponseHandler_ContentErrorExtraction(t *testing.T) {
	t.Run("Should continue when top-level error present", func(t *testing.T) {
		h := NewResponseHandler(&settings{maxSequentialToolErrors: 2})
		ctx := context.Background()
		req := &llmadapter.LLMRequest{}
		state := newLoopState(&settings{maxSequentialToolErrors: 2}, nil, nil)
		request := Request{Action: &agent.ActionConfig{}}
		output, cont, err := h.HandleNoToolCalls(
			ctx,
			&llmadapter.LLMResponse{Content: `{"error":"bad"}`},
			request,
			req,
			state,
		)
		require.NoError(t, err)
		assert.Nil(t, output)
		assert.True(t, cont)
	})
	t.Run("Should error after budget exhausted", func(t *testing.T) {
		h := NewResponseHandler(&settings{maxSequentialToolErrors: 2})
		ctx := context.Background()
		req := &llmadapter.LLMRequest{}
		state := newLoopState(&settings{maxSequentialToolErrors: 2}, nil, nil)
		request := Request{Action: &agent.ActionConfig{}}
		_, cont, err := h.HandleNoToolCalls(
			ctx,
			&llmadapter.LLMResponse{Content: `{"error":"bad"}`},
			request,
			req,
			state,
		)
		require.NoError(t, err)
		assert.True(t, cont)
		_, cont, err = h.HandleNoToolCalls(
			ctx,
			&llmadapter.LLMResponse{Content: `{"error":"bad"}`},
			request,
			req,
			state,
		)
		require.False(t, cont)
		require.Error(t, err)
		var coreErr *core.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeBudgetExceeded, coreErr.Code)
		require.NotNil(t, coreErr.Details)
		assert.Equal(t, "bad", coreErr.Details["details"])
	})
}
