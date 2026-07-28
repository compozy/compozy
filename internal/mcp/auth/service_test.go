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
	"testing"
	"time"
)

func TestOAuthPKCELifecycleWithRefreshAndLogout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	store := newMemoryTokenStore()
	var (
		mu            sync.Mutex
		refreshCalled bool
		revokedToken  string
	)
	handlerErrors := newHandlerErrorRecorder()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case metadataWellKnownPath:
			if err := writeJSON(w, Metadata{
				Issuer:                        "https://issuer.example",
				AuthorizationEndpoint:         "http://" + r.Host + "/authorize",
				TokenEndpoint:                 "http://" + r.Host + "/token",
				RevocationEndpoint:            "http://" + r.Host + "/revoke",
				CodeChallengeMethodsSupported: []string{"S256"},
			}); err != nil {
				handlerErrors.record(err)
			}
		case "/token":
			if err := r.ParseForm(); err != nil {
				handlerErrors.record(fmt.Errorf("ParseForm() error = %w", err))
				http.Error(w, "parse form", http.StatusBadRequest)
				return
			}
			if r.Form.Get("code_verifier") == "auth-code" || strings.Contains(r.Form.Encode(), "access-token") {
				handlerErrors.record(
					fmt.Errorf("token request leaked sensitive values in unexpected field: %s", r.Form.Encode()),
				)
				http.Error(w, "token leak", http.StatusBadRequest)
				return
			}
			switch r.Form.Get("grant_type") {
			case "authorization_code":
				if r.Form.Get("code") != "auth-code" {
					handlerErrors.record(fmt.Errorf("authorization code = %q, want auth-code", r.Form.Get("code")))
					http.Error(w, "bad code", http.StatusBadRequest)
					return
				}
				if r.Form.Get("code_verifier") == "" {
					handlerErrors.record(errors.New("code_verifier = empty"))
					http.Error(w, "missing verifier", http.StatusBadRequest)
					return
				}
				if err := writeJSON(w, map[string]any{
					"access_token":  "access-token-1",
					"refresh_token": "refresh-token-1",
					"token_type":    "Bearer",
					"expires_in":    3600,
					"scope":         "read write",
				}); err != nil {
					handlerErrors.record(err)
				}
			case "refresh_token":
				if r.Form.Get("refresh_token") != "refresh-token-1" {
					handlerErrors.record(
						fmt.Errorf("refresh_token = %q, want persisted token", r.Form.Get("refresh_token")),
					)
					http.Error(w, "bad refresh token", http.StatusBadRequest)
					return
				}
				mu.Lock()
				refreshCalled = true
				mu.Unlock()
				if err := writeJSON(w, map[string]any{
					"access_token": "access-token-2",
					"token_type":   "Bearer",
					"expires_in":   7200,
				}); err != nil {
					handlerErrors.record(err)
				}
			default:
				handlerErrors.record(fmt.Errorf("grant_type = %q", r.Form.Get("grant_type")))
				http.Error(w, "bad grant type", http.StatusBadRequest)
			}
		case "/revoke":
			if err := r.ParseForm(); err != nil {
				handlerErrors.record(fmt.Errorf("ParseForm(revoke) error = %w", err))
				http.Error(w, "parse revoke form", http.StatusBadRequest)
				return
			}
			mu.Lock()
			revokedToken = r.Form.Get("token")
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service, err := NewService(store, WithHTTPClient(server.Client()), WithNow(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	cfg := ServerConfig{
		Target:        globalTestTarget("linear"),
		RemoteURL:     "https://mcp.example/sse",
		Type:          "oauth2_pkce",
		IssuerURL:     server.URL,
		ClientID:      "client-1",
		Scopes:        []string{"read", "write"},
		RevocationURL: server.URL + "/revoke",
	}

	login, err := service.BeginLogin(ctx, cfg, "http://127.0.0.1/callback")
	if err != nil {
		t.Fatalf("BeginLogin() error = %v", err)
	}
	authURL, err := url.Parse(login.AuthorizationURL)
	if err != nil {
		t.Fatalf("Parse authorization URL error = %v", err)
	}
	if authURL.Query().Get("code_challenge") == "" || authURL.Query().Get("code_verifier") != "" {
		t.Fatalf("authorization URL query = %s, want challenge without verifier", authURL.RawQuery)
	}
	if _, err := service.Exchange(ctx, login, "http://127.0.0.1/callback?code=auth-code&state=wrong"); err == nil {
		t.Fatal("Exchange(wrong state) error = nil, want mismatch")
	}

	status, err := service.Exchange(ctx, login, "http://127.0.0.1/callback?code=auth-code&state="+login.State)
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if status.Status != StatusAuthenticated || !status.Refreshable || !status.TokenPresent {
		t.Fatalf("Exchange() status = %#v", status)
	}
	statusJSON, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal(status) error = %v", err)
	}
	if strings.Contains(string(statusJSON), "access-token") || strings.Contains(string(statusJSON), "refresh-token") {
		t.Fatalf("redacted status leaked token material: %s", statusJSON)
	}

	refreshed, err := service.Refresh(ctx, cfg)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.Status != StatusAuthenticated {
		t.Fatalf("Refresh() status = %#v", refreshed)
	}
	mu.Lock()
	if !refreshCalled {
		t.Fatal("refresh endpoint was not called")
	}
	mu.Unlock()
	token, err := store.GetMCPAuthToken(ctx, globalTestTarget("linear"))
	if err != nil {
		t.Fatalf("GetMCPAuthToken() error = %v", err)
	}
	if token.AccessToken != "access-token-2" || token.RefreshToken != "refresh-token-1" {
		t.Fatalf("stored token after refresh = %#v", token)
	}

	loggedOut, err := service.Logout(ctx, cfg)
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if loggedOut.Status != StatusNeedsLogin {
		t.Fatalf("Logout() status = %#v", loggedOut)
	}
	mu.Lock()
	if revokedToken != "refresh-token-1" {
		t.Fatalf("revoked token = %q, want refresh token", revokedToken)
	}
	mu.Unlock()
	if _, err := store.GetMCPAuthToken(ctx, globalTestTarget("linear")); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("GetMCPAuthToken(after logout) error = %v, want ErrTokenNotFound", err)
	}
	handlerErrors.assertEmpty(t)
}

