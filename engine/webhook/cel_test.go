package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/task"
)

func TestCELAdapter_Allow(t *testing.T) {
	eval, err := task.NewCELEvaluator()
	require.NoError(t, err)
	a := NewCELAdapter(eval)
	ctx := context.Background()

	t.Run("Should allow when expression is true", func(t *testing.T) {
		data := map[string]any{"payload": map[string]any{"status": "ok"}}
		allowed, err := a.Allow(ctx, "payload.status == 'ok'", data)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("Should reject when expression is false", func(t *testing.T) {
		data := map[string]any{"payload": map[string]any{"status": "fail"}}
		allowed, err := a.Allow(ctx, "payload.status == 'ok'", data)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("Should return error on invalid syntax", func(t *testing.T) {
		data := map[string]any{"payload": map[string]any{"status": "ok"}}
		_, err := a.Allow(ctx, "payload.status = 'ok'", data)
		require.Error(t, err)
		assert.ErrorContains(t, err, "CEL")
	})
}
