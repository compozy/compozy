package bridgesdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookHandlerRejectsUnsupportedMethodBeforeHandler(t *testing.T) {
	t.Parallel()

	called := false
	handler, err := NewWebhookHandler(WebhookGuardConfig{
		AllowedMethods: []string{http.MethodPost},
	}, func(_ http.ResponseWriter, _ *http.Request, _ WebhookRequest) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("NewWebhookHandler() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/webhook", strings.NewReader("{}"))
	handler.ServeHTTP(recorder, request)

	if got, want := recorder.Code, http.StatusMethodNotAllowed; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if called {
		t.Fatal("handler called = true, want false")
	}
}

func TestWebhookHandlerRejectsOversizedBodyBeforeHandler(t *testing.T) {
	t.Parallel()

	called := false
	handler, err := NewWebhookHandler(WebhookGuardConfig{
		AllowedMethods:      []string{http.MethodPost},
		AllowedContentTypes: []string{"application/json"},
		MaxBodyBytes:        4,
	}, func(_ http.ResponseWriter, _ *http.Request, _ WebhookRequest) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("NewWebhookHandler() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/webhook",
		strings.NewReader(`{"too_big":true}`),
	)
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, request)

	if got, want := recorder.Code, http.StatusRequestEntityTooLarge; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if called {
		t.Fatal("handler called = true, want false")
	}
}

func TestWebhookHandlerRejectsInvalidContentTypeBeforeHandler(t *testing.T) {
	t.Parallel()

	called := false
	handler, err := NewWebhookHandler(WebhookGuardConfig{
		AllowedMethods:      []string{http.MethodPost},
		AllowedContentTypes: []string{"application/json"},
	}, func(_ http.ResponseWriter, _ *http.Request, _ WebhookRequest) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("NewWebhookHandler() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/webhook",
		strings.NewReader("{}"),
	)
	request.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(recorder, request)

	if got, want := recorder.Code, http.StatusUnsupportedMediaType; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if called {
		t.Fatal("handler called = true, want false")
	}
}

func TestWebhookHandlerRejectsRateLimitedRequestsBeforeHandler(t *testing.T) {
	t.Parallel()

	limiter := NewFixedWindowRateLimiter(1, time.Minute)
	called := 0
	handler, err := NewWebhookHandler(WebhookGuardConfig{
		AllowedMethods:      []string{http.MethodPost},
		AllowedContentTypes: []string{"application/json"},
		RateLimiter:         limiter,
		RequestKey: func(*http.Request) string {
			return "same-client"
		},
	}, func(_ http.ResponseWriter, _ *http.Request, _ WebhookRequest) error {
		called++
		return nil
	})
	if err != nil {
		t.Fatalf("NewWebhookHandler() error = %v", err)
	}

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/webhook",
		strings.NewReader("{}"),
	)
	firstReq.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(first, firstReq)

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/webhook",
		strings.NewReader("{}"),
	)
	secondReq.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(second, secondReq)

	if got, want := first.Code, http.StatusOK; got != want {
		t.Fatalf("first status = %d, want %d", got, want)
	}
	if got, want := second.Code, http.StatusTooManyRequests; got != want {
		t.Fatalf("second status = %d, want %d", got, want)
	}
	if got, want := called, 1; got != want {
		t.Fatalf("handler calls = %d, want %d", got, want)
	}
}

func TestInFlightLimiterBoundsConcurrentRequests(t *testing.T) {
	t.Parallel()

	limiter := NewInFlightLimiter(1)
	if limiter == nil {
		t.Fatal("NewInFlightLimiter() = nil, want non-nil")
	}
	if !limiter.TryAcquire() {
		t.Fatal("first TryAcquire() = false, want true")
	}
	if limiter.TryAcquire() {
		t.Fatal("second TryAcquire() = true, want false")
	}
	limiter.Release()
	if !limiter.TryAcquire() {
		t.Fatal("TryAcquire() after Release() = false, want true")
	}
}

func TestWebhookHandlerWritesHTTPErrorFromProviderMapping(t *testing.T) {
	t.Parallel()

	handler, err := NewWebhookHandler(WebhookGuardConfig{
		AllowedMethods:      []string{http.MethodPost},
		AllowedContentTypes: []string{"application/json"},
	}, func(_ http.ResponseWriter, _ *http.Request, _ WebhookRequest) error {
		return &HTTPError{
			StatusCode: http.StatusTooManyRequests,
			Message:    "slow down",
			RetryAfter: 2 * time.Second,
		}
	})
	if err != nil {
		t.Fatalf("NewWebhookHandler() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/webhook",
		strings.NewReader("{}"),
	)
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, request)

	if got, want := recorder.Code, http.StatusTooManyRequests; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := recorder.Header().Get("Retry-After"), "2"; got != want {
		t.Fatalf("Retry-After = %q, want %q", got, want)
	}
}

func TestWebhookLimiterConstructorsHandleDisabledConfig(t *testing.T) {
	t.Parallel()

	if limiter := NewFixedWindowRateLimiter(0, 0); limiter != nil {
		t.Fatalf("NewFixedWindowRateLimiter(0, 0) = %#v, want nil", limiter)
	}
	if limiter := NewInFlightLimiter(0); limiter != nil {
		t.Fatalf("NewInFlightLimiter(0) = %#v, want nil", limiter)
	}
}

func TestFixedWindowRateLimiterEvictsExpiredKeys(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	limiter := NewFixedWindowRateLimiter(1, time.Minute)
	if limiter == nil {
		t.Fatal("NewFixedWindowRateLimiter() = nil, want non-nil")
		return
	}
	limiter.now = func() time.Time { return now }

	if !limiter.Allow("first-client") {
		t.Fatal("Allow(first-client) = false, want true")
	}
	now = now.Add(2 * time.Minute)
	if !limiter.Allow("second-client") {
		t.Fatal("Allow(second-client) = false, want true")
	}

	if _, ok := limiter.counts["first-client"]; ok {
		t.Fatal("expired limiter key retained after window rollover")
	}
	if got, want := len(limiter.counts), 1; got != want {
		t.Fatalf("len(limiter.counts) = %d, want %d", got, want)
	}
}
