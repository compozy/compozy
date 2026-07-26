package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultLoginSessionTTL = 5 * time.Minute

var (
	// ErrLoginSessionNotFound reports a missing or superseded OAuth session.
	ErrLoginSessionNotFound = errors.New("mcp auth: login session not found")
	// ErrLoginSessionExpired reports an OAuth session past its daemon-owned TTL.
	ErrLoginSessionExpired = errors.New("mcp auth: login session expired")
	// ErrLoginSessionStale reports a session whose server definition changed after begin.
	ErrLoginSessionStale = errors.New("mcp auth: login session server definition changed")
	// ErrInvalidExchange reports a malformed manual or callback completion.
	ErrInvalidExchange = errors.New("mcp auth: invalid exchange")
	// ErrTokenMutationStale reports a durable token mutation superseded by a newer action.
	ErrTokenMutationStale = errors.New("mcp auth: token mutation superseded")
	// ErrMutationGenerationUnavailable reports a missing mutation coordinator.
	ErrMutationGenerationUnavailable = errors.New("mcp auth: mutation generation is unavailable")
)

// LifecycleAction identifies one operator-visible OAuth mutation.
type LifecycleAction string

const (
	LifecycleBegin    LifecycleAction = "begin"
	LifecycleExchange LifecycleAction = "exchange"
	LifecycleLogout   LifecycleAction = "logout"
)

// LifecycleOutcome is a secret-free OAuth lifecycle event.
type LifecycleOutcome struct {
	Action     LifecycleAction
	Target     Target
	Outcome    string
	ErrorClass string
}

// LifecycleNotifier receives secret-free OAuth lifecycle outcomes.
type LifecycleNotifier interface {
	NotifyMCPAuth(context.Context, LifecycleOutcome)
}

// BeginResult is the verifier-free authorization handoff returned to callers.
type BeginResult struct {
	AuthorizationURL string    `json:"authorization_url"`
	State            string    `json:"state"`
	ExpiresAt        time.Time `json:"expires_at"`
	CallbackURL      string    `json:"callback_url"`
	ManualSupported  bool      `json:"manual_supported"`
}

// ExchangeInput accepts exactly one manual completion representation.
type ExchangeInput struct {
	Code        string
	RedirectURL string
}

// ManagerOption configures the daemon-owned OAuth session manager.
type ManagerOption func(*Manager)

// Manager owns short-lived PKCE state and delegates protocol operations to Service.
type Manager struct {
	service  *Service
	now      func() time.Time
	ttl      time.Duration
	notifier LifecycleNotifier

	mu         sync.Mutex
	byTarget   map[string]*loginSession
	byState    map[string]string
	revision   map[string]uint64
	generation *MutationGeneration
}

type loginSession struct {
	state                 LoginState
	expiresAt             time.Time
	definitionFingerprint string
	revision              uint64
	generation            uint64
}

// NewManager constructs one process-wide MCP OAuth manager.
func NewManager(service *Service, opts ...ManagerOption) (*Manager, error) {
	if service == nil {
		return nil, errors.New("mcp auth: protocol service is required")
	}
	manager := &Manager{
		service: service,
		now: func() time.Time {
			return time.Now().UTC()
		},
		ttl:        defaultLoginSessionTTL,
		byTarget:   make(map[string]*loginSession),
		byState:    make(map[string]string),
		revision:   make(map[string]uint64),
		generation: service.generation,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(manager)
		}
	}
	if manager.now == nil {
		manager.now = func() time.Time { return time.Now().UTC() }
	}
	if manager.ttl <= 0 {
		return nil, errors.New("mcp auth: login session TTL must be positive")
	}
	return manager, nil
}

// WithManagerNow overrides the manager clock for deterministic tests.
func WithManagerNow(now func() time.Time) ManagerOption {
	return func(manager *Manager) { manager.now = now }
}

// WithLoginSessionTTL overrides the five-minute session TTL for tests.
func WithLoginSessionTTL(ttl time.Duration) ManagerOption {
	return func(manager *Manager) { manager.ttl = ttl }
}

// WithLifecycleNotifier registers the secret-free lifecycle event sink.
func WithLifecycleNotifier(notifier LifecycleNotifier) ManagerOption {
	return func(manager *Manager) { manager.notifier = notifier }
}

