package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/acp"
	monitorpkg "github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type monitorSessionManager interface {
	Status(context.Context, string) (*session.Info, error)
	EnqueueSyntheticWake(context.Context, string, session.SyntheticPromptOpts) (store.SessionInputQueueEntry, error)
}

type sessionCatalogChangePublisher interface {
	PublishSessionCatalogChanged(context.Context, string)
}

// monitorSessionWaker admits a fired monitor to its owning session's durable
// input queue and returns once it is queued: an idle session starts a turn, a
// busy one runs it after the turn. It never reads the woken turn.
type monitorSessionWaker struct {
	sessions func() monitorSessionManager
}

func (w *monitorSessionWaker) WakeSession(ctx context.Context, notification monitorpkg.Notification) error {
	sessions := w.sessions()
	if sessions == nil {
		return errors.New("daemon: session manager is unavailable")
	}
	sessionID := notification.Monitor.SessionID
	info, err := sessions.Status(ctx, sessionID)
	switch {
	case errors.Is(err, session.ErrSessionNotFound), errors.Is(err, store.ErrSessionNotFound):
		return fmt.Errorf("%w: %s", monitorpkg.ErrSessionGone, sessionID)
	case err != nil:
		return fmt.Errorf("daemon: read monitor session %q: %w", sessionID, err)
	case info == nil:
		return fmt.Errorf("%w: %s", monitorpkg.ErrSessionGone, sessionID)
	}
	// A stopped session is the user's decision; a monitor must not revive it.
	// A session still starting is transient and retried by the monitor service.
	if info.State == session.StateStopping || info.State == session.StateStopped {
		return fmt.Errorf("%w: %s is %s", monitorpkg.ErrSessionGone, sessionID, info.State)
	}
	if _, err := sessions.EnqueueSyntheticWake(ctx, sessionID, session.SyntheticPromptOpts{
		MessageID: "monitor:" + notification.Monitor.ID,
		Priority:  1,
		Message:   notification.Message(),
		Metadata: acp.PromptSyntheticMeta{
			Kind:        acp.PromptSyntheticKindMonitor,
			Reason:      "monitor_" + notification.Outcome.Reason,
			Summary:     notification.Monitor.Description,
			WakeEventID: notification.Monitor.ID,
		},
	}); err != nil {
		return fmt.Errorf("daemon: deliver monitor wake to %q: %w", sessionID, err)
	}
	return nil
}

// monitorLifecycleObserver cancels a session's monitors when it stops, so a
// stopped or removed session never keeps watches, history, or cap slots.
type monitorLifecycleObserver struct {
	monitors *monitorpkg.Service
	// restarting reports that the daemon is stopping sessions for a restart. The session's own stop
	// classification is cleared before observers run, so the daemon's restart intent is authoritative.
	restarting func() bool
}

func (o monitorLifecycleObserver) OnSessionCreated(context.Context, *session.Session) {}

func (o monitorLifecycleObserver) OnSessionStopped(_ context.Context, stopped *session.Session) {
	if o.monitors == nil || stopped == nil {
		return
	}
	info := stopped.Info()
	// The stop cause is recorded when the stop is requested; the persisted reason is classified later.
	shutdown := info.StopCause == session.CauseShutdown || info.StopReason == store.StopShutdown ||
		(o.restarting != nil && o.restarting())
	o.release(stopped.ID, shutdown)
}

// release applies the stop's authority to the owner's monitors. A daemon restart interrupts the owner,
// so its monitors survive for Recover to decide at boot. Every other stop is a decision, and nothing
// the session registered may outlive it.
func (o monitorLifecycleObserver) release(sessionID string, shutdown bool) {
	if shutdown {
		o.monitors.SuspendSession(sessionID)
		return
	}
	o.monitors.CancelSession(sessionID)
}

type monitorOwnerManager interface {
	MonitorOwnerBinding(string) (string, error)
	ResumeMonitorOwner(context.Context, string, string) error
}

// monitorSessionOwners captures owner bindings and resumes restart-interrupted owners
// through the session manager, which owns restart eligibility.
type monitorSessionOwners struct {
	sessions func() any
}

func (o monitorSessionOwners) manager() (monitorOwnerManager, error) {
	manager, ok := o.sessions().(monitorOwnerManager)
	if !ok {
		return nil, errors.New("daemon: session manager cannot resume monitor owners")
	}
	return manager, nil
}

