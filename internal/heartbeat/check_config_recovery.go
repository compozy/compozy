package heartbeat

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
)

type configRecoveryStore interface {
	ReconciliationPage(context.Context, string, int) ([]agentinstance.Instance, error)
	RecoverConfigRejectedWake(context.Context, string, time.Time) error
}

func (s *CheckService) RecoverConfiguration(ctx context.Context, matches func(CheckTarget) bool) error {
	storage, ok := s.store.(configRecoveryStore)
	if !ok {
		return nil
	}
	var failures []error
	for after := ""; ; {
		instances, err := storage.ReconciliationPage(ctx, after, 100)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, instance := range instances {
			after = instance.ID
			if instance.State != "active" || !instance.ChecksEnabled {
				continue
			}
			state, err := s.store.WakeState(ctx, instance.ID)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			if state.LastReason != ReasonTriageConfigUnavailable || state.OccurrenceID != "" {
				continue
			}
			target, err := s.environment.ResolveCheck(ctx, instance.ID)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			if target.Gate != "" || !target.Policy.Enabled || (matches != nil && !matches(target)) {
				continue
			}
			// Recovery has no jitter so the retry is due within one resolved cadence.
			if err := storage.RecoverConfigRejectedWake(ctx, instance.ID, s.now().Add(s.checkInterval(&target.Policy))); err != nil {
				failures = append(failures, err)
			}
		}
		if len(instances) < 100 {
			return errors.Join(failures...)
		}
	}
}
