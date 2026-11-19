package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/mcp/securehttp"
	"github.com/compozy/compozy/internal/testutil/mcpfixture"
	"github.com/compozy/compozy/internal/vault"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// Invariant: augment only supported grants and preserve the issuer's authorization parameters.
// Owner: OAuth authorization and token exchange; canonical suite: sdk_adapter_test.go.
func TestOAuthOfflineAccess(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, scopes, grants, prompt string
		configured, want             []string
		refresh                      bool
	}{
		{
			name:   "Should request advertised offline access and preserve issuer consent",
			scopes: "tools.read offline_access", grants: "authorization_code refresh_token", prompt: "consent",
			configured: []string{"tools.read"}, want: []string{"tools.read", "offline_access"}, refresh: true,
		},
		{
			name:   "Should accept access without a refresh token and leave issuer prompts alone",
			scopes: "tools.read offline_access", configured: []string{"tools.read"},
			want: []string{"tools.read", "offline_access"},
		},
		{
			name:   "Should retain an explicitly configured offline scope without duplicating it",
			scopes: "tools.read offline_access", grants: "authorization_code refresh_token",
			configured: []string{"tools.read", "offline_access"}, want: []string{"tools.read", "offline_access"},
			refresh: true,
		},
		{
			name:   "Should preserve existing resource scopes when offline access is not advertised",
			scopes: "tools.read", grants: "authorization_code refresh_token",
			configured: []string{"tools.read"}, want: []string{"tools.read"},
		},
		{
			name:   "Should leave scopes unchanged when refresh is excluded from the grant types",
			scopes: "tools.read offline_access", grants: "authorization_code",
			configured: []string{"tools.read"}, want: []string{"tools.read"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				base := "http://" + r.Host
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/.well-known/oauth-authorization-server":
					endpoint := base + "/authorize"
					if tc.prompt != "" {
						endpoint += "?prompt=" + url.QueryEscape(tc.prompt)
					}
					metadata := map[string]any{
						"issuer":                 base,
						"authorization_endpoint": endpoint,
						"token_endpoint":         base + "/token",
						"code_challenge_methods_supported": []string{
							"S256",
						},
						"scopes_supported": strings.Fields(tc.scopes),
					}
					if tc.grants != "" {
						metadata["grant_types_supported"] = strings.Fields(tc.grants)
					}
					if err := json.NewEncoder(w).Encode(metadata); err != nil {
						t.Error(err)
					}
				case "/token":
					response := `{"access_token":"fixture-access","scope":"tools.read","expires_in":3600}`
					if tc.refresh {
						response = `{"access_token":"fixture-access","refresh_token":"fixture-refresh","expires_in":3600}`
					}
					writeManagerTestResponse(t, w, response)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)
			store := &adapterTestStore{}
			service, cfg := newPreRegisteredTestService(t, store, server)
			cfg.Scopes = tc.configured
			login, err := service.BeginLogin(t.Context(), cfg, "http://127.0.0.1:2123/callback")
			if err != nil {
				t.Fatal(err)
			}
			authURL, err := url.Parse(login.AuthorizationURL)
			if err != nil {
				t.Fatal("unable to parse authorization URL")
			}
			query := authURL.Query()
			if got := strings.Fields(query.Get("scope")); !slices.Equal(got, tc.want) {
				t.Errorf("requested scopes = %v, want %v", got, tc.want)
			}
			if query.Get("resource") != cfg.RemoteURL || query.Get("prompt") != tc.prompt {
				t.Error("resource or issuer consent parameter changed")
			}
			exchanged, err := service.Exchange(
				t.Context(), &login, oauthCallbackURL(t, login.RedirectURL, login.State, ""),
			)
			if err != nil || exchanged.Status != StatusAuthenticated || exchanged.Refreshable != tc.refresh {
				t.Fatalf("exchange status = %+v, err = %v", exchanged, err)
			}
			if tc.refresh && !slices.Equal(exchanged.Scopes, tc.want) {
				t.Errorf("scope omitted by issuer should preserve the request, got %v", exchanged.Scopes)
			}
		})
	}
	t.Run("Should skip offline access for a client document that only declares authorization code", func(t *testing.T) {
		t.Parallel()
		fixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{
				Scopes:                            []string{"tools.read", "offline_access"},
				ClientIDMetadataDocumentSupported: true,
			},
		)
		const callback = "http://127.0.0.1:2123/callback"
		document := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeManagerTestResponse(t, w, fmt.Sprintf(
				`{"client_id":%q,"redirect_uris":[%q],"grant_types":["authorization_code"]}`,
				"https://"+r.Host+r.URL.Path, callback,
			))
		}))
		t.Cleanup(document.Close)
		clientID := document.URL + "/.well-known/mcp-client.json"
		store := &adapterTestStore{}
		service, err := NewService(store, WithRegistrationStore(store),
			withHTTPClientForTest(document.Client()), WithClientMetadataURL(clientID))
		if err != nil {
			t.Fatal(err)
		}
		login, err := service.BeginLogin(t.Context(), ServerConfig{
			Target: Target{Scope: ScopeUser, ServerName: "fixture"}, Type: authTypeOAuth,
			RemoteURL: fixture.Endpoints.MCPURL, Scopes: []string{"tools.read"},
		}, callback)
		if err != nil {
			t.Fatal(err)
		}
		authorize, err := url.Parse(login.AuthorizationURL)
		if err != nil {
			t.Fatal("unable to parse authorization URL")
		}
		query := authorize.Query()
		if query.Get("client_id") != clientID || query.Get("scope") != "tools.read" {
			t.Fatal("authorization did not select the code-only client with its resource scope")
		}
	})
}

