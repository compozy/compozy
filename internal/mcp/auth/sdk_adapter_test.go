package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/mcp/securehttp"
	"github.com/compozy/compozy/internal/testutil/mcpfixture"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func TestHostedOAuthAdapterPersistsDCRBeforeHandoff(t *testing.T) {
	t.Parallel()
	t.Run("Should persist DCR before authorization handoff", func(t *testing.T) {
		t.Parallel()
		fixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{},
		)
		store := &adapterTestStore{}
		service, err := NewService(
			store,
			withHTTPClientForTest(fixture.Server.Client()),
			WithRegistrationStore(store),
			WithSecretRefResolver(func(context.Context, string) (string, error) {
				return store.secrets.ClientSecret, nil
			}),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		state, err := service.BeginLogin(
			t.Context(),
			ServerConfig{
				Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
				Type:         authTypeOAuth,
				RemoteURL:    fixture.Endpoints.MCPURL,
				Registration: RegistrationAuto,
				Scopes:       []string{"tools.read"},
			},
			"http://127.0.0.1:2123/api/mcp/oauth/callback",
		)
		if err != nil {
			t.Fatalf("BeginLogin() error = %v", err)
		}
		if state.AuthorizationURL == "" || store.registration.ClientID == "" {
			t.Fatal("BeginLogin() did not persist DCR before authorization handoff")
		}
		if got, want := state.Config.TokenEndpointAuthMethod, "client_secret_post"; got != want {
			t.Fatalf("BeginLogin() token endpoint auth method = %q, want %q", got, want)
		}
		if got, want := store.registration.TokenEndpointAuthMethod, "client_secret_post"; got != want {
			t.Fatalf("persisted token endpoint auth method = %q, want %q", got, want)
		}
		if store.secrets.ClientSecret == "" || store.secrets.RegistrationAccessToken == "" ||
			store.registration.RegistrationClientURI == "" {
			t.Fatal(
				"DCR management credentials were not supplied as an atomic pair to the store boundary",
			)
		}
	})
}

func TestCatalogOAuthBlocksLoopbackDiscovery(t *testing.T) {
	t.Parallel()
	t.Run("Should keep catalog OAuth discovery on the public-only client", func(t *testing.T) {
		t.Parallel()
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			requests.Add(1)
		}))
		defer server.Close()
		store := &adapterTestStore{}
		service, err := NewService(
			store,
			withHTTPClientForTest(server.Client()),
			WithRegistrationStore(store),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		_, err = service.BeginLogin(
			t.Context(),
			ServerConfig{
				Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
				Type:         authTypeOAuth,
				RemoteURL:    server.URL + "/mcp",
				CatalogEntry: "fixture-catalog",
				Registration: RegistrationAuto,
			},
			"http://127.0.0.1:2123/api/mcp/oauth/callback",
		)
		if err == nil {
			t.Fatal("BeginLogin(catalog loopback) error = nil, want public-only policy rejection")
		}
		if got := requests.Load(); got != 0 {
			t.Fatalf("catalog loopback requests = %d, want 0", got)
		}
	})
}