func TestOAuthRefreshCannotReinsertTokenAfterLogoutAcrossServices(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an older refresh commit after another service logs out", func(t *testing.T) {
		t.Parallel()

		refreshStarted := make(chan struct{})
		releaseRefresh := make(chan struct{})
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(releaseRefresh) }) }
		defer release()
		handlerErrors := newHandlerErrorRecorder()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/token" {
				http.NotFound(w, r)
				return
			}
			if err := r.ParseForm(); err != nil {
				handlerErrors.record(fmt.Errorf("parse refresh form: %w", err))
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			close(refreshStarted)
			<-releaseRefresh
			if err := writeJSON(w, map[string]any{
				"access_token": "refreshed-access",
				"token_type":   "Bearer",
			}); err != nil {
				handlerErrors.record(err)
			}
		}))
		defer func() {
			server.Close()
			handlerErrors.assertEmpty(t)
		}()

		target := globalTestTarget("linear")
		cfg := ServerConfig{
			Target:           target,
			Type:             "oauth2_pkce",
			AuthorizationURL: server.URL + "/authorize",
			TokenURL:         server.URL + "/token",
			ClientID:         "client-id",
		}
		fingerprint, err := ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
		}
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), TokenRecord{
			Target: target, DefinitionFingerprint: fingerprint,
			AccessToken: "existing-access", RefreshToken: "existing-refresh", TokenType: "Bearer",
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken() error = %v", err)
		}
		generation := NewMutationGeneration()
		refreshService, err := NewService(
			store,
			WithHTTPClient(server.Client()),
			WithMutationGeneration(generation),
		)
		if err != nil {
			t.Fatalf("NewService(refresh) error = %v", err)
		}
		logoutService, err := NewService(store, WithMutationGeneration(generation))
		if err != nil {
			t.Fatalf("NewService(logout) error = %v", err)
		}

		refreshErr := make(chan error, 1)
		go func() {
			_, refreshError := refreshService.Refresh(t.Context(), cfg)
			refreshErr <- refreshError
		}()
		<-refreshStarted
		if _, err := logoutService.Logout(t.Context(), cfg); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		release()
		if err := <-refreshErr; !errors.Is(err, ErrTokenMutationStale) {
			t.Fatalf("Refresh() error = %v, want ErrTokenMutationStale", err)
		}
		if _, err := store.GetMCPAuthToken(t.Context(), target); !errors.Is(err, ErrTokenNotFound) {
			t.Fatalf("GetMCPAuthToken(after logout) error = %v, want ErrTokenNotFound", err)
		}
	})

	t.Run("Should reject a refresh commit after a newer Manager exchange", func(t *testing.T) {
		t.Parallel()

		refreshStarted := make(chan struct{})
		releaseRefresh := make(chan struct{})
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(releaseRefresh) }) }
		defer release()
		handlerErrors := newHandlerErrorRecorder()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/token" {
				http.NotFound(w, r)
				return
			}
			if err := r.ParseForm(); err != nil {
				handlerErrors.record(fmt.Errorf("parse token form: %w", err))
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			accessToken := "new-login-access"
			if r.Form.Get("grant_type") == "refresh_token" {
				close(refreshStarted)
				<-releaseRefresh
				accessToken = "stale-refresh-access"
			}
			if err := writeJSON(w, map[string]any{
				"access_token":  accessToken,
				"refresh_token": "new-refresh",
				"token_type":    "Bearer",
			}); err != nil {
				handlerErrors.record(err)
			}
		}))
		defer func() {
			server.Close()
			handlerErrors.assertEmpty(t)
		}()

		target := globalTestTarget("linear")
		cfg := ServerConfig{
			Target: target, Type: "oauth2_pkce",
			AuthorizationURL: server.URL + "/authorize", TokenURL: server.URL + "/token",
			ClientID: "client-id",
		}
		fingerprint, err := ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
		}
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), TokenRecord{
			Target: target, DefinitionFingerprint: fingerprint,
			AccessToken: "existing-access", RefreshToken: "existing-refresh", TokenType: "Bearer",
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken() error = %v", err)
		}
		generation := NewMutationGeneration()
		service, err := NewService(
			store,
			WithHTTPClient(server.Client()),
			WithMutationGeneration(generation),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		manager, err := NewManager(service)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		if _, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL); err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		refreshErr := make(chan error, 1)
		go func() {
			_, refreshError := service.Refresh(t.Context(), cfg)
			refreshErr <- refreshError
		}()
		<-refreshStarted
		if _, err := manager.Exchange(t.Context(), cfg, ExchangeInput{Code: "new-login-code"}); err != nil {
			t.Fatalf("Exchange() error = %v", err)
		}
		release()
		if err := <-refreshErr; !errors.Is(err, ErrTokenMutationStale) {
			t.Fatalf("Refresh() error = %v, want ErrTokenMutationStale", err)
		}
		stored, err := store.GetMCPAuthToken(t.Context(), target)
		if err != nil {
			t.Fatalf("GetMCPAuthToken() error = %v", err)
		}
		if got, want := stored.AccessToken, "new-login-access"; got != want {
			t.Fatalf("durable access token = %q, want %q", got, want)
		}
	})
}