// Begin creates and stores one verifier-bearing session, superseding the prior target session.
func (m *Manager) Begin(
	ctx context.Context,
	cfg ServerConfig,
	callbackURL string,
) (result BeginResult, err error) {
	defer func() { m.notify(ctx, LifecycleBegin, cfg.Target, err) }()
	if ctx == nil {
		return BeginResult{}, errors.New("mcp auth: begin context is required")
	}
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		return BeginResult{}, err
	}
	targetKey, err := cfg.Target.Key()
	if err != nil {
		return BeginResult{}, err
	}
	m.mu.Lock()
	m.pruneExpiredLocked()
	revision := m.revision[targetKey]
	if previous, ok := m.byTarget[targetKey]; ok {
		m.removeSessionLocked(targetKey, previous)
	}
	m.mu.Unlock()
	generation, err := m.generation.Advance(cfg.Target)
	if err != nil {
		return BeginResult{}, err
	}

	state, err := m.service.BeginLogin(ctx, cfg, callbackURL)
	if err != nil {
		return BeginResult{}, err
	}
	expiresAt := m.now().UTC().Add(m.ttl)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpiredLocked()
	currentGeneration, generationErr := m.generation.Snapshot(cfg.Target)
	if generationErr != nil {
		return BeginResult{}, generationErr
	}
	if m.revision[targetKey] != revision || currentGeneration != generation {
		return BeginResult{}, ErrLoginSessionStale
	}
	m.byTarget[targetKey] = &loginSession{
		state:                 state,
		expiresAt:             expiresAt,
		definitionFingerprint: fingerprint,
		revision:              revision,
		generation:            generation,
	}
	m.byState[state.State] = targetKey

	return BeginResult{
		AuthorizationURL: state.AuthorizationURL,
		State:            state.State,
		ExpiresAt:        expiresAt,
		CallbackURL:      state.RedirectURL,
		ManualSupported:  true,
	}, nil
}

// Exchange completes one target's active session using a code or full redirect URL.
func (m *Manager) Exchange(
	ctx context.Context,
	cfg ServerConfig,
	input ExchangeInput,
) (status Status, err error) {
	defer func() { m.notify(ctx, LifecycleExchange, cfg.Target, err) }()
	if ctx == nil {
		return Status{}, errors.New("mcp auth: exchange context is required")
	}
	callbackURL, err := validateExchangeInput(input)
	if err != nil {
		return Status{}, err
	}
	session, err := m.claimByTarget(cfg.Target, callbackURL)
	if err != nil {
		return Status{}, err
	}
	if err := validateLoginSessionDefinition(session, cfg); err != nil {
		return Status{}, err
	}
	if callbackURL == "" {
		callbackURL, err = callbackURLForCode(session.state, strings.TrimSpace(input.Code))
		if err != nil {
			return Status{}, err
		}
	}
	token, err := m.service.exchangeToken(ctx, session.state, callbackURL)
	if err != nil {
		return Status{}, err
	}
	return m.commitExchange(ctx, session, cfg, token)
}

// ExchangeCallback completes one active session selected only by callback state.
func (m *Manager) ExchangeCallback(
	ctx context.Context,
	cfg ServerConfig,
	callbackURL string,
) (status Status, err error) {
	target := Target{}
	defer func() { m.notify(ctx, LifecycleExchange, target, err) }()
	if ctx == nil {
		return Status{}, errors.New("mcp auth: callback context is required")
	}
	state, err := stateFromCallback(callbackURL)
	if err != nil {
		return Status{}, err
	}
	session, err := m.claimByState(state)
	if err != nil {
		return Status{}, err
	}
	target = session.state.Target
	if err := validateLoginSessionDefinition(session, cfg); err != nil {
		return Status{}, err
	}
	if _, err := authorizationCodeFromCallback(callbackURL, session.state.State); err != nil {
		return Status{}, fmt.Errorf("%w: %v", ErrInvalidExchange, err)
	}
	token, err := m.service.exchangeToken(ctx, session.state, callbackURL)
	if err != nil {
		return Status{}, err
	}
	return m.commitExchange(ctx, session, cfg, token)
}

