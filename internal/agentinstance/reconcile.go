package agentinstance

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type StartOutcome struct {
	SessionID string
	State     string
}

type ReconciliationSessions interface {
	StartOutcome(context.Context, Instance) (StartOutcome, error)
	DiscardFailedStart(context.Context, Instance, string) error
}

func (s *Service) EnsureMain(ctx context.Context, id string) (Instance, error) {
	instance, err := s.store.Get(ctx, id)
	if err != nil {
		return Instance{}, err
	}
	if instance.ArchivedAt != nil {
		return Instance{}, ErrNotFound
	}
	if instance.MainSessionID != "" {
		return instance, nil
	}
	if instance.State == "starting" {
		return Instance{}, ErrStarting
	}
	if instance.PauseReason == "manual" {
		return Instance{}, errors.New("agent_instance_paused")
	}
	if err := s.admission.ValidateResume(ctx, instance); err != nil {
		return Instance{}, err
	}
	sessionID, err := s.starter.Start(ctx, StartMainSession{Instance: instance})
	if err != nil {
		return Instance{}, err
	}
	if sessionID == "" {
		return Instance{}, errors.New("agentinstance: empty main session id")
	}
	return s.store.PublishMissingMain(ctx, id, sessionID)
}

func (s *Service) Reconcile(ctx context.Context) error {
	recovery, ok := s.starter.(ReconciliationSessions)
	if !ok {
		return errors.New("agentinstance: session reconciliation unavailable")
	}
	var failures []error
	after := ""
	for {
		instances, err := s.store.ReconciliationPage(ctx, after, 100)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, instance := range instances {
			after = instance.ID
			if err := s.reconcileInstance(ctx, recovery, instance); err != nil {
				failures = append(failures, fmt.Errorf("reconcile instance %s: %w", instance.ID, err))
			}
		}
		if len(instances) < 100 {
			return errors.Join(failures...)
		}
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
	}
}

func (s *Service) reconcileInstance(ctx context.Context, recovery ReconciliationSessions, instance Instance) error {
	if instance.State == "starting" {
		outcome, err := recovery.StartOutcome(ctx, instance)
		if err != nil {
			return err
		}
		switch outcome.State {
		case "accepted":
			_, err = s.store.BindMain(ctx, instance.ID, outcome.SessionID)
			return err
		case "failed":
			if err := recovery.DiscardFailedStart(ctx, instance, outcome.SessionID); err != nil {
				return err
			}
			return s.store.DeleteUnstarted(ctx, instance.ID)
		case "missing":
			if s.now().Sub(instance.CreatedAt) >= 5*time.Minute {
				return s.store.DeleteUnstarted(ctx, instance.ID)
			}
			return nil
		case "pending":
			return nil
		default:
			return fmt.Errorf("agentinstance: unknown start outcome %q", outcome.State)
		}
	}
	err := s.admission.ValidateResume(ctx, instance)
	reason := ""
	switch {
	case errors.Is(err, ErrDefinitionMissing):
		reason = "definition_missing"
	case errors.Is(err, ErrNotStaysOn):
		reason = "definition_not_stays_on"
	default:
		return err
	}
	_, err = s.store.Pause(ctx, instance.ID, reason)
	return err
}
