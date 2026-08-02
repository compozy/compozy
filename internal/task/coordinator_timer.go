package task

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CoordinatorTimerSpec describes one best-effort post-commit timer. Durable
// state remains authoritative and the scheduler backstop recovers missed fires.
type CoordinatorTimerSpec struct {
	LoopRunID      string
	Generation     int
	NodeID         string
	ItemIndex      int
	Attempt        int
	IssuedEpoch    int64
	FireAt         time.Time
	IdempotencyKey string
}

// CoordinatorTimerArmer arms best-effort timers only after coordinator state commits.
type CoordinatorTimerArmer interface {
	ArmCoordinatorTimer(context.Context, CoordinatorTimerSpec, ActorContext) error
}

// Normalize returns a stable timer identity and UTC deadline.
func (s CoordinatorTimerSpec) Normalize() CoordinatorTimerSpec {
	s.LoopRunID = strings.TrimSpace(s.LoopRunID)
	s.NodeID = strings.TrimSpace(s.NodeID)
	s.IdempotencyKey = strings.TrimSpace(s.IdempotencyKey)
	if !s.FireAt.IsZero() {
		s.FireAt = s.FireAt.UTC()
	}
	return s
}

// Validate reports whether the timer can be fenced against one output cell.
func (s CoordinatorTimerSpec) Validate(path string) error {
	if s.LoopRunID == "" || s.NodeID == "" || s.Generation < 1 || s.ItemIndex < 0 ||
		s.Attempt < 1 || s.IssuedEpoch < 1 || s.FireAt.IsZero() || s.IdempotencyKey == "" {
		return fmt.Errorf("%w: %s has incomplete retry timer identity", ErrValidation, path)
	}
	return nil
}

func validateCoordinatorTimerSpecs(specs []CoordinatorTimerSpec, path string) error {
	for idx, spec := range specs {
		normalized := spec.Normalize()
		if err := normalized.Validate(nestedPath(path, fmt.Sprintf("post_commit_timers[%d]", idx))); err != nil {
			return err
		}
	}
	return nil
}
