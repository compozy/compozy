package fetch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestContext(t *testing.T, configure func(*config.Config)) context.Context {
	t.Helper()
	ctx := context.Background()
	ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
	manager := config.NewManager(config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := manager.Get()
	if configure != nil {
		configure(cfg)
	}
	return config.ContextWithManager(ctx, manager)
}

func invoke(ctx context.Context, t *testing.T, payload map[string]any) (core.Output, *core.Error) {
	t.Helper()
	handler := Definition().Handler
	output, err := handler(ctx, payload)
	if err == nil {
		return output, nil
	}
	var coreErr *core.Error
	if errors.As(err, &coreErr) {
		return nil, coreErr
	}
	t.Fatalf("expected core.Error, got %v", err)
	return nil, nil
}

func TestFetchHandler(t *testing.T) {
	t.Run("Should execute GET request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Test", "true")
			_, _ = w.Write([]byte("hello"))
		}))
		t.Cleanup(server.Close)
		ctx := newTestContext(t, nil)
		output, coreErr := invoke(ctx, t, map[string]any{
			"url": server.URL,
		})
		require.Nil(t, coreErr)
		assert.Equal(t, 200, output["status_code"])
		assert.Equal(t, "true", output["headers"].(map[string]string)["X-Test"])
		assert.Equal(t, "hello", output["body"].(string))
	})

	t.Run("Should reject disallowed method", func(t *testing.T) {
		ctx := newTestContext(t, nil)
		_, coreErr := invoke(ctx, t, map[string]any{
			"url":    "https://example.com",
			"method": "TRACE",
		})
		require.NotNil(t, coreErr)
		assert.Equal(t, builtin.CodeInvalidArgument, coreErr.Code)
	})

	t.Run("Should enforce timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(200 * time.Millisecond)
			_, _ = w.Write([]byte("slow"))
		}))
		t.Cleanup(server.Close)
		ctx := newTestContext(t, nil)
		_, coreErr := invoke(ctx, t, map[string]any{
			"url":        server.URL,
			"timeout_ms": 50,
		})
		require.NotNil(t, coreErr)
		assert.Equal(t, builtin.CodeInternal, coreErr.Code)
		assert.Equal(t, true, coreErr.Details["timeout"])
	})

	t.Run("Should truncate large responses", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("abcdefghijklmnop"))
		}))
		t.Cleanup(server.Close)
		ctx := newTestContext(t, func(cfg *config.Config) {
			cfg.Runtime.NativeTools.Fetch.MaxBodyBytes = 8
		})
		output, coreErr := invoke(ctx, t, map[string]any{"url": server.URL})
		require.Nil(t, coreErr)
		assert.Equal(t, "abcdefgh", output["body"].(string))
		assert.True(t, output["body_truncated"].(bool))
	})

	t.Run("Should JSON encode map bodies", func(t *testing.T) {
		var captured atomic.Pointer[string]
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			data, _ := io.ReadAll(req.Body)
			body := string(data)
			captured.Store(&body)
			_, _ = w.Write([]byte("ok"))
		}))
		t.Cleanup(server.Close)
		ctx := newTestContext(t, nil)
		payload := map[string]any{
			"url":    server.URL,
			"method": "POST",
			"body": map[string]any{
				"hello": "world",
			},
		}
		output, coreErr := invoke(ctx, t, payload)
		require.Nil(t, coreErr)
		assert.Equal(t, "ok", output["body"].(string))
		stored := captured.Load()
		require.NotNil(t, stored)
		assert.Contains(t, *stored, `"hello":"world"`)
	})
}
