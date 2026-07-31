// Suite: MCP OAuth manager session ownership
// Invariant: invalidation or supersession prevents stale exchanges from persisting tokens.
// Boundary IN: Manager session maps, mutation generations, and the local OAuth HTTP boundary.
// Boundary OUT: daemon routing and durable SQLite persistence.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerSessionMutationRaces(t *testing.T) {
	t.Parallel()

	t.Run("Should reject token persistence when logout races an exchange", func(t *testing.T) {
		t.Parallel()
		manager, cfg, store, authority := newManagerRaceHarness(t, true)
		begin, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		exchangeErr := startManagerExchange(t, manager, cfg, begin)
		<-authority.tokenStarted
		if _, err := manager.Logout(t.Context(), cfg); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		authority.releaseToken()
		if err := <-exchangeErr; !errors.Is(err, ErrLoginSessionStale) {
			t.Fatalf("Exchange(racing logout) error = %v, want ErrLoginSessionStale", err)
		}
		if _, err := store.GetMCPAuthToken(t.Context(), cfg.Target); !errors.Is(err, ErrTokenNotFound) {
			t.Fatalf("GetMCPAuthToken(after logout) error = %v, want ErrTokenNotFound", err)
		}
	})

	t.Run("Should preserve the durable token when invalidation races an exchange", func(t *testing.T) {
		t.Parallel()
		manager, cfg, store, authority := newManagerRaceHarness(t, true)
		original := managerBoundToken(t, cfg, "original-access")
		if err := store.SaveMCPAuthToken(t.Context(), original); err != nil {
			t.Fatalf("SaveMCPAuthToken(original) error = %v", err)
		}
		begin, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		exchangeErr := startManagerExchange(t, manager, cfg, begin)
		<-authority.tokenStarted
		if err := manager.Invalidate(cfg.Target); err != nil {
			t.Fatalf("Invalidate() error = %v", err)
		}
		authority.releaseToken()
		if err := <-exchangeErr; !errors.Is(err, ErrLoginSessionStale) {
			t.Fatalf("Exchange(racing invalidation) error = %v, want ErrLoginSessionStale", err)
		}
		persisted, err := store.GetMCPAuthToken(t.Context(), cfg.Target)
		if err != nil {
			t.Fatalf("GetMCPAuthToken() error = %v", err)
		}
		if persisted.AccessToken != original.AccessToken {
			t.Fatalf("persisted access token = %q, want %q", persisted.AccessToken, original.AccessToken)
		}
	})

	t.Run("Should keep the superseding session when Begin races an exchange", func(t *testing.T) {
		t.Parallel()
		manager, cfg, store, authority := newManagerRaceHarness(t, true)
		first, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin(first) error = %v", err)
		}
		exchangeErr := startManagerExchange(t, manager, cfg, first)
		<-authority.tokenStarted
		second, err := manager.Begin(t.Context(), cfg, managerTestCallbackURL)
		if err != nil {
			t.Fatalf("Begin(second) error = %v", err)
		}
		authority.releaseToken()
		if err := <-exchangeErr; !errors.Is(err, ErrTokenMutationStale) {
			t.Fatalf("Exchange(superseded) error = %v, want ErrTokenMutationStale", err)
		}
		if _, err := manager.CallbackTarget(managerCallbackURL(second)); err != nil {
			t.Fatalf("CallbackTarget(superseding session) error = %v", err)
		}
		if _, err := store.GetMCPAuthToken(t.Context(), cfg.Target); !errors.Is(err, ErrTokenNotFound) {
			t.Fatalf("GetMCPAuthToken(after supersession) error = %v, want ErrTokenNotFound", err)
		}
	})

	t.Run("Should release the manager lock while validating a durable registration", func(t *testing.T) {
		t.Parallel()
		manager, cfg, store, _ := newManagerRaceHarness(t, false)
		cfg.Registration = RegistrationAuto
		fingerprint, err := ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
		}
		store.registration = ClientRegistration{
			Target:                cfg.Target,
			DefinitionFingerprint: fingerprint,
			ResourceURL:           cfg.RemoteURL,
			Issuer:                cfg.IssuerURL,
			ClientID:              cfg.ClientID,
		}
		store.registrationStarted = make(chan struct{})
		store.releaseRegistration = make(chan struct{})
		manager.service.registrations = store
		token := managerBoundToken(t, cfg, "exchanged-access")

		commitErr := make(chan error, 1)
		go func() {
			_, commitErrValue := manager.commitExchange(
				t.Context(),
				&loginSession{generation: 0},
				cfg,
				token,
			)
			commitErr <- commitErrValue
		}()
		<-store.registrationStarted

		invalidationErr := make(chan error, 1)
		go func() { invalidationErr <- manager.Invalidate(cfg.Target) }()
		select {
		case err := <-invalidationErr:
			if err != nil {
				t.Fatalf("Invalidate() error = %v", err)
			}
		case <-time.After(time.Second):
			close(store.releaseRegistration)
			t.Fatal("Invalidate() blocked behind registration-store I/O")
		}
		close(store.releaseRegistration)
		if err := <-commitErr; !errors.Is(err, ErrLoginSessionStale) {
			t.Fatalf("commitExchange() error = %v, want ErrLoginSessionStale", err)
		}
	})
}