func TestCatalogOAuthBlocksLoopbackCredentialEndpoints(t *testing.T) {
	t.Parallel()
	t.Run("Should route token and revocation requests through the public-only client", func(t *testing.T) {
		t.Parallel()
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			requests.Add(1)
			writer.Header().Set("Content-Type", "application/json")
			if request.URL.Path == "/token" {
				if _, err := writer.Write([]byte(`{"access_token":"access","token_type":"Bearer"}`)); err != nil {
					t.Errorf("write token response: %v", err)
				}
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		service, err := NewService(&adapterTestStore{}, withHTTPClientForTest(server.Client()))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		cfg := ServerConfig{
			Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
			Type:         authTypeOAuth,
			CatalogEntry: "fixture-catalog",
			ClientID:     "client",
		}
		metadata := Metadata{TokenEndpoint: server.URL + "/token", RevocationEndpoint: server.URL + "/revoke"}
		current := TokenRecord{RefreshToken: "refresh", AccessToken: "access"}
		if _, err := service.refreshToken(t.Context(), cfg, metadata, current); err == nil {
			t.Fatal("refreshToken(catalog loopback) error = nil, want public-only policy rejection")
		}
		if err := service.revoke(t.Context(), cfg, metadata, current); err == nil {
			t.Fatal("revoke(catalog loopback) error = nil, want public-only policy rejection")
		}
		if got := requests.Load(); got != 0 {
			t.Fatalf("catalog credential endpoint requests = %d, want 0", got)
		}
	})
}

func TestManagerExchangeSupportsRFC9207Issuer(t *testing.T) {
	t.Parallel()
	t.Run("Should accept the advertised issuer and reject a mismatched issuer", func(t *testing.T) {
		t.Parallel()
		fixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{AuthorizationResponseIssParameterSupported: true},
		)
		store := &adapterTestStore{}
		service, err := NewService(
			store,
			withHTTPClientForTest(fixture.Server.Client()),
			WithRegistrationStore(store),
			WithSecretRefResolver(func(context.Context, string) (string, error) {
				return store.secrets.ClientSecret, nil
			}),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		manager, err := NewManager(service)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cfg := ServerConfig{
			Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
			Type:         authTypeOAuth,
			RemoteURL:    fixture.Endpoints.MCPURL,
			Registration: RegistrationAuto,
		}

		invalid, err := manager.Begin(t.Context(), cfg, "http://127.0.0.1:2123/callback")
		if err != nil {
			t.Fatalf("Begin(invalid issuer) error = %v", err)
		}
		invalidCallback := oauthCallbackURL(t, invalid.CallbackURL, invalid.State, "https://wrong.example")
		if _, err := manager.Exchange(
			t.Context(),
			cfg,
			ExchangeInput{RedirectURL: invalidCallback},
		); !errors.Is(err, ErrInvalidExchange) {
			t.Fatalf("Exchange(mismatched issuer) error = %v, want ErrInvalidExchange", err)
		}

		valid, err := manager.Begin(t.Context(), cfg, "http://127.0.0.1:2123/callback")
		if err != nil {
			t.Fatalf("Begin(valid issuer) error = %v", err)
		}
		validCallback := oauthCallbackURL(t, valid.CallbackURL, valid.State, fixture.Endpoints.IssuerURL)
		if _, err := manager.Exchange(
			t.Context(),
			cfg,
			ExchangeInput{RedirectURL: validCallback},
		); err != nil {
			t.Fatalf("Exchange(advertised issuer) error = %v", err)
		}
	})
}

func TestPreRegisteredAuthorizationBindsResource(t *testing.T) {
	t.Parallel()
	t.Run("Should include the protected resource in the authorization request", func(t *testing.T) {
		t.Parallel()
		fixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{},
		)
		service, err := NewService(&adapterTestStore{}, withHTTPClientForTest(fixture.Server.Client()))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		state, err := service.BeginLogin(
			t.Context(),
			ServerConfig{
				Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
				Type:         authTypeOAuth,
				RemoteURL:    fixture.Endpoints.MCPURL,
				IssuerURL:    fixture.Endpoints.IssuerURL,
				ClientID:     "client",
				Registration: RegistrationPreRegistered,
			},
			"http://127.0.0.1:2123/callback",
		)
		if err != nil {
			t.Fatalf("BeginLogin() error = %v", err)
		}
		parsed, err := url.Parse(state.AuthorizationURL)
		if err != nil {
			t.Fatalf("url.Parse(AuthorizationURL) error = %v", err)
		}
		if got, want := parsed.Query().Get("resource"), fixture.Endpoints.MCPURL; got != want {
			t.Fatalf("authorization resource = %q, want %q", got, want)
		}
	})
}

func TestDynamicRegistrationReuseValidatesBindings(t *testing.T) {
	t.Parallel()
	t.Run("Should re-register when redirect scopes or secret lifetime no longer cover the flow", func(t *testing.T) {
		t.Parallel()
		fixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{Scopes: []string{"tools.read", "tools.write"}},
		)
		store := &adapterTestStore{}
		now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
		service, err := NewService(
			store,
			withHTTPClientForTest(fixture.Server.Client()),
			WithRegistrationStore(store),
			WithSecretRefResolver(func(context.Context, string) (string, error) {
				return store.secrets.ClientSecret, nil
			}),
			WithNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		cfg := ServerConfig{
			Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
			Type:         authTypeOAuth,
			RemoteURL:    fixture.Endpoints.MCPURL,
			Registration: RegistrationAuto,
			Scopes:       []string{"tools.read"},
		}
		firstRedirect := "http://127.0.0.1:2123/callback"
		if _, err := service.BeginLogin(t.Context(), cfg, firstRedirect); err != nil {
			t.Fatalf("BeginLogin(first) error = %v", err)
		}
		if _, err := service.BeginLogin(t.Context(), cfg, firstRedirect); err != nil {
			t.Fatalf("BeginLogin(reuse) error = %v", err)
		}
		if got := len(fixture.Requests().Registration); got != 1 {
			t.Fatalf("registration calls after valid reuse = %d, want 1", got)
		}

		cfg.Scopes = []string{"tools.read", "tools.write"}
		if _, err := service.BeginLogin(t.Context(), cfg, firstRedirect); err != nil {
			t.Fatalf("BeginLogin(step-up scope) error = %v", err)
		}
		secondRedirect := "http://127.0.0.1:2124/callback"
		if _, err := service.BeginLogin(t.Context(), cfg, secondRedirect); err != nil {
			t.Fatalf("BeginLogin(changed redirect) error = %v", err)
		}
		store.registration.ClientSecretExpiresAt = now
		if _, err := service.BeginLogin(t.Context(), cfg, secondRedirect); err != nil {
			t.Fatalf("BeginLogin(expired secret) error = %v", err)
		}
		if got := len(fixture.Requests().Registration); got != 4 {
			t.Fatalf("registration calls after invalidated bindings = %d, want 4", got)
		}
	})
}