// Invariant: a known unusable local registration skips discovery and resumes once repaired.
// Owner: hosted OAuth refresh lifecycle; canonical suite: sdk_adapter_test.go.
func TestOAuthLocalRefreshRepair(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		secretMissing bool
	}{
		{name: "Should stop retrying an expired registration secret and recover with the same grant"},
		{name: "Should stop retrying a missing registration secret and recover with the same grant", secretMissing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var discovery, refreshes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/oauth-protected-resource/mcp":
					discovery.Add(1)
					writeManagerTestResponse(t, w, fmt.Sprintf(
						`{"resource":%q,"authorization_servers":[%q]}`, "http://"+r.Host+"/mcp", "http://"+r.Host,
					))
				case "/token":
					refreshes.Add(1)
					writeManagerTestResponse(t, w, `{"access_token":"renewed","expires_in":3600}`)
				default:
					writeOAuthMetadataResponse(t, w, r, false)
				}
			}))
			t.Cleanup(server.Close)
			now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
			cfg := ServerConfig{
				Target: Target{Scope: ScopeUser, ServerName: "fixture"}, Type: authTypeOAuth,
				RemoteURL: server.URL + "/mcp", Registration: RegistrationAuto,
			}
			fingerprint, err := ServerDefinitionFingerprint(cfg)
			if err != nil {
				t.Fatal(err)
			}
			token := TokenRecord{
				Target: cfg.Target, DefinitionFingerprint: fingerprint, Issuer: server.URL, ClientID: "client",
				AccessToken: "fixture-access", RefreshToken: "fixture-refresh", ExpiresAt: now,
			}
			store := &adapterTestStore{token: token, registration: ClientRegistration{
				Target: cfg.Target, DefinitionFingerprint: fingerprint, ResourceURL: cfg.RemoteURL,
				Issuer: server.URL, ClientID: token.ClientID, TokenEndpointAuthMethod: tokenEndpointAuthMethodPost,
				RedirectURL: "http://127.0.0.1:2123/callback", ClientSecretExpiresAt: now,
				ClientSecretRef: "vault:mcp/fixture/client-secret",
			}}
			secretMissing := tc.secretMissing
			if secretMissing {
				store.registration.ClientSecretExpiresAt = now.Add(time.Hour)
			}
			var logs bytes.Buffer
			service, err := NewService(
				store, WithRegistrationStore(store), withHTTPClientForTest(server.Client()),
				WithNow(func() time.Time { return now }), WithLogger(slog.New(slog.NewJSONHandler(&logs, nil))),
				WithSecretRefResolver(func(_ context.Context, ref string) (string, error) {
					if secretMissing {
						return "", fmt.Errorf("%w: %s", vault.ErrSecretNotFound, ref)
					}
					return "fixture-secret", nil
				}),
			)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				status, err := service.Refresh(t.Context(), cfg)
				if err != nil || status.Status != StatusExpired || !status.TokenPresent || status.Refreshable ||
					!strings.Contains(status.Diagnostic, "login again") {
					t.Errorf("local refresh failure status = %+v, err = %v", status, err)
				}
				now = now.Add(time.Minute)
			}
			if discovery.Load() != 0 || refreshes.Load() != 0 || logs.Len() != 0 {
				t.Error("known local failure started remote or logged refresh attempts")
			}
			assertAdapterTokenUnchanged(t, store, token)
			passive, err := service.Status(t.Context(), cfg)
			if err != nil || passive.Refreshable || !strings.Contains(passive.Diagnostic, "login again") {
				t.Errorf("passive auth status = %+v, err = %v", passive, err)
			}
			store.registration.ClientSecretExpiresAt = now.Add(time.Hour)
			secretMissing = false
			status, err := service.Refresh(t.Context(), cfg)
			if err != nil || status.Status != StatusAuthenticated || refreshes.Load() != 1 {
				t.Fatalf("recovery status = %+v, refresh attempts = %d, err = %v", status, refreshes.Load(), err)
			}
		})
	}
}