func TestManagerConcurrentSessionContract(t *testing.T) {
	t.Parallel()
	t.Run("Should keep one active session and one exchange winner", func(t *testing.T) {
		t.Parallel()
		manager, cfg, _, authority := newManagerRaceHarness(t, false)
		const workers = 16
		var beginSuccesses atomic.Int32
		var beginStale atomic.Int32
		beginErrors := make(chan error, workers)
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
					beginErrors <- err
				}
			})
		}
		begins.Wait()
		close(beginErrors)
		for err := range beginErrors {
			t.Fatalf("Begin(concurrent) unexpected error = %v", err)
		}
		if beginSuccesses.Load() == 0 || beginSuccesses.Load()+beginStale.Load() != workers {
			t.Fatalf(
				"Begin(concurrent) results = successes:%d stale:%d, want %d classified results",
				beginSuccesses.Load(),
				beginStale.Load(),
				workers,
			)
		}

		manager.mu.Lock()
		activeTargets := len(manager.byTarget)
		activeStates := len(manager.byState)
		if activeTargets != 1 || activeStates != 1 {
			manager.mu.Unlock()
			t.Fatalf("active sessions = targets:%d states:%d, want 1/1", activeTargets, activeStates)
		}
		var active *loginSession
		for _, session := range manager.byTarget {
			active = session
		}
		begin := BeginResult{CallbackURL: active.state.RedirectURL, State: active.state.State}
		manager.mu.Unlock()

		var winners atomic.Int32
		var losers atomic.Int32
		exchangeErrors := make(chan error, workers)
		var exchanges sync.WaitGroup
		for range workers {
			exchanges.Go(func() {
				_, err := manager.Exchange(
					t.Context(),
					cfg,
					ExchangeInput{RedirectURL: managerCallbackURL(begin)},
				)
				switch {
				case err == nil:
					winners.Add(1)
				case errors.Is(err, ErrLoginSessionNotFound):
					losers.Add(1)
				default:
					exchangeErrors <- err
				}
			})
		}
		exchanges.Wait()
		close(exchangeErrors)
		for err := range exchangeErrors {
			t.Fatalf("Exchange(concurrent) unexpected error = %v", err)
		}
		if winners.Load() != 1 || losers.Load() != workers-1 {
			t.Fatalf("Exchange(concurrent) results = winners:%d losers:%d", winners.Load(), losers.Load())
		}
		if got := authority.tokenCalls.Load(); got != 1 {
			t.Fatalf("token endpoint calls = %d, want 1", got)
		}
	})
}

const managerTestCallbackURL = "http://127.0.0.1:2123/api/mcp/oauth/callback"

type managerRaceAuthority struct {
	tokenStarted chan struct{}
	release      chan struct{}
	startedOnce  sync.Once
	releaseOnce  sync.Once
	tokenCalls   atomic.Int32
}

