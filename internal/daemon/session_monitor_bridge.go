package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/acp"
	monitorpkg "github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type monitorSessionManager interface {
	Status(context.Context, string) (*session.Info, error)
	PromptSynthetic(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
}

// monitorSessionWaker admits a fired monitor to its owning session's durable
// input queue: an idle session starts a turn, a busy one runs it after the turn.
type monitorSessionWaker struct {
	sessions func() monitorSessionManager
	logger   *slog.Logger
}

func (w *monitorSessionWaker) WakeSession(ctx context.Context, notification monitorpkg.Notification) error {
	sessions := w.sessions()
	if sessions == nil {
		return errors.New("daemon: session manager is unavailable")
	}
	sessionID := notification.Monitor.SessionID
	info, err := sessions.Status(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("daemon: read monitor session %q: %w", sessionID, err)
	}
	// A stopped session is the user's decision; a monitor must not revive it.
	if info == nil || info.State != session.StateActive {
		return fmt.Errorf("daemon: monitor session %q is not active", sessionID)
	}
	events, err := sessions.PromptSynthetic(ctx, sessionID, session.SyntheticPromptOpts{
		Message: notification.Message(),
		Metadata: acp.PromptSyntheticMeta{
			Kind:        acp.PromptSyntheticKindMonitor,
			Reason:      "monitor_" + notification.Outcome.Reason,
			Summary:     notification.Monitor.Description,
			WakeEventID: notification.Monitor.ID,
		},
	})
	if err != nil {
		return fmt.Errorf("daemon: deliver monitor wake to %q: %w", sessionID, err)
	}
	w.drain(ctx, sessionID, notification.Monitor.ID, events)
	return nil
}

// drain consumes the woken turn on the monitor's own goroutine, which the
// monitor service joins at shutdown; ctx ends the drain, not the turn.
func (w *monitorSessionWaker) drain(
	ctx context.Context,
	sessionID, monitorID string,
	events <-chan acp.AgentEvent,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.Type == acp.EventTypeError {
				w.logger.Warn("session.monitor_wake.agent_error", "session_id", sessionID, "monitor_id", monitorID)
			}
		}
	}
}

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
	waker := &monitorSessionWaker{
		sessions: func() monitorSessionManager {
			if state.sessions == nil {
				return nil
			}
			return state.sessions
		},
		logger: logger,
	}
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
