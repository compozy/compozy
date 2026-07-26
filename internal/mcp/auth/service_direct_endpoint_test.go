package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestBeginLoginDirectEndpointConfigurationContract(t *testing.T) {
	t.Parallel()

	t.Run("Should start login from direct authorization and token endpoints", func(t *testing.T) {
		t.Parallel()

		service, err := NewService(newMemoryTokenStore())
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		cfg := ServerConfig{
			Target:           globalTestTarget("mcp"),
			Type:             "oauth2_pkce",
			AuthorizationURL: "https://issuer.example/authorize",
			TokenURL:         "https://issuer.example/token",
			ClientID:         "client",
			Scopes:           []string{"read"},
		}

		login, err := service.BeginLogin(context.Background(), cfg, "http://127.0.0.1/callback")
		if err != nil {
			t.Fatalf("BeginLogin() error = %v", err)
		}
		authURL, err := url.Parse(login.AuthorizationURL)
		if err != nil {
			t.Fatalf("url.Parse(AuthorizationURL) error = %v", err)
		}
		if got, want := authURL.Query().Get("code_challenge_method"), "S256"; got != want {
			t.Fatalf("code_challenge_method = %q, want %q", got, want)
		}
		if authURL.Query().Get("code_challenge") == "" {
			t.Fatal("code_challenge = empty, want generated PKCE challenge")
		}
		if got, want := login.Metadata.AuthorizationEndpoint, cfg.AuthorizationURL; got != want {
			t.Fatalf("Metadata.AuthorizationEndpoint = %q, want %q", got, want)
		}
		if got, want := login.Metadata.TokenEndpoint, cfg.TokenURL; got != want {
			t.Fatalf("Metadata.TokenEndpoint = %q, want %q", got, want)
		}
		if !supportsS256(login.Metadata.CodeChallengeMethodsSupported) {
			t.Fatalf(
				"Metadata.CodeChallengeMethodsSupported = %#v, want S256",
				login.Metadata.CodeChallengeMethodsSupported,
			)
		}
	})

	t.Run("Should prefer discovered metadata when direct endpoints are also configured", func(t *testing.T) {
		t.Parallel()

		handlerErrors := newHandlerErrorRecorder()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/metadata":
				if err := writeJSON(w, Metadata{
					AuthorizationEndpoint:         "http://" + r.Host + "/authorize",
					TokenEndpoint:                 "http://" + r.Host + "/token",
					CodeChallengeMethodsSupported: []string{"S256"},
				}); err != nil {
					handlerErrors.record(err)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer func() {
			server.Close()
			handlerErrors.assertEmpty(t)
		}()
		service, err := NewService(newMemoryTokenStore(), WithHTTPClient(server.Client()))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		cfg := ServerConfig{
			Target:           globalTestTarget("mcp"),
			Type:             "oauth2_pkce",
			MetadataURL:      server.URL + "/metadata",
			AuthorizationURL: "https://manual.example/authorize",
			TokenURL:         "https://manual.example/token",
			ClientID:         "client",
		}

		login, err := service.BeginLogin(context.Background(), cfg, "http://127.0.0.1/callback")
		if err != nil {
			t.Fatalf("BeginLogin() error = %v", err)
		}
		authURL, err := url.Parse(login.AuthorizationURL)
		if err != nil {
			t.Fatalf("url.Parse(AuthorizationURL) error = %v", err)
		}
		metadataURL, err := url.Parse(server.URL + "/authorize")
		if err != nil {
			t.Fatalf("url.Parse(metadata authorization URL) error = %v", err)
		}
		if got, want := authURL.Host, metadataURL.Host; got != want {
			t.Fatalf("authorization host = %q, want discovered metadata host %q", got, want)
		}
		if got, want := authURL.Path, "/authorize"; got != want {
			t.Fatalf("authorization path = %q, want %q", got, want)
		}
	})
}
