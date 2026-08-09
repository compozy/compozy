package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	defaultProviderHealthInterval   = 30 * time.Second
	defaultProviderHealthTimeout    = 5 * time.Second
	defaultProviderTeardownTimeout  = 5 * time.Second
	defaultProviderEstablishTimeout = time.Minute
)

// ProviderDegradationHandler removes one generation's advertisement and records a redacted failure.
type ProviderDegradationHandler func(context.Context, ProviderActivation, error) error

// ProviderDegradationSource publishes supervised provider outages to the reconciler.
type ProviderDegradationSource interface {
	SetProviderDegradationHandler(ProviderDegradationHandler)
}

type supervisedProvider struct {
	activation ProviderActivation
	source     ConnectivitySource
	reachable  Reachability
	bound      netip.AddrPort
	path       string
	nonce      string
	cleanup    func()
	verified   bool
	cancel     context.CancelFunc
	done       chan struct{}
}

// ProviderSupervisor adapts extension connectivity services to the reconciler effect boundary.
type ProviderSupervisor struct {
	resolver       ConnectivitySourceResolver
	trust          ProviderTrustResolver
	identities     ProviderIdentityStore
	challenges     *ChallengeRegistry
	verifier       EndpointVerification
	now            func() time.Time
	healthEvery    time.Duration
	healthTimeout  time.Duration
	establishAfter time.Duration
	teardownAfter  time.Duration
	logger         *slog.Logger
	audit          ProviderAuditSink

	mu        sync.Mutex
	sessions  map[Tier]*supervisedProvider
	onDegrade ProviderDegradationHandler
}

// ProviderSupervisorOption customizes bounded lifecycle timing.
type ProviderSupervisorOption func(*ProviderSupervisor)

// WithProviderEstablishTimeout installs the bound for one provider activation.
func WithProviderEstablishTimeout(timeout time.Duration) ProviderSupervisorOption {
	return func(supervisor *ProviderSupervisor) {
		if timeout > 0 {
			supervisor.establishAfter = timeout
		}
	}
}

// WithProviderSupervisorClock installs a deterministic clock.
func WithProviderSupervisorClock(now func() time.Time) ProviderSupervisorOption {
	return func(supervisor *ProviderSupervisor) {
		if now != nil {
			supervisor.now = now
		}
	}
}

// WithProviderSupervisorIntervals installs health and teardown bounds.
func WithProviderSupervisorIntervals(
	healthEvery time.Duration,
	healthTimeout time.Duration,
	teardownTimeout time.Duration,
) ProviderSupervisorOption {
	return func(supervisor *ProviderSupervisor) {
		if healthEvery > 0 {
			supervisor.healthEvery = healthEvery
		}
		if healthTimeout > 0 {
			supervisor.healthTimeout = healthTimeout
		}
		if teardownTimeout > 0 {
			supervisor.teardownAfter = teardownTimeout
		}
	}
}

// WithProviderSupervisorLogger installs the structured lifecycle logger.
func WithProviderSupervisorLogger(logger *slog.Logger) ProviderSupervisorOption {
	return func(supervisor *ProviderSupervisor) {
		if logger != nil {
			supervisor.logger = logger
		}
	}
}

// WithProviderAuditSink installs the durable provider contract audit boundary.
func WithProviderAuditSink(audit ProviderAuditSink) ProviderSupervisorOption {
	return func(supervisor *ProviderSupervisor) {
		if audit != nil {
			supervisor.audit = audit
		}
	}
}

