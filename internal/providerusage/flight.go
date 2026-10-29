package providerusage

import (
	"context"
	"time"
)

func (s *Manager) runFlight(ctx context.Context, profile, id string, state *targetState, f *flight) {
	defer s.wg.Done()
	defer func() { s.mu.Lock(); defer s.mu.Unlock(); s.finishFlightLocked(profile, state, f) }()
	timeout := 30 * time.Second
	if id == codexProviderID || id == claudeProviderID {
		timeout = 60 * time.Second
	}
	defer f.cancel()
	deadline := s.options.Clock.AfterFunc(timeout, f.cancel)
	defer deadline.Stop()
	intent := f.intent
	for ctx.Err() == nil {
		outcome, retry := s.collectOnce(ctx, profile, id, state, f, intent)
		if retry {
			continue
		}
		s.mu.Lock()
		follow := intent == IntentBackground && f.explicit && explicitRecovery(id, outcome)
		if follow {
			intent = IntentExplicit
			f.intent = IntentExplicit
		}
		if !follow {
			s.finishFlightLocked(profile, state, f)
		}
		s.mu.Unlock()
		if !follow {
			return
		}
	}
}

func explicitRecovery(provider string, e *UsageError) bool {
	if e == nil {
		return false
	}
	switch e.Code {
	case ErrorCodeAuthMissing:
		return provider == claudeProviderID
	case ErrorCodeKeychainConsentRequired,
		ErrorCodeKeychainDenied,
		ErrorCodeKeychainReadTimeout,
		ErrorCodeKeychainTokenExpired,
		ErrorCodeAuthExpired,
		ErrorCodeProviderRateLimited:
		return true
	default:
		return false
	}
}

func (s *Manager) collectOnce(
	ctx context.Context,
	profile, id string,
	state *targetState,
	f *flight,
	intent Intent,
) (*UsageError, bool) {
	s.mu.Lock()
	if !s.currentLocked(profile, id, state) {
		s.mu.Unlock()
		return nil, false
	}
	target := state.target
	epoch := state.epoch
	collector := s.options.Collectors[id]
	s.mu.Unlock()
	started := s.options.Clock.Now()
	identity, err := collector.Identity(ctx, target, intent)
	s.mu.Lock()
	if !s.currentLocked(profile, id, state) {
		s.mu.Unlock()
		return nil, false
	}
	if state.target.RouteFingerprint != target.RouteFingerprint || state.epoch != epoch {
		s.mu.Unlock()
		return nil, true
	}
	allowed, skippedOutcome := s.admitLocked(profile, id, state, f, intent, identity, err, started)
	s.mu.Unlock()
	if !allowed {
		return skippedOutcome, false
	}
	snapshot := Snapshot{}
	if err == nil {
		snapshot, err = collector.Collect(ctx, target, intent, identity)
	}
	var outcome *UsageError
	if err != nil {
		outcome = AsUsageError(err, target.ProviderName)
	}
	if ctx.Err() != nil && outcome == nil {
		outcome = NewUsageError(ErrorCodeProviderUnavailable, target.ProviderName)
	}
	if err, recollect := s.resolvePublication(ctx, profile, id, state, epoch, identity, intent, started); err != nil {
		return err, recollect
	}
	// A completed timeout still needs a bounded ownership check before publishing its error.
	fenceCtx, fenceCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer fenceCancel()
	currentTarget, valid := s.fenceTarget(profile, id, state, epoch)
	if !valid {
		return outcome, true
	}

	if identity != (CredentialIdentity{}) {
		mismatch, validationErr := validatePublication(fenceCtx, collector, currentTarget, identity)
		if mismatch {
			s.discardOwnership(profile, id, state)
			return outcome, true
		}
		if validationErr != nil && outcome == nil {
			outcome = AsUsageError(validationErr, target.ProviderName)
		}
	}

	s.mu.Lock()
	if !s.currentLocked(profile, id, state) {
		s.mu.Unlock()
		return outcome, false
	}
	if state.target.RouteFingerprint != target.RouteFingerprint || state.epoch != epoch {
		s.mu.Unlock()
		return outcome, true
	}
	s.publishLocked(profile, id, state, target, identity, snapshot, outcome)
	status := state.snapshot.Status
	source := snapshotSource(state.snapshot)
	s.mu.Unlock()
	s.observeCollection(profile, id, source, status, intent, started, outcome)

	return outcome, false
}

func (s *Manager) fenceTarget(profile, id string, state *targetState, epoch uint64) (Target, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.currentLocked(profile, id, state) || state.epoch != epoch {
		return Target{}, false
	}
	return state.target, true
}

func (s *Manager) currentLocked(profile, id string, state *targetState) bool {
	return s.enabled && !s.stopped && s.profiles[profile] != nil && s.profiles[profile].targets[id] == state &&
		state.target.Status == ""
}

func (s *Manager) finishFlightLocked(profile string, state *targetState, f *flight) {
	if state.flight != f {
		return
	}
	state.flight = nil
	if p := s.profiles[profile]; s.enabled && !s.stopped && p != nil {
		s.changedLocked(p)
	}
	close(f.done)
}

func validatePublication(
	ctx context.Context,
	collector Collector,
	target Target,
	identity CredentialIdentity,
) (bool, error) {
	if validator, ok := collector.(PublicationValidator); ok {
		err := validator.ValidateIdentity(ctx, target, identity)
		return err != nil, err
	}
	current, err := collector.Identity(ctx, target, IntentBackground)
	return !identity.Same(current), err
}

func (s *Manager) discardOwnership(profile, id string, state *targetState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.currentLocked(profile, id, state) {
		return
	}
	state.snapshot = initialSnapshot(state.target, s.options.Collectors[id])
	state.hasIdentity = false
	state.lastSuccess = time.Time{}
	state.cooldown = nil
	s.changedLocked(s.profileLocked(profile))
}
