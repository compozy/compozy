package webhook

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderTemplate(t *testing.T) {
	t.Run("Should render simple extraction", func(t *testing.T) {
		payload := map[string]any{
			"id":   "evt_123",
			"data": map[string]any{"object": map[string]any{"amount_total": 123}},
		}
		input := map[string]string{"event_id": "{{ .payload.id }}", "amount": "{{ .payload.data.object.amount_total }}"}
		out, err := RenderTemplate(context.Background(), RenderContext{Payload: payload}, input)
		require.NoError(t, err)
		assert.Equal(t, "evt_123", out["event_id"])
		assert.Equal(t, 123, out["amount"])
	})

	t.Run("Should support toJson function", func(t *testing.T) {
		payload := map[string]any{"id": "evt_456", "ok": true}
		input := map[string]string{"raw": "{{ .payload | toJson }}"}
		out, err := RenderTemplate(context.Background(), RenderContext{Payload: payload}, input)
		require.NoError(t, err)
		switch v := out["raw"].(type) {
		case string:
			assert.Contains(t, v, "\"id\":\"evt_456\"")
			assert.Contains(t, v, "\"ok\":true")
		case map[string]any:
			assert.Equal(t, "evt_456", v["id"])
			assert.Equal(t, true, v["ok"])
		default:
			t.Fatalf("unexpected type for raw: %T", v)
		}
	})

	t.Run("Should render missing fields as empty string", func(t *testing.T) {
		payload := map[string]any{"id": "evt_789"}
		input := map[string]string{"missing": "{{ .payload.missing_field }}"}
		out, err := RenderTemplate(context.Background(), RenderContext{Payload: payload}, input)
		require.NoError(t, err)
		assert.Equal(t, "", out["missing"])
	})
}

func TestValidateTemplate(t *testing.T) {
	t.Run("Should fail when schema validation fails", func(t *testing.T) {
		s := schema.Schema{
			"type":       "object",
			"properties": schema.Schema{"eventId": schema.Schema{"type": "string"}},
			"required":   []any{"eventId"},
		}
		err := ValidateTemplate(context.Background(), map[string]any{"other": "x"}, &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation failed")
	})

	t.Run("Should pass when schema matches", func(t *testing.T) {
		s := schema.Schema{
			"type":       "object",
			"properties": schema.Schema{"eventId": schema.Schema{"type": "string"}},
			"required":   []any{"eventId"},
		}
		err := ValidateTemplate(context.Background(), map[string]any{"eventId": "evt_123"}, &s)
		require.NoError(t, err)
	})
}