func TestTokenResponseRejectsMalformedPayload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemoryTokenStore()
	handlerErrors := newHandlerErrorRecorder()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if err := writeJSON(w, map[string]any{"refresh_token": "refresh"}); err != nil {
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
	defer server.Close()

	service, err := NewService(store, WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	cfg := ServerConfig{
		Target:      globalTestTarget("bad"),
		Type:        "oauth2_pkce",
		MetadataURL: server.URL,
		ClientID:    "client",
	}
	login, err := service.BeginLogin(ctx, cfg, "http://127.0.0.1/callback")
	if err != nil {
		t.Fatalf("BeginLogin() error = %v", err)
	}
	_, err = service.Exchange(ctx, login, "http://127.0.0.1/callback?code=ok&state="+login.State)
	if err == nil || !strings.Contains(err.Error(), "access_token is required") {
		t.Fatalf("Exchange() error = %v, want malformed token response", err)
	}
	handlerErrors.assertEmpty(t)
}

func TestLogoutDeletesLocalTokenWhenRemoteRevocationFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemoryTokenStore()
	handlerErrors := newHandlerErrorRecorder()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case metadataWellKnownPath:
			if err := writeJSON(w, Metadata{
				AuthorizationEndpoint:         "http://" + r.Host + "/authorize",
				TokenEndpoint:                 "http://" + r.Host + "/token",
				RevocationEndpoint:            "http://" + r.Host + "/revoke",
				CodeChallengeMethodsSupported: []string{"S256"},
			}); err != nil {
				handlerErrors.record(err)
			}
		case "/revoke":
			http.Error(w, "revocation unavailable", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service, err := NewService(store, WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	cfg := ServerConfig{
		Target:    globalTestTarget("linear"),
		RemoteURL: "https://mcp.example/sse",
		Type:      "oauth2_pkce",
		IssuerURL: server.URL,
		ClientID:  "client-1",
	}
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
	}
	if err := store.SaveMCPAuthToken(ctx, TokenRecord{
		Target: cfg.Target, DefinitionFingerprint: fingerprint,
		AccessToken: "access-token", RefreshToken: "refresh-token", TokenType: "Bearer",
		ObtainedAt: time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("SaveMCPAuthToken() error = %v", err)
	}

	status, err := service.Logout(ctx, cfg)
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if status.Status != StatusNeedsLogin || !strings.Contains(status.Diagnostic, "remote revocation failed") {
		t.Fatalf("Logout() status = %#v, want local logout diagnostic", status)
	}
	if _, err := store.GetMCPAuthToken(ctx, globalTestTarget("linear")); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("GetMCPAuthToken(after failed revocation logout) error = %v, want ErrTokenNotFound", err)
	}
	handlerErrors.assertEmpty(t)
}

func TestOAuthFailuresPreserveExistingTokenAndRedactSecretInputs(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve existing tokens and redact failed OAuth inputs", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		target := globalTestTarget("linear")
		issuedAt := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
		existing := TokenRecord{
			Target:       target,
			ClientID:     "client-id",
			AccessToken:  "existing-sensitive-access-token",
			RefreshToken: "existing-sensitive-refresh-token",
			TokenType:    "Bearer",
			ObtainedAt:   issuedAt,
			UpdatedAt:    issuedAt,
		}
		store := newMemoryTokenStore()
		handlerErrors := newHandlerErrorRecorder()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/metadata":
				if err := writeJSON(w, Metadata{
					AuthorizationEndpoint:         "http://" + r.Host + "/authorize",
					TokenEndpoint:                 "http://" + r.Host + "/token",
					CodeChallengeMethodsSupported: []string{pkceS256Value},
				}); err != nil {
					handlerErrors.record(err)
				}
			case "/token":
				if err := r.ParseForm(); err != nil {
					handlerErrors.record(fmt.Errorf("parse failure token form: %w", err))
					http.Error(w, "invalid form", http.StatusBadRequest)
					return
				}
				echoedSecret := r.Form.Get("code")
				if echoedSecret == "" {
					echoedSecret = r.Form.Get(serviceRefreshTokenKey)
				}
				w.WriteHeader(http.StatusBadRequest)
				if err := writeJSON(w, map[string]string{"error": echoedSecret}); err != nil {
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
		service, err := NewService(
			store,
			WithHTTPClient(server.Client()),
			WithNow(func() time.Time { return issuedAt }),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		cfg := ServerConfig{
			Target:      target,
			Type:        "oauth2_pkce",
			MetadataURL: server.URL + "/metadata",
			ClientID:    "client-id",
		}
		fingerprint, err := ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
		}
		existing.DefinitionFingerprint = fingerprint
		if err := store.SaveMCPAuthToken(ctx, existing); err != nil {
			t.Fatalf("SaveMCPAuthToken(existing) error = %v", err)
		}
		login, err := service.BeginLogin(ctx, cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("BeginLogin() error = %v", err)
		}
		sensitiveCode := "sensitive-authorization-code"
		callbackURL := callbackWithCodeAndState(t, sensitiveCode, login.State)
		if _, err := service.Exchange(ctx, login, callbackURL); err == nil {
			t.Fatal("Exchange(failed token request) error = nil")
		} else if strings.Contains(err.Error(), sensitiveCode) || strings.Contains(err.Error(), callbackURL) {
			t.Fatalf("Exchange() error leaked inbound secret material: %v", err)
		}
		assertStoredMCPTokenUnchanged(t, store, target, existing)

		if _, err := service.Refresh(ctx, cfg); err == nil {
			t.Fatal("Refresh(failed token request) error = nil")
		} else if strings.Contains(err.Error(), existing.RefreshToken) || strings.Contains(err.Error(), existing.AccessToken) {
			t.Fatalf("Refresh() error leaked token material: %v", err)
		}
		assertStoredMCPTokenUnchanged(t, store, target, existing)
	})
}