func TestTokenEndpointClientAuthentication(t *testing.T) {
	t.Parallel()
	t.Run("Should use HTTP Basic for a DCR client registered with client_secret_basic", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if got, want := request.Header.Get("Content-Type"), "application/x-www-form-urlencoded"; got != want {
				http.Error(writer, "unexpected content type", http.StatusUnsupportedMediaType)
				return
			}
			if got, want := request.Header.Get("Accept"), "application/json"; got != want {
				http.Error(writer, "unexpected accept header", http.StatusNotAcceptable)
				return
			}
			clientID, clientSecret, ok := request.BasicAuth()
			if !ok || clientID != "client" || clientSecret != "secret" {
				http.Error(writer, "missing basic client authentication", http.StatusUnauthorized)
				return
			}
			if err := request.ParseForm(); err != nil {
				http.Error(writer, "invalid form", http.StatusBadRequest)
				return
			}
			if request.Form.Get("client_secret") != "" {
				http.Error(writer, "client secret leaked into form", http.StatusBadRequest)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			if _, err := writer.Write([]byte(`{"access_token":"access","token_type":"Bearer"}`)); err != nil {
				t.Errorf("write token response: %v", err)
			}
		}))
		defer server.Close()

		service, err := NewService(&adapterTestStore{}, withHTTPClientForTest(server.Client()))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		cfg := ServerConfig{
			Target:                  Target{Scope: ScopeUser, ServerName: "fixture"},
			Type:                    authTypeOAuth,
			ClientID:                "client",
			ClientSecret:            "secret",
			TokenEndpointAuthMethod: "client_secret_basic",
		}
		state := LoginState{
			Target:      cfg.Target,
			RedirectURL: "http://127.0.0.1:2123/callback",
			Verifier:    strings.Repeat("v", 43),
			Metadata:    Metadata{TokenEndpoint: server.URL},
			Config:      cfg,
		}
		if _, err := service.exchangeCode(t.Context(), &state, "code"); err != nil {
			t.Fatalf("exchangeCode(client_secret_basic) error = %v", err)
		}
	})

	t.Run("Should default an omitted public DCR auth method to none", func(t *testing.T) {
		t.Parallel()
		method, err := registrationTokenEndpointAuthMethod("", "")
		if err != nil {
			t.Fatalf("registrationTokenEndpointAuthMethod() error = %v", err)
		}
		if got, want := method, tokenEndpointAuthMethodNone; got != want {
			t.Fatalf("registrationTokenEndpointAuthMethod() = %q, want %q", got, want)
		}
	})
}

func TestProtectedResourceMetadataURLValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should accept advertised metadata on the resource host", func(t *testing.T) {
		t.Parallel()
		if err := validateAdvertisedMetadataURL(
			"https://mcp.example.test/metadata",
			"https://mcp.example.test/connect",
		); err != nil {
			t.Fatalf("validateAdvertisedMetadataURL() error = %v", err)
		}
	})

	t.Run("Should reject advertised metadata on another host", func(t *testing.T) {
		t.Parallel()
		if err := validateAdvertisedMetadataURL(
			"https://attacker.example.test/metadata",
			"https://mcp.example.test/connect",
		); err == nil {
			t.Fatal("validateAdvertisedMetadataURL() error = nil, want host rejection")
		}
	})
}

func TestProtectedResourceMetadataURLs(t *testing.T) {
	t.Parallel()
	t.Run("Should omit resource query and fragment from endpoint metadata candidates", func(t *testing.T) {
		t.Parallel()
		candidates := protectedResourceMetadataURLs("", "https://mcp.example.test/connect?tenant=a#section")
		if len(candidates) == 0 {
			t.Fatal("protectedResourceMetadataURLs() returned no candidates")
		}
		endpoint, err := url.Parse(candidates[0].URL)
		if err != nil {
			t.Fatalf("url.Parse(endpoint candidate) error = %v", err)
		}
		if endpoint.RawQuery != "" || endpoint.Fragment != "" {
			t.Fatalf("endpoint candidate = %q, want no query or fragment", candidates[0].URL)
		}
	})
}

