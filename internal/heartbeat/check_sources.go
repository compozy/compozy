package heartbeat

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type sourceEnvironment interface {
	PollSources(context.Context, *CheckTarget, bool) (PollBatch, error)
}
type sourceWakeStore interface {
	DeferWake(context.Context, string, string, time.Time) (bool, error)
}

func (s *CheckService) collectSources(
	ctx context.Context,
	state InstanceWakeState,
	target *CheckTarget,
	manual, dry bool,
) (CheckResult, error) {
	env, ok := s.environment.(sourceEnvironment)
	if !ok {
		return CheckResult{}, errors.New("heartbeat: source environment unavailable")
	}

	s.mu.Lock()
	if _, exists := s.collecting[state.InstanceID]; exists {
		s.mu.Unlock()
		return CheckResult{Gate: checkSourcePollInFlight}, nil
	}
	if s.closed {
		s.mu.Unlock()
		return CheckResult{}, errors.New("heartbeat: checks are stopped")
	}
	select {
	case s.pollSem <- struct{}{}:
	default:
		s.mu.Unlock()
		return CheckResult{Gate: checkNoSlot}, nil
	}
	lifetime := s.ctx
	if manual || dry {
		lifetime = ctx
	}
	collection, cancel := context.WithCancel(lifetime)
	s.collecting[state.InstanceID] = cancel
	cleanup := func() { cancel(); s.mu.Lock(); delete(s.collecting, state.InstanceID); <-s.pollSem; s.mu.Unlock() }
	if manual || dry {
		s.wg.Add(1)
		defer s.wg.Done()
		s.mu.Unlock()
		defer cleanup()
		return s.pollSourceTarget(collection, env, state, target, manual, dry)
	}
	storage, ok := s.store.(sourceWakeStore)
	if !ok {
		s.mu.Unlock()
		cleanup()
		return CheckResult{}, errors.New("heartbeat: source wake store unavailable")
	}
	until := s.now().Add(s.config.Sources.Timeout + 30*time.Second)
	changed, err := storage.DeferWake(ctx, state.InstanceID, state.NextWakeAt, until)
	if err != nil || !changed {
		s.mu.Unlock()
		cleanup()
		return CheckResult{}, err
	}
	state.NextWakeAt = store.FormatTimestamp(until)
	state.Deferred = true
	s.wg.Go(func() {
		defer cleanup()
		if _, err := s.pollSourceTarget(
			collection,
			env,
			state,
			target,
			manual,
			dry,
		); err != nil &&
			!errors.Is(err, context.Canceled) {
			slog.Error("heartbeat source collection failed", "instance_id", state.InstanceID, "error", err)
		}
	})
	s.mu.Unlock()
	return CheckResult{Gate: checkSourcePollInFlight}, nil
}

func (s *CheckService) SourceStatuses(ctx context.Context, id string) ([]SourceStatus, error) {
	storage, ok := s.store.(interface {
		SourceStatuses(context.Context, string) ([]SourceStatus, error)
	})
	if !ok {
		return []SourceStatus{}, nil
	}
	statuses, err := storage.SourceStatuses(ctx, id)
	if err != nil {
		slog.Warn("heartbeat source statuses unavailable", "instance_id", id, "error", err)
		return []SourceStatus{}, nil
	}
	return statuses, nil
}

func (s *CheckService) recordSourceMetrics(batch PollBatch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sourcePollCounts == nil {
		s.sourcePollCounts = map[string]int64{}
	}
	for _, p := range batch.Sources {
		if !p.Polled {
			continue
		}
		s.sourcePollCounts[SourceResultStatus(p.Err)]++
		s.sourcePollDuration += p.DurationMS
	}
}
func (s *CheckService) SourceMetrics() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := map[string]int64{}
	var total int64
	for result, count := range s.sourcePollCounts {
		counts[result] = count
		total += count
	}
	return map[string]any{
		"heartbeat_source_polls_total":          counts,
		"heartbeat_source_poll_duration_ms":     map[string]int64{"count": total, "sum": s.sourcePollDuration},
		"heartbeat_source_collections_inflight": len(s.collecting),
	}
}

func (s *CheckService) pollSourceTarget(
	ctx context.Context,
	env sourceEnvironment,
	state InstanceWakeState,
	target *CheckTarget,
	manual, dry bool,
) (CheckResult, error) {
	if !dry {
		source := checkTimerSource
		if manual {
			source = checkManualSource
		}
		reason, err := s.environment.BeforeCheck(ctx, target, source)
		if err != nil {
			return CheckResult{}, s.rescheduleCheckFailure(
				ctx,
				state,
				manual,
				s.checkInterval(&target.Policy),
				true,
				err,
			)
		}
		if reason != "" {
			_, err := s.store.Skip(
				ctx,
				SkipRequest{
					Deferred:    state.Deferred,
					ManualCheck: manual,
					InstanceID:  state.InstanceID,
					ExpectedDue: state.NextWakeAt,
					Reason:      checkHookCancelled,
					Detail:      reason,
					Now:         s.now(),
					NextWakeAt:  NextCheckTime(s.now(), s.checkInterval(&target.Policy), state.InstanceID),
					ExpiresAt:   s.now().Add(s.config.Retention),
				},
			)
			return CheckResult{Gate: checkHookCancelled}, err
		}
	}
	batch, err := env.PollSources(ctx, target, dry)
	if !dry {
		s.recordSourceMetrics(batch)
	}
	if err != nil {
		return CheckResult{}, s.rescheduleCheckFailure(
			ctx,
			state,
			manual || dry,
			s.checkInterval(&target.Policy),
			true,
			err,
		)
	}
	if err := ctx.Err(); err != nil {
		return CheckResult{}, err
	}
	current, err := s.environment.ResolveCheck(ctx, state.InstanceID)
	if err != nil {
		return CheckResult{}, err
	}
	if current.Policy.Digest != target.Policy.Digest {
		return CheckResult{Gate: "heartbeat_invalid"}, nil
	}
	if gate, err := s.checkGate(&current, dry, s.now()); err != nil || gate != "" {
		return CheckResult{Gate: gate, Sources: batch.Outcomes()}, err
	}
	result, err := s.collectTarget(ctx, state, &current, manual, dry, batch)
	result.Sources = batch.Outcomes()
	return result, err
}

func (s *CheckService) hasSourcesAfterResolveFailure(ctx context.Context, target *CheckTarget) bool {
	if len(target.Policy.Preferences.Context.Sources) > 0 {
		return true
	}
	statuses, err := s.SourceStatuses(ctx, target.Instance.ID)
	return err == nil && len(statuses) > 0
}
