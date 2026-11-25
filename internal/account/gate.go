package account

import (
	"context"
	"encoding/json/v2"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	GateOff      = "off"
	GateEnforced = "enforced"
	GateGrace    = "grace"

	policyInterval = 15 * time.Minute
	offlineGrace   = 7 * 24 * time.Hour
)

type gateCloud interface {
	GatePolicy(context.Context) (string, error)
	JWKS(context.Context) (remoteproto.JWKS, error)
}

type gateState struct {
	policy   remoteproto.GatePolicy
	nextPoll time.Time
	wake     chan struct{}
	changed  chan struct{}
	lifetime context.Context
	cancel   context.CancelCauseFunc
	blocked  bool
}

// WithSignInRequired lets the isolated remote integration harness exercise the
// release policy without overriding the production Cloud origin.
func WithSignInRequired(required bool) Option {
	return func(s *Service) { s.signInRequired = required }
}

// AdmitNewWork is shared by local admission points, including internal callers.
// A nil service is used by runtimes without a registered installation account.
func (s *Service) AdmitNewWork() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateGateLocked()
	if s.gate.blocked {
		return ErrSignInRequired
	}
	return nil
}

// OperatorLifetime ends when the local sign-in gate closes. It owns transport
// subscriptions only; runtimes and claimed work have separate lifetimes.
func (s *Service) OperatorLifetime() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateGateLocked()
	return s.gate.lifetime
}

func (s *Service) WaitForWorkAdmission(ctx context.Context) error {
	for {
		s.mu.Lock()
		s.updateGateLocked()
		blocked, changed := s.gate.blocked, s.gate.changed
		s.mu.Unlock()
		if !blocked {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-changed:
		}
	}
}

func (s *Service) initializeGate(ctx context.Context) {
	s.gate.wake = make(chan struct{}, 1)
	s.gate.nextPoll = s.clock.Now()
	s.gate.changed = make(chan struct{})
	s.gate.lifetime, s.gate.cancel = context.WithCancelCause(s.ctx)
	if !s.signInRequired {
		return
	}
	current := s.installation
	var keys remoteproto.JWKS
	if current.GatePolicyJWS != "" && json.Unmarshal([]byte(current.GatePolicyKeys), &keys) == nil {
		s.gate.policy = verifiedGatePolicy(ctx, keys, current.GatePolicyJWS, s.clock.Now())
	}
	s.updateGateLocked()
}

func (s *Service) startGate(ctx context.Context) {
	if !s.signInRequired {
		return
	}
	if cloud, ok := s.cloud.(gateCloud); ok {
		if s.installation.State != SignedIn {
			s.fetchGatePolicy(ctx, cloud)
		}
		s.workers.Go(func() { s.runGate(cloud) })
	}
}

func (s *Service) gateStatusLocked() (string, *Grace) {
	if !s.signInRequired {
		return GateOff, nil
	}
	current, now := s.installation, s.clock.Now()
	if current.State == SignedOut {
		if s.gate.policy.GraceAt(now) {
			return GateGrace, &Grace{Source: "policy", Until: *s.gate.policy.Until, Message: s.gate.policy.Message}
		}
		if current.PreAccountInstall && current.AuthGeneration == 0 && current.GatePolicyFetchedAt.IsZero() &&
			!current.OfflineGraceStartedAt.IsZero() {
			until := current.OfflineGraceStartedAt.Add(offlineGrace)
			if now.Before(until) {
				return GateGrace, &Grace{Source: "offline", Until: until}
			}
		}
	}
	return GateEnforced, nil
}

func (s *Service) updateGateLocked() {
	mode, _ := s.gateStatusLocked()
	blocked := mode == GateEnforced && s.installation.State != SignedIn
	if blocked != s.gate.blocked {
		s.gate.blocked = blocked
		if blocked {
			s.gate.cancel(ErrSignInRequired)
		} else {
			s.gate.lifetime, s.gate.cancel = context.WithCancelCause(s.ctx)
		}
		close(s.gate.changed)
		s.gate.changed = make(chan struct{})
		s.sendProgressLocked(SignInEvent{State: string(s.installation.State),
			SignedOutReason: s.installation.SignedOutReason})
	}
}

