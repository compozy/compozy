package daemon

// Suite: terminal-to-window-manager composition bridge.
// Invariant: one agent-actor PTY opened event materializes exactly one Terminal
// window in the owning workspace model; pipe, human, system, duplicate, and
// failing events materialize none and never escape the observer.

import (
	"context"
	"errors"
	"testing"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/windowmanager"
)

const bridgeTestWorkspace = "workspace-a"

type staticWindowManagerProvider struct {
	manager *windowmanager.Manager
	err     error
}

func (p *staticWindowManagerProvider) For(string) (*windowmanager.Manager, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.manager, nil
}

func newBridgeTestManager(t *testing.T) *windowmanager.Manager {
	t.Helper()
	manager, err := windowmanager.NewService(
		windowmanager.NewMemoryRepository(),
		windowmanager.NewMemoryWorkspaceResolver(bridgeTestWorkspace),
		nil,
		windowmanager.DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := manager.Close(); closeErr != nil {
			t.Errorf("Manager.Close() error = %v", closeErr)
		}
	})
	return manager
}

func agentPTYOpenedEvent(terminalID string) terminalpkg.Event {
	return terminalpkg.Event{
		Kind:        terminalpkg.EventKindOpened,
		WorkspaceID: bridgeTestWorkspace,
		ProfileID:   "profile-a",
		TerminalID:  terminalpkg.ID(terminalID),
		Actor: terminalpkg.Actor{
			Kind: terminalpkg.ActorKindAgent, ID: "agent-a", ProfileID: "profile-a",
			SessionID: "session-a", RunID: "run-a", Generation: 3,
		},
		Detail: &terminalpkg.EventDetail{Mode: terminalpkg.ModePTY, Title: "bun run dev"},
		At:     time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC),
	}
}

func terminalWindowsIn(snapshot windowmanager.Snapshot, terminalID string) []windowmanager.Window {
	windows := make([]windowmanager.Window, 0, 1)
	for _, window := range snapshot.Windows {
		if window.App == terminalWindowApp && window.InstanceKey != nil && *window.InstanceKey == terminalID {
			windows = append(windows, window)
		}
	}
	return windows
}

