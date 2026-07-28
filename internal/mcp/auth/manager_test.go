package auth

import (
	"context"
	"encoding/base64"
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
)

func TestManagerPKCESessionContract(t *testing.T) {
	t.Parallel()

	t.Run("Should supersede state and consume a successful session once", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
		manager, cfg, _, closeHarness := newManagerTestHarness(t, func() time.Time { return now })
		defer closeHarness()

		first, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin(first) error = %v", err)
		}
		second, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin(second) error = %v", err)
		}
		if first.State == second.State {
			t.Fatalf("successive state values collided: %q", first.State)
		}
		if got, want := second.ExpiresAt, now.Add(5*time.Minute); !got.Equal(want) {
			t.Fatalf("ExpiresAt = %s, want %s", got, want)
		}
		if !second.ManualSupported || second.CallbackURL != managerTestCallbackURL {
			t.Fatalf("BeginResult = %#v", second)
		}

		oldCallback := callbackWithCodeAndState(t, "old-code", first.State)
		if _, err := manager.ExchangeCallback(t.Context(), cfg, oldCallback); !errors.Is(err, ErrLoginSessionNotFound) {
			t.Fatalf("ExchangeCallback(superseded) error = %v, want ErrLoginSessionNotFound", err)
		}
		status, err := manager.Exchange(t.Context(), cfg, ExchangeInput{Code: "accepted-code"})
		if err != nil {
			t.Fatalf("Exchange(code) error = %v", err)
		}
		if status.Status != StatusAuthenticated || !status.TokenPresent {
			t.Fatalf("Exchange(code) status = %#v", status)
		}
		if _, err := manager.Exchange(
			t.Context(),
			cfg,
			ExchangeInput{Code: "reused-code"},
		); !errors.Is(
			err,
			ErrLoginSessionNotFound,
		) {
			t.Fatalf("Exchange(reused) error = %v, want ErrLoginSessionNotFound", err)
		}
		third, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin(third) error = %v", err)
		}
		callbackStatus, err := manager.ExchangeCallback(
			t.Context(),
			cfg,
			callbackWithCodeAndState(t, "browser-code", third.State),
		)
		if err != nil {
			t.Fatalf("ExchangeCallback(browser) error = %v", err)
		}
		if callbackStatus.Status != StatusAuthenticated || !callbackStatus.TokenPresent {
			t.Fatalf("ExchangeCallback(browser) status = %#v", callbackStatus)
		}
		durableStatus, err := manager.Status(t.Context(), cfg)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if durableStatus.Status != StatusAuthenticated || !durableStatus.TokenPresent {
			t.Fatalf("Status() = %#v, want authenticated token", durableStatus)
		}
		loggedOutStatus, err := manager.Logout(t.Context(), cfg)
		if err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		if loggedOutStatus.Status != StatusNeedsLogin || loggedOutStatus.TokenPresent {
			t.Fatalf("Logout() status = %#v, want needs_login without token", loggedOutStatus)
		}
		afterLogout, err := manager.Status(t.Context(), cfg)
		if err != nil {
			t.Fatalf("Status(after logout) error = %v", err)
		}
		if afterLogout.Status != StatusNeedsLogin || afterLogout.TokenPresent {
			t.Fatalf("Status(after logout) = %#v, want needs_login without token", afterLogout)
		}
	})

	t.Run("Should invalidate a pending session when the target logs out", func(t *testing.T) {
		t.Parallel()

		manager, cfg, _, closeHarness := newManagerTestHarness(t, time.Now)
		defer closeHarness()
		login, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		if _, err := manager.Logout(t.Context(), cfg); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		if _, err := manager.Exchange(
			t.Context(),
			cfg,
			ExchangeInput{Code: "delayed-code"},
		); !errors.Is(err, ErrLoginSessionNotFound) {
			t.Fatalf("Exchange(after logout) error = %v, want ErrLoginSessionNotFound", err)
		}
		if _, err := manager.ExchangeCallback(
			t.Context(),
			cfg,
			callbackWithCodeAndState(t, "delayed-code", login.State),
		); !errors.Is(err, ErrLoginSessionNotFound) {
			t.Fatalf("ExchangeCallback(after logout) error = %v, want ErrLoginSessionNotFound", err)
		}
	})

	t.Run("Should reject an in-flight Begin after logout completes", func(t *testing.T) {
		t.Parallel()

		handlerErrors := newHandlerErrorRecorder()
		metadataStarted := make(chan struct{})
		releaseMetadata := make(chan struct{})
		var releaseMetadataOnce sync.Once
		releaseBegin := func() { releaseMetadataOnce.Do(func() { close(releaseMetadata) }) }
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/metadata" {
				http.NotFound(w, r)
				return
			}
			close(metadataStarted)
			<-releaseMetadata
			if err := writeJSON(w, Metadata{
				AuthorizationEndpoint:         "http://" + r.Host + "/authorize",
				TokenEndpoint:                 "http://" + r.Host + "/token",
				CodeChallengeMethodsSupported: []string{pkceS256Value},
			}); err != nil {
				handlerErrors.record(err)
			}
		}))
		defer func() {
			releaseBegin()
			server.Close()
			handlerErrors.assertEmpty(t)
		}()

		store := newMemoryTokenStore()
		service, err := NewService(store, WithHTTPClient(server.Client()))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		manager, err := NewManager(service)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cfg := ServerConfig{
			Target: globalTestTarget("linear"), Type: "oauth2_pkce",
			MetadataURL: server.URL + "/metadata", ClientID: "client-id",
		}
		beginResult := make(chan error, 1)
		go func() {
			_, beginErr := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
			beginResult <- beginErr
		}()
		<-metadataStarted

		if _, err := manager.Logout(t.Context(), cfg); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		releaseBegin()
		if err := <-beginResult; !errors.Is(err, ErrLoginSessionStale) {
			t.Fatalf("Begin(racing logout) error = %v, want ErrLoginSessionStale", err)
		}
	})

	t.Run("Should refuse token persistence when logout races an in-flight exchange", func(t *testing.T) {
		t.Parallel()

		handlerErrors := newHandlerErrorRecorder()
		tokenStarted := make(chan struct{})
		releaseToken := make(chan struct{})
		var releaseTokenOnce sync.Once
		releaseExchange := func() { releaseTokenOnce.Do(func() { close(releaseToken) }) }
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
				close(tokenStarted)
				<-releaseToken
				if err := writeJSON(w, map[string]any{
					"access_token": "logged-out-access", "token_type": "Bearer", "expires_in": 3600,
				}); err != nil {
					handlerErrors.record(err)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer func() {
			releaseExchange()
			server.Close()
			handlerErrors.assertEmpty(t)
		}()

		store := newMemoryTokenStore()
		service, err := NewService(store, WithHTTPClient(server.Client()))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		manager, err := NewManager(service)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cfg := ServerConfig{
			Target: globalTestTarget("linear"), Type: "oauth2_pkce",
			MetadataURL: server.URL + "/metadata", ClientID: "client-id",
		}
		if _, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL); err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		exchangeResult := make(chan error, 1)
		go func() {
			_, exchangeErr := manager.Exchange(t.Context(), cfg, ExchangeInput{Code: "racing-code"})
			exchangeResult <- exchangeErr
		}()
		<-tokenStarted

		if _, err := manager.Logout(t.Context(), cfg); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		releaseExchange()
		if err := <-exchangeResult; !errors.Is(err, ErrLoginSessionStale) {
			t.Fatalf("Exchange(racing logout) error = %v, want ErrLoginSessionStale", err)
		}
		if _, err := store.GetMCPAuthToken(t.Context(), cfg.Target); !errors.Is(err, ErrTokenNotFound) {
			t.Fatalf("GetMCPAuthToken(after racing logout) error = %v, want ErrTokenNotFound", err)
		}
	})

	t.Run("Should reject an expired session", func(t *testing.T) {
		t.Parallel()

		var nowMu sync.Mutex
		now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
		nowFn := func() time.Time {
			nowMu.Lock()
			defer nowMu.Unlock()
			return now
		}
		manager, cfg, _, closeHarness := newManagerTestHarness(t, nowFn)
		defer closeHarness()
		if _, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL); err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		nowMu.Lock()
		now = now.Add(5*time.Minute + time.Nanosecond)
		nowMu.Unlock()
		if _, err := manager.Exchange(
			t.Context(),
			cfg,
			ExchangeInput{Code: "late-code"},
		); !errors.Is(
			err,
			ErrLoginSessionExpired,
		) {
			t.Fatalf("Exchange(expired) error = %v, want ErrLoginSessionExpired", err)
		}
	})

	t.Run("Should generate unique 256-bit state values", func(t *testing.T) {
		t.Parallel()

		manager, cfg, _, closeHarness := newManagerTestHarness(t, time.Now)
		defer closeHarness()
		seen := make(map[string]struct{}, 64)
		for range 64 {
			result, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
			if err != nil {
				t.Fatalf("Begin() error = %v", err)
			}
			decoded, err := base64.RawURLEncoding.DecodeString(result.State)
			if err != nil {
				t.Fatalf("DecodeString(state) error = %v", err)
			}
			if len(decoded) != stateBytes {
				t.Fatalf("decoded state bytes = %d, want %d", len(decoded), stateBytes)
			}
			if _, duplicate := seen[result.State]; duplicate {
				t.Fatalf("duplicate state = %q", result.State)
			}
			seen[result.State] = struct{}{}
		}
	})
}