func TestRegisterClientReportsRejectionStatus(t *testing.T) {
	t.Parallel()
	t.Run("Should include only the HTTP status for a rejected registration", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, "sensitive provider detail", http.StatusForbidden)
		}))
		defer server.Close()
		service, err := NewService(&adapterTestStore{}, withHTTPClientForTest(server.Client()))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		_, err = service.registerClient(
			t.Context(),
			ServerConfig{},
			server.URL,
			&oauthex.ClientRegistrationMetadata{},
		)
		if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
			t.Fatalf("registerClient(rejected) error = %v, want safe HTTP status", err)
		}
		if strings.Contains(err.Error(), "sensitive provider detail") {
			t.Fatalf("registerClient(rejected) leaked response body: %v", err)
		}
	})

	t.Run("Should bound a successful dynamic registration response", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusCreated)
			oversized := strings.Repeat("x", int(maxDynamicRegistrationResponseBytes)+1)
			if _, err := writer.Write([]byte(oversized)); err != nil {
				t.Errorf("write oversized DCR response: %v", err)
			}
		}))
		defer server.Close()
		service, err := NewService(&adapterTestStore{}, withHTTPClientForTest(server.Client()))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		_, err = service.registerClient(
			t.Context(),
			ServerConfig{},
			server.URL,
			&oauthex.ClientRegistrationMetadata{},
		)
		if !errors.Is(err, securehttp.ErrResponseBodyTooLarge) {
			t.Fatalf("registerClient(oversized) error = %v, want ErrResponseBodyTooLarge", err)
		}
	})
}

func TestAuthorizationMetadataErrorsRedactRedirectSecrets(t *testing.T) {
	t.Parallel()
	t.Run("Should omit a rejected redirect URL from SDK discovery errors", func(t *testing.T) {
		t.Parallel()
		const sensitiveCode = "sensitive-authorization-code"
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(
				writer,
				request,
				"https://10.0.0.1/callback?code="+sensitiveCode,
				http.StatusFound,
			)
		}))
		defer server.Close()
		service, err := NewService(
			&adapterTestStore{},
			WithSecureHTTPClient(securehttp.NewClient(securehttp.WithAllowLoopback(true))),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		_, err = service.BeginLogin(
			t.Context(),
			ServerConfig{
				Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
				Type:         authTypeOAuth,
				IssuerURL:    server.URL,
				ClientID:     "client",
				Registration: RegistrationPreRegistered,
			},
			managerTestCallbackURL,
		)
		if err == nil {
			t.Fatal("BeginLogin(rejected metadata redirect) error = nil")
		}
		if strings.Contains(err.Error(), sensitiveCode) || strings.Contains(err.Error(), "10.0.0.1") {
			t.Fatalf("BeginLogin() error leaked redirect URL material: %v", err)
		}
	})
}

func TestManagerExchangeRejectsMismatchedRedirectURL(t *testing.T) {
	t.Parallel()
	t.Run("Should retain the pending flow when callback origin differs", func(t *testing.T) {
		t.Parallel()
		fixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{},
		)
		store := &adapterTestStore{}
		service, err := NewService(
			store,
			withHTTPClientForTest(fixture.Server.Client()),
			WithRegistrationStore(store),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		manager, err := NewManager(service)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cfg := ServerConfig{
			Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
			Type:         authTypeOAuth,
			RemoteURL:    fixture.Endpoints.MCPURL,
			Registration: RegistrationAuto,
		}
		begin, err := manager.Begin(
			t.Context(),
			cfg,
			"http://127.0.0.1:2123/api/mcp/oauth/callback",
		)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		_, err = manager.Exchange(
			t.Context(),
			cfg,
			ExchangeInput{
				RedirectURL: "http://evil.example/callback?code=" + mcpfixture.AuthorizationCode + "&state=" + begin.State,
			},
		)
		if !errors.Is(err, ErrInvalidExchange) {
			t.Fatalf("Exchange(mismatched redirect) error = %v, want invalid exchange", err)
		}
		_, err = manager.Exchange(
			t.Context(),
			cfg,
			ExchangeInput{
				RedirectURL: begin.CallbackURL + "?code=" + mcpfixture.AuthorizationCode + "&state=" + begin.State,
			},
		)
		if err != nil {
			t.Fatalf("Exchange(valid redirect after rejection) error = %v", err)
		}
	})
}

