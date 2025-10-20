package orchestrator

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseHandler_FinalizationRetries(t *testing.T) {
	ctx := t.Context()
	schemaDef := &schema.Schema{"type": "object"}
	action := &agent.ActionConfig{OutputSchema: schemaDef}
	settings := &settings{finalizeOutputRetries: 2}
	h := NewResponseHandler(settings)
	req := &llmadapter.LLMRequest{}
	state := newLoopState(settings, nil, action)
	request := Request{Action: action}
	resp := &llmadapter.LLMResponse{Content: "not-json"}

	_, cont, err := h.HandleNoToolCalls(ctx, resp, request, req, state)
	require.NoError(t, err)
	assert.True(t, cont)
	assert.Equal(t, 1, state.finalizeAttemptNumber())
	require.NotEmpty(t, req.Messages)
	msg := req.Messages[len(req.Messages)-1].Content
	assert.Contains(t, msg, "FINALIZATION_FEEDBACK")
	assert.Contains(t, msg, "Respond ONLY with a valid JSON object")

	_, cont, err = h.HandleNoToolCalls(ctx, resp, request, req, state)
	require.NoError(t, err)
	assert.True(t, cont)
	assert.Equal(t, 2, state.finalizeAttemptNumber())

	_, cont, err = h.HandleNoToolCalls(ctx, resp, request, req, state)
	require.Error(t, err)
	assert.False(t, cont)
	var coreErr *core.Error
	require.ErrorAs(t, err, &coreErr)
	assert.Equal(t, ErrCodeInvalidResponse, coreErr.Code)
}

func TestResponseHandler_ParseContent(t *testing.T) {
	h := NewResponseHandler(&settings{})
	t.Run("Should treat JSON-looking content as plain text when no schema", func(t *testing.T) {
		ctx := t.Context()
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
		assert.Equal(t, `{"k":"v"}`, (*output)["response"])
	})
	t.Run("Should accept top-level array when schema expects array", func(t *testing.T) {
		action := &agent.ActionConfig{OutputSchema: &schema.Schema{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		}}
		output, err := h.(*responseHandler).parseContent(t.Context(), `["a","b"]`, action)
		require.NoError(t, err)
		require.NotNil(t, output)
		values, ok := (*output)[core.OutputRootKey].([]any)
		require.True(t, ok)
		require.Len(t, values, 2)
		assert.Equal(t, "a", values[0])
		assert.Equal(t, "b", values[1])
	})
	t.Run("Should error when JSON required but got text", func(t *testing.T) {
		action := &agent.ActionConfig{OutputSchema: &schema.Schema{"type": "object"}}
		_, err := h.(*responseHandler).parseContent(t.Context(), "plain", action)
		require.Error(t, err)
		var coreErr *core.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeInvalidResponse, coreErr.Code)
		require.ErrorContains(t, err, "expected structured JSON output")
	})
	t.Run("Should extract embedded JSON when schema required", func(t *testing.T) {
		action := &agent.ActionConfig{OutputSchema: &schema.Schema{"type": "object"}}
		content := `Sure! Here is the result:\n\n{"pokemon":"Pikachu","confidence":0.82}`
		output, err := h.(*responseHandler).parseContent(t.Context(), content, action)
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Equal(t, "Pikachu", (*output)["pokemon"])
	})
	t.Run("Should extract embedded array when schema required", func(t *testing.T) {
		action := &agent.ActionConfig{OutputSchema: &schema.Schema{"type": "array"}}
		content := `Some text before [1,2,3] and after`
		output, err := h.(*responseHandler).parseContent(t.Context(), content, action)
		require.NoError(t, err)
		require.NotNil(t, output)
		values, ok := (*output)[core.OutputRootKey].([]any)
		require.True(t, ok)
		assert.ElementsMatch(t, []any{float64(1), float64(2), float64(3)}, values)
	})
}

func TestResponseHandler_TopLevelErrorRetries(t *testing.T) {
	ctx := t.Context()
	settings := &settings{finalizeOutputRetries: 2}
	h := NewResponseHandler(settings)
	req := &llmadapter.LLMRequest{}
	state := newLoopState(settings, nil, &agent.ActionConfig{})
	request := Request{Action: &agent.ActionConfig{}}
	resp := &llmadapter.LLMResponse{Content: `{"error":"bad"}`}

	_, cont, err := h.HandleNoToolCalls(ctx, resp, request, req, state)
	require.NoError(t, err)
	assert.True(t, cont)
	assert.Equal(t, 1, state.finalizeAttemptNumber())

	_, cont, err = h.HandleNoToolCalls(ctx, resp, request, req, state)
	require.NoError(t, err)
	assert.True(t, cont)
	assert.Equal(t, 2, state.finalizeAttemptNumber())

	_, cont, err = h.HandleNoToolCalls(ctx, resp, request, req, state)
	require.Error(t, err)
	assert.False(t, cont)
	var coreErr *core.Error
	require.ErrorAs(t, err, &coreErr)
	assert.Equal(t, ErrCodeOutputValidation, coreErr.Code)
}

