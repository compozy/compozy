package githubauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestResolver(t *testing.T) {
	t.Parallel()
	t.Run("Should prefer the owner's environment without invoking gh", func(t *testing.T) {
		t.Parallel()
		resolver := NewResolver(func(key string) string {
			if key == "GITHUB_TOKEN" {
				return " environment-sentinel "
			}
			return ""
		})
		resolver.run = func(context.Context) ([]byte, error) {
			t.Error("environment credentials must take precedence")
			return nil, nil
		}
		token, err := resolver.Resolve(t.Context())
		if err != nil || token != "environment-sentinel" {
			t.Fatal("environment credential was not resolved")
		}
	})
	t.Run("Should bound gh lookup and reuse only its in-memory result", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
		resolver := NewResolver(func(string) string { return "" })
		resolver.now = func() time.Time { return now }
		calls := 0
		resolver.run = func(ctx context.Context) ([]byte, error) {
			calls++
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > lookupTimeout {
				t.Error("lookup is not bounded")
			}
			return []byte("cli-sentinel\n"), nil
		}
		for range 2 {
			token, err := resolver.Resolve(t.Context())
			if err != nil || token != "cli-sentinel" {
				t.Fatal("CLI credential was not resolved")
			}
		}
		if calls != 1 {
			t.Fatalf("gh calls = %d, want 1", calls)
		}
		now = now.Add(time.Minute)
		if _, err := resolver.Resolve(t.Context()); err != nil || calls != 2 {
			t.Fatal("expired cache was not refreshed")
		}
	})
	t.Run("Should fall back anonymously without exposing failed CLI output", func(t *testing.T) {
		t.Parallel()
		resolver := NewResolver(func(string) string { return "" })
		resolver.run = func(context.Context) ([]byte, error) {
			return []byte("private-output-sentinel"), errors.New("private-error-sentinel")
		}
		token, err := resolver.Resolve(t.Context())
		if err != nil || token != "" {
			t.Fatal("CLI failure did not fall back anonymously")
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := resolver.Resolve(ctx); !errors.Is(err, context.Canceled) {
			t.Fatal("caller cancellation was lost")
		}
	})
}

func TestRateLimit(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name  string
		retry string
		reset time.Time
		want  time.Time
	}{
		{"Should honor the later reset deadline", "120", now.Add(time.Hour), now.Add(time.Hour)},
		{"Should honor an HTTP date retry deadline", now.Add(2 * time.Hour).Format(http.TimeFormat), now.Add(time.Hour), now.Add(2 * time.Hour)},
		{"Should retain the minimum backoff for malformed headers", "invalid", now, now.Add(time.Minute)},
		{"Should not overflow an excessive retry duration", "9223372036854775807", now, now.Add(time.Minute)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			limited := RateLimit(&http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{
				"Retry-After": {test.retry}, "X-Ratelimit-Reset": {strconv.FormatInt(test.reset.Unix(), 10)},
			}}, now)
			if limited == nil || !limited.RetryAt.Equal(test.want) || !errors.Is(limited, ErrRateLimited) ||
				!strings.Contains(limited.Error(), "gh auth login") {
				t.Fatalf("rate limit = %v", limited)
			}
		})
	}
}

func TestHTTPClient(t *testing.T) {
	t.Parallel()
	t.Run("Should reject credential forwarding outside the configured HTTPS origin", func(t *testing.T) {
		t.Parallel()
		resolver := NewResolver(func(string) string { return "boundary-sentinel" })
		for _, destination := range []string{"http://api.github.com", "https://api.github.com.evil.test", "https://api.github.com:8443"} {
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, destination, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			if err := Authorize(t.Context(), request, "https://api.github.com", resolver); err != nil {
				t.Fatal(err)
			}
			if request.Header.Get("Authorization") != "" {
				t.Fatal("credential crossed its origin boundary")
			}
		}
	})
	t.Run("Should redact echoed credentials while retaining error identity", func(t *testing.T) {
		t.Parallel()
		request := &http.Request{Header: http.Header{"Authorization": {"Bearer error-sentinel"}}}
		cause := errors.New("upstream echoed error-sentinel")
		err := RedactError(request, cause)
		if !errors.Is(err, cause) || strings.Contains(err.Error(), "error-sentinel") {
			t.Fatal("unsafe error redaction")
		}
		if got := RedactResponse(&http.Response{Request: request}, "error-sentinel"); got != "[REDACTED]" {
			t.Fatal("response diagnostic exposed a credential")
		}
		if RedactError(request, nil) != nil || RedactError(nil, cause) != cause {
			t.Fatal("unrelated error was changed")
		}
	})
	t.Run("Should keep authorization on its exact HTTPS origin through redirects", func(t *testing.T) {
		t.Parallel()
		remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				t.Error("redirect leaked a credential")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(remote.Close)
		origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer origin-sentinel" {
				t.Error("origin credential is missing")
			}
			http.Redirect(w, r, remote.URL, http.StatusFound)
		}))
		t.Cleanup(origin.Close)
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, origin.URL, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resolver := NewResolver(func(string) string { return "origin-sentinel" })
		if err := Authorize(t.Context(), request, origin.URL, resolver); err != nil {
			t.Fatal(err)
		}
		client := HTTPClient(origin.Client(), origin.URL)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			t.Error(err)
		}
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
		if response.StatusCode != http.StatusNoContent {
			t.Fatal("redirect did not reach destination")
		}
	})
}