func (s *Service) wakeGateLocked() {
	select {
	case s.gate.wake <- struct{}{}:
	default:
	}
}

func (s *Service) runGate(cloud gateCloud) {
	for s.ctx.Err() == nil {
		s.mu.Lock()
		s.updateGateLocked()
		var deadline time.Time
		if s.installation.State != SignedIn {
			deadline = s.gate.nextPoll
			if _, grace := s.gateStatusLocked(); grace != nil && grace.Until.Before(deadline) {
				deadline = grace.Until
			}
		}
		s.mu.Unlock()
		if deadline.IsZero() {
			select {
			case <-s.ctx.Done():
				return
			case <-s.gate.wake:
				continue
			}
		}
		timer := s.clock.NewTimer(max(0, s.clock.Until(deadline)))
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-s.gate.wake:
			timer.Stop()
		case <-timer.Chan():
			s.mu.Lock()
			s.updateGateLocked()
			due := s.installation.State != SignedIn && !s.clock.Now().Before(s.gate.nextPoll)
			s.mu.Unlock()
			if due {
				s.fetchGatePolicy(s.ctx, cloud)
			}
		}
	}
}

func (s *Service) fetchGatePolicy(ctx context.Context, cloud gateCloud) {
	token, err := cloud.GatePolicy(ctx)
	failure, typed := errors.AsType[*cloudclient.Failure](err)
	unavailable := err != nil && (!typed || failure.Temporary())
	var policy remoteproto.GatePolicy
	var encodedKeys []byte
	if err == nil {
		var keys remoteproto.JWKS
		keys, err = cloud.JWKS(ctx)
		if err == nil {
			policy = verifiedGatePolicy(ctx, keys, token, s.clock.Now())
			if policy.Mode != "" {
				encodedKeys, err = json.Marshal(keys)
			}
		}
		unavailable = err != nil
	}
	if len(encodedKeys) == 0 {
		token = ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	s.gate.nextPoll = now.Add(max(policyInterval, cloudclient.RetryDelay(now, err)))
	if ctx.Err() != nil {
		return
	}
	next := s.installation
	if unavailable {
		if next.PreAccountInstall && next.AuthGeneration == 0 && next.GatePolicyFetchedAt.IsZero() &&
			next.OfflineGraceStartedAt.IsZero() {
			next.OfflineGraceStartedAt = now
		}
	} else {
		next.GatePolicyJWS, next.GatePolicyKeys, next.GatePolicyFetchedAt = token, string(encodedKeys), now
		// A reachable but invalid response revokes any previously cached grace.
		s.gate.policy = policy
	}
	if next.GatePolicyJWS != s.installation.GatePolicyJWS ||
		next.GatePolicyKeys != s.installation.GatePolicyKeys ||
		next.GatePolicyFetchedAt != s.installation.GatePolicyFetchedAt ||
		next.OfflineGraceStartedAt != s.installation.OfflineGraceStartedAt {
		if err := s.store.SaveInstallationAccount(ctx, next.AuthGeneration, next); err != nil {
			s.logger.WarnContext(ctx, "Could not save the sign-in gate policy", "error", err)
		} else {
			s.installation = next
		}
	}
	s.updateGateLocked()
}

func verifiedGatePolicy(
	ctx context.Context,
	keys remoteproto.JWKS,
	token string,
	now time.Time,
) remoteproto.GatePolicy {
	verifier, err := remoteproto.NewJWSVerifier(remoteproto.CloudIssuer, keys, nil)
	if err != nil {
		return remoteproto.GatePolicy{}
	}
	policy, err := verifier.VerifyGatePolicy(ctx, token, now)
	if err != nil {
		return remoteproto.GatePolicy{}
	}
	return policy
}
