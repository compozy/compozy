package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTokenResponseExpiresInValidationContract(t *testing.T) {
	t.Parallel()

	const overflowingExpiresIn = int64(1<<63-1)/int64(time.Second) + 1
	for _, tc := range []struct {
		name        string
		expiresIn   int64
		wantSnippet string
	}{
		{
			name:        "Should reject negative expires_in before persisting a token",
			expiresIn:   -1,
			wantSnippet: "expires_in must not be negative",
		},
		{
			name:        "Should reject duration-overflowing expires_in before persisting a token",
			expiresIn:   overflowingExpiresIn,
			wantSnippet: "expires_in overflows duration",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			store := newMemoryTokenStore()
			service, login, closeServer := newTokenExpiresInService(t, store, tc.expiresIn)
			defer closeServer()

			_, err := service.Exchange(ctx, login, "http://127.0.0.1/callback?code=ok&state="+login.State)
			if err == nil || !strings.Contains(err.Error(), tc.wantSnippet) {
				t.Fatalf("Exchange() error = %v, want %q", err, tc.wantSnippet)
			}
			if _, err := store.GetMCPAuthToken(ctx, globalTestTarget("linear")); !errors.Is(err, ErrTokenNotFound) {
				t.Fatalf("GetMCPAuthToken() error = %v, want ErrTokenNotFound", err)
			}
		})
	}

	t.Run("Should treat explicit zero expires_in as immediately expired", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := newMemoryTokenStore()
		service, login, closeServer := newTokenExpiresInService(t, store, 0)
		defer closeServer()

		status, err := service.Exchange(ctx, login, "http://127.0.0.1/callback?code=ok&state="+login.State)
		if err != nil {
			t.Fatalf("Exchange() error = %v", err)
		}
		if status.Status != StatusExpired || status.ExpiresAt == nil {
			t.Fatalf("Exchange() status = %#v, want expired status with expires_at", status)
		}
		token, err := store.GetMCPAuthToken(ctx, globalTestTarget("linear"))
		if err != nil {
			t.Fatalf("GetMCPAuthToken() error = %v", err)
		}
		if !token.ExpiresAt.Equal(status.ExpiresAt.UTC()) {
			t.Fatalf("token.ExpiresAt = %s, want %s", token.ExpiresAt, status.ExpiresAt.UTC())
		}
	})
}

func newTokenExpiresInService(
	t *testing.T,
	store TokenStore,
	expiresIn int64,
) (*Service, LoginState, func()) {
	t.Helper()

	handlerErrors := newHandlerErrorRecorder()
	now := time.Date(2026, 5, 17, 18, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if err := writeJSON(w, map[string]any{
				"access_token": "access-token",
				"token_type":   "Bearer",
				"expires_in":   expiresIn,
			}); err != nil {
				handlerErrors.record(err)
			}
		default:
			if err := writeJSON(w, Metadata{
				AuthorizationEndpoint:         "http://" + r.Host + "/authorize",
				TokenEndpoint:                 "http://" + r.Host + "/token",
				CodeChallengeMethodsSupported: []string{"S256"},
			}); err != nil {
				handlerErrors.record(err)
			}
		}
	}))
	service, err := NewService(
		store,
		WithHTTPClient(server.Client()),
		WithNow(func() time.Time { return now }),
	)
	if err != nil {
		server.Close()
		t.Fatalf("NewService() error = %v", err)
	}
	cfg := ServerConfig{
		Target:      globalTestTarget("linear"),
		Type:        "oauth2_pkce",
		MetadataURL: server.URL,
		ClientID:    "client",
	}
	login, err := service.BeginLogin(context.Background(), cfg, "http://127.0.0.1/callback")
	if err != nil {
		server.Close()
		t.Fatalf("BeginLogin() error = %v", err)
	}
	closeServer := func() {
		server.Close()
		handlerErrors.assertEmpty(t)
	}
	return service, login, closeServer
}