func TestHostedOAuthAdapterPersistsDCRBeforeHandoff(t *testing.T) {
	t.Parallel()
	// Invariant: a configured issuer restricts discovery before client registration or token use.
	// Owner: hosted OAuth discovery. Canonical suite: sdk_adapter_test.go, using its real HTTP fixture.
	for _, matches := range []bool{true, false} {
		t.Run(
			fmt.Sprintf("Should enforce pinned issuer during hosted authorization matches=%v", matches),
			func(t *testing.T) {
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
					t.Fatal(err)
				}
				issuer := fixture.Endpoints.IssuerURL
				if !matches {
					issuer = "https://another-issuer.example"
				}
				cfg := ServerConfig{
					Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
					Type:         authTypeOAuth,
					RemoteURL:    fixture.Endpoints.MCPURL,
					Registration: RegistrationAuto,
					IssuerURL:    issuer,
					Scopes:       []string{"tools.read"},
				}
				state, err := service.BeginLogin(t.Context(), cfg, "http://127.0.0.1:2123/api/mcp/oauth/callback")
				if matches {
					if err != nil {
						t.Fatal(err)
					}
					if state.Metadata.Issuer != issuer || len(fixture.Requests().Registration) != 1 {
						t.Fatalf("state=%#v requests=%#v", state.Metadata, fixture.Requests())
					}
				} else {
					if err == nil || !strings.Contains(err.Error(), "configured issuer") {
						t.Fatalf("login = %v", err)
					}
					if len(fixture.Requests().Registration) != 0 || len(fixture.Requests().Token) != 0 ||
						store.registration.ClientID != "" {
						t.Fatal("issuer rejection occurred after registration or token use")
					}
				}
			},
		)
	}
	t.Run("Should select the pinned issuer when several authorities are advertised", func(t *testing.T) {
		t.Parallel()
		selected, err := hostedAuthorizationServer(
			ServerConfig{IssuerURL: "https://chosen.example"},
			[]string{"https://first.example", "https://chosen.example"},
		)
		if err != nil || selected != "https://chosen.example" {
			t.Fatalf("selected=%q err=%v", selected, err)
		}
	})

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
		// Invariant: only reusable persisted clients allow refresh, without registering replacements or sending tokens.
		// Owner: hosted OAuth registration validation; canonical suite: TestHostedOAuthAdapterPersistsDCRBeforeHandoff.
		for _, registration := range []struct {
			name        string
			change      func(*ClientRegistration)
			refreshable bool
			wantErr     error
			wantError   string
		}{
			{
				name:        "Should keep a reusable client refreshable",
				change:      func(*ClientRegistration) {},
				refreshable: true,
			},
			{
				name:    "Should reject a missing client",
				change:  func(r *ClientRegistration) { r.ClientID = "" },
				wantErr: ErrRegistrationNotFound,
			},
			{
				name:      "Should reject a changed definition",
				change:    func(r *ClientRegistration) { r.DefinitionFingerprint = "changed" },
				wantError: "stored registration is no longer valid",
			},
			{
				name:      "Should reject a changed issuer",
				change:    func(r *ClientRegistration) { r.Issuer = "https://other.example" },
				wantError: "stored registration is no longer valid",
			},
			{
				name:      "Should reject insufficient scopes",
				change:    func(r *ClientRegistration) { r.Scopes = nil },
				wantError: "stored registration is no longer valid",
			},
			{
				name: "Should reject an expired client secret",
				change: func(r *ClientRegistration) {
					r.ClientSecretExpiresAt = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
				},
				wantError: "stored registration is no longer valid",
			},
		} {
			t.Run(registration.name, func(t *testing.T) {
				t.Parallel()
				local := &adapterTestStore{registration: store.registration}
				registration.change(&local.registration)
				refresh, err := NewService(local,
					withHTTPClientForTest(fixture.Server.Client()), WithRegistrationStore(local),
					WithSecretRefResolver(func(context.Context, string) (string, error) {
						return store.secrets.ClientSecret, nil
					}))
				if err != nil {
					t.Fatal(err)
				}
				_, _, err = refresh.hostedRefreshConfig(t.Context(), state.Config)
				switch {
				case registration.wantErr != nil:
					if !errors.Is(err, registration.wantErr) {
						t.Fatalf("registration error = %v, want %v", err, registration.wantErr)
					}
				case registration.wantError != "":
					if err == nil || !strings.Contains(err.Error(), registration.wantError) {
						t.Fatalf("registration error = %v, want %q", err, registration.wantError)
					}
				case err != nil:
					t.Fatalf("reusable registration error = %v", err)
				}
				local.token = TokenRecord{
					Target: state.Config.Target, DefinitionFingerprint: store.registration.DefinitionFingerprint,
					Issuer: store.registration.Issuer, ClientID: store.registration.ClientID,
					Scopes: []string{"tools.read"}, AccessToken: "expired", RefreshToken: "refresh",
					ExpiresAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
				}
				status, err := refresh.Status(t.Context(), state.Config)
				if err != nil || status.Refreshable != registration.refreshable {
					t.Fatalf("status = %+v, error = %v, want refreshable = %v", status, err, registration.refreshable)
				}
				requests := fixture.Requests()
				if len(requests.Registration) != 1 || len(requests.Token) != 0 {
					t.Fatal("refresh registered another client or submitted credentials")
				}
			})
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
	t.Run("Should discover a separate issuer for pre-registered credentials", func(t *testing.T) {
		t.Parallel()

		var issuerURL, resourceURL string
		var metadataDown atomic.Bool
		var metadataRequests atomic.Int32
		resource := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			metadataRequests.Add(1)
			if metadataDown.Load() {
				http.Error(w, "metadata unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).
				Encode(map[string]any{"resource": resourceURL, "authorization_servers": []string{issuerURL}}); err != nil {
				t.Error(err)
			}
		}))
		t.Cleanup(resource.Close)
		resourceURL = strings.Replace(resource.URL, "127.0.0.1", "localhost", 1) + "/mcp"
		fixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{ResourceURL: resourceURL},
		)
		issuerURL = fixture.Endpoints.IssuerURL
		service, err := NewService(&adapterTestStore{}, withHTTPClientForTest(fixture.Server.Client()))
		if err != nil {
			t.Fatal(err)
		}
		state, err := service.BeginLogin(t.Context(), ServerConfig{
			Target: Target{Scope: ScopeUser, ServerName: "separate"}, Type: authTypeOAuth,
			RemoteURL: resourceURL, ClientID: "registered-client", Registration: RegistrationPreRegistered,
		}, "http://127.0.0.1:2123/callback")
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(state.AuthorizationURL)
		if err != nil {
			t.Fatal(err)
		}
		if state.Metadata.Issuer != fixture.Endpoints.IssuerURL ||
			parsed.Query().Get("client_id") != "registered-client" ||
			parsed.Query().Get("resource") != resourceURL {
			t.Fatalf("state = %+v", state.Metadata)
		}
		if len(fixture.Requests().Registration) != 0 {
			t.Fatal("pre-registered flow registered a client")
		}
		cfg := state.Config
		callback := oauthCallbackURL(t, state.RedirectURL, state.State, "")
		if _, err := service.Exchange(t.Context(), &state, callback); err != nil {
			t.Fatal(err)
		}
		assertUsable := func() {
			t.Helper()
			status, err := service.Status(t.Context(), cfg)
			if err != nil || !status.TokenPresent {
				t.Fatalf("status = %+v, err = %v", status, err)
			}
			if _, err := service.AuthorizationToken(t.Context(), cfg); err != nil {
				t.Fatal(err)
			}
			refreshed, err := service.Refresh(t.Context(), cfg)
			if err != nil || !refreshed.TokenPresent {
				t.Fatalf("refresh = %+v, err = %v", refreshed, err)
			}
		}
		assertUsable()
		manager, err := NewManager(service)
		if err != nil {
			t.Fatal(err)
		}
		begin, err := manager.Begin(t.Context(), cfg, state.RedirectURL)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.ExchangeCallback(
			t.Context(),
			cfg,
			oauthCallbackURL(t, begin.CallbackURL, begin.State, ""),
		); err != nil {
			t.Fatal(err)
		}
		assertUsable()
		metadataDown.Store(true)
		requestsBefore := metadataRequests.Load()
		for range 2 {
			status, err := service.Status(t.Context(), cfg)
			if err != nil || !status.TokenPresent {
				t.Fatalf("offline status = %+v, err = %v", status, err)
			}
			if _, err := service.AuthorizationToken(t.Context(), cfg); err != nil {
				t.Fatalf("offline bearer: %v", err)
			}
		}
		if got := metadataRequests.Load(); got != requestsBefore {
			t.Fatalf("token checks fetched metadata: %d -> %d", requestsBefore, got)
		}
	})

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

