package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/schema"
	"github.com/stretchr/testify/require"
)

func TestResponseHandler_ParseContent_SchemaError(t *testing.T) {
	h := NewResponseHandler(&settings{})
	sc := schema.Schema{
		"type":       "object",
		"properties": map[string]any{"x": map[string]any{"type": "string"}},
		"required":   []any{"x"},
	}
	action := &agent.ActionConfig{OutputSchema: &sc}
	_, err := h.(*responseHandler).parseContent(context.Background(), `{"x": 1}`, action)
	require.Error(t, err)
	require.ErrorContains(t, err, "schema validation failed")
	require.ErrorContains(t, err, "x")
}