func TestManagerRedirectURLExchangeRequiresMatchingState(t *testing.T) {
	t.Parallel()

	t.Run("Should accept a redirect URL only when its state matches", func(t *testing.T) {
		t.Parallel()

		manager, cfg, _, closeHarness := newManagerTestHarness(t, time.Now)
		defer closeHarness()
		login, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		status, err := manager.Exchange(t.Context(), cfg, ExchangeInput{
			RedirectURL: callbackWithCodeAndState(t, "manual-code", login.State),
		})
		if err != nil {
			t.Fatalf("Exchange(redirect URL) error = %v", err)
		}
		if status.Status != StatusAuthenticated || !status.TokenPresent {
			t.Fatalf("Exchange(redirect URL) status = %#v", status)
		}

		secondLogin, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin(mismatch) error = %v", err)
		}
		_, err = manager.Exchange(t.Context(), cfg, ExchangeInput{
			RedirectURL: callbackWithCodeAndState(t, "mismatched-code", "wrong-state"),
		})
		if !errors.Is(err, ErrInvalidExchange) {
			t.Fatalf("Exchange(mismatched state) error = %v, want ErrInvalidExchange", err)
		}
		status, err = manager.Exchange(t.Context(), cfg, ExchangeInput{
			RedirectURL: callbackWithCodeAndState(t, "accepted-after-mismatch", secondLogin.State),
		})
		if err != nil {
			t.Fatalf("Exchange(correct state after mismatch) error = %v", err)
		}
		if status.Status != StatusAuthenticated || !status.TokenPresent {
			t.Fatalf("Exchange(correct state after mismatch) status = %#v", status)
		}
	})
}