func TestCIMDRegistrationRefresh(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		legacy              bool
		change              func(*ClientRegistration)
		redirectChanged     bool
		replaceRegistration bool
		wantUsable          bool
	}{
		{name: "Should refresh a newly authorized CIMD client", wantUsable: true},
		{name: "Should refresh a stored CIMD client with an empty auth method", legacy: true, wantUsable: true},
		{name: "Should require reauthorization after changing the CIMD redirect URI", legacy: true, redirectChanged: true},
		{
			name: "Should reject a different CIMD client selected during refresh discovery", legacy: true,
			replaceRegistration: true,
		},
		{
			name:   "Should reject an empty auth method on a registration with a client secret",
			legacy: true,
			change: func(registration *ClientRegistration) {
				registration.ClientSecretRef = "vault:mcp/test/client-secret"
			},
		},
		{
			name:   "Should reject an empty auth method on a DCR client with a URL client ID",
			legacy: true,
			change: func(registration *ClientRegistration) {
				registration.RegistrationClientURI = "https://issuer.example/registration/client"
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := mcpfixture.StartOAuthHTTP(t, mcpfixture.MustNew(mcpfixture.ProfileModern2026),
				mcpfixture.OAuthConfig{ClientIDMetadataDocumentSupported: true})
			const redirect = "http://127.0.0.1:2123/api/mcp/oauth/callback"
			document := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/.well-known/mcp-client.json" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if _, err := fmt.Fprintf(w, `{"client_id":%q,"redirect_uris":[%q],"token_endpoint_auth_method":"none"}`,
					"https://"+r.Host+r.URL.Path, redirect); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(document.Close)
			clientID := document.URL + "/.well-known/mcp-client.json"
			now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
			store := &adapterTestStore{}
			cfg := ServerConfig{
				Target: Target{Scope: ScopeUser, ServerName: "fixture"}, Type: authTypeOAuth,
				RemoteURL: fixture.Endpoints.MCPURL, Registration: RegistrationAuto, Scopes: []string{"tools.read"},
			}
			options := []ServiceOption{
				withHTTPClientForTest(document.Client()), WithRegistrationStore(store),
				WithNow(func() time.Time { return now }), WithRedirectURI(redirect),
			}
			codeExchanges := 0
			if tc.legacy {
				fingerprint, err := ServerDefinitionFingerprint(cfg)
				if err != nil {
					t.Fatal(err)
				}
				store.registration = ClientRegistration{
					Target: cfg.Target, DefinitionFingerprint: fingerprint, ResourceURL: cfg.RemoteURL,
					Issuer: fixture.Endpoints.IssuerURL, ClientID: clientID, RedirectURL: redirect, Scopes: cfg.Scopes,
				}
				store.token = TokenRecord{
					Target: cfg.Target, DefinitionFingerprint: fingerprint, Issuer: fixture.Endpoints.IssuerURL,
					ClientID: clientID, Scopes: cfg.Scopes, AccessToken: "expired-fixture-access",
					RefreshToken: mcpfixture.RefreshToken, ObtainedAt: now, ExpiresAt: now.Add(time.Hour),
				}
			} else {
				service, err := NewService(store, append(options, WithClientMetadataURL(clientID))...)
				if err != nil {
					t.Fatal(err)
				}
				state, err := service.BeginLogin(t.Context(), cfg, redirect)
				if err != nil {
					t.Fatal(err)
				}
				if state.Config.ClientID != clientID || len(fixture.Requests().Registration) != 0 {
					t.Fatal("sign-in did not select CIMD")
				}
				callback := oauthCallbackURL(t, redirect, state.State, "")
				if _, err := service.Exchange(t.Context(), &state, callback); err != nil {
					t.Fatal(err)
				}
				codeExchanges = 1
			}
			if tc.change != nil {
				tc.change(&store.registration)
			}
			if tc.redirectChanged {
				options = append(options, WithRedirectURI(redirect+"-changed"))
			}
			if tc.replaceRegistration {
				replacement := store.registration
				replacement.ClientID = "https://replacement.example/.well-known/mcp-client.json"
				var once sync.Once
				var saveErr error
				options = append(options, WithNetworkAuthorization(func(ctx context.Context, _ ServerConfig) error {
					once.Do(func() {
						_, saveErr = store.SaveMCPAuthRegistration(ctx, replacement, RegistrationSecrets{})
					})
					return saveErr
				}))
			}
			// The executor uses the registration that was saved at sign-in, including for a custom CIMD URL.
			renewal, err := NewService(store, options...)
			if err != nil {
				t.Fatal(err)
			}
			now = now.Add(time.Hour)
			before, err := renewal.Status(t.Context(), cfg)
			if err != nil || before.Status != StatusExpired || !before.TokenPresent ||
				before.Refreshable != (tc.wantUsable || tc.replaceRegistration) {
				t.Fatalf("expired status = %+v, err = %v, want refreshable = %t", before, err,
					tc.wantUsable || tc.replaceRegistration)
			}
			after, err := renewal.Refresh(t.Context(), cfg)
			requests := fixture.Requests()
			if tc.replaceRegistration {
				if !errors.Is(err, ErrTokenBindingInvalid) || len(requests.Token) != codeExchanges {
					t.Fatalf("refresh error = %v, want binding rejection before contacting the token endpoint", err)
				}
				return
			}
			if !tc.wantUsable {
				if err != nil || after.Status != StatusExpired || after.Refreshable || !after.TokenPresent ||
					!strings.Contains(after.Diagnostic, "stored registration is no longer valid") {
					t.Fatalf("refresh = %+v, %v; want an expired retained token requiring reauthorization", after, err)
				}
				if len(requests.Token) != codeExchanges {
					t.Fatal("invalid registration reached the refresh endpoint")
				}
				return
			}
			if err != nil || after.Status != StatusAuthenticated || !after.Refreshable || !after.TokenPresent {
				t.Fatalf("refreshed status = %+v, err = %v", after, err)
			}
			if len(requests.Registration) != 0 || len(requests.Token) != codeExchanges+1 {
				t.Fatal("refresh did not reuse the existing client")
			}
			request := requests.Token[codeExchanges]
			if request.Get("grant_type") != "refresh_token" || request.Get("client_id") != clientID ||
				request.Get("refresh_token") != mcpfixture.RefreshToken || request.Has("client_secret") ||
				request.Get("resource") != cfg.RemoteURL {
				t.Fatal("refresh did not use a resource-bound public client grant")
			}
			token, err := renewal.AuthorizationToken(t.Context(), cfg)
			if err != nil || !token.ExpiresAt.Equal(now.Add(time.Hour)) {
				t.Fatal("the refreshed token was not persisted and usable")
			}
			if tc.legacy {
				return
			}
			t.Run("Should use DCR when the newly configured CIMD document is unavailable", func(t *testing.T) {
				fallback, err := NewService(store, append(options, WithClientMetadataURL(document.URL+"/missing"))...)
				if err != nil {
					t.Fatal(err)
				}
				login, err := fallback.BeginLogin(t.Context(), cfg, redirect)
				if err != nil {
					t.Fatal(err)
				}
				if login.Config.ClientID == clientID || len(fixture.Requests().Registration) != 1 {
					t.Fatal("fallback reused a CIMD client instead of registering dynamically")
				}
			})
		})
	}
}

