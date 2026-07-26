package consolidation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/agh/internal/memory"
)

// Enabled reports whether dream consolidation is available.
func (r *Runtime) Enabled() bool {
	return r != nil && r.enabled != nil && r.enabled()
}

// LastConsolidatedAt returns the most recent lock timestamp.
func (r *Runtime) LastConsolidatedAt() (time.Time, error) {
	if r == nil || r.lastConsolidatedAt == nil {
		return time.Time{}, nil
	}
	return r.lastConsolidatedAt()
}

// Trigger runs dream consolidation immediately when enabled and gates pass.
func (r *Runtime) Trigger(ctx context.Context, workspace string) (bool, string, error) {
	if !r.Enabled() || r.service == nil || r.spawner == nil {
		return false, "dream consolidation is disabled", nil
	}

	shouldRun, err := r.service.ShouldRun()
	if err != nil {
		return false, "", err
	}
	if !shouldRun {
		return false, dreamGatesNotSatisfiedReason, nil
	}
	if err := r.service.Run(ctx, r.spawner, strings.TrimSpace(workspace)); err != nil {
		if errors.Is(err, memory.ErrLockUnavailable) {
			return false, "dream consolidation is already running", nil
		}
		if errors.Is(err, memory.ErrDreamGateNotSatisfied) {
			return false, dreamGatesNotSatisfiedReason, nil
		}
		if errors.Is(err, memory.ErrDreamRoleDisabled) {
			return false, "dream role is disabled", nil
		}
		return false, "", err
	}

	return true, "", nil
}