func TestManagerLifecycleNotificationsExcludeOAuthMaterial(t *testing.T) {
	t.Parallel()

	t.Run("Should exclude OAuth material from lifecycle notifications", func(t *testing.T) {
		t.Parallel()

		manager, cfg, _, closeHarness := newManagerTestHarness(t, time.Now)
		defer closeHarness()
		notifier := &recordingLifecycleNotifier{}
		configuredManager, err := NewManager(
			manager.service,
			WithManagerNow(time.Now),
			WithLifecycleNotifier(notifier),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		manager = configuredManager
		login, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		secretCode := "sensitive-notification-code"
		if _, err := manager.Exchange(t.Context(), cfg, ExchangeInput{Code: secretCode}); err != nil {
			t.Fatalf("Exchange() error = %v", err)
		}
		if _, err := manager.Logout(t.Context(), cfg); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		if len(notifier.outcomes) != 3 {
			t.Fatalf("lifecycle outcome count = %d, want 3", len(notifier.outcomes))
		}
		serialized := fmt.Sprintf("%#v", notifier.outcomes)
		for _, forbidden := range []string{
			secretCode,
			login.State,
			login.AuthorizationURL,
			managerTestCallbackURL,
			"manager-access-token",
			"code_verifier",
		} {
			if strings.Contains(serialized, forbidden) {
				t.Fatalf("lifecycle notifications leaked %q: %s", forbidden, serialized)
			}
		}
	})
}

func TestManagerConfigurationContract(t *testing.T) {
	t.Parallel()

	t.Run("Should validate dependencies and apply a custom session TTL", func(t *testing.T) {
		t.Parallel()

		manager, cfg, _, closeHarness := newManagerTestHarness(t, time.Now)
		defer closeHarness()
		if _, err := NewManager(nil); err == nil {
			t.Fatal("NewManager(nil) error = nil, want required service failure")
		}
		if _, err := NewManager(manager.service, WithLoginSessionTTL(0)); err == nil {
			t.Fatal("NewManager(zero TTL) error = nil, want positive TTL failure")
		}

		now := time.Date(2026, 7, 13, 18, 0, 0, 0, time.UTC)
		shortManager, err := NewManager(
			manager.service,
			WithManagerNow(func() time.Time { return now }),
			WithLoginSessionTTL(time.Minute),
		)
		if err != nil {
			t.Fatalf("NewManager(custom TTL) error = %v", err)
		}
		result, err := shortManager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		if got, want := result.ExpiresAt, now.Add(time.Minute); !got.Equal(want) {
			t.Fatalf("ExpiresAt = %s, want %s", got, want)
		}
	})
}

func TestManagerServerDefinitionBinding(t *testing.T) {
	t.Parallel()

	t.Run("Should reject manual and callback completion after the server definition changes", func(t *testing.T) {
		t.Parallel()

		manager, cfg, tokenCalls, closeHarness := newManagerTestHarness(t, time.Now)
		defer closeHarness()
		_, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin(manual) error = %v", err)
		}
		changed := cfg
		changed.RemoteURL = "https://replacement.example/mcp"
		if _, err := manager.Exchange(
			t.Context(),
			changed,
			ExchangeInput{Code: "stale-code"},
		); !errors.Is(err, ErrLoginSessionStale) {
			t.Fatalf("Exchange(changed definition) error = %v, want ErrLoginSessionStale", err)
		}
		if tokenCalls.Load() != 0 {
			t.Fatalf("token endpoint calls after stale manual exchange = %d, want 0", tokenCalls.Load())
		}

		callback, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin(callback) error = %v", err)
		}
		callbackURL := callbackWithCodeAndState(t, "stale-callback-code", callback.State)
		target, err := manager.CallbackTarget(callbackURL)
		if err != nil {
			t.Fatalf("CallbackTarget() error = %v", err)
		}
		if target != cfg.Target.Normalize() {
			t.Fatalf("CallbackTarget() = %#v, want %#v", target, cfg.Target.Normalize())
		}
		if _, err := manager.ExchangeCallback(
			t.Context(),
			changed,
			callbackURL,
		); !errors.Is(err, ErrLoginSessionStale) {
			t.Fatalf("ExchangeCallback(changed definition) error = %v, want ErrLoginSessionStale", err)
		}
		if tokenCalls.Load() != 0 {
			t.Fatalf("token endpoint calls after stale callback exchange = %d, want 0", tokenCalls.Load())
		}
	})

	t.Run("Should reject an older Begin while the superseding generation exchanges", func(t *testing.T) {
		t.Parallel()

		handlerErrors := newHandlerErrorRecorder()
		firstMetadataStarted := make(chan struct{})
		releaseFirstMetadata := make(chan struct{})
		tokenStarted := make(chan struct{})
		releaseToken := make(chan struct{})
		var metadataCalls atomic.Int32
		var releaseMetadataOnce sync.Once
		var releaseTokenOnce sync.Once
		releaseMetadata := func() { releaseMetadataOnce.Do(func() { close(releaseFirstMetadata) }) }
		releaseExchange := func() { releaseTokenOnce.Do(func() { close(releaseToken) }) }
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/metadata":
				if metadataCalls.Add(1) == 1 {
					close(firstMetadataStarted)
					<-releaseFirstMetadata
				}
				if err := writeJSON(w, Metadata{
					AuthorizationEndpoint:         "http://" + r.Host + "/authorize",
					TokenEndpoint:                 "http://" + r.Host + "/token",
					CodeChallengeMethodsSupported: []string{pkceS256Value},
				}); err != nil {
					handlerErrors.record(err)
				}
			case "/token":
				close(tokenStarted)
				<-releaseToken
				if err := writeJSON(w, map[string]any{
					"access_token": "superseding-access", "token_type": "Bearer", "expires_in": 3600,
				}); err != nil {
					handlerErrors.record(err)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer func() {
			releaseMetadata()
			releaseExchange()
			server.Close()
			handlerErrors.assertEmpty(t)
		}()

		store := newMemoryTokenStore()
		service, err := NewService(store, WithHTTPClient(server.Client()))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		manager, err := NewManager(service)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cfg := ServerConfig{
			Target: globalTestTarget("linear"), Type: "oauth2_pkce",
			MetadataURL: server.URL + "/metadata", ClientID: "client-id",
		}
		firstBegin := make(chan error, 1)
		go func() {
			_, beginErr := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
			firstBegin <- beginErr
		}()
		<-firstMetadataStarted

		if _, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL); err != nil {
			t.Fatalf("Begin(superseding) error = %v", err)
		}
		exchangeResult := make(chan error, 1)
		go func() {
			_, exchangeErr := manager.Exchange(t.Context(), cfg, ExchangeInput{Code: "superseding-code"})
			exchangeResult <- exchangeErr
		}()
		<-tokenStarted
		releaseMetadata()
		if err := <-firstBegin; !errors.Is(err, ErrLoginSessionStale) {
			t.Fatalf("Begin(older) error = %v, want ErrLoginSessionStale", err)
		}
		releaseExchange()
		if err := <-exchangeResult; err != nil {
			t.Fatalf("Exchange(superseding) error = %v", err)
		}
		persisted, err := store.GetMCPAuthToken(t.Context(), cfg.Target)
		if err != nil {
			t.Fatalf("GetMCPAuthToken() error = %v", err)
		}
		if persisted.AccessToken != "superseding-access" {
			t.Fatalf("persisted access token = %q, want superseding-access", persisted.AccessToken)
		}
	})

	t.Run("Should invalidate pending state without deleting a durable token", func(t *testing.T) {
		t.Parallel()

		manager, cfg, tokenCalls, closeHarness := newManagerTestHarness(t, time.Now)
		defer closeHarness()
		fingerprint, err := ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
		}
		if err := manager.service.store.SaveMCPAuthToken(t.Context(), TokenRecord{
			Target: cfg.Target, DefinitionFingerprint: fingerprint, ClientID: cfg.ClientID,
			AccessToken: "durable-access", TokenType: "Bearer", UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken(durable) error = %v", err)
		}
		if _, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL); err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		if err := manager.Invalidate(cfg.Target); err != nil {
			t.Fatalf("Invalidate() error = %v", err)
		}
		if _, err := manager.Exchange(
			t.Context(),
			cfg,
			ExchangeInput{Code: "delayed-code"},
		); !errors.Is(err, ErrLoginSessionNotFound) {
			t.Fatalf("Exchange(after invalidate) error = %v, want ErrLoginSessionNotFound", err)
		}
		preserved, err := manager.service.store.GetMCPAuthToken(t.Context(), cfg.Target)
		if err != nil {
			t.Fatalf("GetMCPAuthToken() error = %v", err)
		}
		if preserved.AccessToken != "durable-access" {
			t.Fatalf("preserved token access value = %q, want durable-access", preserved.AccessToken)
		}
		if tokenCalls.Load() != 0 {
			t.Fatalf("token endpoint calls after invalidation = %d, want 0", tokenCalls.Load())
		}
	})

	t.Run("Should refuse persistence when invalidation races an in-flight token exchange", func(t *testing.T) {
		t.Parallel()

		handlerErrors := newHandlerErrorRecorder()
		tokenStarted := make(chan struct{})
		releaseToken := make(chan struct{})
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
				close(tokenStarted)
				<-releaseToken
				if err := writeJSON(w, map[string]any{
					"access_token": "racing-access", "token_type": "Bearer", "expires_in": 3600,
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
		store := newMemoryTokenStore()
		service, err := NewService(store, WithHTTPClient(server.Client()))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		manager, err := NewManager(service)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cfg := ServerConfig{
			Target: globalTestTarget("linear"), Type: "oauth2_pkce",
			MetadataURL: server.URL + "/metadata", ClientID: "client-id",
		}
		fingerprint, err := ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
		}
		if err := store.SaveMCPAuthToken(t.Context(), TokenRecord{
			Target: cfg.Target, DefinitionFingerprint: fingerprint, ClientID: cfg.ClientID,
			AccessToken: "preserved-access", TokenType: "Bearer", UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken(preserved) error = %v", err)
		}
		if _, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL); err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		exchangeResult := make(chan error, 1)
		go func() {
			_, exchangeErr := manager.Exchange(t.Context(), cfg, ExchangeInput{Code: "racing-code"})
			exchangeResult <- exchangeErr
		}()
		<-tokenStarted
		if err := manager.Invalidate(cfg.Target); err != nil {
			t.Fatalf("Invalidate() error = %v", err)
		}
		close(releaseToken)
		if err := <-exchangeResult; !errors.Is(err, ErrLoginSessionStale) {
			t.Fatalf("Exchange(racing invalidation) error = %v, want ErrLoginSessionStale", err)
		}
		preserved, err := store.GetMCPAuthToken(t.Context(), cfg.Target)
		if err != nil {
			t.Fatalf("GetMCPAuthToken() error = %v", err)
		}
		if preserved.AccessToken != "preserved-access" {
			t.Fatalf("preserved token access value = %q, want preserved-access", preserved.AccessToken)
		}
	})
}

func TestManagerPrunesAbandonedExpiredSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should prune abandoned expired sessions", func(t *testing.T) {
		t.Parallel()

		var nowMu sync.Mutex
		now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
		nowFn := func() time.Time {
			nowMu.Lock()
			defer nowMu.Unlock()
			return now
		}
		manager, cfg, _, closeHarness := newManagerTestHarness(t, nowFn)
		defer closeHarness()
		first, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin(first) error = %v", err)
		}
		nowMu.Lock()
		now = now.Add(defaultLoginSessionTTL + time.Nanosecond)
		nowMu.Unlock()
		secondConfig := cfg
		secondConfig.Target = globalTestTarget("github")
		if _, err := manager.Begin(t.Context(), secondConfig, managerTestCallbackURL); err != nil {
			t.Fatalf("Begin(second) error = %v", err)
		}
		manager.mu.Lock()
		targetCount, stateCount := len(manager.byTarget), len(manager.byState)
		_, staleStatePresent := manager.byState[first.State]
		manager.mu.Unlock()
		if targetCount != 1 || stateCount != 1 || staleStatePresent {
			t.Fatalf(
				"sessions after prune = targets:%d states:%d stale:%t, want 1/1/false",
				targetCount,
				stateCount,
				staleStatePresent,
			)
		}
	})
}

