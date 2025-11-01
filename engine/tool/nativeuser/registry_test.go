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
	t.Run("Should register tool and retrieve it via Lookup", func(t *testing.T) {
		Reset()
		t.Cleanup(Reset)
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
	})
}

func TestRegisterValidation(t *testing.T) {
	t.Run("Should reject empty ID", func(t *testing.T) {
		Reset()
		t.Cleanup(Reset)
		assert.Equal(
			t,
			ErrInvalidID,
			Register("", func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
				return nil, nil
			}),
		)
	})

	t.Run("Should reject nil handler", func(t *testing.T) {
		Reset()
		t.Cleanup(Reset)
		assert.Equal(t, ErrNilHandler, Register("tool", nil))
	})
}

func TestRegisterDuplicate(t *testing.T) {
	t.Run("Should reject duplicate registration", func(t *testing.T) {
		Reset()
		t.Cleanup(Reset)
		h := func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
			return nil, nil
		}
		require.NoError(t, Register("dup", h))
		err := Register("dup", h)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAlreadyRegistered)
	})
}

func TestRegisterConcurrent(t *testing.T) {
	t.Run("Should register handlers concurrently without conflicts", func(t *testing.T) {
		Reset()
		t.Cleanup(Reset)
		var wg sync.WaitGroup
		ctx := t.Context()
		errCh := make(chan error, 25)
		for i := 0; i < 25; i++ {
			toolID := fmt.Sprintf("tool-%d", i)
			wg.Go(func() {
				h := func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
					return map[string]any{"id": toolID}, nil
				}
				errCh <- Register(toolID, h)
			})
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
	})
}
