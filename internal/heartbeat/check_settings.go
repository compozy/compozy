package heartbeat

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
)

type CheckSettingsStore interface {
	SetChecks(context.Context, string, bool, time.Duration) (agentinstance.Instance, error)
}

func (s *CheckService) SetChecks(ctx context.Context, id string, enabled bool) (agentinstance.Instance, error) {
	storage, ok := s.store.(CheckSettingsStore)
	if !ok {
		return agentinstance.Instance{}, errors.New("heartbeat: check settings unavailable")
	}
	cadence := s.config.Interval
	if enabled {
		target, err := s.environment.ResolveCheck(ctx, id)
		if err != nil {
			return agentinstance.Instance{}, err
		}
		cadence = s.checkInterval(&target.Policy)
	}
	return storage.SetChecks(ctx, id, enabled, cadence)
}

func (s *CheckService) checkInterval(policy *ResolvedPolicy) time.Duration {
	if policy.Preferences.MinInterval > 0 {
		return policy.Preferences.MinInterval
	}
	return s.config.Interval
}

func (s *CheckService) checkBackoff(cadence time.Duration, state InstanceWakeState) time.Duration {
	if state.FailureStreak > 0 {
		return CheckBackoff(cadence, s.config.MaxInterval, state.FailureStreak)
	}
	maximum := s.config.MaxInterval
	if s.config.QuietBackoffCap > 0 {
		maximum = s.config.QuietBackoffCap
	}
	return CheckBackoff(cadence, maximum, state.QuietStreak/3)
}

func CheckBackoff(interval, maximum time.Duration, streak int64) time.Duration {
	maximum = max(interval, maximum)
	delay := interval
	for streak > 0 && delay < maximum {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
		streak--
	}
	return delay
}