func TestManagerStepUpPreservesApprovedScopes(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should exchange and report explicitly approved scopes against the baseline definition",
		func(t *testing.T) {
			t.Parallel()
			fixture := mcpfixture.StartOAuthHTTP(
				t,
				mcpfixture.MustNew(mcpfixture.ProfileModern2026),
				mcpfixture.OAuthConfig{Scopes: []string{"tools.read", "tools.write"}},
			)
			store := &adapterTestStore{}
			service, err := NewService(
				store,
				withHTTPClientForTest(fixture.Server.Client()),
				WithRegistrationStore(store),
			)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			manager, err := NewManager(service)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			cfg := ServerConfig{
				Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
				Type:         authTypeOAuth,
				RemoteURL:    fixture.Endpoints.MCPURL,
				Registration: RegistrationAuto,
				Scopes:       []string{"tools.read"},
			}
			if _, err := manager.BeginStepUp(
				t.Context(),
				cfg,
				"http://127.0.0.1:2123/api/mcp/oauth/callback",
				[]string{"tools.write"},
				false,
			); err == nil {
				t.Fatal("BeginStepUp(false) error = nil, want explicit approval failure")
			}
			begin, err := manager.BeginStepUp(
				t.Context(),
				cfg,
				"http://127.0.0.1:2123/api/mcp/oauth/callback",
				[]string{"tools.write"},
				true,
			)
			if err != nil {
				t.Fatalf("BeginStepUp(true) error = %v", err)
			}
			_, err = manager.Exchange(
				t.Context(),
				cfg,
				ExchangeInput{
					RedirectURL: begin.CallbackURL + "?code=" + mcpfixture.AuthorizationCode + "&state=" + begin.State,
				},
			)
			if err != nil {
				t.Fatalf("Exchange(step-up redirect) error = %v", err)
			}
			status, err := manager.Status(t.Context(), cfg)
			if err != nil {
				t.Fatalf("Status(baseline definition) error = %v", err)
			}
			if len(status.Scopes) != 2 || status.Scopes[1] != "tools.write" {
				t.Fatalf("Status().Scopes = %#v, want approved step-up scope", status.Scopes)
			}
		},
	)
}

func TestAuthorizationTokenRequiresBaselineScopes(t *testing.T) {
	t.Parallel()
	t.Run("Should accept step-up scopes and reject a missing baseline scope", func(t *testing.T) {
		t.Parallel()
		store := &adapterTestStore{}
		service, err := NewService(store)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		cfg := ServerConfig{
			Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
			Type:         authTypeOAuth,
			IssuerURL:    "https://issuer.example",
			ClientID:     "client",
			Registration: RegistrationPreRegistered,
			Scopes:       []string{"tools.read"},
		}
		fingerprint, err := ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
		}
		store.token = TokenRecord{
			Target:                cfg.Target,
			DefinitionFingerprint: fingerprint,
			Issuer:                cfg.IssuerURL,
			ClientID:              cfg.ClientID,
			AccessToken:           "bearer",
			Scopes:                []string{"tools.read", "tools.write"},
		}
		if _, err := service.AuthorizationToken(t.Context(), cfg); err != nil {
			t.Fatalf("AuthorizationToken(step-up scopes) error = %v", err)
		}
		cfg.Scopes = []string{"tools.read", "admin"}
		if _, err := service.AuthorizationToken(t.Context(), cfg); !errors.Is(
			err,
			ErrTokenBindingInvalid,
		) {
			t.Fatalf("AuthorizationToken(missing baseline) error = %v, want invalid binding", err)
		}
	})
}

func TestOAuthTokenResponseLifecycleContract(t *testing.T) {
	t.Parallel()
	service, err := NewService(
		&adapterTestStore{},
		WithNow(func() time.Time { return time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC) }),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	cfg := ServerConfig{
		Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
		Type:         authTypeOAuth,
		IssuerURL:    "https://issuer.example",
		ClientID:     "client",
		Registration: RegistrationPreRegistered,
	}
	metadata := Metadata{Issuer: cfg.IssuerURL}

	t.Run("Should reject negative and overflowing expires_in values", func(t *testing.T) {
		t.Parallel()
		const overflowingExpiresIn = int64(1<<63-1)/int64(time.Second) + 1
		for _, test := range []struct {
			name      string
			expiresIn int64
			want      string
		}{
			{name: "Should reject a negative value", expiresIn: -1, want: "must not be negative"},
			{name: "Should reject an overflowing value", expiresIn: overflowingExpiresIn, want: "overflows duration"},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				_, err := service.tokenRecordFromResponse(
					cfg,
					metadata,
					tokenEndpointResponse{
						AccessToken: "access",
						TokenType:   serviceBearerValue,
						ExpiresIn:   tokenExpiresIn{present: true, value: test.expiresIn},
					},
					TokenRecord{},
				)
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("tokenRecordFromResponse(expires_in) error = %v, want %q", err, test.want)
				}
			})
		}
	})

	t.Run("Should treat zero expires_in as expired and retain an omitted refresh token", func(t *testing.T) {
		t.Parallel()
		current := managerBoundToken(t, cfg, "old-access")
		current.RefreshToken = "retained-refresh"
		token, err := service.tokenRecordFromResponse(
			cfg,
			metadata,
			tokenEndpointResponse{
				AccessToken: "new-access",
				TokenType:   serviceBearerValue,
				ExpiresIn:   tokenExpiresIn{present: true, value: 0},
			},
			current,
		)
		if err != nil {
			t.Fatalf("tokenRecordFromResponse() error = %v", err)
		}
		if token.RefreshToken != current.RefreshToken {
			t.Fatalf("refreshed token = %q, want retained %q", token.RefreshToken, current.RefreshToken)
		}
		if !token.ExpiresAt.Equal(service.now().UTC()) {
			t.Fatalf("ExpiresAt = %s, want %s", token.ExpiresAt, service.now().UTC())
		}
		if token.DefinitionFingerprint != current.DefinitionFingerprint {
			t.Fatalf("definition fingerprint changed across refresh: %q", token.DefinitionFingerprint)
		}
	})

	t.Run("Should reject malformed access-token responses", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name     string
			response tokenEndpointResponse
			want     string
		}{
			{
				name:     "Should require an access token",
				response: tokenEndpointResponse{TokenType: serviceBearerValue},
				want:     "access_token is required",
			},
			{
				name:     "Should require a Bearer token type",
				response: tokenEndpointResponse{AccessToken: "access", TokenType: "MAC"},
				want:     "token_type must be Bearer",
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				_, err := service.tokenRecordFromResponse(cfg, metadata, test.response, TokenRecord{})
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("tokenRecordFromResponse(malformed) error = %v, want %q", err, test.want)
				}
			})
		}
	})
}