// CallbackTarget resolves a callback to its unexpired pending target without consuming it.
func (m *Manager) CallbackTarget(callbackURL string) (Target, error) {
	state, err := stateFromCallback(callbackURL)
	if err != nil {
		return Target{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	targetKey, ok := m.byState[strings.TrimSpace(state)]
	if !ok {
		m.pruneExpiredLocked()
		return Target{}, ErrLoginSessionNotFound
	}
	session, ok := m.byTarget[targetKey]
	if !ok {
		delete(m.byState, strings.TrimSpace(state))
		m.pruneExpiredLocked()
		return Target{}, ErrLoginSessionNotFound
	}
	if !session.expiresAt.After(m.now().UTC()) {
		m.removeSessionLocked(targetKey, session)
		m.pruneExpiredLocked()
		return Target{}, ErrLoginSessionExpired
	}
	m.pruneExpiredLocked()
	return session.state.Target, nil
}

// Status returns redacted durable state for one scoped server.
func (m *Manager) Status(ctx context.Context, cfg ServerConfig) (Status, error) {
	return m.service.Status(ctx, cfg)
}

// Logout revokes remotely when possible and always removes local scoped state.
func (m *Manager) Logout(ctx context.Context, cfg ServerConfig) (status Status, err error) {
	defer func() { m.notify(ctx, LifecycleLogout, cfg.Target, err) }()
	// Advance the definition revision before remote/local token removal. Any
	// Begin or Exchange already outside the mutex can no longer install state
	// after this logout completes.
	if err := m.Invalidate(cfg.Target); err != nil {
		return Status{}, err
	}
	return m.service.Logout(ctx, cfg)
}

// Invalidate discards pending sessions and advances the target definition revision.
// Durable token state is intentionally left untouched.
func (m *Manager) Invalidate(target Target) error {
	key, err := target.Key()
	if err != nil {
		return err
	}
	if _, err := m.generation.Advance(target); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revision[key]++
	if session, ok := m.byTarget[key]; ok {
		m.removeSessionLocked(key, session)
	}
	m.pruneExpiredLocked()
	return nil
}

func (m *Manager) claimByTarget(target Target, callbackURL string) (*loginSession, error) {
	key, err := target.Key()
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.byTarget[key]
	if !ok {
		m.pruneExpiredLocked()
		return nil, ErrLoginSessionNotFound
	}
	if !session.expiresAt.After(m.now().UTC()) {
		m.removeSessionLocked(key, session)
		m.pruneExpiredLocked()
		return nil, ErrLoginSessionExpired
	}
	if callbackURL != "" {
		if _, err := authorizationCodeFromCallback(callbackURL, session.state.State); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidExchange, err)
		}
	}
	m.removeSessionLocked(key, session)
	m.pruneExpiredLocked()
	return session, nil
}

func (m *Manager) claimByState(state string) (*loginSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	targetKey, ok := m.byState[strings.TrimSpace(state)]
	if !ok {
		m.pruneExpiredLocked()
		return nil, ErrLoginSessionNotFound
	}
	session, ok := m.byTarget[targetKey]
	if !ok {
		delete(m.byState, strings.TrimSpace(state))
		m.pruneExpiredLocked()
		return nil, ErrLoginSessionNotFound
	}
	m.removeSessionLocked(targetKey, session)
	m.pruneExpiredLocked()
	if !session.expiresAt.After(m.now().UTC()) {
		return nil, ErrLoginSessionExpired
	}
	return session, nil
}

func (m *Manager) removeSessionLocked(targetKey string, session *loginSession) {
	delete(m.byTarget, targetKey)
	delete(m.byState, session.state.State)
}

func (m *Manager) pruneExpiredLocked() {
	now := m.now().UTC()
	for targetKey, session := range m.byTarget {
		if !session.expiresAt.After(now) {
			m.removeSessionLocked(targetKey, session)
		}
	}
}

func validateLoginSessionDefinition(session *loginSession, cfg ServerConfig) error {
	if session == nil {
		return ErrLoginSessionNotFound
	}
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		return err
	}
	if fingerprint != session.definitionFingerprint || cfg.Target.Normalize() != session.state.Target.Normalize() {
		return ErrLoginSessionStale
	}
	return nil
}

func (m *Manager) commitExchange(
	ctx context.Context,
	session *loginSession,
	cfg ServerConfig,
	token TokenRecord,
) (Status, error) {
	if session == nil {
		return Status{}, ErrLoginSessionNotFound
	}
	targetKey, err := cfg.Target.Key()
	if err != nil {
		return Status{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.revision[targetKey] != session.revision {
		return Status{}, ErrLoginSessionStale
	}
	if !tokenMatchesServerDefinition(token, cfg) {
		return Status{}, ErrLoginSessionStale
	}
	return m.service.persistReplacementTokenAtGeneration(ctx, cfg, token, session.generation)
}

func (m *Manager) notify(ctx context.Context, action LifecycleAction, target Target, err error) {
	if m == nil || m.notifier == nil {
		return
	}
	outcome := "success"
	errorClass := ""
	if err != nil {
		outcome = "failure"
		errorClass = managerErrorClass(err)
	}
	m.notifier.NotifyMCPAuth(ctx, LifecycleOutcome{
		Action: action, Target: target.Normalize(), Outcome: outcome, ErrorClass: errorClass,
	})
}

func managerErrorClass(err error) string {
	switch {
	case errors.Is(err, ErrLoginSessionExpired):
		return "expired"
	case errors.Is(err, ErrLoginSessionNotFound):
		return "not_found"
	default:
		return "oauth"
	}
}
