package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
			WithHTTPClient(fixture.Server.Client()),
			WithRegistrationStore(store),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		state, err := service.BeginLogin(
			t.Context(),
			ServerConfig{
				Target:       Target{Scope: ScopeGlobal, ServerName: "fixture"},
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
		fixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{},
		)
		store := &adapterTestStore{}
		service, err := NewService(
			store,
			WithHTTPClient(fixture.Server.Client()),
			WithRegistrationStore(store),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		_, err = service.BeginLogin(
			t.Context(),
			ServerConfig{
				Target:       Target{Scope: ScopeGlobal, ServerName: "fixture"},
				Type:         authTypeOAuth,
				RemoteURL:    fixture.Endpoints.MCPURL,
				CatalogEntry: "fixture-catalog",
				Registration: RegistrationAuto,
			},
			"http://127.0.0.1:2123/api/mcp/oauth/callback",
		)
		if err == nil {
			t.Fatal("BeginLogin(catalog loopback) error = nil, want public-only policy rejection")
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
			WithHTTPClient(fixture.Server.Client()),
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
			Target:       Target{Scope: ScopeGlobal, ServerName: "fixture"},
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
				WithHTTPClient(fixture.Server.Client()),
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
				Target:       Target{Scope: ScopeGlobal, ServerName: "fixture"},
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
			Target:       Target{Scope: ScopeGlobal, ServerName: "fixture"},
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
			server := httptest.NewServer(
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					if request.Method != http.MethodPost {
						t.Errorf("request method = %s, want POST", request.Method)
					}
					writer.Header().Set("Content-Type", "application/json")
					writer.WriteHeader(http.StatusCreated)
					if _, err := writer.Write([]byte(`{"client_id":"client-123","registration_client_uri":"/register/client-123"}`)); err != nil {
						t.Errorf("write DCR response: %v", err)
					}
				}),
			)
			defer server.Close()

			store := &adapterTestStore{}
			service, err := NewService(
				store,
				WithHTTPClient(server.Client()),
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
				ServerConfig{Target: Target{Scope: ScopeGlobal, ServerName: "linear"}},
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
				Target:       Target{Scope: ScopeGlobal, ServerName: "fixture"},
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
	registration              ClientRegistration
	secrets                   RegistrationSecrets
	authorizationStateDeleted bool
	token                     TokenRecord
}

func (s *adapterTestStore) SaveMCPAuthToken(_ context.Context, token TokenRecord) error {
	s.token = token
	return nil
}
func (s *adapterTestStore) GetMCPAuthToken(context.Context, Target) (TokenRecord, error) {
	if s.token.AccessToken == "" {
		return TokenRecord{}, ErrTokenNotFound
	}
	return s.token, nil
}

func (s *adapterTestStore) ListMCPAuthTokens(
	context.Context,
) ([]TokenRecord, error) {
	return nil, nil
}
func (s *adapterTestStore) DeleteMCPAuthToken(context.Context, Target) error { return nil }
func (s *adapterTestStore) DeleteMCPAuthorizationState(context.Context, Target) error {
	s.authorizationStateDeleted = true
	return nil
}

func (s *adapterTestStore) GetMCPAuthRegistration(
	context.Context,
	Target,
) (ClientRegistration, error) {
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
	s.registration = registration
	s.secrets = secrets
	return registration, nil
}
func (s *adapterTestStore) DeleteMCPAuthRegistration(context.Context, Target) error { return nil }

var _ TokenStore = (*adapterTestStore)(nil)
var _ RegistrationStore = (*adapterTestStore)(nil)
