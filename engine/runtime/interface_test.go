package runtime_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRuntime is a mock implementation of the Runtime interface for testing
type MockRuntime struct {
	ExecuteToolFunc            func(ctx context.Context, toolID string, toolExecID core.ID, input *core.Input, env core.EnvMap) (*core.Output, error)
	ExecuteToolWithTimeoutFunc func(ctx context.Context, toolID string, toolExecID core.ID, input *core.Input, env core.EnvMap, timeout time.Duration) (*core.Output, error)
	GetGlobalTimeoutFunc       func() time.Duration
}

func (m *MockRuntime) ExecuteTool(
	ctx context.Context,
	toolID string,
	toolExecID core.ID,
	input *core.Input,
	env core.EnvMap,
) (*core.Output, error) {
	if m.ExecuteToolFunc != nil {
		return m.ExecuteToolFunc(ctx, toolID, toolExecID, input, env)
	}
	return &core.Output{}, nil
}

func (m *MockRuntime) ExecuteToolWithTimeout(
	ctx context.Context,
	toolID string,
	toolExecID core.ID,
	input *core.Input,
	env core.EnvMap,
	timeout time.Duration,
) (*core.Output, error) {
	if m.ExecuteToolWithTimeoutFunc != nil {
		return m.ExecuteToolWithTimeoutFunc(ctx, toolID, toolExecID, input, env, timeout)
	}
	return &core.Output{}, nil
}

func (m *MockRuntime) GetGlobalTimeout() time.Duration {
	if m.GetGlobalTimeoutFunc != nil {
		return m.GetGlobalTimeoutFunc()
	}
	return 60 * time.Second
}

func TestRuntimeInterface(t *testing.T) {
	t.Run("Should verify Manager implements Runtime interface", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir, err := os.MkdirTemp("", "runtime-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// This test verifies compile-time check in interface.go
		ctx := context.Background()

		// Create a Manager instance
		manager, err := runtime.NewRuntimeManager(ctx, tmpDir)
		require.NoError(t, err)

		// Verify it implements the Runtime interface
		var rt runtime.Runtime = manager
		assert.NotNil(t, rt)
	})

	t.Run("Should demonstrate Runtime interface usage", func(t *testing.T) {
		// Create a mock runtime
		mockOutput := &core.Output{"result": "test"}
		mockTimeout := 30 * time.Second

		mock := &MockRuntime{
			ExecuteToolFunc: func(_ context.Context, toolID string, _ core.ID, _ *core.Input, _ core.EnvMap) (*core.Output, error) {
				assert.Equal(t, "test-tool", toolID)
				return mockOutput, nil
			},
			ExecuteToolWithTimeoutFunc: func(_ context.Context, toolID string, _ core.ID, _ *core.Input, _ core.EnvMap, timeout time.Duration) (*core.Output, error) {
				assert.Equal(t, "test-tool-timeout", toolID)
				assert.Equal(t, mockTimeout, timeout)
				return mockOutput, nil
			},
			GetGlobalTimeoutFunc: func() time.Duration {
				return mockTimeout
			},
		}

		// Use the mock as a Runtime
		var rt runtime.Runtime = mock
		ctx := context.Background()
		toolExecID, _ := core.NewID()
		input := &core.Input{}
		env := core.EnvMap{}

		// Test ExecuteTool
		output, err := rt.ExecuteTool(ctx, "test-tool", toolExecID, input, env)
		require.NoError(t, err)
		assert.Equal(t, mockOutput, output)

		// Test ExecuteToolWithTimeout
		output, err = rt.ExecuteToolWithTimeout(ctx, "test-tool-timeout", toolExecID, input, env, mockTimeout)
		require.NoError(t, err)
		assert.Equal(t, mockOutput, output)

		// Test GetGlobalTimeout
		timeout := rt.GetGlobalTimeout()
		assert.Equal(t, mockTimeout, timeout)
	})
}
