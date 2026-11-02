package daemon

import (
	"context"
	"errors"
	"slices"

	monitorpkg "github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

type monitorTargetManager interface {
	Status(context.Context, string) (*session.Info, error)
	InputQueueSummary(context.Context, string) (session.InputQueueSummary, error)
	SubscribeSessionCatalogEvents(
		context.Context,
		session.CatalogScope,
	) (<-chan session.CatalogEvent, func(), error)
}

// monitorSessionTargets gives session monitors read access to their targets. Every state comes from a
// fresh Status read; catalog events only say when to read again.
type monitorSessionTargets struct {
	sessions func() any
}

func (t monitorSessionTargets) manager() (monitorTargetManager, error) {
	manager, ok := t.sessions().(monitorTargetManager)
	if !ok {
		return nil, errors.New("daemon: session manager cannot serve monitor targets")
	}
	return manager, nil
}

func (t monitorSessionTargets) ReadTargets(ctx context.Context, spec monitorpkg.Spec) []monitorpkg.TargetStatus {
	statuses := make([]monitorpkg.TargetStatus, 0, len(spec.Targets))
	manager, err := t.manager()
	for _, target := range spec.Targets {
		status := monitorpkg.TargetStatus{SessionID: target, State: monitorpkg.TargetStateUnknown}
		if err == nil {
			status = readMonitorTarget(ctx, manager, target)
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func readMonitorTarget(
	ctx context.Context,
	manager monitorTargetManager,
	target string,
) monitorpkg.TargetStatus {
	status := monitorpkg.TargetStatus{SessionID: target, State: monitorpkg.TargetStateUnknown}
	info, err := manager.Status(ctx, target)
	switch {
	case errors.Is(err, session.ErrSessionNotFound),
		errors.Is(err, store.ErrSessionNotFound),
		errors.Is(err, store.ErrSessionSuperseded):
		status.State = monitorpkg.TargetStateGone
	case err == nil && info != nil:
		status.Name = info.Name
		status.State = monitorTargetState(info)
		if settledBadgeState(status.State) {
			status.State = queuedTargetState(ctx, manager, target, status.State)
		}
	}
	return status
}

func settledBadgeState(state string) bool {
	return state == monitorpkg.TargetStateIdle || state == monitorpkg.TargetStateDone
}

// queuedTargetState keeps a target with queued input from reading as settled: between two turns its
// badge is idle or done for a moment, but its next turn is already committed.
func queuedTargetState(
	ctx context.Context,
	manager monitorTargetManager,
	target, state string,
) string {
	queue, err := manager.InputQueueSummary(ctx, target)
	switch {
	case err != nil:
		return monitorpkg.TargetStateUnknown
	case queue.PendingInputs > 0:
		return string(session.BadgeRunning)
	default:
		return state
	}
}

// monitorTargetState folds a stopped session to a final state: its badge can still describe why it
// stopped, such as pending authentication, but a stopped session cannot make progress on its own.
// A session the daemon stopped and did not resume is reported apart, because its work did not end.
func monitorTargetState(info *session.Info) string {
	badge := session.BadgeForInfo(info)
	if info.State != session.StateStopped || badge == session.BadgeFailed {
		return string(badge)
	}
	if info.StopReason == store.StopShutdown {
		return monitorpkg.TargetStateInterrupted
	}
	return monitorpkg.TargetStateStopped
}

// TargetChanges reduces the workspace catalog to a signal for spec's targets. The catalog closes a
// slow subscriber's channel, which closes the signal and leaves the monitor on its sweep.
func (t monitorSessionTargets) TargetChanges(
	ctx context.Context,
	spec monitorpkg.Spec,
) (<-chan struct{}, func(), error) {
	manager, err := t.manager()
	if err != nil {
		return nil, nil, err
	}
	watchCtx, stop := context.WithCancel(ctx)
	events, cancel, err := manager.SubscribeSessionCatalogEvents(watchCtx, session.CatalogScope{
		ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: spec.WorkspaceID,
	})
	if err != nil {
		stop()
		return nil, nil, err
	}
	changes := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(changes)
		forwardMonitorTargetChanges(watchCtx, events, spec.Targets, changes)
	}()
	return changes, func() {
		stop()
		cancel()
		<-done
	}, nil
}

func forwardMonitorTargetChanges(
	ctx context.Context,
	events <-chan session.CatalogEvent,
	targets []string,
	changes chan<- struct{},
) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, open := <-events:
			if !open {
				return
			}
			if !slices.Contains(targets, event.SessionID) {
				continue
			}
			select {
			case changes <- struct{}{}:
			default:
			}
		}
	}
}