func (o monitorSessionOwners) Binding(_ context.Context, sessionID string) (string, error) {
	manager, err := o.manager()
	if err != nil {
		return "", err
	}
	return manager.MonitorOwnerBinding(sessionID)
}

func (o monitorSessionOwners) Resume(ctx context.Context, sessionID, binding string) error {
	manager, err := o.manager()
	if err != nil {
		return err
	}
	err = manager.ResumeMonitorOwner(ctx, sessionID, binding)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, session.ErrMonitorOwnerStopped):
		return monitorpkg.ErrOwnerStopped
	case errors.Is(err, session.ErrSessionNotFound), errors.Is(err, store.ErrSessionNotFound):
		return monitorpkg.ErrSessionGone
	}
	if ineligible, ok := errors.AsType[*session.MonitorOwnerIneligibleError](err); ok {
		return &monitorpkg.OwnerIneligibleError{Reason: ineligible.Reason}
	}
	return err
}

var _ sessionLifecycleObserver = monitorLifecycleObserver{}

// monitorTerminalWatcher resolves a terminal in the monitor owner's profile and
// waits on it through the same Wait path compozy__terminal_wait uses.
type monitorTerminalWatcher struct {
	terminals func() terminalpkg.Manager
}

func (w *monitorTerminalWatcher) WatchTerminal(
	ctx context.Context,
	spec monitorpkg.Spec,
) (monitorpkg.TerminalWatch, error) {
	manager := w.terminals()
	if manager == nil {
		return nil, fmt.Errorf("terminal service is unavailable: %w", terminalpkg.ErrServiceUnavailable)
	}
	handle, err := manager.Handle(ctx, spec.WorkspaceID, spec.ProfileID, terminalpkg.ID(spec.TerminalID))
	if err != nil {
		return nil, err
	}
	condition := terminalpkg.WaitCondition{
		Until: spec.Until, Pattern: spec.Pattern, TimeoutMs: int(spec.Timeout.Milliseconds()),
	}
	return func(waitCtx context.Context) (monitorpkg.Outcome, error) {
		result, err := handle.Wait(waitCtx, condition)
		if err != nil {
			return monitorpkg.Outcome{}, err
		}
		if result == nil {
			return monitorpkg.Outcome{}, errors.New("terminal wait returned no result")
		}
		return monitorpkg.Outcome{Reason: result.Reason, ExitCode: result.ExitCode, Output: result.Screen}, nil
	}, nil
}

func newSessionMonitorService(ctx context.Context, state *bootState) (*monitorpkg.Service, error) {
	logger := state.logger
	if logger == nil {
		logger = slog.Default()
	}
	waker := &monitorSessionWaker{sessions: func() monitorSessionManager {
		if manager, ok := state.sessions.(monitorSessionManager); ok {
			return manager
		}
		return nil
	}}
	watcher := &monitorTerminalWatcher{terminals: func() terminalpkg.Manager {
		if state.terminals == nil {
			return nil
		}
		return state.terminals
	}}
	// Every change to a session's monitors re-reads its catalog row, so lists and
	// the open session see the summary move. The service calls this off the
	// monitor and session-manager paths, bounded and canceled at shutdown.
	publish := func(publishCtx context.Context, sessionID string) {
		if manager, ok := state.sessions.(sessionCatalogChangePublisher); ok {
			manager.PublishSessionCatalogChanged(publishCtx, sessionID)
		}
	}
	opts := []monitorpkg.Option{
		monitorpkg.WithLogger(logger), monitorpkg.WithObserver(publish),
		monitorpkg.WithOwners(monitorSessionOwners{sessions: func() any { return state.sessions }}),
	}
	if persistence, ok := state.registry.(monitorpkg.Store); ok {
		opts = append(opts, monitorpkg.WithStore(persistence))
	}
	service, err := monitorpkg.NewService(ctx, watcher, waker, opts...)
	if err != nil {
		return nil, fmt.Errorf("daemon: create session monitor service: %w", err)
	}
	return service, nil
}

// recoverBootMonitors restores durable monitors after interrupted owners have been
// resumed, so a recovered wake finds its owner already admitted or reports why not.
func recoverBootMonitors(ctx context.Context, state *bootState) error {
	if state.sessionMonitors == nil {
		return nil
	}
	if err := state.sessionMonitors.Recover(ctx); err != nil {
		return fmt.Errorf("daemon: recover session monitors: %w", err)
	}
	return nil
}