func newManagerRaceHarness(
	t *testing.T,
	blockToken bool,
) (*Manager, ServerConfig, *managerTestStore, *managerRaceAuthority) {
	t.Helper()
	authority := &managerRaceAuthority{
		tokenStarted: make(chan struct{}),
		release:      make(chan struct{}),
	}
	if !blockToken {
		authority.releaseToken()
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration":
			writeManagerTestResponse(t, writer, fmt.Sprintf(
				`{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"code_challenge_methods_supported":["S256"]}`,
				"http://"+request.Host,
				"http://"+request.Host+"/authorize",
				"http://"+request.Host+"/token",
			))
		case "/token":
			authority.tokenCalls.Add(1)
			authority.startedOnce.Do(func() { close(authority.tokenStarted) })
			<-authority.release
			writeManagerTestResponse(t, writer, `{"access_token":"exchanged-access","token_type":"Bearer"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(func() {
		authority.releaseToken()
		server.Close()
	})
	store := &managerTestStore{}
	service, err := NewService(store, withHTTPClientForTest(server.Client()))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	manager, err := NewManager(service)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	cfg := ServerConfig{
		Target:       Target{Scope: ScopeGlobal, ServerName: "race"},
		Type:         authTypeOAuth,
		RemoteURL:    server.URL + "/mcp",
		IssuerURL:    server.URL,
		ClientID:     "client",
		Registration: RegistrationPreRegistered,
	}
	return manager, cfg, store, authority
}

func (a *managerRaceAuthority) releaseToken() {
	a.releaseOnce.Do(func() { close(a.release) })
}

func startManagerExchange(
	t *testing.T,
	manager *Manager,
	cfg ServerConfig,
	begin BeginResult,
) <-chan error {
	t.Helper()
	result := make(chan error, 1)
	go func() {
		_, err := manager.Exchange(
			t.Context(),
			cfg,
			ExchangeInput{RedirectURL: managerCallbackURL(begin)},
		)
		result <- err
	}()
	return result
}

func managerCallbackURL(begin BeginResult) string {
	return begin.CallbackURL + "?code=manager-code&state=" + begin.State
}

func managerBoundToken(t *testing.T, cfg ServerConfig, accessToken string) TokenRecord {
	t.Helper()
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
	}
	return TokenRecord{
		Target:                cfg.Target,
		DefinitionFingerprint: fingerprint,
		Issuer:                cfg.IssuerURL,
		ClientID:              cfg.ClientID,
		AccessToken:           accessToken,
		TokenType:             serviceBearerValue,
	}
}

func writeManagerTestResponse(t *testing.T, writer http.ResponseWriter, body string) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if _, err := writer.Write([]byte(body)); err != nil {
		t.Errorf("write OAuth response: %v", err)
	}
}

type managerTestStore struct {
	mu                  sync.Mutex
	token               TokenRecord
	deleted             bool
	registration        ClientRegistration
	registrationStarted chan struct{}
	registrationOnce    sync.Once
	releaseRegistration chan struct{}
}

func (s *managerTestStore) SaveMCPAuthToken(_ context.Context, token TokenRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
	s.deleted = false
	return nil
}

func (s *managerTestStore) GetMCPAuthToken(context.Context, Target) (TokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleted || strings.TrimSpace(s.token.AccessToken) == "" {
		return TokenRecord{}, ErrTokenNotFound
	}
	return s.token, nil
}

func (s *managerTestStore) ListMCPAuthTokens(context.Context) ([]TokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleted || strings.TrimSpace(s.token.AccessToken) == "" {
		return nil, nil
	}
	return []TokenRecord{s.token}, nil
}

func (s *managerTestStore) DeleteMCPAuthToken(context.Context, Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = TokenRecord{}
	return nil
}

func (s *managerTestStore) DeleteMCPAuthorizationState(context.Context, Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = TokenRecord{}
	s.deleted = true
	return nil
}

func (s *managerTestStore) GetMCPAuthRegistration(
	context.Context,
	Target,
) (ClientRegistration, error) {
	s.mu.Lock()
	registration := s.registration
	started := s.registrationStarted
	release := s.releaseRegistration
	s.mu.Unlock()
	if started != nil {
		s.registrationOnce.Do(func() { close(started) })
	}
	if release != nil {
		<-release
	}
	if strings.TrimSpace(registration.ClientID) == "" {
		return ClientRegistration{}, ErrRegistrationNotFound
	}
	return registration, nil
}

func (s *managerTestStore) SaveMCPAuthRegistration(
	_ context.Context,
	registration ClientRegistration,
	_ RegistrationSecrets,
) (ClientRegistration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registration = registration
	return registration, nil
}

func (s *managerTestStore) DeleteMCPAuthRegistration(context.Context, Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registration = ClientRegistration{}
	return nil
}

var _ TokenStore = (*managerTestStore)(nil)
var _ RegistrationStore = (*managerTestStore)(nil)
