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
		requireEqual(t, false, a.Same(NewCredentialIdentity(SourceKindOAuthHTTP, []byte("secret-two"), "other-account")))
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
				http.Redirect(w, r, foreign.URL, 302)
				return
			}
			_, err := w.Write([]byte(strings.Repeat("x", MaxResponseBytes+1)))
			requireNoError(t, err)
		})
		client := NewHTTPClient(origin.URL, nil, fixtureClock(t).Now)
		_, err := client.Do(t.Context(), "GET", "/large", nil, nil, "Codex")
		requireEqual(t, ErrorCodeParseError, AsUsageError(err, "Codex").Code)
		_, err = client.Do(t.Context(), "GET", "/redirect", http.Header{"Authorization": []string{"Bearer secret"}}, nil, "Codex")
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
