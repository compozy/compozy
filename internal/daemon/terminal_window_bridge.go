package daemon

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/windowmanager"
)

const (
	terminalWindowApp            = "terminal"
	sessionWindowApp             = "session"
	terminalWindowOpenOrigin     = "terminal.open"
	terminalWindowOpenTimeout    = 5 * time.Second
	terminalWindowOpenMaxRetries = 3
)

// attachTerminalWindowBridge materializes a desktop Terminal window for every
// agent-opened interactive terminal. One-shot per opened event: a window the
// human later closes is never reopened for the same terminal.
func attachTerminalWindowBridge(
	terminals *terminalpkg.Service,
	windows windowManagerProvider,
	logger *slog.Logger,
) {
	if terminals == nil || windows == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	terminals.Observe(func(ctx context.Context, event terminalpkg.Event) {
		if !terminalWindowBridgeApplies(event) {
			return
		}
		openCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalWindowOpenTimeout)
		defer cancel()
		if err := openTerminalWindow(openCtx, windows, event); err != nil {
			logger.Warn(
				"daemon: open terminal window",
				"terminal_id", event.TerminalID,
				"workspace_id", event.WorkspaceID,
				"actor_kind", event.Actor.Kind,
				"actor_id", event.Actor.ID,
				"error", err,
			)
		}
	})
}

func terminalWindowBridgeApplies(event terminalpkg.Event) bool {
	return event.Kind == terminalpkg.EventKindOpened &&
		event.Actor.Kind == terminalpkg.ActorKindAgent &&
		event.DetailValue().Mode == terminalpkg.ModePTY
}

func openTerminalWindow(
	ctx context.Context,
	windows windowManagerProvider,
	event terminalpkg.Event,
) error {
	manager, err := windows.For(event.ProfileID)
	if err != nil {
		if errors.Is(err, errWindowManagerProfileDeleted) {
			return nil
		}
		return fmt.Errorf("resolve window manager for profile %q: %w", event.ProfileID, err)
	}
	workspaceID := windowmanager.WorkspaceID(event.WorkspaceID)
	terminalID := string(event.TerminalID)
	for range terminalWindowOpenMaxRetries {
		if err := ctx.Err(); err != nil {
			return err
		}
		snapshot, err := manager.Snapshot(ctx, workspaceID)
		if err != nil {
			if errors.Is(err, windowmanager.ErrWorkspaceNotFound) {
				return nil
			}
			return fmt.Errorf("load workspace %q snapshot: %w", event.WorkspaceID, err)
		}
		if terminalWindowExists(snapshot, terminalID) {
			return nil
		}
		sessionWindow := boundSessionWindow(snapshot, event)
		_, err = manager.Execute(ctx, windowmanager.CommandRequest{
			WorkspaceID:      workspaceID,
			ExpectedRevision: snapshot.Revision,
			Actor: windowmanager.Actor{
				Kind: string(event.Actor.Kind),
				ID:   cmp.Or(event.Actor.ID, event.Actor.SessionID),
			},
			Origin: terminalWindowOpenOrigin,
			Payload: windowmanager.OpenWindowCommand{Window: windowmanager.WindowSpec{
				App:         terminalWindowApp,
				InstanceKey: &terminalID,
				Route: windowmanager.RouteIntent{
					Pathname: "/terminal/" + url.PathEscape(terminalID),
					Search:   windowmanager.RouteSearch{},
				},
				DesktopID:      sessionWindowDesktop(sessionWindow),
				BesideWindowID: sessionWindowID(sessionWindow),
			}},
		})
		if err == nil {
			return nil
		}
		if !errors.Is(err, windowmanager.ErrRevisionConflict) {
			return fmt.Errorf("open terminal window for %q: %w", terminalID, err)
		}
	}
	return fmt.Errorf("open terminal window for %q: %w", terminalID, windowmanager.ErrRevisionConflict)
}

func terminalWindowExists(snapshot windowmanager.Snapshot, terminalID string) bool {
	for _, window := range snapshot.Windows {
		if window.App == terminalWindowApp && window.InstanceKey != nil && *window.InstanceKey == terminalID {
			return true
		}
	}
	return false
}

// boundSessionWindow is the window already showing the terminal's bound
// session. The terminal lands on its desktop and, under the tab or
// beside_focus policy, tiles right after it — the agent's shell belongs next
// to the agent, and the open carries no client, so nothing takes focus.
// Without one the reducer default applies.
func boundSessionWindow(
	snapshot windowmanager.Snapshot,
	event terminalpkg.Event,
) *windowmanager.Window {
	if event.Info == nil || event.Info.BoundRun == nil {
		return nil
	}
	sessionID := event.Info.BoundRun.SessionID
	for _, window := range snapshot.Windows {
		if window.App == sessionWindowApp && window.InstanceKey != nil && *window.InstanceKey == sessionID {
			return &window
		}
	}
	return nil
}

func sessionWindowDesktop(window *windowmanager.Window) windowmanager.DesktopID {
	if window == nil {
		return ""
	}
	return window.DesktopID
}

func sessionWindowID(window *windowmanager.Window) *windowmanager.WindowID {
	if window == nil {
		return nil
	}
	return new(window.ID)
}