type recordingLifecycleNotifier struct {
	outcomes []LifecycleOutcome
}

func (r *recordingLifecycleNotifier) NotifyMCPAuth(_ context.Context, outcome LifecycleOutcome) {
	r.outcomes = append(r.outcomes, outcome)
}

func TestManagerConcurrentSessionContract(t *testing.T) {
	t.Parallel()

	t.Run("Should keep one active session and one exchange winner under concurrency", func(t *testing.T) {
		t.Parallel()

		manager, cfg, tokenCalls, closeHarness := newManagerTestHarness(t, time.Now)
		defer closeHarness()
		const workers = 24

		var beginSuccesses atomic.Int32
		var beginStale atomic.Int32
		var beginErrors atomic.Int32
		var begins sync.WaitGroup
		for range workers {
			begins.Go(func() {
				_, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
				switch {
				case err == nil:
					beginSuccesses.Add(1)
				case errors.Is(err, ErrLoginSessionStale):
					beginStale.Add(1)
				default:
					beginErrors.Add(1)
				}
			})
		}
		begins.Wait()
		if beginSuccesses.Load() == 0 || beginSuccesses.Load()+beginStale.Load() != workers || beginErrors.Load() != 0 {
			t.Fatalf(
				"concurrent Begin results = successes:%d stale:%d other:%d",
				beginSuccesses.Load(),
				beginStale.Load(),
				beginErrors.Load(),
			)
		}
		manager.mu.Lock()
		activeTargets := len(manager.byTarget)
		activeStates := len(manager.byState)
		manager.mu.Unlock()
		if activeTargets != 1 || activeStates != 1 {
			t.Fatalf("active sessions = targets:%d states:%d, want 1/1", activeTargets, activeStates)
		}

		var successes atomic.Int32
		var deterministicLosers atomic.Int32
		var exchanges sync.WaitGroup
		for worker := range workers {
			exchanges.Go(func() {
				_, err := manager.Exchange(
					t.Context(),
					cfg,
					ExchangeInput{Code: fmt.Sprintf("concurrent-code-%d", worker)},
				)
				switch {
				case err == nil:
					successes.Add(1)
				case errors.Is(err, ErrLoginSessionNotFound):
					deterministicLosers.Add(1)
				default:
					beginErrors.Add(1)
				}
			})
		}
		exchanges.Wait()
		if successes.Load() != 1 || deterministicLosers.Load() != workers-1 || beginErrors.Load() != 0 {
			t.Fatalf(
				"exchange results = successes:%d losers:%d other:%d",
				successes.Load(),
				deterministicLosers.Load(),
				beginErrors.Load(),
			)
		}
		if tokenCalls.Load() != 1 {
			t.Fatalf("token endpoint calls = %d, want 1", tokenCalls.Load())
		}
	})
}

