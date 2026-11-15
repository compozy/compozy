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
	"testing/iotest"
	"testing/synctest"
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
		token, err := resolver.Resolve(t.Context(), "https://api.github.com")
		if err != nil || token != "environment-sentinel" {
			t.Fatal("environment credential was not resolved")
		}
	})
	t.Run("Should never consult the github.com CLI login for a custom API origin", func(t *testing.T) {
		t.Parallel()
		resolver := NewResolver(func(string) string { return "" })
		resolver.run = func(context.Context) ([]byte, error) {
			t.Fatal("custom API attempted to read a github.com credential")
			return nil, nil
		}
		for _, origin := range []string{"https://github.enterprise.test", "http://api.github.com", "https://api.github.com:8443"} {
			if token, err := resolver.Resolve(t.Context(), origin); err != nil || token != "" {
				t.Fatal("custom API did not remain anonymous")
			}
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
			token, err := resolver.Resolve(t.Context(), "https://api.github.com")
			if err != nil || token != "cli-sentinel" {
				t.Fatal("CLI credential was not resolved")
			}
		}
		if calls != 1 {
			t.Fatalf("gh calls = %d, want 1", calls)
		}
		now = now.Add(time.Minute)
		if _, err := resolver.Resolve(t.Context(), "https://api.github.com"); err != nil || calls != 2 {
			t.Fatal("expired cache was not refreshed")
		}
	})
	t.Run("Should cancel a waiter without canceling its owner's active CLI lookup", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			resolver := NewResolver(func(string) string { return "" })
			started, release := make(chan struct{}), make(chan struct{})
			calls := 0
			resolver.run = func(context.Context) ([]byte, error) {
				calls++
				close(started)
				<-release
				return []byte("cached-sentinel"), nil
			}
			ownerDone := make(chan error, 1)
			go func() { _, err := resolver.Resolve(t.Context(), "https://api.github.com"); ownerDone <- err }()
			<-started
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			waiterDone := make(chan error, 1)
			go func() { _, err := resolver.Resolve(ctx, "https://api.github.com"); waiterDone <- err }()
			synctest.Wait()
			cancel()
			if err := <-waiterDone; !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled waiter = %v", err)
			}
			close(release)
			if err := <-ownerDone; err != nil {
				t.Fatal(err)
			}
			if token, err := resolver.Resolve(t.Context(), "https://api.github.com"); err != nil ||
				token != "cached-sentinel" || calls != 1 {
				t.Fatal("waiter cancellation invalidated the owner's lookup")
			}
		})
	})
	t.Run("Should fall back anonymously without exposing failed CLI output", func(t *testing.T) {
		t.Parallel()
		resolver := NewResolver(func(string) string { return "" })
		resolver.run = func(context.Context) ([]byte, error) {
			return []byte("private-output-sentinel"), errors.New("private-error-sentinel")
		}
		token, err := resolver.Resolve(t.Context(), "https://api.github.com")
		if err != nil || token != "" {
			t.Fatal("CLI failure did not fall back anonymously")
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := resolver.Resolve(ctx, "https://api.github.com"); !errors.Is(err, context.Canceled) {
			t.Fatal("caller cancellation was lost")
		}
	})
}

func TestRateLimit(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name      string
		retry     string
		reset     time.Time
		remaining string
		want      time.Time
	}{
		{"Should honor an exhausted primary quota reset", "120", now.Add(time.Hour), "0", now.Add(time.Hour)},
		{"Should use retry-after for a secondary quota", "120", now.Add(time.Hour), "42", now.Add(120 * time.Second)},
		{"Should honor an HTTP date retry deadline", now.Add(2 * time.Hour).Format(http.TimeFormat), now.Add(time.Hour), "0", now.Add(2 * time.Hour)},
		{"Should retain the minimum backoff for malformed headers", "invalid", now, "0", now.Add(time.Minute)},
		{"Should not overflow an excessive retry duration", "9223372036854775807", now, "0", now.Add(time.Minute)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			limited := RateLimit(&http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{
				"X-Ratelimit-Remaining": {test.remaining}, "Retry-After": {test.retry}, "X-Ratelimit-Reset": {strconv.FormatInt(test.reset.Unix(), 10)},
			}}, now)
			if limited == nil || !limited.RetryAt.Equal(test.want) || !errors.Is(limited, ErrRateLimited) ||
				!strings.Contains(limited.Error(), "gh auth login") {
				t.Fatalf("rate limit = %v", limited)
			}
		})
	}
}

func TestRateLimitResponseClassification(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, body string
		limited    bool
	}{
		{"Should recognize secondary quota without retry headers", `{"message":"You have exceeded a secondary rate limit."}`, true},
		{"Should recognize legacy abuse detection", `{"message":"You have triggered an abuse detection mechanism."}`, true},
		{"Should retain ordinary permission-denied responses", `{"message":"Resource not accessible by integration"}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			now := time.Now()
			response := &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{"X-Ratelimit-Remaining": {"42"}},
				Body: io.NopCloser(strings.NewReader(test.body))}
			limited := RateLimit(response, now)
			if (limited != nil) != test.limited || (limited != nil && !limited.RetryAt.Equal(now.Add(time.Minute))) {
				t.Fatalf("quota classification = %v", limited)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil || string(body) != test.body {
				t.Fatal("classification consumed the diagnostic body")
			}
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRateLimitResponseReadFailure(t *testing.T) {
	t.Parallel()
	cause := errors.New("response read failed")
	response := &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(iotest.ErrReader(cause))}
	if RateLimit(response, time.Now()) != nil {
		t.Fatal("read failure was misclassified as quota")
	}
	if _, err := io.ReadAll(response.Body); !errors.Is(err, cause) {
		t.Fatalf("lost body failure: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRateLimitGate(t *testing.T) {
	t.Parallel()
	now := time.Now()
	gate := &RateLimitGate{}
	later := &RateLimitError{RetryAt: now.Add(time.Hour)}
	gate.Record("https://api.github.com", later)
	gate.Record("https://api.github.com", &RateLimitError{RetryAt: now.Add(time.Minute)})
	if gate.Check("https://api.github.com", now.Add(2*time.Minute)) != later {
		t.Fatal("shorter response replaced a longer deadline")
	}
	if gate.Check("https://enterprise.test", now) != nil {
		t.Fatal("quota crossed an API origin boundary")
	}
	if gate.Check("https://api.github.com", now.Add(time.Hour)) != nil {
		t.Fatal("expired quota did not recover")
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