func TestOAuthTokenDefinitionBinding(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	target := globalTestTarget("linear")
	original := ServerConfig{
		Target: target, Transport: "http", RemoteURL: "https://original.example/mcp",
		Type: "oauth2_pkce", AuthorizationURL: "https://issuer.example/authorize",
		TokenURL: "https://issuer.example/token", ClientID: "client-id", Scopes: []string{"read"},
	}
	replacement := original
	replacement.RemoteURL = "https://replacement.example/mcp"
	fingerprint, err := ServerDefinitionFingerprint(original)
	if err != nil {
		t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
	}
	store := newMemoryTokenStore()
	if err := store.SaveMCPAuthToken(ctx, TokenRecord{
		Target: target, DefinitionFingerprint: fingerprint, ClientID: "client-id",
		AccessToken: "original-access", RefreshToken: "original-refresh", TokenType: "Bearer",
		UpdatedAt: time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("SaveMCPAuthToken() error = %v", err)
	}
	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	for operation, load := range map[string]func() (Status, error){
		"status":  func() (Status, error) { return service.Status(ctx, replacement) },
		"refresh": func() (Status, error) { return service.Refresh(ctx, replacement) },
	} {
		status, loadErr := load()
		if loadErr != nil {
			t.Fatalf("%s replacement definition error = %v", operation, loadErr)
		}
		if status.Status != StatusNeedsLogin || status.TokenPresent ||
			!strings.Contains(status.Diagnostic, "different server definition") {
			t.Fatalf("%s replacement definition status = %#v", operation, status)
		}
	}
	preserved, err := store.GetMCPAuthToken(ctx, target)
	if err != nil {
		t.Fatalf("GetMCPAuthToken() error = %v", err)
	}
	if preserved.AccessToken != "original-access" || preserved.DefinitionFingerprint != fingerprint {
		t.Fatalf("preserved token = %#v", preserved)
	}

	loggedOut, err := service.Logout(ctx, replacement)
	if err != nil {
		t.Fatalf("Logout(replacement definition) error = %v", err)
	}
	if loggedOut.Status != StatusNeedsLogin ||
		!strings.Contains(loggedOut.Diagnostic, "different server definition") {
		t.Fatalf("Logout(replacement definition) status = %#v", loggedOut)
	}
	if _, err := store.GetMCPAuthToken(ctx, target); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("GetMCPAuthToken(after logout) error = %v, want ErrTokenNotFound", err)
	}
}

