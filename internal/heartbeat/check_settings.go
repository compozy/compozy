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
		cadence = max(cadence, target.Policy.Preferences.MinInterval)
	}
	return storage.SetChecks(ctx, id, enabled, cadence)
}
