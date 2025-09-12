package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sizemw "github.com/compozy/compozy/engine/infra/server/middleware/size"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/webhook"
	pkgcfg "github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memRedis is a simple in-memory Redis client stub for testing
type memRedis struct {
	data map[string]any
}

func (m *memRedis) SetNX(_ context.Context, key string, value any, _ time.Duration) (bool, error) {
	if m.data == nil {
		m.data = make(map[string]any)
	}
	if _, exists := m.data[key]; exists {
		return false, nil
	}
	m.data[key] = value
	return true, nil
}

func setupWebhookRouter(t *testing.T, o *webhook.Orchestrator, maxBody int64) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group(routes.Hooks())
	g.Use(sizemw.BodySizeLimiter(maxBody))
	webhook.RegisterPublic(g, o)
	return r
}

func newOrchestrator(
	t *testing.T,
	reg webhook.Lookup,
	disp services.SignalDispatcher,
	maxBody int64,
) *webhook.Orchestrator {
	t.Helper()
	eval, err := task.NewCELEvaluator()
	require.NoError(t, err)
	filter := webhook.NewCELAdapter(eval)
	idem := webhook.NewRedisService(&memRedis{})
	cfg := &pkgcfg.Config{}
	cfg.Webhooks.DefaultMaxBody = maxBody
	cfg.Webhooks.DefaultDedupeTTL = time.Minute
	cfg.Webhooks.StripeSkew = 5 * time.Minute
	return webhook.NewOrchestrator(cfg, reg, filter, disp, idem, maxBody, time.Minute)
}

type testRegistry struct {
	e  webhook.RegistryEntry
	ok bool
}

func (tr *testRegistry) Get(_ string) (webhook.RegistryEntry, bool) { return tr.e, tr.ok }

func TestWebhook_PublicRoute(t *testing.T) {
	t.Run("Should return 202 for valid webhook", func(t *testing.T) {
		reg := &testRegistry{
			e: webhook.RegistryEntry{
				Webhook: &webhook.Config{
					Slug: "ok",
					Events: []webhook.EventConfig{
						{Name: "ev", Filter: "true", Input: map[string]string{"id": "{{ .payload.id }}"}},
					},
				},
			},
			ok: true,
		}
		disp := services.NewMockSignalDispatcher()
		o := newOrchestrator(t, reg, disp, 1024*64)
		router := setupWebhookRouter(t, o, 1024*64)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, routes.Hooks()+"/ok", bytes.NewBufferString(`{"id":"1"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)

		// Assert dispatcher received exactly one call with expected signal/payload
		assert.Len(t, disp.Calls, 1)
		assert.Equal(t, "ev", disp.Calls[0].SignalName)
		assert.Equal(t, map[string]any{"id": "1"}, disp.Calls[0].Payload)
		assert.NotEmpty(t, disp.Calls[0].CorrelationID)
	})

	t.Run("Should return 404 for unknown slug", func(t *testing.T) {
		reg := &testRegistry{ok: false}
		disp := services.NewMockSignalDispatcher()
		o := newOrchestrator(t, reg, disp, 1024)
		router := setupWebhookRouter(t, o, 1024)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, routes.Hooks()+"/missing", bytes.NewBufferString(`{"a":1}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Should return 400 for oversized body", func(t *testing.T) {
		reg := &testRegistry{
			e: webhook.RegistryEntry{
				Webhook: &webhook.Config{
					Slug: "small",
					Events: []webhook.EventConfig{
						{Name: "ev", Filter: "true", Input: map[string]string{"id": "{{ .payload.id }}"}},
					},
				},
			},
			ok: true,
		}
		disp := services.NewMockSignalDispatcher()
		o := newOrchestrator(t, reg, disp, 16)
		router := setupWebhookRouter(t, o, 16)
		big := bytes.Repeat([]byte("a"), 100)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, routes.Hooks()+"/small", bytes.NewBuffer(big))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("Should return 401 on verification failure", func(t *testing.T) {
		reg := &testRegistry{
			e: webhook.RegistryEntry{
				Webhook: &webhook.Config{
					Slug:   "v",
					Verify: &webhook.VerifySpec{Strategy: webhook.StrategyHMAC, Header: "X-Sig", Secret: "s"},
					Events: []webhook.EventConfig{
						{Name: "ev", Filter: "true", Input: map[string]string{"id": "{{ .payload.id }}"}},
					},
				},
			},
			ok: true,
		}
		disp := services.NewMockSignalDispatcher()
		o := newOrchestrator(t, reg, disp, 1024)
		router := setupWebhookRouter(t, o, 1024)
		body := []byte(`{"id":"1"}`)
		// Intentionally wrong signature
		mac := hmac.New(sha256.New, []byte("bad"))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, routes.Hooks()+"/v", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Sig", sig)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Should return 200 with status for no matching event", func(t *testing.T) {
		reg := &testRegistry{
			e: webhook.RegistryEntry{
				Webhook: &webhook.Config{
					Slug: "flt",
					Events: []webhook.EventConfig{
						{Name: "ev", Filter: "false", Input: map[string]string{"id": "{{ .payload.id }}"}},
					},
				},
			},
			ok: true,
		}
		disp := services.NewMockSignalDispatcher()
		o := newOrchestrator(t, reg, disp, 1024)
		router := setupWebhookRouter(t, o, 1024)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, routes.Hooks()+"/flt", bytes.NewBufferString(`{"id":"1"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "no_matching_event")
	})
}