func TestLogoutCleansUpAfterRevocationFailure(t *testing.T) {
	t.Parallel()
	t.Run("Should delete local authorization state when remote revocation fails", func(t *testing.T) {
		t.Parallel()
		var revocations atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration":
				writeOAuthMetadataResponse(t, writer, request, true)
			case "/revoke":
				revocations.Add(1)
				http.Error(writer, "provider detail", http.StatusBadGateway)
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()
		store := &adapterTestStore{}
		service, cfg := newPreRegisteredTestService(t, store, server)
		store.token = managerBoundToken(t, cfg, "access")
		store.token.RefreshToken = "refresh"
		status, err := service.Logout(t.Context(), cfg)
		if err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		if !store.authorizationStateDeleted || status.Status != StatusNeedsLogin ||
			!strings.Contains(status.Diagnostic, "remote revocation failed") {
			t.Fatalf("Logout() status = %#v, deleted = %t", status, store.authorizationStateDeleted)
		}
		if got := revocations.Load(); got != 1 {
			t.Fatalf("revocation calls = %d, want 1", got)
		}
	})
}

func TestOAuthRefreshCannotReinsertTokenAfterLogout(t *testing.T) {
	t.Parallel()
	t.Run("Should reject a refresh commit after another service logs out", func(t *testing.T) {
		t.Parallel()
		refreshStarted := make(chan struct{})
		releaseRefresh := make(chan struct{})
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(releaseRefresh) }) }
		t.Cleanup(release)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration":
				writeOAuthMetadataResponse(t, writer, request, false)
			case "/token":
				close(refreshStarted)
				<-releaseRefresh
				writer.Header().Set("Content-Type", "application/json")
				if _, err := writer.Write(
					[]byte(`{"access_token":"stale-refresh","token_type":"Bearer"}`),
				); err != nil {
					t.Errorf("write refresh response: %v", err)
				}
			default:
				http.NotFound(writer, request)
			}
		}))
		t.Cleanup(server.Close)
		store := &adapterTestStore{}
		generation := NewMutationGeneration()
		refreshService, cfg := newPreRegisteredTestService(
			t,
			store,
			server,
			WithMutationGeneration(generation),
		)
		logoutService, _ := newPreRegisteredTestService(
			t,
			store,
			server,
			WithMutationGeneration(generation),
		)
		store.token = managerBoundToken(t, cfg, "existing-access")
		store.token.RefreshToken = "existing-refresh"
		refreshErr := make(chan error, 1)
		go func() {
			_, err := refreshService.Refresh(t.Context(), cfg)
			refreshErr <- err
		}()
		<-refreshStarted
		if _, err := logoutService.Logout(t.Context(), cfg); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		release()
		if err := <-refreshErr; !errors.Is(err, ErrTokenMutationStale) {
			t.Fatalf("Refresh(racing logout) error = %v, want ErrTokenMutationStale", err)
		}
		if _, err := store.GetMCPAuthToken(t.Context(), cfg.Target); !errors.Is(err, ErrTokenNotFound) {
			t.Fatalf("GetMCPAuthToken(after logout) error = %v, want ErrTokenNotFound", err)
		}
	})
}

