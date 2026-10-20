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
	case errors.Is(err, session.ErrSessionNotFound):
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
}

func (o monitorLifecycleObserver) OnSessionCreated(context.Context, *session.Session) {}

func (o monitorLifecycleObserver) OnSessionStopped(_ context.Context, stopped *session.Session) {
	if o.monitors != nil && stopped != nil {
		o.monitors.CancelSession(stopped.ID)
	}
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
	service, err := monitorpkg.NewService(ctx, watcher, waker, monitorpkg.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("daemon: create session monitor service: %w", err)
	}
	return service, nil
}
