package daemon

import (
	"context"
	"fmt"
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	"github.com/compozy/compozy/internal/store"
)

type loopTargetHealthSlot struct {
	mu      sync.RWMutex
	service *deadentity.Service
}

func (s *loopTargetHealthSlot) Swap(service *deadentity.Service) {
	if s == nil || service == nil {
		return
	}
	s.mu.Lock()
	s.service = service
	s.mu.Unlock()
}

func (s *loopTargetHealthSlot) BeforeProbe(
	ctx context.Context,
	key store.DeadEntityKey,
) (deadentity.ProbeDecision, error) {
	service, err := s.current()
	if err != nil {
		return deadentity.ProbeDecision{}, err
	}
	return service.BeforeProbe(ctx, key)
}

func (s *loopTargetHealthSlot) RecordFailure(
	ctx context.Context,
	key store.DeadEntityKey,
	class deadentity.FailureClass,
	reason string,
) error {
	service, err := s.current()
	if err != nil {
		return err
	}
	return service.RecordFailure(ctx, key, class, reason)
}

func (s *loopTargetHealthSlot) RecordSuccess(ctx context.Context, key store.DeadEntityKey) error {
	service, err := s.current()
	if err != nil {
		return err
	}
	return service.RecordSuccess(ctx, key)
}

func (s *loopTargetHealthSlot) Status(
	ctx context.Context,
	key store.DeadEntityKey,
) (deadentity.Status, error) {
	service, err := s.current()
	if err != nil {
		return deadentity.Status{}, err
	}
	return service.Status(ctx, key)
}

func (s *loopTargetHealthSlot) current() (*deadentity.Service, error) {
	if s == nil {
		return nil, fmt.Errorf("daemon: loop target health is unavailable")
	}
	s.mu.RLock()
	service := s.service
	s.mu.RUnlock()
	if service == nil {
		return nil, fmt.Errorf("daemon: loop target health is unavailable")
	}
	return service, nil
}

func (d *Daemon) newLoopTargetHealthService(
	state *bootState,
	cfg compozyconfig.LoopBreakerConfig,
) (*deadentity.Service, error) {
	if state == nil || state.registry == nil {
		return nil, fmt.Errorf("daemon: global registry is required for loop target health")
	}
	deadStore, ok := state.registry.(store.DeadEntityStore)
	if !ok {
		return nil, fmt.Errorf("daemon: global registry does not support loop target health")
	}
	policy, err := loopBreakerPolicyFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return deadentity.New(
		deadStore,
		deadentity.WithClock(d.now),
		deadentity.WithLogger(state.logger),
		deadentity.WithEventStore(state.registry),
		deadentity.WithPermanentFailureThreshold(policy.Threshold),
		deadentity.WithRecoveryInterval(policy.ProbeInterval),
	), nil
}