func TestOAuthFailuresPreserveTokensAndRedactSecrets(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve the existing token without returning OAuth secrets", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration":
				writeOAuthMetadataResponse(t, writer, request, false)
			case "/token":
				if err := request.ParseForm(); err != nil {
					http.Error(writer, "invalid form", http.StatusBadRequest)
					return
				}
				secret := firstNonEmpty(request.Form.Get("code"), request.Form.Get("refresh_token"))
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusBadRequest)
				if err := json.NewEncoder(writer).Encode(map[string]string{"error": secret}); err != nil {
					t.Errorf("encode token error response: %v", err)
				}
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()
		store := &adapterTestStore{}
		service, cfg := newPreRegisteredTestService(t, store, server)
		existing := managerBoundToken(t, cfg, "existing-access")
		existing.RefreshToken = "sensitive-refresh-token"
		store.token = existing

		sensitiveCode := "sensitive-authorization-code"
		state := LoginState{
			Target:      cfg.Target,
			RedirectURL: managerTestCallbackURL,
			State:       "expected-state",
			Verifier:    strings.Repeat("v", 43),
			Metadata: Metadata{
				Issuer:        cfg.IssuerURL,
				TokenEndpoint: server.URL + "/token",
			},
			Config: cfg,
		}
		callback := managerTestCallbackURL + "?code=" + sensitiveCode + "&state=" + state.State
		if _, err := service.Exchange(t.Context(), &state, callback); err == nil {
			t.Fatal("Exchange(failed request) error = nil")
		} else if strings.Contains(err.Error(), sensitiveCode) || strings.Contains(err.Error(), callback) {
			t.Fatalf("Exchange() error leaked OAuth material: %v", err)
		}
		assertAdapterTokenUnchanged(t, store, existing)

		if _, err := service.Refresh(t.Context(), cfg); err == nil {
			t.Fatal("Refresh(failed request) error = nil")
		} else if strings.Contains(err.Error(), existing.RefreshToken) ||
			strings.Contains(err.Error(), existing.AccessToken) {
			t.Fatalf("Refresh() error leaked token material: %v", err)
		}
		assertAdapterTokenUnchanged(t, store, existing)
	})
}

