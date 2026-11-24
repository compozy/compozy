package account

import (
	"cmp"
	"context"
	"crypto"
	"errors"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/vault"
	"github.com/compozy/compozy/internal/version"
	"github.com/jonboulle/clockwork"
)

const (
	dailyRefresh      = 24 * time.Hour
	minRefreshBackoff = time.Minute
	maxRefreshBackoff = time.Hour
	accessTTL         = remoteproto.DeviceAccessTTL - 15*time.Second
)

type Option func(*Service)

func WithClock(clock clockwork.Clock) Option { return func(s *Service) { s.clock = clock } }
func WithKeyring(ring vault.Keyring) Option  { return func(s *Service) { s.ring = ring } }
func WithLogger(logger *slog.Logger) Option  { return func(s *Service) { s.logger = logger } }

type Service struct {
	store           Store
	cloud           Cloud
	clock           clockwork.Clock
	logger          *slog.Logger
	credentials     *credentials
	ring            vault.Keyring
	name, platform  string
	ctx             context.Context
	cancel          context.CancelFunc
	workers         sync.WaitGroup
	wakeRefresh     chan struct{}
	signInRequired  bool
	gate            gateState
	token           string
	linkedAccountID string
	linkedEmail     string

	// Take sessionMu before mu when rotating or committing a credential.
	sessionMu    sync.Mutex
	mu           sync.Mutex
	closed       bool
	installation Installation
	generation   *generation
	attempt      *loginAttempt
	progress     map[chan SignInEvent]struct{}
	session      deviceSession
	retryAt      time.Time
	retryErr     error
	backoff      time.Duration
}

type deviceSession struct {
	key             crypto.Signer
	access, refresh string
	accessExpires   time.Time
	unsaved         bool
}

func New(ctx context.Context, store Store, cloud Cloud, home string, opts ...Option) (*Service, error) {
	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		name = "CompozyOS"
	}
	s := &Service{
		store: store, cloud: cloud, clock: clockwork.NewRealClock(), logger: slog.Default(),
		name: name, platform: runtime.GOOS + "_" + runtime.GOARCH,
		wakeRefresh: make(chan struct{}, 1), progress: make(map[chan SignInEvent]struct{}),
		signInRequired: version.SignInRequired == "true",
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.credentials, err = newCredentials(home, s.ring); err != nil {
		return nil, err
	}
	installation, err := store.ReadInstallationAccount(ctx)
	if err != nil {
		return nil, err
	}
	if installation.State == SignedIn {
		// An in-memory event cursor from an earlier daemon cannot resume this ring.
		installation.AuthGeneration++
		if err := store.SaveInstallationAccount(ctx, installation.AuthGeneration-1, installation); err != nil {
			return nil, err
		}
	}
	s.ctx, s.cancel = context.WithCancel(context.WithoutCancel(ctx))
	s.installation = installation
	s.generation = newGeneration(s.ctx, installation.AuthGeneration)
	s.initializeGate(ctx)
	if err := s.restoreHostLink(ctx); err != nil {
		s.cancel()
		return nil, err
	}
	if err := s.startToken(ctx); err != nil {
		s.cancel()
		return nil, err
	}
	s.startGate(ctx)
	s.workers.Go(s.refreshLoop)
	return s, nil
}

func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	for sub := range s.progress {
		s.closeProgressLocked(sub)
	}
	s.mu.Unlock()
	s.workers.Wait()
}

func (s *Service) Status() Installation {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.installation
	result.InstallationLinked = s.linkedAccountID != ""
	result.Gate, result.Grace = s.gateStatusLocked()
	if s.attempt != nil && result.State != SignedIn {
		result.State = SigningIn
	}
	return withVisibleNotices(result, s.clock.Now())
}

// SessionLifetime ends when the current identity stops owning remote connections.
func (s *Service) SessionLifetime() (context.Context, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.installation.State != SignedIn {
		return nil, "", ErrSignInRequired
	}
	return s.generation.ctx, s.installation.AccountID, nil
}