func TestDynamicRegistrationReuseValidatesBindings(t *testing.T) {
	t.Parallel()
	t.Run("Should reuse a public DCR client whose issued ID is an HTTPS URL", func(t *testing.T) {
		t.Parallel()
		fixture := mcpfixture.StartOAuthHTTP(
			t, mcpfixture.MustNew(mcpfixture.ProfileModern2026), mcpfixture.OAuthConfig{},
		)
		cfg := ServerConfig{
			Target: Target{Scope: ScopeUser, ServerName: "fixture"}, Type: authTypeOAuth,
			RemoteURL: fixture.Endpoints.MCPURL, Registration: RegistrationAuto,
		}
		fingerprint, err := ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatal(err)
		}
		const clientID = "https://issuer.example/clients/public"
		const redirect = "http://127.0.0.1:2123/api/mcp/oauth/callback"
		store := &adapterTestStore{registration: ClientRegistration{
			Target: cfg.Target, DefinitionFingerprint: fingerprint, ResourceURL: cfg.RemoteURL,
			Issuer: fixture.Endpoints.IssuerURL, RedirectURL: redirect, ClientID: clientID,
			TokenEndpointAuthMethod: tokenEndpointAuthMethodNone,
		}}
		service, err := NewService(store, withHTTPClientForTest(fixture.Server.Client()), WithRegistrationStore(store))
		if err != nil {
			t.Fatal(err)
		}
		login, err := service.BeginLogin(t.Context(), cfg, redirect)
		if err != nil {
			t.Fatal(err)
		}
		if login.Config.ClientID != clientID || len(fixture.Requests().Registration) != 0 {
			t.Fatal("sign-in replaced the existing public DCR client")
		}
	})
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
			false,
		); err != nil {
			t.Fatalf("validateAdvertisedMetadataURL() error = %v", err)
		}
	})

	t.Run("Should reject advertised metadata on another host", func(t *testing.T) {
		t.Parallel()
		if err := validateAdvertisedMetadataURL(
			"https://attacker.example.test/metadata",
			"https://mcp.example.test/connect",
			false,
		); err == nil {
			t.Fatal("validateAdvertisedMetadataURL() error = nil, want host rejection")
		}
	})

	t.Run("Should require the plugin Bearer metadata origin without changing legacy discovery", func(t *testing.T) {
		t.Parallel()
		const resource = "http://127.0.0.1:4001/mcp"
		const metadata = "https://127.0.0.1:4001/metadata"
		header := http.Header{}
		header.Set("WWW-Authenticate", fmt.Sprintf("Bearer resource_metadata=%q", metadata))
		if got := AdvertisedProtectedResourceMetadataURL(header, resource); got != "" {
			t.Fatalf("plugin discovery accepted an advertised URL from another origin: %s", got)
		}
		if got := advertisedProtectedResourceMetadataURL(header, resource, false); got != metadata {
			t.Fatalf("legacy discovery rejected the metadata URL on the same host: %s", got)
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

func TestHostedOAuthRootResourceDiscovery(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		resourceSuffix string
		otherOrigin    bool
		rootStatus     int
		wantLogin      bool
	}{
		{name: "Should authorize with a slash-terminated root resource", resourceSuffix: "/", wantLogin: true},
		{name: "Should authorize with an empty-path root resource", wantLogin: true},
		{name: "Should reject a root resource on another origin", otherOrigin: true},
		{name: "Should reject an unrelated resource path", resourceSuffix: "/other"},
		{name: "Should bound discovery when root metadata returns HTTP 429", rootStatus: http.StatusTooManyRequests},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := mcpfixture.StartOAuthHTTP(
				t, mcpfixture.MustNew(mcpfixture.ProfileModern2026), mcpfixture.OAuthConfig{},
			)
			var probes, endpointRequests, rootRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/mcp":
					probes.Add(1)
					w.WriteHeader(http.StatusUnauthorized)
				case "/.well-known/oauth-protected-resource/mcp":
					endpointRequests.Add(1)
					http.NotFound(w, r)
				case "/.well-known/oauth-protected-resource":
					rootRequests.Add(1)
					if tc.rootStatus != 0 {
						w.WriteHeader(tc.rootStatus)
						return
					}
					origin := "http://" + r.Host
					resource := origin + tc.resourceSuffix
					if tc.otherOrigin {
						resource = "https://other.example/"
					}
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(oauthex.ProtectedResourceMetadata{
						Resource: resource, AuthorizationServers: []string{origin},
					}); err != nil {
						t.Errorf("encode protected resource metadata: %v", err)
					}
				default:
					fixture.Server.Config.Handler.ServeHTTP(w, r)
				}
			}))
			t.Cleanup(server.Close)
			client := server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			store := &adapterTestStore{}
			service, err := NewService(
				store,
				withHTTPClientForTest(client),
				WithRegistrationStore(store),
				WithSecretRefResolver(func(context.Context, string) (string, error) {
					return store.secrets.ClientSecret, nil
				}),
			)
			if err != nil {
				t.Fatal(err)
			}
			manager, err := NewManager(service)
			if err != nil {
				t.Fatal(err)
			}
			cfg := ServerConfig{
				Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
				Type:         authTypeOAuth,
				RemoteURL:    server.URL + "/mcp",
				Registration: RegistrationAuto,
			}
			begin, err := manager.Begin(t.Context(), cfg, "http://127.0.0.1:2123/callback")
			if probe, endpoint, root := probes.Load(), endpointRequests.Load(), rootRequests.Load(); probe != 1 ||
				endpoint != 1 || root < 1 || root > 2 {
				t.Fatalf("discovery requests: probe=%d endpoint=%d root=%d", probe, endpoint, root)
			}
			if tc.wantLogin && tc.resourceSuffix == "" && rootRequests.Load() != 1 {
				t.Fatal("discovery fetched root metadata again after accepting the empty-path resource")
			}
			if !tc.wantLogin {
				const want = "mcp auth: protected resource metadata discovery failed"
				if err == nil || err.Error() != want {
					t.Fatalf("Begin() error = %v, want %q", err, want)
				}
				requests := fixture.Requests()
				if len(requests.Registration) != 0 || len(requests.Token) != 0 {
					t.Fatal("invalid metadata reached registration or token exchange")
				}
				return
			}
			if err != nil {
				t.Fatalf("Begin() error = %v", err)
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, begin.AuthorizationURL, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := drainAndCloseResponseBody(resp.Body); err != nil {
					t.Errorf("close authorization response: %v", err)
				}
			}()
			if resp.StatusCode != http.StatusFound {
				t.Fatalf("authorize HTTP status = %d, want %d", resp.StatusCode, http.StatusFound)
			}
			authorizations := fixture.Requests().Authorization
			if len(authorizations) != 1 || authorizations[0].Get("code_challenge_method") != "S256" {
				t.Fatal("authorization did not use S256 PKCE")
			}
			if _, err := manager.Exchange(
				t.Context(), cfg, ExchangeInput{RedirectURL: resp.Header.Get("Location")},
			); err != nil {
				t.Fatalf("Exchange() error = %v", err)
			}
			status, err := manager.Status(t.Context(), cfg)
			if err != nil || status.Status != StatusAuthenticated || !status.TokenPresent {
				t.Fatalf("Status() = %+v, error = %v, want authenticated with a token", status, err)
			}
		})
	}
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
	t.Run("Should probe an implicit remote when its old credential no longer matches", func(t *testing.T) {
		t.Parallel()
		store := &adapterTestStore{}
		service, err := NewService(store)
		if err != nil {
			t.Fatal(err)
		}
		cfg := ServerConfig{
			Implicit:     true,
			Target:       Target{Scope: ScopeUser, ServerName: "fixture"},
			Type:         authTypeOAuth,
			Transport:    "http",
			RemoteURL:    "https://old.example/mcp",
			Registration: RegistrationAuto,
		}
		fingerprint, err := ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatal(err)
		}
		store.token = TokenRecord{Target: cfg.Target, DefinitionFingerprint: fingerprint, AccessToken: "old-token"}
		cfg.RemoteURL = "https://new.example/mcp"
		status, err := service.Status(t.Context(), cfg)
		if err != nil || status.Status != StatusUnconfigured || status.TokenPresent {
			t.Fatalf("implicit status = %#v, %v", status, err)
		}
		if _, err := service.AuthorizationToken(t.Context(), cfg); !errors.Is(err, ErrTokenBindingInvalid) {
			t.Fatalf("old credential became sendable: %v", err)
		}
	})

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

