package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/webhook"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubService struct {
	key string
	ttl time.Duration
	err error
}

func (s *stubService) CheckAndSet(_ context.Context, key string, ttl time.Duration) error {
	s.key = key
	s.ttl = ttl
	return s.err
}

func TestAPIIdempotency_HeaderPrecedence(t *testing.T) {
	t.Run("Should use header key when present", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("POST", "/api/v0/agents/a1/executions", bytes.NewReader([]byte(`{"a":1}`)))
		manager := config.NewManager(config.NewService())
		_, err := manager.Load(context.Background(), config.NewDefaultProvider())
		require.NoError(t, err)
		req = req.WithContext(config.ContextWithManager(req.Context(), manager))
		req.Header.Set(webhook.HeaderIdempotencyKey, "  key-123  ")
		c.Request = req
		stub := &stubService{}
		idem := NewAPIIdempotency(stub)
		unique, reason, err := idem.CheckAndSet(req.Context(), c, "agents", nil, 0)
		require.NoError(t, err)
		require.True(t, unique)
		require.Empty(t, reason)
		require.Equal(t, "idempotency:api:execs:agents:key-123", stub.key)
		require.Equal(t, defaultIdempotencyTTL, stub.ttl)
	})
}

func TestAPIIdempotency_HashFallback(t *testing.T) {
	t.Run("Should derive stable hash from body", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		stub := &stubService{}
		idem := NewAPIIdempotency(stub)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		body := []byte(`{"b":2,"a":1}`)
		req := httptest.NewRequest("POST", "/api/v0/agents/a1/executions", bytes.NewReader(body))
		manager := config.NewManager(config.NewService())
		_, err := manager.Load(context.Background(), config.NewDefaultProvider())
		require.NoError(t, err)
		req = req.WithContext(config.ContextWithManager(req.Context(), manager))
		c.Request = req
		unique, reason, err := idem.CheckAndSet(req.Context(), c, "agents", body, 0)
		require.NoError(t, err)
		require.True(t, unique)
		require.Empty(t, reason)
		firstKey := stub.key
		stub.key = ""
		c2, _ := gin.CreateTestContext(httptest.NewRecorder())
		body2 := []byte(`{"a":1,"b":2}`)
		req2 := httptest.NewRequest("POST", "/api/v0/agents/a1/executions", bytes.NewReader(body2))
		req2 = req2.WithContext(config.ContextWithManager(req2.Context(), manager))
		c2.Request = req2
		_, _, err = idem.CheckAndSet(req2.Context(), c2, "agents", body2, 0)
		require.NoError(t, err)
		require.Equal(t, firstKey, stub.key)
	})
}

func TestAPIIdempotency_Duplicate(t *testing.T) {
	t.Run("Should return duplicate flag when key exists", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		stub := &stubService{err: webhook.ErrDuplicate}
		idem := NewAPIIdempotency(stub)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest("POST", "/api/v0/tasks/t1/executions", http.NoBody)
		manager := config.NewManager(config.NewService())
		_, err := manager.Load(context.Background(), config.NewDefaultProvider())
		require.NoError(t, err)
		req = req.WithContext(config.ContextWithManager(req.Context(), manager))
		c.Request = req
		unique, reason, err := idem.CheckAndSet(req.Context(), c, "tasks", nil, 0)
		require.NoError(t, err)
		require.False(t, unique)
		require.Equal(t, "duplicate", reason)
	})
}

func TestAPIIdempotency_HeaderLengthValidation(t *testing.T) {
	t.Run("Should reject header exceeding limit", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		stub := &stubService{}
		idem := NewAPIIdempotency(stub)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest("POST", "/api/v0/agents/a1/executions", http.NoBody)
		manager := config.NewManager(config.NewService())
		_, err := manager.Load(context.Background(), config.NewDefaultProvider())
		require.NoError(t, err)
		req = req.WithContext(config.ContextWithManager(req.Context(), manager))
		req.Header.Set(webhook.HeaderIdempotencyKey, strings.Repeat("x", maxIdempotencyKeyBytes+1))
		c.Request = req
		unique, reason, err := idem.CheckAndSet(req.Context(), c, "agents", nil, 0)
		require.Error(t, err)
		require.False(t, unique)
		require.Empty(t, reason)
	})
}

func TestAPIIdempotency_BodySizeLimit(t *testing.T) {
	ginmode.EnsureGinTestMode()
	stub := &stubService{}
	idem := NewAPIIdempotency(stub)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := bytes.Repeat([]byte("a"), 32)
	req := httptest.NewRequest("POST", "/api/v0/agents/a1/executions", bytes.NewReader(body))
	manager := config.NewManager(config.NewService())
	_, err := manager.Load(context.Background(), config.NewDefaultProvider())
	require.NoError(t, err)
	manager.Get().Webhooks.DefaultMaxBody = 16
	req = req.WithContext(config.ContextWithManager(req.Context(), manager))
	c.Request = req
	unique, reason, err := idem.CheckAndSet(req.Context(), c, "agents", body, 0)
	require.Error(t, err)
	require.False(t, unique)
	require.Empty(t, reason)
}