func TestNormalizeRegistrationClientURI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		endpoint string
		raw      string
		want     string
		wantErr  bool
	}{
		{
			name:     "resolve a relative URI on the registration endpoint origin",
			endpoint: "https://mcp.linear.app/register",
			raw:      "/register/client-123",
			want:     "https://mcp.linear.app/register/client-123",
		},
		{
			name:     "accept an absolute URI on the registration endpoint origin",
			endpoint: "https://mcp.linear.app/register",
			raw:      "https://mcp.linear.app/register/client-123",
			want:     "https://mcp.linear.app/register/client-123",
		},
		{
			name:     "reject a cross origin URI",
			endpoint: "https://mcp.linear.app/register",
			raw:      "https://attacker.example/register/client-123",
			wantErr:  true,
		},
		{
			name:     "reject a credentialed URI",
			endpoint: "https://mcp.linear.app/register",
			raw:      "https://client:secret@mcp.linear.app/register/client-123",
			wantErr:  true,
		},
		{
			name:     "reject an unsafe URI scheme",
			endpoint: "https://mcp.linear.app/register",
			raw:      "javascript:alert(1)",
			wantErr:  true,
		},
	}
	for _, test := range tests {
		t.Run("Should "+test.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeRegistrationClientURI(test.endpoint, test.raw)
			if test.wantErr {
				if err == nil {
					t.Fatal("normalizeRegistrationClientURI() error = nil, want rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeRegistrationClientURI() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("normalizeRegistrationClientURI() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRegisterClientDiscardsIncompleteDCRManagementCredentials(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should persist a Linear-style relative management URI without a token as an empty pair",
		func(t *testing.T) {
			t.Parallel()
			responseBody := []byte(
				`{"client_id":"client-123","token_endpoint_auth_method":"none","registration_client_uri":"/register/client-123"}`,
			)
			server := httptest.NewServer(
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					if request.Method != http.MethodPost {
						t.Errorf("request method = %s, want POST", request.Method)
					}
					writer.Header().Set("Content-Type", "application/json")
					writer.WriteHeader(http.StatusCreated)
					if _, err := writer.Write(responseBody); err != nil {
						t.Errorf("write DCR response: %v", err)
					}
				}),
			)
			defer server.Close()

			store := &adapterTestStore{}
			service, err := NewService(
				store,
				withHTTPClientForTest(server.Client()),
				WithRegistrationStore(store),
			)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			response, err := service.registerClient(
				t.Context(),
				ServerConfig{},
				server.URL+"/register",
				&oauthex.ClientRegistrationMetadata{},
			)
			if err != nil {
				t.Fatalf("registerClient() error = %v", err)
			}
			if response.RegistrationAccessToken != "" || response.RegistrationClientURI != "" {
				t.Fatalf(
					"registerClient() management credentials = (%q, %q), want empty pair",
					response.RegistrationAccessToken,
					response.RegistrationClientURI,
				)
			}
			_, _, err = service.persistDynamicRegistration(
				t.Context(),
				ServerConfig{Target: Target{Scope: ScopeUser, ServerName: "linear"}},
				server.URL+"/mcp",
				"http://127.0.0.1:2123/api/mcp/oauth/callback",
				server.URL,
				"fingerprint",
				response,
			)
			if err != nil {
				t.Fatalf("persistDynamicRegistration() error = %v", err)
			}
			if store.registration.RegistrationClientURI != "" ||
				store.secrets.RegistrationAccessToken != "" {
				t.Fatalf(
					"persisted management credentials = (%q, %q), want empty pair",
					store.secrets.RegistrationAccessToken,
					store.registration.RegistrationClientURI,
				)
			}
		},
	)
}

func TestLogoutDeletesAuthorizationStateAtomically(t *testing.T) {
	t.Parallel()
	t.Run("Should delete tokens registrations and Vault references together", func(t *testing.T) {
		t.Parallel()
		store := &adapterTestStore{}
		service, err := NewService(store)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		_, err = service.Logout(
			t.Context(),
			ServerConfig{
				Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
				Type:         authTypeOAuth,
				RemoteURL:    "https://fixture.example/mcp",
				Registration: RegistrationAuto,
			},
		)
		if err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		if !store.authorizationStateDeleted {
			t.Fatal("Logout() did not delete the complete authorization state")
		}
	})
}

type adapterTestStore struct {
	mu                        sync.Mutex
	registration              ClientRegistration
	secrets                   RegistrationSecrets
	authorizationStateDeleted bool
	token                     TokenRecord
}

func (s *adapterTestStore) SaveMCPAuthToken(_ context.Context, token TokenRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
	return nil
}
func (s *adapterTestStore) GetMCPAuthToken(context.Context, Target) (TokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token.AccessToken == "" {
		return TokenRecord{}, ErrTokenNotFound
	}
	return s.token, nil
}

func (s *adapterTestStore) ListMCPAuthTokens(
	context.Context,
) ([]TokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil, nil
}
func (s *adapterTestStore) DeleteMCPAuthToken(context.Context, Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = TokenRecord{}
	return nil
}
func (s *adapterTestStore) DeleteMCPAuthorizationState(context.Context, Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authorizationStateDeleted = true
	s.token = TokenRecord{}
	s.registration = ClientRegistration{}
	s.secrets = RegistrationSecrets{}
	return nil
}

func (s *adapterTestStore) GetMCPAuthRegistration(
	context.Context,
	Target,
) (ClientRegistration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.registration.ClientID == "" {
		return ClientRegistration{}, ErrRegistrationNotFound
	}
	return s.registration, nil
}
func (s *adapterTestStore) SaveMCPAuthRegistration(
	_ context.Context,
	registration ClientRegistration,
	secrets RegistrationSecrets,
) (ClientRegistration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(secrets.ClientSecret) != "" {
		registration.ClientSecretRef = "vault:mcp/test/client-secret"
	}
	s.registration = registration
	s.secrets = secrets
	return registration, nil
}
func (s *adapterTestStore) DeleteMCPAuthRegistration(context.Context, Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registration = ClientRegistration{}
	s.secrets = RegistrationSecrets{}
	return nil
}

func newPreRegisteredTestService(
	t *testing.T,
	store TokenStore,
	server *httptest.Server,
	extraOptions ...ServiceOption,
) (*Service, ServerConfig) {
	t.Helper()
	options := []ServiceOption{withHTTPClientForTest(server.Client())}
	options = append(options, extraOptions...)
	service, err := NewService(store, options...)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service, ServerConfig{
		Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
		Type:         authTypeOAuth,
		RemoteURL:    server.URL + "/mcp",
		IssuerURL:    server.URL,
		ClientID:     "client",
		Registration: RegistrationPreRegistered,
	}
}

func writeOAuthMetadataResponse(
	t *testing.T,
	writer http.ResponseWriter,
	request *http.Request,
	withRevocation bool,
) {
	t.Helper()
	revocation := ""
	if withRevocation {
		revocation = "http://" + request.Host + "/revoke"
	}
	writer.Header().Set("Content-Type", "application/json")
	if _, err := fmt.Fprintf(
		writer,
		`{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"revocation_endpoint":%q,"code_challenge_methods_supported":["S256"]}`,
		"http://"+request.Host,
		"http://"+request.Host+"/authorize",
		"http://"+request.Host+"/token",
		revocation,
	); err != nil {
		t.Errorf("write OAuth metadata response: %v", err)
	}
}

func assertAdapterTokenUnchanged(t *testing.T, store *adapterTestStore, want TokenRecord) {
	t.Helper()
	got, err := store.GetMCPAuthToken(t.Context(), want.Target)
	if err != nil {
		t.Fatalf("GetMCPAuthToken() error = %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken ||
		got.DefinitionFingerprint != want.DefinitionFingerprint {
		t.Fatalf("stored token changed after failed OAuth operation: %#v", got)
	}
}

func oauthCallbackURL(t *testing.T, redirectURL string, state string, issuer string) string {
	t.Helper()
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("url.Parse(redirect URL) error = %v", err)
	}
	values := parsed.Query()
	values.Set("code", mcpfixture.AuthorizationCode)
	values.Set("state", state)
	values.Set("iss", issuer)
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

var _ TokenStore = (*adapterTestStore)(nil)
var _ RegistrationStore = (*adapterTestStore)(nil)
