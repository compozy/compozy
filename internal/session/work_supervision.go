package session

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/events"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

var errSupervisionActivityResumed = errors.New("session: activity resumed before inactivity stop")

// Supervise inspects live sessions; the daemon owns its joined periodic cadence.
func (m *Manager) Supervise(ctx context.Context, now time.Time) error {
	m.supervisionMu.Lock()
	defer m.supervisionMu.Unlock()
	m.mu.RLock()
	targets := make([]*Session, 0, len(m.sessions))
	for _, target := range m.sessions {
		targets = append(targets, target)
	}
	m.mu.RUnlock()
	var errs []error
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return errors.Join(errors.Join(errs...), err)
		}
		if target.Info().State != StateActive {
			continue
		}
		if err := m.superviseSession(ctx, target, now.UTC()); err != nil {
			errs = append(errs, fmt.Errorf("supervise session %s: %w", target.ID, err))
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) superviseSession(ctx context.Context, target *Session, now time.Time) error {
	target.mu.RLock()
	progress := target.supervisionProgressAt
	target.mu.RUnlock()
	policy := m.supervision.QuietPolicy()
	state := m.workSignals.Inspect(ctx, target.ID, now)
	target.mu.Lock()
	if target.State != StateActive || target.supervisionProgressAt != progress {
		target.mu.Unlock()
		return nil
	}
	previous := CloneSupervisionState(target.supervisionState)
	state.QuietWarning = previous.QuietWarning
	updateSupervisionQuietStateLocked(target, state, policy, now)
	if !supervisionNeedsAttention(state) {
		target.supervisionReportedSources = nil
	}
	target.supervisionState = state
	changed := !reflect.DeepEqual(previous, state)
	attentionChanged := supervisionNeedsAttention(state) &&
		!reflect.DeepEqual(target.supervisionReportedSources, state.Sources)
	warning := state.QuietWarning != nil && !target.supervisionWarningRecordedAt.Equal(state.QuietWarning.WarnedAt)
	stop := !target.supervisionStopAt.IsZero() && !now.Before(target.supervisionStopAt)
	target.mu.Unlock()
	if changed {
		m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, target.Info()))
	}
	if attentionChanged {
		if err := m.recordSupervisionEvent(ctx, target, events.SessionSupervisionSourceError, state); err != nil {
			return err
		}
		target.mu.Lock()
		target.supervisionReportedSources = slices.Clone(state.Sources)
		target.mu.Unlock()
	}
	if warning {
		if err := m.recordSupervisionEvent(ctx, target, events.SessionSupervisionWarning, state); err != nil {
			return err
		}
		target.mu.Lock()
		target.supervisionWarningRecordedAt = state.QuietWarning.WarnedAt
		target.mu.Unlock()
	}

	if stop {
		err := m.RequestStopWithCause(ctx, target.ID, CauseInactivity, "inactivity")
		if errors.Is(err, errSupervisionActivityResumed) {
			return nil
		}
		return err
	}
	return nil
}

// updateSupervisionQuietStateLocked advances only the currently observed quiet episode.
// The caller holds target.mu across observation validation and publication decisions.
func updateSupervisionQuietStateLocked(
	target *Session, state *SupervisionState, policy compozyconfig.QuietPolicy, now time.Time,
) {
	unknown := slices.ContainsFunc(
		state.Sources,
		func(source SignalSourceState) bool { return source.State == signalSourceUnknown },
	)
	switch {
	case len(state.WorkSignals) > 0 || (policy.WarningAfter == 0 && policy.StopAfterQuiet == 0):
		target.supervisionQuietSince = time.Time{}
		state.QuietWarning = nil
	case unknown:
		// Do not accrue unobserved silence; a recovered source starts a new episode.
		target.supervisionQuietSince = time.Time{}
		state.QuietWarning = nil
	default:
		if target.supervisionQuietSince.IsZero() {
			target.supervisionQuietSince = now
		}
		if policy.WarningAfter > 0 && state.QuietWarning == nil &&
			now.Sub(target.supervisionQuietSince) >= policy.WarningAfter {
			state.QuietWarning = &QuietWarning{QuietSince: target.supervisionQuietSince, WarnedAt: now}
			if policy.StopGrace > 0 {
				at := now.Add(policy.StopGrace)
				state.QuietWarning.StopAt = &at
			}
		}
	}
	target.supervisionStopAt = time.Time{}
	if !target.supervisionQuietSince.IsZero() {
		if policy.StopAfterQuiet > 0 {
			target.supervisionStopAt = target.supervisionQuietSince.Add(policy.StopAfterQuiet)
			if state.QuietWarning != nil {
				at := target.supervisionStopAt
				state.QuietWarning.StopAt = &at
			}
		} else if state.QuietWarning != nil && state.QuietWarning.StopAt != nil {
			target.supervisionStopAt = *state.QuietWarning.StopAt
		}
	}
}
