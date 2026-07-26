package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

type fakeOnboardingStore struct {
	status store.OnboardingStatus
}

func newFakeOnboardingStore() *fakeOnboardingStore {
	return &fakeOnboardingStore{}
}

func (f *fakeOnboardingStore) GetOnboardingStatus(context.Context) (store.OnboardingStatus, error) {
	return f.status, nil
}

func (f *fakeOnboardingStore) CompleteOnboarding(
	_ context.Context,
	completedAt string,
) (store.OnboardingStatus, error) {
	if !f.status.Completed {
		f.status = store.OnboardingStatus{Completed: true, CompletedAt: completedAt}
	}
	return f.status, nil
}

func (f *fakeOnboardingStore) ResetOnboarding(context.Context) (store.OnboardingStatus, error) {
	f.status = store.OnboardingStatus{}
	return f.status, nil
}

func newOnboardingFixture(t *testing.T, store core.OnboardingStore) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	homePaths := testutil.NewTestHomePaths(t)
	cfg := aghconfig.DefaultWithHome(homePaths)

	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: "api-core-test",
		Onboarding:    store,
		HomePaths:     homePaths,
		Config:        cfg,
		Logger:        testutil.DiscardLogger(),
		StartedAt:     time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		Now: func() time.Time {
			return time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
		},
	})

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.GET("/api/onboarding", handlers.GetOnboardingStatus)
	engine.POST("/api/onboarding/complete", handlers.CompleteOnboarding)
	engine.DELETE("/api/onboarding", handlers.ResetOnboarding)
	return engine
}

func decodeOnboarding(t *testing.T, body []byte) contract.OnboardingStatusPayload {
	t.Helper()
	var resp contract.OnboardingStatusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode onboarding response: %v (body=%s)", err, string(body))
	}
	return resp.Onboarding
}

func onboardingRequest(method string, path string) *http.Request {
	return httptest.NewRequestWithContext(context.Background(), method, path, http.NoBody)
}

func TestOnboardingHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should report not completed on a fresh instance", func(t *testing.T) {
		t.Parallel()
		engine := newOnboardingFixture(t, newFakeOnboardingStore())

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, onboardingRequest(http.MethodGet, "/api/onboarding"))

		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		payload := decodeOnboarding(t, rec.Body.Bytes())
		if payload.Completed {
			t.Fatalf("Completed = true, want false")
		}
	})

	t.Run("Should mark completed and persist completed_at", func(t *testing.T) {
		t.Parallel()
		fake := newFakeOnboardingStore()
		engine := newOnboardingFixture(t, fake)

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, onboardingRequest(http.MethodPost, "/api/onboarding/complete"))
		if rec.Code != http.StatusOK {
			t.Fatalf("POST complete = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		payload := decodeOnboarding(t, rec.Body.Bytes())
		if !payload.Completed || payload.CompletedAt == "" {
			t.Fatalf("complete payload = %#v, want completed with timestamp", payload)
		}

		getRec := httptest.NewRecorder()
		engine.ServeHTTP(getRec, onboardingRequest(http.MethodGet, "/api/onboarding"))
		got := decodeOnboarding(t, getRec.Body.Bytes())
		if !got.Completed || got.CompletedAt != payload.CompletedAt {
			t.Fatalf("GET after complete = %#v, want persisted %q", got, payload.CompletedAt)
		}
	})

	t.Run("Should keep the original timestamp on repeated completion", func(t *testing.T) {
		t.Parallel()
		fake := newFakeOnboardingStore()
		fake.status = store.OnboardingStatus{Completed: true, CompletedAt: "2026-01-01T00:00:00Z"}
		engine := newOnboardingFixture(t, fake)

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, onboardingRequest(http.MethodPost, "/api/onboarding/complete"))
		if rec.Code != http.StatusOK {
			t.Fatalf("POST repeated complete = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		payload := decodeOnboarding(t, rec.Body.Bytes())
		if payload.CompletedAt != "2026-01-01T00:00:00Z" {
			t.Fatalf("CompletedAt = %q, want preserved original", payload.CompletedAt)
		}
	})

	t.Run("Should reset completion so the wizard runs again", func(t *testing.T) {
		t.Parallel()
		fake := newFakeOnboardingStore()
		fake.status = store.OnboardingStatus{Completed: true, CompletedAt: "2026-01-01T00:00:00Z"}
		engine := newOnboardingFixture(t, fake)

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, onboardingRequest(http.MethodDelete, "/api/onboarding"))
		if rec.Code != http.StatusOK {
			t.Fatalf("DELETE = %d, want 200", rec.Code)
		}
		payload := decodeOnboarding(t, rec.Body.Bytes())
		if payload.Completed {
			t.Fatalf("Completed = true after reset, want false")
		}
		if fake.status.Completed || fake.status.CompletedAt != "" {
			t.Fatalf("store status after reset = %#v, want empty", fake.status)
		}
	})

	t.Run("Should return 503 when the store is not configured", func(t *testing.T) {
		t.Parallel()
		engine := newOnboardingFixture(t, nil)

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, onboardingRequest(http.MethodPost, "/api/onboarding/complete"))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("POST complete without store = %d, want 503 (body=%s)", rec.Code, rec.Body.String())
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode onboarding error response: %v (body=%s)", err, rec.Body.String())
		}
		if payload.Error != "api: onboarding store is not configured" {
			t.Fatalf("onboarding error = %q, want store-not-configured message", payload.Error)
		}
	})
}