// IdentityLifetime changes on every authentication transition, including sign-in.
func (s *Service) IdentityLifetime() (context.Context, Installation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generation.ctx, s.installation
}

func (s *Service) transitionLocked(ctx context.Context, next Installation, session deviceSession) error {
	previous := s.installation.AuthGeneration
	next.AuthGeneration = previous + 1
	if err := s.store.SaveInstallationAccount(ctx, previous, next); err != nil {
		return err
	}
	if s.installation.State == SignedIn {
		s.generation.evictRemote()
	}
	s.generation.cancel()
	s.generation = newGeneration(s.ctx, next.AuthGeneration)
	s.installation = next
	s.updateGateLocked()
	s.wakeGateLocked()
	s.session = session
	s.retryAt, s.backoff = time.Time{}, 0
	s.retryErr = nil
	s.signalRefresh()
	return nil
}

func (s *Service) signalRefresh() {
	select {
	case s.wakeRefresh <- struct{}{}:
	default:
	}
}

func (s *Service) signedOutLocked(ctx context.Context, state State, reason string) error {
	previous := s.installation
	next := previous
	next.State, next.SignedOutReason = state, reason
	next.Source, next.AccountID, next.TokenName = "", "", ""
	next.Profile, next.LastValidatedAt = remoteproto.AccountSummary{}, time.Time{}
	if err := s.transitionLocked(ctx, next, deviceSession{}); err != nil {
		return err
	}
	s.sendProgressLocked(SignInEvent{State: string(state), SignedOutReason: reason})
	if previous.Source == "token" {
		return s.credentials.remove(tokenEntry, previous.InstallationID)
	}
	if previous.Source == "session" && previous.AccountID != "" {
		if err := s.credentials.remove(refreshEntry, previous.AccountID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Logout(ctx context.Context) error {
	if err := s.CancelLogin(ctx, ""); err != nil {
		return err
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.mu.Lock()
	current, session, linked := s.installation, s.session, s.linkedAccountID != ""
	s.mu.Unlock()
	if linked {
		return ErrLogoutRequiresUnlink
	}
	if current.State == SignedOut {
		s.mu.Lock()
		s.token = ""
		s.signalRefresh()
		s.mu.Unlock()
		return nil
	}
	if current.State == SignedIn && current.Source == "session" {
		var err error
		session, err = s.loadSession(current, session)
		if err != nil {
			return err
		}
		if err := s.cloud.Logout(s.ctx, session.refresh, session.key); err != nil && reauthReason(err) == "" {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.signedOutLocked(s.ctx, SignedOut, "signed_out"); err != nil {
		return err
	}
	s.token = ""
	return nil
}

func (s *Service) loadSession(current Installation, session deviceSession) (deviceSession, error) {
	if session.key != nil {
		return session, nil
	}
	key, err := s.credentials.deviceKey(current.AccountID, false)
	if err != nil {
		return deviceSession{}, err
	}
	refresh, err := s.credentials.read(refreshEntry, current.AccountID)
	if err != nil || refresh == "" {
		return deviceSession{}, errKeyring
	}
	return deviceSession{key: key, refresh: refresh}, nil
}

func reauthReason(err error) string {
	failure, ok := errors.AsType[*cloudclient.Failure](err)
	if !ok || failure.Code != "reauth_required" {
		return ""
	}
	switch failure.Reason {
	case "device_revoked", "account_deleted", "account_suspended", "session_invalid":
		return failure.Reason
	default:
		return ""
	}
}

func (s *Service) postponeRefreshLocked(err error) {
	s.backoff = min(maxRefreshBackoff, max(minRefreshBackoff, 2*s.backoff))
	now := s.clock.Now()
	s.retryAt = now.Add(max(s.backoff, cloudclient.RetryDelay(now, err)))
	s.retryErr = err
	s.signalRefresh()
}

func (s *Service) refreshLoop() {
	for s.ctx.Err() == nil {
		s.mu.Lock()
		current, generation := s.installation, s.generation
		var next time.Time
		token := s.token != "" && (current.Source == "token" || current.State != SignedIn)
		if token || (current.State == SignedIn && current.Source == "session") {
			next = cmp.Or(s.retryAt, current.LastValidatedAt.Add(dailyRefresh))
		}
		s.mu.Unlock()
		if next.IsZero() {
			select {
			case <-s.ctx.Done():
				return
			case <-s.wakeRefresh:
				continue
			}
		}
		timer := s.clock.NewTimer(max(0, s.clock.Until(next)))
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-s.wakeRefresh:
			timer.Stop()
		case <-timer.Chan():
			if token {
				s.exchangeToken(s.ctx)
				continue
			}
			if _, err := s.refresh(
				generation,
				false,
			); err != nil && s.ctx.Err() == nil &&
				!errors.Is(err, ErrSignInRequired) {
				s.logger.Debug("account refresh deferred", "code", cloudclient.FailureCode(err))
			}
		}
	}
}

func (s *Service) refresh(g *generation, forAccess bool) (cloudclient.Credentials, error) {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.mu.Lock()
	current, session, retryAt, retryErr := s.installation, s.session, s.retryAt, s.retryErr
	valid := !s.closed && s.generation == g && current.State == SignedIn
	s.mu.Unlock()
	if !valid {
		return cloudclient.Credentials{}, ErrSignInRequired
	}
	switch current.Source {
	case "token":
		return cloudclient.Credentials{}, ErrTokenLocalOnly
	case "link":
		return cloudclient.Credentials{}, ErrSignInRequired
	}
	now := s.clock.Now()
	if forAccess && session.usable(now) {
		return cloudclient.Credentials{Key: session.key, AccessToken: session.access}, nil
	}
	if now.Before(retryAt) ||
		(!forAccess && now.Before(current.LastValidatedAt.Add(dailyRefresh)) && !session.unsaved) {
		if !forAccess {
			return cloudclient.Credentials{}, nil
		}
		return cloudclient.Credentials{}, retryErr
	}
	session, err := s.rotateSession(current, session, now)
	s.mu.Lock()
	defer s.mu.Unlock()
	if reason := reauthReason(err); reason != "" {
		if stateErr := s.rejectDeviceLocked(reason); stateErr != nil {
			s.postponeRefreshLocked(stateErr)
			return cloudclient.Credentials{}, stateErr
		}
		return cloudclient.Credentials{}, err
	}
	if err == nil {
		err = s.saveRefreshLocked(session, now)
	}
	if err != nil {
		s.postponeRefreshLocked(err)
		return cloudclient.Credentials{}, err
	}
	return cloudclient.Credentials{Key: session.key, AccessToken: session.access}, nil
}

func (s deviceSession) usable(now time.Time) bool {
	return !s.unsaved && s.access != "" && now.Before(s.accessExpires)
}

func (s *Service) rotateSession(current Installation, session deviceSession, now time.Time) (deviceSession, error) {
	session, err := s.loadSession(current, session)
	if err != nil || session.unsaved {
		return session, err
	}
	tokens, err := s.cloud.Refresh(s.ctx, session.refresh, session.key)
	if err != nil {
		return session, err
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		return session, errKeyring
	}
	session.access, session.refresh, session.accessExpires, session.unsaved = tokens.AccessToken, tokens.RefreshToken,
		now.Add(accessTTL), true
	return session, nil
}

func (s *Service) saveRefreshLocked(session deviceSession, now time.Time) error {
	s.session = session
	next := s.installation
	if err := s.credentials.write(refreshEntry, next.AccountID, session.refresh); err != nil {
		return err
	}
	next.LastValidatedAt = now
	if err := s.store.SaveInstallationAccount(s.ctx, next.AuthGeneration, next); err != nil {
		return err
	}
	s.session.unsaved = false
	s.installation = next
	s.retryAt, s.backoff = time.Time{}, 0
	s.retryErr = nil
	s.signalRefresh()
	return nil
}