// Invariant: rediscovery cannot send persisted credentials to a replacement issuer.
// Owner: OAuth token lifecycle; canonical suite: sdk_adapter_test.go.
func TestOAuthLifecycleRejectsReplacementIssuer(t *testing.T) {
	t.Parallel()
	for _, logout := range []bool{false, true} {
		name := "Should reject refresh after advertised issuer changes"
		if logout {
			name = "Should clean up logout without revoking at a replacement issuer"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var replacementRequests atomic.Int32
			replacement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" || r.URL.Path == "/revoke" {
					replacementRequests.Add(1)
				}
				writeOAuthMetadataResponse(t, w, r, true)
			}))
			t.Cleanup(replacement.Close)
			var switched atomic.Bool
			original := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration":
					writeOAuthMetadataResponse(t, w, r, true)
				case "/.well-known/oauth-protected-resource/mcp":
					issuer := "http://" + r.Host
					if switched.Load() {
						issuer = replacement.URL
					}
					w.Header().Set("Content-Type", "application/json")
					if _, err := fmt.Fprintf(
						w,
						`{"resource":%q,"authorization_servers":[%q]}`,
						"http://"+r.Host+"/mcp",
						issuer,
					); err != nil {
						t.Error(err)
					}
				case "/token":
					w.Header().Set("Content-Type", "application/json")
					if _, err := w.Write(
						[]byte(`{"access_token":"access-A","refresh_token":"refresh-A","token_type":"Bearer"}`),
					); err != nil {
						t.Error(err)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(original.Close)
			store := &adapterTestStore{}
			service, cfg := newPreRegisteredTestService(t, store, original)
			cfg.IssuerURL = ""
			state, err := service.BeginLogin(t.Context(), cfg, "http://127.0.0.1:2123/callback")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Exchange(
				t.Context(),
				&state,
				oauthCallbackURL(t, state.RedirectURL, state.State, ""),
			); err != nil {
				t.Fatal(err)
			}
			switched.Store(true)
			if logout {
				status, err := service.Logout(t.Context(), cfg)
				if err != nil || !store.authorizationStateDeleted || store.token.AccessToken != "" ||
					status.Status != StatusNeedsLogin {
					t.Fatalf("logout = %+v, %v; deleted=%v", status, err, store.authorizationStateDeleted)
				}
			} else {
				if _, err := service.Refresh(t.Context(), cfg); !errors.Is(err, ErrTokenBindingInvalid) {
					t.Fatalf("refresh = %v", err)
				}
				if store.token.AccessToken != "access-A" {
					t.Fatal("refresh changed persisted token")
				}
			}
			if replacementRequests.Load() != 0 {
				t.Fatal("replacement issuer received credentials")
			}
		})
	}
}