// NewProviderSupervisor constructs the trust-first provider lifecycle adapter.
func NewProviderSupervisor(
	resolver ConnectivitySourceResolver,
	trust ProviderTrustResolver,
	identities ProviderIdentityStore,
	challenges *ChallengeRegistry,
	verifier EndpointVerification,
	options ...ProviderSupervisorOption,
) (*ProviderSupervisor, error) {
	switch {
	case resolver == nil:
		return nil, errors.New("gateway: connectivity source resolver is required")
	case trust == nil:
		return nil, errors.New("gateway: provider trust resolver is required")
	case identities == nil:
		return nil, errors.New("gateway: provider identity store is required")
	case challenges == nil:
		return nil, errors.New("gateway: challenge registry is required")
	case verifier == nil || verifier.Timeout() <= 0:
		return nil, errors.New("gateway: endpoint verifier is required")
	}
	supervisor := &ProviderSupervisor{
		resolver: resolver, trust: trust, identities: identities,
		challenges: challenges, verifier: verifier,
		now:         func() time.Time { return time.Now().UTC() },
		healthEvery: defaultProviderHealthInterval, healthTimeout: defaultProviderHealthTimeout,
		establishAfter: defaultProviderEstablishTimeout, teardownAfter: defaultProviderTeardownTimeout,
		logger: slog.Default(), audit: noopProviderAuditSink{},
		sessions: make(map[Tier]*supervisedProvider),
	}
	for _, option := range options {
		if option != nil {
			option(supervisor)
		}
	}
	return supervisor, nil
}

// SetProviderDegradationHandler wires the supervisor to runtime withdrawal.
func (s *ProviderSupervisor) SetProviderDegradationHandler(handler ProviderDegradationHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDegrade = handler
}

// RecordProviderStateChanged forwards the reconciler-owned lifecycle transition to the audit sink.
func (s *ProviderSupervisor) RecordProviderStateChanged(
	ctx context.Context,
	activation ProviderActivation,
	state ProviderObservedState,
	cause string,
) error {
	if err := s.audit.RecordProviderStateChanged(ctx, activation, state, cause); err != nil {
		return fmt.Errorf("gateway: record provider state change: %w", err)
	}
	return nil
}

// Establish authorizes the exact live artifact before resolving or calling its process.
func (s *ProviderSupervisor) Establish(
	ctx context.Context,
	activation ProviderActivation,
	bound netip.AddrPort,
) (result Reachability, resultErr error) {
	if s == nil {
		return Reachability{}, errors.New("gateway: provider supervisor is required")
	}
	if err := s.PreflightProvider(ctx, activation); err != nil {
		return Reachability{}, err
	}
	if reachability, ok := s.pendingReachability(activation, bound); ok {
		return reachability, nil
	}
	source, err := s.resolver.ResolveConnectivitySource(ctx, activation.ProviderName)
	if err != nil {
		return Reachability{}, fmt.Errorf("gateway: resolve connectivity provider: %w", err)
	}
	if err := s.replacePriorSession(ctx, activation); err != nil {
		return Reachability{}, err
	}
	path, nonce, cleanup, err := s.challenges.Begin(activation.Tier)
	if err != nil {
		return Reachability{}, fmt.Errorf("gateway: begin provider verification challenge: %w", err)
	}
	ownedChallenge := true
	defer func() {
		if ownedChallenge {
			cleanup()
		}
	}()
	deadline := s.now().Add(s.establishAfter)
	callCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	reachability, err := source.Establish(callCtx, EstablishRequest{
		Tier: activation.Tier, ForwardTarget: bound, ChallengePath: path, Deadline: deadline,
	})
	if err != nil {
		if errors.Is(err, ErrEndpointUnverified) || errors.Is(err, ErrExposureRefused) {
			kind := ProviderRefusalAuthority
			if errors.Is(err, ErrEndpointUnverified) {
				kind = ProviderRefusalEndpoint
			}
			err = errors.Join(err, s.audit.RecordProviderRefusal(ctx, activation, kind, err))
		}
		return Reachability{}, errors.Join(
			fmt.Errorf("gateway: establish connectivity provider: %w", err),
			s.teardownSource(ctx, source, activation.Tier),
		)
	}
	if err := validateProviderReachability(activation.Tier, reachability); err != nil {
		return Reachability{}, errors.Join(
			err,
			s.audit.RecordProviderRefusal(ctx, activation, ProviderRefusalEndpoint, err),
			s.teardownSource(ctx, source, activation.Tier),
		)
	}
	stableReachability := cloneReachability(reachability)
	session := &supervisedProvider{
		activation: activation, source: source, reachable: stableReachability,
		bound: bound, path: path, nonce: nonce, cleanup: cleanup,
	}
	s.mu.Lock()
	s.sessions[activation.Tier] = session
	s.mu.Unlock()
	ownedChallenge = false
	return cloneReachability(stableReachability), nil
}

