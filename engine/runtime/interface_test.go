package runtime_test

import (
	"os"
	"testing"

	"github.com/compozy/compozy/engine/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeInterface(t *testing.T) {
	t.Run("Should verify BunManager implements Runtime interface", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir, err := os.MkdirTemp("", "runtime-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// This test verifies compile-time check in interface.go
		ctx := t.Context()

		// Create a Runtime instance using factory
		config := runtime.TestConfig()
		factory := runtime.NewDefaultFactory(tmpDir)
		manager, err := factory.CreateRuntime(ctx, config)
		require.NoError(t, err)

		// Verify it implements the Runtime interface
		var rt = manager
		assert.NotNil(t, rt)
	})
}