func TestTerminalWindowBridgeApplies(t *testing.T) {
	t.Parallel()

	base := agentPTYOpenedEvent("term-0000000000aa")
	cases := []struct {
		name   string
		mutate func(event terminalpkg.Event) terminalpkg.Event
		want   bool
	}{
		{
			name:   "Should apply to an agent-actor pty opened event",
			mutate: func(event terminalpkg.Event) terminalpkg.Event { return event },
			want:   true,
		},
		{
			name: "Should skip pipe-mode terminals",
			mutate: func(event terminalpkg.Event) terminalpkg.Event {
				event.Detail = &terminalpkg.EventDetail{Mode: terminalpkg.ModePipe}
				return event
			},
			want: false,
		},
		{
			name: "Should skip events without mode detail",
			mutate: func(event terminalpkg.Event) terminalpkg.Event {
				event.Detail = nil
				return event
			},
			want: false,
		},
		{
			name: "Should skip human-opened terminals",
			mutate: func(event terminalpkg.Event) terminalpkg.Event {
				event.Actor.Kind = terminalpkg.ActorKindHuman
				return event
			},
			want: false,
		},
		{
			name: "Should skip system-actor terminals",
			mutate: func(event terminalpkg.Event) terminalpkg.Event {
				event.Actor.Kind = terminalpkg.ActorKindSystem
				return event
			},
			want: false,
		},
		{
			name: "Should skip non-opened lifecycle events",
			mutate: func(event terminalpkg.Event) terminalpkg.Event {
				event.Kind = terminalpkg.EventKindClosed
				return event
			},
			want: false,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := terminalWindowBridgeApplies(testCase.mutate(base)); got != testCase.want {
				t.Fatalf("terminalWindowBridgeApplies() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestOpenTerminalWindow(t *testing.T) {
	t.Parallel()

	t.Run("Should open exactly one terminal window for an agent pty terminal", func(t *testing.T) {
		t.Parallel()
		manager := newBridgeTestManager(t)
		provider := &staticWindowManagerProvider{manager: manager}
		event := agentPTYOpenedEvent("term-0000000000ab")

		if err := openTerminalWindow(testutil.Context(t), provider, event); err != nil {
			t.Fatalf("openTerminalWindow() error = %v", err)
		}

		snapshot, err := manager.Snapshot(testutil.Context(t), bridgeTestWorkspace)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		windows := terminalWindowsIn(snapshot, "term-0000000000ab")
		if len(windows) != 1 {
			t.Fatalf("terminal windows = %#v, want exactly one", windows)
		}
		window := windows[0]
		if window.Route.Pathname != "/terminal/term-0000000000ab" {
			t.Fatalf("window route = %q, want /terminal/term-0000000000ab", window.Route.Pathname)
		}
		if window.DesktopID != "desktop-default" {
			t.Fatalf("window desktop = %q, want desktop-default", window.DesktopID)
		}
	})

	t.Run("Should not open a second window for a duplicate opened event", func(t *testing.T) {
		t.Parallel()
		manager := newBridgeTestManager(t)
		provider := &staticWindowManagerProvider{manager: manager}
		event := agentPTYOpenedEvent("term-0000000000ac")

		for range 2 {
			if err := openTerminalWindow(testutil.Context(t), provider, event); err != nil {
				t.Fatalf("openTerminalWindow() error = %v", err)
			}
		}

		snapshot, err := manager.Snapshot(testutil.Context(t), bridgeTestWorkspace)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if windows := terminalWindowsIn(snapshot, "term-0000000000ac"); len(windows) != 1 {
			t.Fatalf("terminal windows after duplicate event = %#v, want exactly one", windows)
		}
	})

	t.Run("Should land the window on the desktop showing the bound session", func(t *testing.T) {
		t.Parallel()
		manager := newBridgeTestManager(t)
		provider := &staticWindowManagerProvider{manager: manager}
		seedSessionWindowOnSecondDesktop(t, manager, "session-a")

		event := agentPTYOpenedEvent("term-0000000000ad")
		event.Info = &terminalpkg.Info{
			BoundRun: &terminalpkg.RunRef{SessionID: "session-a", RunID: "run-a", Generation: 3},
		}
		if err := openTerminalWindow(testutil.Context(t), provider, event); err != nil {
			t.Fatalf("openTerminalWindow() error = %v", err)
		}

		snapshot, err := manager.Snapshot(testutil.Context(t), bridgeTestWorkspace)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		windows := terminalWindowsIn(snapshot, "term-0000000000ad")
		if len(windows) != 1 || windows[0].DesktopID != "desktop-two" {
			t.Fatalf("terminal windows = %#v, want one window on desktop-two", windows)
		}
	})

	t.Run("Should skip a deleted profile without error", func(t *testing.T) {
		t.Parallel()
		provider := &staticWindowManagerProvider{err: errWindowManagerProfileDeleted}
		if err := openTerminalWindow(
			testutil.Context(t),
			provider,
			agentPTYOpenedEvent("term-0000000000ae"),
		); err != nil {
			t.Fatalf("openTerminalWindow() error = %v, want benign skip", err)
		}
	})

	t.Run("Should surface a provider failure as an error", func(t *testing.T) {
		t.Parallel()
		providerErr := errors.New("registry unavailable")
		provider := &staticWindowManagerProvider{err: providerErr}
		err := openTerminalWindow(testutil.Context(t), provider, agentPTYOpenedEvent("term-0000000000af"))
		if !errors.Is(err, providerErr) {
			t.Fatalf("openTerminalWindow() error = %v, want wrapped provider failure", err)
		}
	})

	t.Run("Should skip an unknown workspace without error", func(t *testing.T) {
		t.Parallel()
		manager := newBridgeTestManager(t)
		provider := &staticWindowManagerProvider{manager: manager}
		event := agentPTYOpenedEvent("term-0000000000b0")
		event.WorkspaceID = "workspace-unknown"
		if err := openTerminalWindow(testutil.Context(t), provider, event); err != nil {
			t.Fatalf("openTerminalWindow() error = %v, want benign skip", err)
		}
	})
}

func seedSessionWindowOnSecondDesktop(t *testing.T, manager *windowmanager.Manager, sessionID string) {
	t.Helper()
	ctx := testutil.Context(t)
	snapshot, err := manager.Snapshot(ctx, bridgeTestWorkspace)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if _, err := manager.Execute(ctx, windowmanager.CommandRequest{
		WorkspaceID:      bridgeTestWorkspace,
		ExpectedRevision: snapshot.Revision,
		Actor:            windowmanager.Actor{Kind: "system", ID: "test.seed"},
		Payload: windowmanager.CreateDesktopCommand{
			DesktopID: "desktop-two", Name: "Two", Purpose: windowmanager.DesktopPurposeStandard,
		},
	}); err != nil {
		t.Fatalf("Execute(desktop.create) error = %v", err)
	}
	snapshot, err = manager.Snapshot(ctx, bridgeTestWorkspace)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if _, err := manager.Execute(ctx, windowmanager.CommandRequest{
		WorkspaceID:      bridgeTestWorkspace,
		ExpectedRevision: snapshot.Revision,
		Actor:            windowmanager.Actor{Kind: "system", ID: "test.seed"},
		Payload: windowmanager.OpenWindowCommand{Window: windowmanager.WindowSpec{
			App:         sessionWindowApp,
			InstanceKey: &sessionID,
			Route: windowmanager.RouteIntent{
				Pathname: "/agents/mock/sessions/" + sessionID,
				Search:   windowmanager.RouteSearch{},
			},
			DesktopID: "desktop-two",
		}},
	}); err != nil {
		t.Fatalf("Execute(window.open session) error = %v", err)
	}
}

func TestAttachTerminalWindowBridge(t *testing.T) {
	t.Parallel()

	t.Run("Should tolerate nil dependencies without registering", func(t *testing.T) {
		t.Parallel()
		attachTerminalWindowBridge(nil, nil, nil)
	})

	t.Run("Should recover from a panicking provider through the notifier", func(t *testing.T) {
		t.Parallel()
		notifier := terminalpkg.NewNotifier(discardLogger())
		notifier.Observe(func(_ context.Context, event terminalpkg.Event) {
			if !terminalWindowBridgeApplies(event) {
				return
			}
			panic("provider blew up")
		})
		notifier.Notify(testutil.Context(t), agentPTYOpenedEvent("term-0000000000b1"))
	})
}
