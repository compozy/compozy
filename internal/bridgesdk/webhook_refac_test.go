package bridgesdk

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookRefacs(t *testing.T) {
	t.Parallel()

	t.Run("Should ignore close-only request body errors after a successful read", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/webhook", nil)
		request.Body = closeErrorReadCloser{Reader: strings.NewReader("{}")}
		body, err := readBodyWithLimit(httptest.NewRecorder(), request, defaultWebhookBodyLimit)
		if err != nil {
			t.Fatalf("readBodyWithLimit() error = %v, want nil", err)
		}
		if got, want := string(body), "{}"; got != want {
			t.Fatalf("readBodyWithLimit() body = %q, want %q", got, want)
		}
	})

	t.Run("Should round positive subsecond retry after to one second", func(t *testing.T) {
		t.Parallel()

		handler, err := NewWebhookHandler(WebhookGuardConfig{
			AllowedMethods:      []string{http.MethodPost},
			AllowedContentTypes: []string{"application/json"},
		}, func(http.ResponseWriter, *http.Request, WebhookRequest) error {
			return &HTTPError{
				StatusCode: http.StatusTooManyRequests,
				Message:    "slow down",
				RetryAfter: 100 * time.Millisecond,
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
		if got, want := recorder.Header().Get("Retry-After"), "1"; got != want {
			t.Fatalf("Retry-After = %q, want %q", got, want)
		}
		if got, want := recorder.Body.String(), "slow down\n"; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})
}

type closeErrorReadCloser struct {
	io.Reader
}

func (closeErrorReadCloser) Close() error {
	return errors.New("close failed")
}
