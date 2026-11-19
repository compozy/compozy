package mcpoauthpage_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/mcpoauthpage"
)

func TestRender(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.November, 19, 8, 41, 0, 0, time.UTC)

	t.Run("Should allow exactly the inline style and script through the CSP hashes", func(t *testing.T) {
		t.Parallel()

		body := render(t, mcpoauthpage.Page{
			Outcome:    mcpoauthpage.OutcomeFailed,
			Server:     &mcpoauthpage.Server{Name: "linear"},
			Diagnostic: "reason: unclassified",
			Now:        now,
		})
		policy := mcpoauthpage.ContentSecurityPolicy()
		for _, element := range []struct{ open, close, directive string }{
			{open: "<style>", close: "</style>", directive: "style-src"},
			{open: "<script>", close: "</script>", directive: "script-src"},
		} {
			source := between(t, body, element.open, element.close)
			sum := sha256.Sum256([]byte(source))
			want := element.directive + " 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
			if !strings.Contains(policy, want) {
				t.Fatalf("CSP %q does not admit the inline %s (want %q)", policy, element.open, want)
			}
		}
		if !strings.HasPrefix(policy, "default-src 'none'") {
			t.Fatalf("CSP = %q, want default-src 'none'", policy)
		}
	})

	t.Run("Should escape server names and ship no script without a diagnostic", func(t *testing.T) {
		t.Parallel()

		body := render(t, mcpoauthpage.Page{
			Outcome: mcpoauthpage.OutcomeConnected,
			Server:  &mcpoauthpage.Server{Name: `<img src=x onerror=alert(1)>`},
			Now:     now,
		})
		if strings.Contains(body, "<img src=x") {
			t.Fatal("server name rendered as markup")
		}
		if strings.Contains(body, "<script>") {
			t.Fatal("connected page shipped a script")
		}
	})

	t.Run("Should state when a non-refreshable token expires", func(t *testing.T) {
		t.Parallel()

		expiresAt := now.Add(8*time.Hour + 10*time.Minute)
		body := render(t, mcpoauthpage.Page{
			Outcome:   mcpoauthpage.OutcomeConnected,
			Server:    &mcpoauthpage.Server{Name: "github"},
			ExpiresAt: &expiresAt,
			Now:       now,
		})
		if !strings.Contains(body, "<dd>Expires in 8 hours</dd>") {
			t.Fatal("connected page missing the token expiry")
		}
	})
}

func render(t *testing.T, page mcpoauthpage.Page) string {
	t.Helper()
	var body bytes.Buffer
	if err := mcpoauthpage.Render(&body, page); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return body.String()
}

func between(t *testing.T, body, start, end string) string {
	t.Helper()
	_, rest, found := strings.Cut(body, start)
	if !found {
		t.Fatalf("page has no %s", start)
	}
	source, _, found := strings.Cut(rest, end)
	if !found {
		t.Fatalf("page has no %s", end)
	}
	return source
}
