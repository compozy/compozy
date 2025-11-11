package compozy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModeRuntimeStateCleanup(t *testing.T) {
	t.Parallel()
	t.Run("Should invoke cleanup functions exactly once and ignore nil entries", func(t *testing.T) {
		t.Parallel()
		state := &modeRuntimeState{}
		counter := 0
		state.addCleanup(func(context.Context) error {
			counter++
			return nil
		})
		state.addCleanup(nil)
		err := state.cleanup(t.Context())
		assert.NoError(t, err)
		assert.Equal(t, 1, counter)
	})
	t.Run("Should invoke cleanup on error paths and log failures", func(t *testing.T) {
		t.Parallel()
		state := &modeRuntimeState{}
		counter := 0
		state.addCleanup(func(context.Context) error {
			counter++
			return errors.New("failure")
		})
		state.cleanupOnError(t.Context())
		assert.Equal(t, 1, counter)
	})
}

func TestBootstrapModeUnsupported(t *testing.T) {
	t.Parallel()
	t.Run("Should return error for unsupported mode", func(t *testing.T) {
		t.Parallel()
		engine := &Engine{mode: Mode("legacy")}
		_, err := engine.bootstrapMode(t.Context(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported engine mode")
	})
}
