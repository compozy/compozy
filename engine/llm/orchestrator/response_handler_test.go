package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseHandler_JSONModeAndContentErrors(t *testing.T) {
	t.Run("Should continue loop when non-JSON in JSON mode", func(t *testing.T) {
		h := NewResponseHandler(&settings{maxSequentialToolErrors: 2})
		req := &llmadapter.LLMRequest{Options: llmadapter.CallOptions{UseJSONMode: true}}
		state := newLoopState(&settings{maxSequentialToolErrors: 2}, nil, nil)
		cont, err := h.(*responseHandler).handleJSONMode(context.Background(), "not-json", req, state)
		require.NoError(t, err)
		assert.True(t, cont)
		require.True(t, len(req.Messages) >= 2)
		assert.Equal(t, "assistant", req.Messages[len(req.Messages)-2].Role)
		assert.Equal(t, "tool", req.Messages[len(req.Messages)-1].Role)
	})
	t.Run("Should error after exceeding JSON-mode budget", func(t *testing.T) {
		h := NewResponseHandler(&settings{maxSequentialToolErrors: 2})
		req := &llmadapter.LLMRequest{Options: llmadapter.CallOptions{UseJSONMode: true}}
		state := newLoopState(&settings{maxSequentialToolErrors: 2}, nil, nil)
		// First attempt: continue
		cont, err := h.(*responseHandler).handleJSONMode(context.Background(), "still-not-json", req, state)
		require.NoError(t, err)
		assert.True(t, cont)
		// Second attempt: exceeds budget → error
		_, err = h.(*responseHandler).handleJSONMode(context.Background(), "still-not-json", req, state)
		require.Error(t, err)
	})
}

func TestResponseHandler_ParseContent(t *testing.T) {
	h := NewResponseHandler(&settings{})
	t.Run("Should parse JSON object to output map", func(t *testing.T) {
		out, err := h.(*responseHandler).parseContent(context.Background(), `{"k":"v"}`, &agent.ActionConfig{})
		require.NoError(t, err)
		assert.Equal(t, "v", (*out)["k"])
	})
	t.Run("Should error when JSON not an object", func(t *testing.T) {
		_, err := h.(*responseHandler).parseContent(context.Background(), `[]`, &agent.ActionConfig{})
		require.Error(t, err)
	})
	t.Run("Should error when JSON required but got text", func(t *testing.T) {
		action := &agent.ActionConfig{JSONMode: true}
		_, err := h.(*responseHandler).parseContent(context.Background(), "plain", action)
		require.Error(t, err)
	})
}

func TestResponseHandler_ContentErrorExtraction(t *testing.T) {
	t.Run("Should continue when top-level error present", func(t *testing.T) {
		h := NewResponseHandler(&settings{maxSequentialToolErrors: 2})
		req := &llmadapter.LLMRequest{}
		state := newLoopState(&settings{maxSequentialToolErrors: 2}, nil, nil)
		cont, err := h.(*responseHandler).handleContentError(context.Background(), `{"error":"bad"}`, req, state)
		require.NoError(t, err)
		assert.True(t, cont)
	})
	t.Run("Should error after budget exhausted", func(t *testing.T) {
		h := NewResponseHandler(&settings{maxSequentialToolErrors: 2})
		req := &llmadapter.LLMRequest{}
		state := newLoopState(&settings{maxSequentialToolErrors: 2}, nil, nil)
		// First attempt: continue
		cont, err := h.(*responseHandler).handleContentError(context.Background(), `{"error":"bad"}`, req, state)
		require.NoError(t, err)
		assert.True(t, cont)
		// Second attempt: exceeds budget → error
		_, err = h.(*responseHandler).handleContentError(context.Background(), `{"error":"bad"}`, req, state)
		require.Error(t, err)
	})
}