func TestLogoutCleansUpAfterRevocationFailure(t *testing.T) {
	t.Parallel()
	for _, metadataOutage := range []bool{false, true} {
		name := "Should delete local authorization state when remote revocation fails"
		if metadataOutage {
			name = "Should delete local authorization state when omitted issuer discovery fails"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var revocations atomic.Int32
			var metadataDown atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if metadataDown.Load() {
					http.Error(writer, "metadata unavailable", http.StatusServiceUnavailable)
					return
				}
				switch request.URL.Path {
				case "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration":
					writeOAuthMetadataResponse(t, writer, request, true)
				case "/.well-known/oauth-protected-resource/mcp":
					writer.Header().Set("Content-Type", "application/json")
					if _, err := fmt.Fprintf(
						writer,
						`{"resource":%q,"authorization_servers":[%q]}`,
						"http://"+request.Host+"/mcp",
						"http://"+request.Host,
					); err != nil {
						t.Error(err)
					}
				case "/token":
					writer.Header().Set("Content-Type", "application/json")
					if _, err := writer.Write(
						[]byte(`{"access_token":"access","refresh_token":"refresh","token_type":"Bearer"}`),
					); err != nil {
						t.Error(err)
					}
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
			if metadataOutage {
				cfg.IssuerURL = ""
			}
			state, err := service.BeginLogin(t.Context(), cfg, "http://127.0.0.1:2123/callback")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Exchange(
				t.Context(),
				&state,
				oauthCallbackURL(t, state.RedirectURL, state.State, ""),
			); err != nil {
				t.Fatal(err)
			}
			metadataDown.Store(metadataOutage)
			status, err := service.Logout(t.Context(), cfg)
			if err != nil {
				t.Fatalf("Logout() error = %v", err)
			}
			if !store.authorizationStateDeleted || store.token.AccessToken != "" ||
				store.registration.ClientID != "" || store.secrets.ClientSecret != "" || status.Status != StatusNeedsLogin ||
				!strings.Contains(status.Diagnostic, "remote revocation failed") {
				t.Fatalf("Logout() status = %#v, deleted = %t", status, store.authorizationStateDeleted)
			}
			wantRevocations := int32(1)
			if metadataOutage {
				wantRevocations = 0
			}
			if got := revocations.Load(); got != wantRevocations {
				t.Fatalf("revocation calls = %d, want %d", got, wantRevocations)
			}
		})
	}
}