// PreflightProvider re-derives authority before the reconciler withdraws prior reachability.
func (s *ProviderSupervisor) PreflightProvider(ctx context.Context, activation ProviderActivation) error {
	if s == nil {
		return errors.New("gateway: provider supervisor is required")
	}
	if err := s.authorize(ctx, activation); err != nil {
		return errors.Join(err, s.audit.RecordProviderRefusal(ctx, activation, ProviderRefusalAuthority, err))
	}
	if err := s.requireSelectedProvider(activation); err != nil {
		return errors.Join(err, s.audit.RecordProviderRefusal(ctx, activation, ProviderRefusalAuthority, err))
	}
	return nil
}

// Verify proves every endpoint reaches the active challenge on the assigned tier.
func (s *ProviderSupervisor) Verify(ctx context.Context, reachability Reachability) error {
	s.mu.Lock()
	session := s.sessions[reachability.Tier]
	s.mu.Unlock()
	if session == nil {
		return fmt.Errorf("%w: no active provider attempt", ErrEndpointUnverified)
	}
	verifyCtx, cancel := context.WithTimeout(ctx, s.verifier.Timeout())
	defer cancel()
	if session.reachable.Tier != reachability.Tier ||
		session.reachable.Health != reachability.Health ||
		!slices.Equal(session.reachable.Endpoints, reachability.Endpoints) {
		return fmt.Errorf("%w: reachability changed before verification", ErrEndpointUnverified)
	}
	for _, endpoint := range reachability.Endpoints {
		if err := s.verifier.Verify(verifyCtx, reachability.Tier, endpoint, session.path, session.nonce); err != nil {
			return errors.Join(
				err,
				s.audit.RecordProviderRefusal(ctx, session.activation, ProviderRefusalEndpoint, err),
			)
		}
	}
	for _, endpoint := range reachability.Endpoints {
		if err := s.audit.RecordEndpointVerified(ctx, session.activation, endpoint); err != nil {
			return fmt.Errorf("gateway: record verified provider endpoint: %w", err)
		}
	}
	session.cleanup()
	s.mu.Lock()
	if current := s.sessions[reachability.Tier]; current == session {
		current.verified = true
	}
	s.mu.Unlock()
	return nil
}

func (s *ProviderSupervisor) pendingReachability(
	activation ProviderActivation,
	bound netip.AddrPort,
) (Reachability, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[activation.Tier]
	if session == nil || session.verified || session.bound != bound ||
		session.activation.ProviderName != activation.ProviderName ||
		session.activation.Generation != activation.Generation {
		return Reachability{}, false
	}
	session.activation = activation
	return cloneReachability(session.reachable), true
}

// Advertise commits verified reachability and starts bounded health supervision.
func (s *ProviderSupervisor) Advertise(
	_ context.Context,
	reachability Reachability,
	_ []Surface,
) error {
	s.mu.Lock()
	session := s.sessions[reachability.Tier]
	if session == nil || !session.verified ||
		session.reachable.Tier != reachability.Tier ||
		session.reachable.Health != reachability.Health ||
		!slices.Equal(session.reachable.Endpoints, reachability.Endpoints) {
		s.mu.Unlock()
		return fmt.Errorf("%w: provider reachability is not verified", ErrEndpointUnverified)
	}
	if session.cancel != nil {
		s.mu.Unlock()
		return nil
	}
	monitorCtx, cancel := context.WithCancel(context.Background())
	session.cancel = cancel
	session.done = make(chan struct{})
	s.mu.Unlock()
	go s.monitor(monitorCtx, session)
	return nil
}

func cloneReachability(value Reachability) Reachability {
	value.Endpoints = slices.Clone(value.Endpoints)
	return value
}

// Withdraw stops accepting health state before teardown removes provider forwarding.
func (s *ProviderSupervisor) Withdraw(ctx context.Context, tier Tier) error {
	s.mu.Lock()
	session := s.sessions[tier]
	s.mu.Unlock()
	return s.stopMonitor(ctx, session)
}

// Teardown stops supervision and bounds the provider's cooperative cleanup.
func (s *ProviderSupervisor) Teardown(ctx context.Context, activation ProviderActivation) error {
	s.mu.Lock()
	session := s.sessions[activation.Tier]
	if session != nil {
		delete(s.sessions, activation.Tier)
	}
	s.mu.Unlock()
	if session == nil {
		return nil
	}
	session.cleanup()
	return errors.Join(s.stopMonitor(ctx, session), s.teardownSource(ctx, session.source, activation.Tier))
}

