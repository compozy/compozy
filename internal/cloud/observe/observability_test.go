// SPDX-License-Identifier: FSL-1.1-MIT

package observe

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSentryReporting(t *testing.T) {
	t.Parallel()
	t.Run("Should send only allowed cloud diagnostics in a Sentry envelope", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		var envelopes [][]byte
		backing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/11/envelope/" || r.Method != http.MethodPost ||
				r.Header.Get("Content-Type") != "application/x-sentry-envelope" ||
				!strings.Contains(r.Header.Get("X-Sentry-Auth"), "sentry_key=public-key") {
				t.Error("invalid Sentry event delivery")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			mu.Lock()
			envelopes = append(envelopes, body)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(backing.Close)
		dsn := strings.Replace(backing.URL, "http://", "http://public-key@", 1) + "/11"
		sentry, err := NewSentry(dsn, "development")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		sentry.Capture(ctx, "http_request", Client{
			Version: "0.8.0-beta.4", Platform: "linux",
		})
		sentry.Capture(ctx, "stripe_webhook", Client{
			Version: "/home/operator/prompt", Platform: "token-looking-value",
		})
		for range 2 {
			select {
			case payload := <-sentry.queue:
				if err := sentry.send(ctx, payload); err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("Sentry did not prepare an event")
			}
		}
		mu.Lock()
		defer mu.Unlock()
		if got := len(envelopes); got != 2 {
			t.Fatalf("Sentry received %d events, want 2", got)
		}
		for _, wanted := range []string{"http_request", "development", "0.8.0-beta.4", "linux"} {
			if !bytes.Contains(envelopes[0], []byte(wanted)) {
				t.Fatalf("Sentry envelope missing %q", wanted)
			}
		}
		for _, forbidden := range []string{"/home/operator/prompt", "token-looking-value", "client_version", "client_platform"} {
			if bytes.Contains(envelopes[1], []byte(forbidden)) {
				t.Fatalf("Sentry envelope exposed %q", forbidden)
			}
		}
	})
}