func TestOAuthStatusTargetNormalization(t *testing.T) {
	t.Parallel()

	t.Run("Should validate and normalize unconfigured and configured status targets", func(t *testing.T) {
		t.Parallel()

		service, err := NewService(newMemoryTokenStore())
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		unconfigured, err := service.Status(t.Context(), ServerConfig{
			Target: Target{Scope: " global ", ServerName: " linear "},
		})
		if err != nil {
			t.Fatalf("Status(unconfigured padded target) error = %v", err)
		}
		if unconfigured.Scope != ScopeGlobal || unconfigured.ServerName != "linear" || unconfigured.WorkspaceID != "" {
			t.Fatalf("Status(unconfigured padded target) = %#v, want normalized global identity", unconfigured)
		}
		if _, err := service.Status(t.Context(), ServerConfig{
			Target: Target{Scope: ScopeWorkspace, ServerName: "linear"},
		}); err == nil || !strings.Contains(err.Error(), "workspace target requires workspace_id") {
			t.Fatalf("Status(invalid unconfigured workspace target) error = %v", err)
		}

		configured, err := service.Status(t.Context(), ServerConfig{
			Target:           Target{Scope: " global ", ServerName: " linear "},
			Type:             "oauth2_pkce",
			AuthorizationURL: "https://auth.example/authorize",
			TokenURL:         "https://auth.example/token",
			ClientID:         "client-id",
		})
		if err != nil {
			t.Fatalf("Status(configured padded target) error = %v", err)
		}
		if configured.Scope != ScopeGlobal || configured.ServerName != "linear" || configured.WorkspaceID != "" {
			t.Fatalf("Status(configured padded target) = %#v, want normalized global identity", configured)
		}
	})
}

