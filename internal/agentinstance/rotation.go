package agentinstance

import (
	"context"
	"errors"
)

var ErrRotationChanged = errors.New("agentinstance: rotation state changed")

type RotationPublication struct {
	InstanceID     string
	PredecessorID  string
	SuccessorID    string
	AdmissionCount int64
}

type RotationDecision struct {
	Publication RotationPublication
	Eligible    bool
}

type MainSessionRotator interface {
	Eligible(context.Context, Instance) (RotationDecision, error)
	Rotate(context.Context, Instance, RotationDecision) error
	Cleanup(context.Context, Instance) error
}

func WithMainSessionRotator(rotator MainSessionRotator) Option {
	return func(s *Service) { s.rotator = rotator }
}

func (s *Service) rotateMain(ctx context.Context, instance Instance) error {
	if s.rotator == nil || instance.MainSessionID == "" {
		return nil
	}
	if err := s.rotator.Cleanup(ctx, instance); err != nil {
		return err
	}
	if instance.State != "active" || instance.ArchivedAt != nil {
		return nil
	}
	decision, err := s.rotator.Eligible(ctx, instance)
	if err != nil || !decision.Eligible {
		return err
	}
	if decision.Publication.InstanceID != instance.ID || decision.Publication.PredecessorID != instance.MainSessionID {
		return ErrRotationChanged
	}
	return s.rotator.Rotate(ctx, instance, decision)
}