func TestResponseHandler_FinalizationPlainTextFeedback(t *testing.T) {
	ctx := t.Context()
	settings := &settings{finalizeOutputRetries: 2}
	h := NewResponseHandler(settings)
	req := &llmadapter.LLMRequest{}
	state := newLoopState(settings, nil, nil)
	request := Request{}
	resp := &llmadapter.LLMResponse{Content: `{"error":"bad"}`}

	_, cont, err := h.HandleNoToolCalls(ctx, resp, request, req, state)
	require.NoError(t, err)
	assert.True(t, cont)
	msg := req.Messages[len(req.Messages)-1].Content
	assert.Contains(t, msg, "FINALIZATION_FEEDBACK")
	assert.Contains(t, msg, "Respond with a plain-text answer")
	assert.NotContains(t, msg, "valid JSON object")
}

func TestResponseHandler_FinalizationFeedbackReplaced(t *testing.T) {
	ctx := t.Context()
	settings := &settings{finalizeOutputRetries: 3}
	h := NewResponseHandler(settings)
	req := &llmadapter.LLMRequest{
		Messages: []llmadapter.Message{{Role: "user", Content: "hello"}},
	}
	state := newLoopState(settings, nil, &agent.ActionConfig{OutputSchema: &schema.Schema{"type": "object"}})
	request := Request{Action: state.actionConfig()}
	resp := &llmadapter.LLMResponse{Content: `not-json`}

	_, cont, err := h.HandleNoToolCalls(ctx, resp, request, req, state)
	require.NoError(t, err)
	require.True(t, cont)
	require.Len(t, req.Messages, 2)
	first := req.Messages[1].Content

	_, cont, err = h.HandleNoToolCalls(ctx, resp, request, req, state)
	require.NoError(t, err)
	require.True(t, cont)
	require.Len(t, req.Messages, 2)
	second := req.Messages[1].Content
	require.NotEqual(t, first, second)
}

func TestExtractJSONObject(t *testing.T) {
	t.Run("Should return first balanced object", func(t *testing.T) {
		jsonText := "prefix {\"a\":1,\"b\":{\"c\":2}} suffix"
		snippet, ok := extractJSONObject(jsonText)
		require.True(t, ok)
		assert.Equal(t, "{\"a\":1,\"b\":{\"c\":2}}", snippet)
	})
	t.Run("Should ignore braces inside strings", func(t *testing.T) {
		jsonText := "text {\"a\":\"{nested}\"} tail"
		snippet, ok := extractJSONObject(jsonText)
		require.True(t, ok)
		assert.Equal(t, "{\"a\":\"{nested}\"}", snippet)
	})
	t.Run("Should return first balanced array", func(t *testing.T) {
		jsonText := "pre [1,{\"x\":2}] post"
		snippet, ok := extractJSONObject(jsonText)
		require.True(t, ok)
		assert.Equal(t, "[1,{\"x\":2}]", snippet)
	})
	t.Run("Should return false when missing object", func(t *testing.T) {
		snippet, ok := extractJSONObject("no json here")
		require.False(t, ok)
		assert.Empty(t, snippet)
	})
}

func TestResponseHandler_ParseContent_SchemaError(t *testing.T) {
	h := NewResponseHandler(&settings{})
	sc := schema.Schema{
		"type":       "object",
		"properties": map[string]any{"x": map[string]any{"type": "string"}},
		"required":   []any{"x"},
	}
	action := &agent.ActionConfig{OutputSchema: &sc}
	_, err := h.(*responseHandler).parseContent(t.Context(), `{"x": 1}`, action)
	require.Error(t, err)
	require.ErrorContains(t, err, "schema validation failed")
	require.ErrorContains(t, err, "x")
}

func TestExtractTopLevelErrorMessage_Variants(t *testing.T) {
	t.Run("Should extract message field", func(t *testing.T) {
		msg, ok := extractTopLevelErrorMessage(`{"error":{"message":"abc"}}`)
		assert.True(t, ok)
		assert.Equal(t, "abc", msg)
	})
	t.Run("Should serialize map without message field", func(t *testing.T) {
		msg, ok := extractTopLevelErrorMessage(`{"error":{"code":1}}`)
		assert.True(t, ok)
		assert.Contains(t, msg, "code")
	})
	t.Run("Should stringify numeric top-level error value", func(t *testing.T) {
		msg, ok := extractTopLevelErrorMessage(`{"error": 123}`)
		assert.True(t, ok)
		assert.Equal(t, "123", msg)
	})
}