func assertStoredMCPTokenUnchanged(
	t *testing.T,
	store TokenStore,
	target Target,
	want TokenRecord,
) {
	t.Helper()

	got, err := store.GetMCPAuthToken(t.Context(), target)
	if err != nil {
		t.Fatalf("GetMCPAuthToken() error = %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken ||
		!got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("stored token changed after failed OAuth operation: %#v", got)
	}
}

func TestSupportsS256RequiresAdvertisedMethod(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		methods []string
		want    bool
	}{
		{name: "Should reject missing metadata methods", methods: nil, want: false},
		{name: "Should reject non S256 methods", methods: []string{"plain"}, want: false},
		{name: "Should accept advertised S256 method", methods: []string{" plain ", " s256 "}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := supportsS256(tc.methods); got != tc.want {
				t.Fatalf("supportsS256(%#v) = %v, want %v", tc.methods, got, tc.want)
			}
		})
	}
}

func TestNewServiceDefaultsHTTPClientTimeout(t *testing.T) {
	t.Parallel()

	t.Run("Should configure bounded HTTP client by default", func(t *testing.T) {
		t.Parallel()

		service, err := NewService(newMemoryTokenStore())
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		if service.client == nil || service.client.Timeout != defaultMetadataClientTimeout {
			t.Fatalf("NewService().client = %#v, want timeout %s", service.client, defaultMetadataClientTimeout)
		}
	})
}

type memoryTokenStore struct {
	mu     sync.Mutex
	tokens map[string]TokenRecord
}

func newMemoryTokenStore() *memoryTokenStore {
	return &memoryTokenStore{tokens: map[string]TokenRecord{}}
}

func (s *memoryTokenStore) SaveMCPAuthToken(_ context.Context, token TokenRecord) error {
	key, err := token.Target.Key()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[key] = token
	return nil
}

func (s *memoryTokenStore) GetMCPAuthToken(_ context.Context, target Target) (TokenRecord, error) {
	key, err := target.Key()
	if err != nil {
		return TokenRecord{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.tokens[key]
	if !ok {
		return TokenRecord{}, ErrTokenNotFound
	}
	return token, nil
}

func (s *memoryTokenStore) ListMCPAuthTokens(context.Context) ([]TokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tokens := make([]TokenRecord, 0, len(s.tokens))
	for _, token := range s.tokens {
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func (s *memoryTokenStore) DeleteMCPAuthToken(_ context.Context, target Target) error {
	key, err := target.Key()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, key)
	return nil
}

func globalTestTarget(serverName string) Target {
	return Target{Scope: ScopeGlobal, ServerName: serverName}
}

type handlerErrorRecorder struct {
	mu   sync.Mutex
	errs []error
}

func newHandlerErrorRecorder() *handlerErrorRecorder {
	return &handlerErrorRecorder{}
}

func (r *handlerErrorRecorder) record(err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, err)
}

func (r *handlerErrorRecorder) assertEmpty(t *testing.T) {
	t.Helper()

	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.errs) > 0 {
		t.Fatalf("handler errors = %v", errors.Join(r.errs...))
	}
}

func writeJSON(w http.ResponseWriter, value any) error {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return fmt.Errorf("json.Encode() error = %w", err)
	}
	return nil
}
