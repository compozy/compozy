package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/webhook"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type serviceCall struct {
	key string
	ttl time.Duration
}

type recordingService struct {
	mu    sync.Mutex
	calls []serviceCall
	err   error
}

func newRecordingService(err error) *recordingService {
	return &recordingService{err: err}
}

func (s *recordingService) CheckAndSet(_ context.Context, key string, ttl time.Duration) error {
	s.mu.Lock()
	s.calls = append(s.calls, serviceCall{key: key, ttl: ttl})
	s.mu.Unlock()
	return s.err
}

func (s *recordingService) LastCall() (serviceCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return serviceCall{}, false
	}
	return s.calls[len(s.calls)-1], true
}

func (s *recordingService) Calls() []serviceCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]serviceCall, len(s.calls))
	copy(result, s.calls)
	return result
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
		svc := newRecordingService(nil)
		idem := NewAPIIdempotency(svc)
		unique, reason, err := idem.CheckAndSet(req.Context(), c, "agents", nil, 0)
		require.NoError(t, err)
		require.True(t, unique)
		require.Empty(t, reason)
		call, ok := svc.LastCall()
		require.True(t, ok)
		require.Equal(t, "idempotency:api:execs:agents:key-123", call.key)
		require.Equal(t, defaultIdempotencyTTL, call.ttl)
	})
}

func TestAPIIdempotency_HashFallback(t *testing.T) {
	t.Run("Should derive stable hash from body", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		svc := newRecordingService(nil)
		idem := NewAPIIdempotency(svc)
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
		firstCall, ok := svc.LastCall()
		require.True(t, ok)
		c2, _ := gin.CreateTestContext(httptest.NewRecorder())
		body2 := []byte(`{"a":1,"b":2}`)
		req2 := httptest.NewRequest("POST", "/api/v0/agents/a1/executions", bytes.NewReader(body2))
		req2 = req2.WithContext(config.ContextWithManager(req2.Context(), manager))
		c2.Request = req2
		_, _, err = idem.CheckAndSet(req2.Context(), c2, "agents", body2, 0)
		require.NoError(t, err)
		latest, ok := svc.LastCall()
		require.True(t, ok)
		require.Equal(t, firstCall.key, latest.key)
	})
}

func TestAPIIdempotency_Duplicate(t *testing.T) {
	t.Run("Should return duplicate flag when key exists", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		svc := newRecordingService(webhook.ErrDuplicate)
		idem := NewAPIIdempotency(svc)
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
		svc := newRecordingService(nil)
		idem := NewAPIIdempotency(svc)
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
	svc := newRecordingService(nil)
	idem := NewAPIIdempotency(svc)
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

func TestAPIIdempotency_CustomTTL(t *testing.T) {
	t.Run("Should use provided TTL when greater than zero", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		svc := newRecordingService(nil)
		idem := NewAPIIdempotency(svc)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("POST", "/api/v0/agents/a1/executions", http.NoBody)
		manager := config.NewManager(config.NewService())
		_, err := manager.Load(context.Background(), config.NewDefaultProvider())
		require.NoError(t, err)
		req = req.WithContext(config.ContextWithManager(req.Context(), manager))
		c.Request = req
		customTTL := 2 * time.Hour
		unique, reason, err := idem.CheckAndSet(req.Context(), c, "agents", nil, customTTL)
		require.NoError(t, err)
		require.True(t, unique)
		require.Empty(t, reason)
		call, ok := svc.LastCall()
		require.True(t, ok)
		require.Equal(t, customTTL, call.ttl)
	})
}