// Bind and Unbind are daemon-listener effects and deliberately unavailable on the provider adapter.
func (*ProviderSupervisor) Bind(context.Context, Tier) (netip.AddrPort, error) {
	return netip.AddrPort{}, errors.New("gateway: provider supervisor does not own tier listeners")
}

func (*ProviderSupervisor) Unbind(context.Context, Tier) error {
	return errors.New("gateway: provider supervisor does not own tier listeners")
}

func (s *ProviderSupervisor) authorize(ctx context.Context, activation ProviderActivation) error {
	fresh, err := s.trust.ResolveProviderTrust(ctx, activation.ProviderName)
	if err != nil {
		return fmt.Errorf("gateway: derive live provider trust: %w", err)
	}
	wantedScope := ProviderChannelScope(activation.Tier)
	if !slices.Contains(fresh.ChannelScopes, wantedScope) {
		return fmt.Errorf("%w: provider does not declare %s", ErrExposureRefused, wantedScope)
	}
	persisted, err := s.identities.ProviderIdentity(ctx, activation.ProviderName)
	if err != nil {
		return fmt.Errorf("gateway: load confirmed provider identity: %w", err)
	}
	if strings.TrimSpace(persisted.InstallSource) != strings.TrimSpace(fresh.InstallSource) {
		return ErrProviderTrustStale
	}
	if fresh.InstallSource == ProviderInstallSourceBundled {
		return nil
	}
	if fresh.ControlDigest == "" || persisted.DigestConfirmed == "" || fresh.ConfirmedDigest == "" {
		return ErrDigestConfirmationRequired
	}
	if persisted.DigestConfirmed != fresh.ControlDigest || fresh.ConfirmedDigest != fresh.ControlDigest {
		return ErrProviderTrustStale
	}
	return nil
}

func (s *ProviderSupervisor) requireSelectedProvider(activation ProviderActivation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prior := s.sessions[activation.Tier]
	if prior != nil && prior.activation.ProviderName != activation.ProviderName {
		return ErrTierProviderConflict
	}
	return nil
}

func (s *ProviderSupervisor) replacePriorSession(ctx context.Context, activation ProviderActivation) error {
	s.mu.Lock()
	prior := s.sessions[activation.Tier]
	if prior != nil && prior.activation.ProviderName != activation.ProviderName {
		s.mu.Unlock()
		return ErrTierProviderConflict
	}
	if prior != nil {
		delete(s.sessions, activation.Tier)
	}
	s.mu.Unlock()
	if prior == nil {
		return nil
	}
	prior.cleanup()
	if err := errors.Join(
		s.stopMonitor(ctx, prior),
		s.teardownSource(ctx, prior.source, activation.Tier),
	); err != nil {
		return fmt.Errorf("gateway: replace connectivity provider: %w", err)
	}
	return nil
}

func validateProviderReachability(tier Tier, reachability Reachability) error {
	if reachability.Tier != tier {
		return fmt.Errorf("%w: provider returned the wrong tier", ErrEndpointUnverified)
	}
	if reachability.Health != HealthHealthy {
		return fmt.Errorf("%w: provider did not report healthy", ErrProviderDegraded)
	}
	if len(reachability.Endpoints) == 0 {
		return fmt.Errorf("%w: provider returned no endpoints", ErrEndpointUnverified)
	}
	for _, endpoint := range reachability.Endpoints {
		if err := endpoint.Validate(); err != nil {
			return fmt.Errorf("%w: invalid provider endpoint", ErrEndpointUnverified)
		}
		if tier == TierPublic && endpoint.Stability != EndpointStable {
			return fmt.Errorf("%w: public provider endpoint must be stable", ErrEndpointUnverified)
		}
	}
	return nil
}

var (
	_ EffectDriver              = (*ProviderSupervisor)(nil)
	_ ProviderPreflight         = (*ProviderSupervisor)(nil)
	_ ProviderDegradationSource = (*ProviderSupervisor)(nil)
	_ ProviderStateReporter     = (*ProviderSupervisor)(nil)
)
