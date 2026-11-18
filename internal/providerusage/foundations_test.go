package providerusage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func credentialFile(t *testing.T, name, content string) string {
	t.Helper()
	home := t.TempDir()
	requireNoError(t, os.WriteFile(filepath.Join(home, name), []byte(content), 0o600))
	return home
}
func fixtureServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}
func fixtureClock(t *testing.T) *clockwork.FakeClock {
	t.Helper()
	return clockwork.NewFakeClockAt(time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC))
}

func TestFoundations(t *testing.T) {
	t.Run("Should compare ownership without exposing credential material", func(t *testing.T) {
		t.Parallel()
		a := NewCredentialIdentity(SourceKindOAuthHTTP, []byte("secret-one"), "private-account")
		b := NewCredentialIdentity(SourceKindOAuthHTTP, []byte("secret-two"), "private-account")
		requireEqual(t, true, a.Same(b))
		requireEqual(t, false, a.Digest == b.Digest)
		requireEqual(
			t,
			false,
			a.Same(NewCredentialIdentity(SourceKindOAuthHTTP, []byte("secret-two"), "other-account")),
		)
		encoded, err := json.Marshal(a)
		requireNoError(t, err)
		requireEqual(t, `{}`, string(encoded))
		requireEqual(t, "[redacted credential identity]", fmt.Sprintf("%+v", a))
	})
	t.Run("Should reject oversized bodies and cross origin redirects", func(t *testing.T) {
		t.Parallel()
		foreign := fixtureServer(t, func(w http.ResponseWriter, _ *http.Request) {
			t.Error("credential sent to another origin")
			w.WriteHeader(500)
		})
		origin := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/redirect" {
				http.Redirect(w, r, foreign.URL, http.StatusFound)
				return
			}
			_, err := w.Write([]byte(strings.Repeat("x", MaxResponseBytes+1)))
			requireNoError(t, err)
		})
		client := NewHTTPClient(origin.URL, nil, fixtureClock(t).Now)
		_, err := client.Do(t.Context(), "GET", "/large", nil, nil, "Codex")
		requireEqual(t, ErrorCodeParseError, AsUsageError(err, "Codex").Code)
		_, err = client.Do(
			t.Context(),
			"GET",
			"/redirect",
			http.Header{"Authorization": []string{"Bearer secret"}},
			nil,
			"Codex",
		)
		requireEqual(t, ErrorCodeProviderUnavailable, AsUsageError(err, "Codex").Code)
	})
	t.Run("Should bound provider cooldowns", func(t *testing.T) {
		t.Parallel()
		now := fixtureClock(t).Now()
		for _, tc := range []struct {
			header string
			delay  time.Duration
		}{{"", 5 * time.Minute}, {"120", 2 * time.Minute}, {"7200", time.Hour}, {"-1", 5 * time.Minute}, {now.Add(2 * time.Hour).Format(http.TimeFormat), time.Hour}} {
			requireEqual(t, now.Add(tc.delay), ParseRetryAfter(tc.header, now))
		}
	})
	t.Run("Should floor and back off rate limit cooldowns up to the cap", func(t *testing.T) {
		t.Parallel()
		now := fixtureClock(t).Now()
		at := func(d time.Duration) *time.Time { return new(now.Add(d)) }
		for _, tc := range []struct {
			name       string
			retryAfter *time.Time
			strikes    int
			delay      time.Duration
		}{
			{"Should floor a zero Retry-After", at(0), 1, 5 * time.Minute},
			{"Should floor a short Retry-After", at(2 * time.Minute), 1, 5 * time.Minute},
			{"Should keep a longer Retry-After", at(30 * time.Minute), 1, 30 * time.Minute},
			{"Should double on the second strike", at(0), 2, 10 * time.Minute},
			{"Should double on the third strike", at(0), 3, 20 * time.Minute},
			{"Should double on the fourth strike", at(0), 4, 40 * time.Minute},
			{"Should cap the fifth strike", at(0), 5, time.Hour},
			{"Should cap without overflow", at(0), 64, time.Hour},
			{"Should floor a missing Retry-After", nil, 1, 5 * time.Minute},
			{"Should cap a long Retry-After", at(2 * time.Hour), 1, time.Hour},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				if got := rateLimitUntil(now, tc.retryAfter, tc.strikes); !got.Equal(now.Add(tc.delay)) {
					t.Errorf("rateLimitUntil = %v, want %v", got, now.Add(tc.delay))
				}
			})
		}
	})
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func requireEqual[T comparable](t *testing.T, want, got T) {
	t.Helper()
	if want != got {
		t.Fatalf("want %v, got %v", want, got)
	}
}
