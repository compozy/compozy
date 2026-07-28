package daytona

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRESTSSHTokenSourceFetchesTokenAndExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(30 * time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/sandbox/sandbox-token/ssh-access"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("expiresInMinutes"), "60"; got != want {
			t.Fatalf("expiresInMinutes = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer api-key"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		writeJSON(t, w, map[string]any{
			"token":     "ssh-token",
			"expiresAt": expiresAt.Format(time.RFC3339),
		})
	}))
	defer server.Close()
	source := &restSSHTokenSource{
		httpClient: server.Client(),
		apiKey:     func() string { return "api-key" },
		now:        func() time.Time { return now },
	}
	access, err := source.FetchSSHAccess(context.Background(), server.URL, "sandbox-token", time.Hour)
	if err != nil {
		t.Fatalf("FetchSSHAccess() error = %v", err)
	}
	if access.Token != "ssh-token" || !access.IssuedAt.Equal(now) || !access.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("access = %#v, want parsed token/expiry", access)
	}
}

func TestRESTSSHTokenSourceRejectsMissingKeyAndBadStatus(t *testing.T) {
	t.Parallel()

	source := &restSSHTokenSource{apiKey: func() string { return "" }, now: time.Now}
	if _, err := source.FetchSSHAccess(context.Background(), defaultAPIURL, "sandbox", time.Hour); err == nil {
		t.Fatal("FetchSSHAccess(missing key) error = nil, want error")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer server.Close()
	source = &restSSHTokenSource{
		httpClient: server.Client(),
		apiKey:     func() string { return "api-key" },
		now:        time.Now,
	}
	if _, err := source.FetchSSHAccess(context.Background(), server.URL, "sandbox", time.Hour); err == nil {
		t.Fatal("FetchSSHAccess(bad status) error = nil, want error")
	}
}
