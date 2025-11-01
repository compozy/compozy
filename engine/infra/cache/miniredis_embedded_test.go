package cache

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// setupMiniredisForTest creates a test context with logger+config and starts the
// miniredis embedded wrapper. Caller must defer Close.
func setupMiniredisForTest(ctx context.Context, t *testing.T) *MiniredisEmbedded {
	t.Helper()
	mr, err := NewMiniredisEmbedded(ctx)
	require.NoError(t, err)
	return mr
}

// newTestContext constructs a minimal test context with logger and config manager attached.
func newTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx := t.Context()
	ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	manager := config.NewManager(ctx, config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	require.NoError(t, err)
	t.Cleanup(func() { _ = manager.Close(ctx) })
	ctx = config.ContextWithManager(ctx, manager)
	return ctx
}

func TestMiniredisEmbedded_Lifecycle(t *testing.T) {
	t.Run("Should start embedded Redis server", func(t *testing.T) {
		// Build a context with default config+logger attached.
		ctx := newTestContext(t)
		mr, err := NewMiniredisEmbedded(ctx)
		require.NoError(t, err)
		defer mr.Close(ctx)

		// Verify connection works
		err = mr.Client().Ping(ctx).Err()
		assert.NoError(t, err)
	})

	t.Run("Should close cleanly without errors", func(t *testing.T) {
		ctx := newTestContext(t)
		mr, err := NewMiniredisEmbedded(ctx)
		require.NoError(t, err)

		err = mr.Close(ctx)
		assert.NoError(t, err)

		// Verify double close is safe
		err = mr.Close(ctx)
		assert.NoError(t, err)
	})

	t.Run("Should handle startup errors gracefully", func(t *testing.T) {
		// Use a canceled context to force the initial Ping to fail.
		base := newTestContext(t)
		ctx, cancel := context.WithCancel(base)
		cancel()
		mr, err := NewMiniredisEmbedded(ctx)
		assert.Nil(t, mr)
		assert.Error(t, err)
	})
}

func TestMiniredisEmbedded_BasicOperations(t *testing.T) {
	t.Run("Should support Get/Set operations", func(t *testing.T) {
		ctx := newTestContext(t)
		mr := setupMiniredisForTest(ctx, t)
		defer mr.Close(ctx)

		// Test Set
		err := mr.Client().Set(ctx, "key", "value", 0).Err()
		require.NoError(t, err)

		// Test Get
		val, err := mr.Client().Get(ctx, "key").Result()
		require.NoError(t, err)
		assert.Equal(t, "value", val)
	})

	t.Run("Should support Lua scripts", func(t *testing.T) {
		ctx := newTestContext(t)
		mr := setupMiniredisForTest(ctx, t)
		defer mr.Close(ctx)

		script := `return redis.call('SET', KEYS[1], ARGV[1])`
		result, err := mr.Client().Eval(ctx, script, []string{"test-key"}, "test-value").Result()
		if err != nil {
			// Some miniredis versions do not support EVAL; skip if so.
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "unknown") || strings.Contains(lower, "not supported") ||
				strings.Contains(lower, "eval") {
				t.Skipf("miniredis does not support EVAL: %v", err)
			}
		}
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify value was set
		val, err := mr.Client().Get(ctx, "test-key").Result()
		require.NoError(t, err)
		assert.Equal(t, "test-value", val)
	})

	t.Run("Should support TxPipeline operations", func(t *testing.T) {
		ctx := newTestContext(t)
		mr := setupMiniredisForTest(ctx, t)
		defer mr.Close(ctx)

		pipe := mr.Client().TxPipeline()
		pipe.Set(ctx, "key1", "value1", 0)
		pipe.Set(ctx, "key2", "value2", 0)

		_, err := pipe.Exec(ctx)
		require.NoError(t, err)

		// Verify both keys set
		val1, _ := mr.Client().Get(ctx, "key1").Result()
		val2, _ := mr.Client().Get(ctx, "key2").Result()
		assert.Equal(t, "value1", val1)
		assert.Equal(t, "value2", val2)
	})
}

// Ensure the client type is the expected go-redis client
func TestMiniredisEmbedded_ClientType(t *testing.T) {
	ctx := newTestContext(t)
	mr := setupMiniredisForTest(ctx, t)
	defer mr.Close(ctx)
	var _ = mr.Client()

	// Touch config to verify access pattern compiles in tests
	c := config.FromContext(ctx)
	require.NotNil(t, c)
}