func TestManagerExchangeInputContract(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		input ExchangeInput
		valid bool
	}{
		{name: "Should accept a code", input: ExchangeInput{Code: "code"}, valid: true},
		{
			name:  "Should accept an absolute redirect URL",
			input: ExchangeInput{RedirectURL: managerTestCallbackURL + "?code=code&state=state"},
			valid: true,
		},
		{name: "Should reject no input", input: ExchangeInput{}},
		{name: "Should reject both inputs", input: ExchangeInput{Code: "code", RedirectURL: managerTestCallbackURL}},
		{name: "Should reject a relative redirect URL", input: ExchangeInput{RedirectURL: "/callback"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := validateExchangeInput(tc.input)
			if tc.valid && err != nil {
				t.Fatalf("validateExchangeInput() error = %v", err)
			}
			if !tc.valid && !errors.Is(err, ErrInvalidExchange) {
				t.Fatalf("validateExchangeInput() error = %v, want ErrInvalidExchange", err)
			}
		})
	}
}

const managerTestCallbackURL = "http://127.0.0.1:2123/api/mcp/oauth/callback"

func newManagerTestHarness(
	t *testing.T,
	now func() time.Time,
) (*Manager, ServerConfig, *atomic.Int32, func()) {
	t.Helper()

	handlerErrors := newHandlerErrorRecorder()
	var tokenCalls atomic.Int32
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
			tokenCalls.Add(1)
			if err := r.ParseForm(); err != nil {
				handlerErrors.record(fmt.Errorf("parse token form: %w", err))
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(r.Form.Get("code_verifier")) == "" {
				handlerErrors.record(errors.New("token request code_verifier is empty"))
				http.Error(w, "invalid verifier", http.StatusBadRequest)
				return
			}
			if err := writeJSON(w, map[string]any{
				"access_token": "manager-access-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}); err != nil {
				handlerErrors.record(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	store := newMemoryTokenStore()
	service, err := NewService(store, WithHTTPClient(server.Client()), WithNow(now))
	if err != nil {
		server.Close()
		t.Fatalf("NewService() error = %v", err)
	}
	manager, err := NewManager(service, WithManagerNow(now))
	if err != nil {
		server.Close()
		t.Fatalf("NewManager() error = %v", err)
	}
	cfg := ServerConfig{
		Target:      globalTestTarget("linear"),
		Type:        "oauth2_pkce",
		MetadataURL: server.URL + "/metadata",
		ClientID:    "client-id",
		Scopes:      []string{"read"},
	}
	closeHarness := func() {
		server.Close()
		handlerErrors.assertEmpty(t)
	}
	return manager, cfg, &tokenCalls, closeHarness
}

func callbackWithCodeAndState(t *testing.T, code string, state string) string {
	t.Helper()

	parsed, err := url.Parse(managerTestCallbackURL)
	if err != nil {
		t.Fatalf("url.Parse(manager callback) error = %v", err)
	}
	query := parsed.Query()
	query.Set("code", code)
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