func TestOAuthRefreshCannotReinsertTokenAfterLogout(t *testing.T) {
	t.Parallel()
	t.Run("Should reject a refresh commit after another service logs out", func(t *testing.T) {
		t.Parallel()
		refreshStarted := make(chan struct{})
		releaseRefresh := make(chan struct{})
		release := sync.OnceFunc(func() { close(releaseRefresh) })
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
	for _, test := range []struct {
		code       string
		status     int
		definitive bool
	}{
		{code: "invalid_grant", status: 400, definitive: true},
		{code: "revoked", status: 400, definitive: true},
		{code: "invalid_client", status: 401, definitive: true},
		{code: "unauthorized_client", status: 400, definitive: true},
		{code: "invalid_request", status: 400},
		{code: "invalid_scope", status: 400},
		{code: "unsupported_grant_type", status: 400},
		{status: 400},
		{status: 401},
		{code: "temporarily_unavailable", status: 400},
		{code: "sensitive-refresh-token", status: 400},
		{status: 403},
		{status: 408},
		{status: 409},
		{code: "invalid_grant", status: 429},
		{code: "invalid_grant", status: 503},
	} {
		name := fmt.Sprintf(
			"Should classify HTTP %d refresh rejection and redact diagnostics for %s",
			test.status,
			test.code,
		)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			var recovered atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/token" {
					writeOAuthMetadataResponse(t, w, r, false)
					return
				}
				requests.Add(1)
				if recovered.Load() {
					writeManagerTestResponse(
						t,
						w,
						`{"access_token":"recovered-access","expires_in":3600,"token_type":"Bearer"}`,
					)
					return
				}
				w.WriteHeader(test.status)
				writeManagerTestResponse(
					t,
					w,
					fmt.Sprintf(
						`{"error":%q,"error_description":"sensitive-access-token sensitive-refresh-token"}`,
						test.code,
					),
				)
			}))
			t.Cleanup(server.Close)
			now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
			var logs bytes.Buffer
			store := &adapterTestStore{}
			service, cfg := newPreRegisteredTestService(t, store, server,
				WithNow(func() time.Time { return now }), WithLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
			token := managerBoundToken(t, cfg, "sensitive-access-token")
			token.RefreshToken, token.ExpiresAt = "sensitive-refresh-token", now
			if err := store.SaveMCPAuthToken(t.Context(), token); err != nil {
				t.Fatal(err)
			}
			before, err := service.Status(t.Context(), cfg)
			if err != nil || before.Status != StatusExpired || !before.Refreshable || !before.TokenPresent {
				t.Fatalf("expired refreshable status = %+v, %v", before, err)
			}
			status, refreshErr := service.Refresh(t.Context(), cfg)
			if test.definitive {
				if refreshErr != nil || status.Status != StatusNeedsLogin || status.Refreshable || status.TokenPresent {
					t.Fatalf("definitive rejection status = %+v, %v", status, refreshErr)
				}
				if _, err := store.GetMCPAuthToken(t.Context(), cfg.Target); !errors.Is(err, ErrTokenNotFound) {
					t.Fatalf("rejected token remains: %v", err)
				}
			} else {
				assertRejection := func(err error) {
					t.Helper()
					endpoint, ok := errors.AsType[*tokenEndpointError](err)
					cause := test.code
					switch cause {
					case "":
						cause = "invalid_response"
					case "sensitive-refresh-token":
						cause = "token_endpoint"
					}
					if !ok || endpoint.status != test.status || endpoint.cause != cause {
						t.Fatalf("transient rejection = %v, want HTTP %d (%s)", err, test.status, cause)
					}
				}
				assertRejection(refreshErr)
				assertAdapterTokenUnchanged(t, store, token)
				if _, err := service.Refresh(t.Context(), cfg); !errors.Is(err, ErrRefreshBackoff) {
					t.Fatalf("retry before backoff = %v", err)
				}
				if requests.Load() != 1 {
					t.Fatal("retry contacted token endpoint during backoff")
				}
				now = now.Add(2 * time.Second)
				_, retryErr := service.Refresh(t.Context(), cfg)
				assertRejection(retryErr)
				if !strings.Contains(logs.String(), `"attempt":2`) {
					t.Fatal("retry attempt count missing")
				}
				recovered.Store(true)
				now = now.Add(time.Minute)
				status, err = service.Refresh(t.Context(), cfg)
				if err != nil || status.Status != StatusAuthenticated {
					t.Fatalf("recovery = %+v, %v", status, err)
				}
			}
			var logged map[string]any
			if err := json.NewDecoder(strings.NewReader(logs.String())).Decode(&logged); err != nil {
				t.Fatal(err)
			}
			causeClass, ok := logged["cause_class"].(string)
			if logged["server"] != cfg.Target.ServerName || logged["attempt"] != float64(1) ||
				!ok || causeClass == "" {
				t.Fatalf("missing diagnostic fields: %+v", logged)
			}
			if strings.Contains(logs.String(), token.AccessToken) ||
				strings.Contains(logs.String(), token.RefreshToken) {
				t.Fatal("refresh logs leaked token material")
			}
		})
	}
	t.Run("Should bound renewal with its configured timeout and back off", func(t *testing.T) {
		t.Parallel()
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/token" {
				writeOAuthMetadataResponse(t, w, r, false)
				return
			}
			<-release
		}))
		t.Cleanup(func() { close(release); server.Close() })
		store := &adapterTestStore{}
		service, cfg := newPreRegisteredTestService(t, store, server, WithRefreshTimeout(50*time.Millisecond))
		token := managerBoundToken(t, cfg, "expired")
		token.RefreshToken = "refresh"
		if err := store.SaveMCPAuthToken(t.Context(), token); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Refresh(t.Context(), cfg); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("timed renewal = %v, want deadline exceeded", err)
		}
		assertAdapterTokenUnchanged(t, store, token)
		if _, err := service.Refresh(t.Context(), cfg); !errors.Is(err, ErrRefreshBackoff) {
			t.Fatalf("retry after timeout = %v, want backoff", err)
		}
	})
	t.Run("Should share one refresh across concurrent services and cancel a waiter", func(t *testing.T) {
		t.Parallel()
		started, release := make(chan struct{}), make(chan struct{})
		unblock := sync.OnceFunc(func() { close(release) })
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/token" {
				writeOAuthMetadataResponse(t, w, r, false)
				return
			}
			if requests.Add(1) == 1 {
				close(started)
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			writeManagerTestResponse(t, w, `{"access_token":"renewed","expires_in":3600,"token_type":"Bearer"}`)
		}))
		t.Cleanup(func() { unblock(); server.Close() })
		store := &observedRefreshStore{TokenStore: &adapterTestStore{}, reads: make(chan struct{}, 32)}
		generation := NewMutationGeneration()
		first, cfg := newPreRegisteredTestService(t, store, server, WithMutationGeneration(generation))
		second, _ := newPreRegisteredTestService(t, store, server, WithMutationGeneration(generation))
		token := managerBoundToken(t, cfg, "expired")
		token.RefreshToken = "refresh"
		if err := store.SaveMCPAuthToken(t.Context(), token); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		owner, cancelOwner := context.WithCancel(ctx)
		defer cancelOwner()
		results := make(chan error, 8)
		go func() { _, err := first.Refresh(owner, cfg); results <- err }()
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		for range 7 {
			go func() { _, err := second.Refresh(ctx, cfg); results <- err }()
		}
		for range 9 {
			select {
			case <-store.reads:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
		canceled, stop := context.WithCancel(ctx)
		stop()
		if _, err := second.Refresh(canceled, cfg); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled waiter = %v", err)
		}
		cancelOwner()
		unblock()
		for range 8 {
			select {
			case err := <-results:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
		if _, err := second.Refresh(ctx, cfg); err != nil {
			t.Fatalf("refresh after concurrent renewal = %v", err)
		}
		if requests.Load() != 1 {
			t.Fatalf("refresh requests = %d, want 1", requests.Load())
		}
	})
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

type observedRefreshStore struct {
	TokenStore
	reads chan struct{}
}

func (s *observedRefreshStore) GetMCPAuthToken(ctx context.Context, target Target) (TokenRecord, error) {
	token, err := s.TokenStore.GetMCPAuthToken(ctx, target)
	select {
	case s.reads <- struct{}{}:
	default:
	}
	return token, err
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

// Invariant: a retained OAuth client rechecks consent for each network request,
// including redirects, and cannot reuse admission after revoke. Owner: auth HTTP boundary.
func TestOAuthNetworkConsentRevalidation(t *testing.T) {
	t.Parallel()
	t.Run("Should block redirect and cached-client traffic after revocation", func(t *testing.T) {
		t.Parallel()
		var admitted atomic.Bool
		admitted.Store(true)
		var requests atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			admitted.Store(false)
			http.Redirect(w, r, "/continued", http.StatusFound)
		}))
		defer server.Close()
		denied := errors.New("waiting for approval")
		service, err := NewService(
			&adapterTestStore{},
			withHTTPClientForTest(server.Client()),
			WithNetworkAuthorization(func(context.Context, ServerConfig) error {
				if !admitted.Load() {
					return denied
				}
				return nil
			}),
		)
		if err != nil {
			t.Fatal(err)
		}
		client := service.sdkHTTPClientFor(ServerConfig{})
		for attempt := range 2 {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Do(req)
			if response != nil && response.Body != nil {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Error(closeErr)
				}
			}
			if !errors.Is(err, denied) || requests.Load() != 1 {
				t.Fatalf("revoked attempt=%d requests=%d err=%v", attempt, requests.Load(), err)
			}
		}
	})
}
