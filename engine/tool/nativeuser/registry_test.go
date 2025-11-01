package nativeuser

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterAndLookup(t *testing.T) {
	Reset()
	ctx := t.Context()
	h := func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	}
	require.NoError(t, Register("test-tool", h))
	def, ok := Lookup("test-tool")
	require.True(t, ok)
	assert.Equal(t, "test-tool", def.ID)
	res, err := def.Handler(ctx, map[string]any{}, map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"ok": true}, res)
}

func TestRegisterValidation(t *testing.T) {
	Reset()
	assert.Equal(
		t,
		ErrInvalidID,
		Register("", func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
			return nil, nil
		}),
	)
	assert.Equal(t, ErrNilHandler, Register("tool", nil))
}

func TestRegisterDuplicate(t *testing.T) {
	Reset()
	h := func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
		return nil, nil
	}
	require.NoError(t, Register("dup", h))
	err := Register("dup", h)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAlreadyRegistered)
}

func TestRegisterConcurrent(t *testing.T) {
	Reset()
	var wg sync.WaitGroup
	ctx := t.Context()
	errCh := make(chan error, 25)
	for i := 0; i < 25; i++ {
		wg.Add(1)
		id := fmt.Sprintf("tool-%d", i)
		go func(toolID string) {
			defer wg.Done()
			h := func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
				return map[string]any{"id": toolID}, nil
			}
			errCh <- Register(toolID, h)
		}(id)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	ids := IDs()
	assert.Len(t, ids, 25)
	for _, id := range ids {
		def, ok := Lookup(id)
		require.True(t, ok)
		res, err := def.Handler(ctx, map[string]any{}, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, id, res["id"])
	}
}
