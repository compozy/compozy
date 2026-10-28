package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

// updateChildActivity records whether a settled delegation's child session is
// working, idle, or stopped. The delegation outcome stays terminal: status,
// settlement, result, and delivery never change here.
func (s *subagentService) updateChildActivity(ctx context.Context, row store.SessionSubagent) error {
	if row.Origin != store.SubagentOriginDelegated || row.ChildSessionID == nil {
		return nil
	}
	child, err := s.runtime.Snapshot(ctx, *row.ChildSessionID)
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	activity := subagentChildActivity(child)
	if activity == store.SubagentChildActivityUnknown || activity == row.ChildActivity {
		return nil
	}
	updated, changed, err := s.store.UpdateSubagentChildActivity(ctx, row.ID, activity, s.now().UTC())
	if err == nil && changed {
		s.publish(ctx, updated)
	}
	return err
}

// subagentChildActivity classifies a child snapshot. Transitional states and a
// daemon-shutdown stop report nothing: the next lifecycle edge, or boot
// reconciliation, records the settled activity.
func subagentChildActivity(child subagentSnapshot) store.SubagentChildActivity {
	switch {
	case child.Info == nil:
		return store.SubagentChildActivityUnknown
	case child.Info.State == StateStopped:
		if child.Info.StopReason == store.StopShutdown {
			return store.SubagentChildActivityUnknown
		}
		return store.SubagentChildActivityStopped
	case child.Active || child.Queued > 0:
		return store.SubagentChildActivityWorking
	case child.Info.State == StateStarting || child.Info.State == StateStopping:
		return store.SubagentChildActivityUnknown
	default:
		return store.SubagentChildActivityIdle
	}
}

// recoverChildActivity marks settled delegations whose child stopped while the
// daemon was down (or at shutdown) as stopped.
func (s *subagentService) recoverChildActivity(ctx context.Context) error {
	rows, err := s.store.ReconcileStoppedChildActivity(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		s.publish(ctx, row)
		s.recoveryResult(ctx, row.ID, "child_activity_stopped", nil)
	}
	return nil
}
