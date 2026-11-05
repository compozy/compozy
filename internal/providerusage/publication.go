package providerusage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// admitLocked decides whether a flight may collect. It returns the skip outcome when not, and
// whether that outcome was published here (an identity cooldown this target had not shown yet).
func (s *Manager) admitLocked(
	profile, id string,
	state *targetState,
	f *flight,
	intent Intent,
	identity CredentialIdentity,
	err error,
	started time.Time,
) (bool, *UsageError, bool) {
	target := state.target
	collector := s.options.Collectors[target.Collector()]
	known := identity != (CredentialIdentity{})
	if state.hasIdentity &&
		((known && !state.identity.Same(identity)) ||
			(!known && err != nil && AsUsageError(err, id).Code == ErrorCodeAuthMissing)) {
		state.snapshot = initialSnapshot(target, collector)
		state.lastSuccess = time.Time{}
		state.hasIdentity = false
		state.cooldownUntil = time.Time{}
		s.changedLocked(s.profileLocked(profile))
		s.options.Logger.Info("provider_usage.ownership_changed", "provider", id)
	}
	if err == nil && !state.lastSuccess.IsZero() && started.Sub(state.lastSuccess) < f.maxAge {
		return false, nil, false
	}
	if err != nil {
		return true, nil, false
	}
	c := s.activeCooldownLocked(newAccountKey(profile, target, identity), started)
	if c == nil {
		return true, nil, false
	}
	// The bypass itself is spent atomically with the request in joinAccountLocked.
	if intent == IntentExplicit && !c.bypassed {
		return true, nil, false
	}
	outcome := RateLimited(target.ProviderName, c.until)
	if state.cooldownUntil.Equal(c.until) {
		return false, outcome, false
	}
	// Another target of this identity earned the cooldown; show it without a request.
	s.publishLocked(profile, id, state, target, identity, Snapshot{}, outcome)
	return false, outcome, true
}

func (s *Manager) publishLocked(
	profile, id string,
	state *targetState,
	target Target,
	identity CredentialIdentity,
	snapshot Snapshot,
	outcome *UsageError,
) {
	now := s.options.Clock.Now()
	if outcome == nil {
		snapshot.ProviderID = id
		snapshot.ProviderName = target.ProviderName
		snapshot.Status = StatusOK
		snapshot.Error = nil
		if snapshot.MeasuredAt == nil {
			snapshot.MeasuredAt = &now
		}
		snapshot.CheckedAt = &now
		next := now.Add(s.options.RefreshInterval)
		snapshot.NextRefreshAt = &next
		state.snapshot = cloneSnapshot(snapshot)
		state.identity = identity
		state.hasIdentity = true
		state.lastSuccess = now
		state.cooldownUntil = time.Time{}
	} else {
		if state.hasIdentity && state.identity.Same(identity) && state.snapshot.MeasuredAt != nil {
			state.snapshot.Status = StatusStale
		} else {
			state.snapshot = cloneSnapshot(snapshot)
			state.lastSuccess = time.Time{}
			state.snapshot.ProviderID = id
			state.snapshot.ProviderName = target.ProviderName
			state.snapshot.Status = outcome.Status()
			state.snapshot.MeasuredAt = nil
			state.snapshot.NextRefreshAt = nil
			state.snapshot.Refreshing = false
			state.identity = identity
			state.hasIdentity = identity != (CredentialIdentity{})
		}
		state.snapshot.Error = clonePointer(outcome)
		state.snapshot.CheckedAt = &now
		if outcome.Code == ErrorCodeProviderRateLimited {
			until := now.Add(rateLimitFloor)
			if outcome.RetryAfter != nil {
				until = *outcome.RetryAfter
			}
			state.cooldownUntil = until
			state.snapshot.NextRefreshAt = &until
		}
	}
	s.changedLocked(s.profileLocked(profile))
}

func (s *Manager) observeCollection(
	profile, id, source string,
	status Status,
	intent Intent,
	started time.Time,
	outcome *UsageError,
) {
	hash := sha256.Sum256([]byte(profile))
	code := ErrorCode("")
	if outcome != nil {
		code = outcome.Code
	}
	s.options.Logger.Info(
		"provider_usage.collect",
		"provider",
		id,
		"profile_hash",
		hex.EncodeToString(hash[:8]),
		"source",
		source,
		"status",
		status,
		"intent",
		intent,
		"duration_ms",
		s.options.Clock.Now().Sub(started).Milliseconds(),
		"error_code",
		code,
	)
	if s.options.Metrics != nil {
		s.options.Metrics.Collect(id, status, s.options.Clock.Now().Sub(started))
	}
}

// resolvePublication retries reconciliation of the captured result, never collection.
func (s *Manager) resolvePublication(
	ctx context.Context,
	profile, id string,
	state *targetState,
	epoch uint64,
	identity CredentialIdentity,
	intent Intent,
	started time.Time,
) (*UsageError, bool) {
	outcome := NewUsageError(ErrorCodeProviderUnavailable, id)
	outcome.Message = "Usage could not be verified; retry shortly"
	for {
		fenceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		_, err := s.resolveTargets(fenceCtx, profile, state)
		cancel()
		if err == nil {
			return nil, false
		}
		if !s.retryFence(ctx, profile, id, state, epoch, identity, intent, started, outcome) {
			// An eligible replacement needs collection; cancellation and removal stop this flight.
			s.mu.Lock()
			recollect := ctx.Err() == nil && s.currentLocked(profile, id, state) && state.epoch != epoch
			s.mu.Unlock()
			return outcome, recollect
		}
	}
}

// retryFence retains same-owner last-good data while awaiting a fresh ownership check.
// It never publishes the newly collected data until reconciliation succeeds.
func (s *Manager) retryFence(
	ctx context.Context,
	profile, id string,
	state *targetState,
	epoch uint64,
	identity CredentialIdentity,
	intent Intent,
	started time.Time,
	outcome *UsageError,
) bool {
	s.mu.Lock()
	if !s.currentLocked(profile, id, state) || state.epoch != epoch {
		s.mu.Unlock()
		return false
	}
	s.publishLocked(profile, id, state, state.target, identity, Snapshot{}, outcome)
	status := state.snapshot.Status
	next := s.options.Clock.Now().Add(5 * time.Second)
	state.snapshot.NextRefreshAt = &next
	timer := s.options.Clock.NewTimer(5 * time.Second)
	s.mu.Unlock()
	defer timer.Stop()
	s.observeCollection(profile, id, "", status, intent, started, outcome)
	select {
	case <-ctx.Done():
		return false
	case <-timer.Chan():
		return true
	}
}

func snapshotSource(snapshot Snapshot) string {
	if snapshot.Source == nil {
		return ""
	}
	return string(snapshot.Source.Kind)
}
