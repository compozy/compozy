package daemon

// Invariant: agent terminal lifecycle cannot litter durable desktops; this bridge suite owns event composition.
import (
	"testing"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
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

func TestAttachTerminalWindowBridge(t *testing.T) {
	t.Parallel()
	for _, kind := range []terminalpkg.EventKind{terminalpkg.EventKindClosed, terminalpkg.EventKindDeleted} {
		t.Run("Should retire persisted windows on "+string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			manager := newBridgeTestManager(t)
			notifier := terminalpkg.NewNotifier(discardLogger())
			terminals, err := terminalpkg.NewManager(
				terminalpkg.WithNotifier(notifier),
				terminalpkg.WithJournal(nativeTerminalJournalStub{}),
			)
			if err != nil {
				t.Fatal(err)
			}
			attachTerminalWindowBridge(terminals, &staticWindowManagerProvider{manager: manager}, discardLogger())
			before, err := manager.Snapshot(ctx, bridgeTestWorkspace)
			if err != nil {
				t.Fatal(err)
			}
			for range 5 {
				notifier.Notify(
					ctx,
					terminalpkg.Event{
						Kind:        terminalpkg.EventKindOpened,
						WorkspaceID: bridgeTestWorkspace,
						ProfileID:   "profile-a",
						Actor:       terminalpkg.Actor{Kind: terminalpkg.ActorKindAgent},
						Detail:      &terminalpkg.EventDetail{Mode: terminalpkg.ModePTY},
					},
				)
			}
			after, err := manager.Snapshot(ctx, bridgeTestWorkspace)
			if err != nil {
				t.Fatal(err)
			}
			if after.Revision != before.Revision || len(after.Windows) != 0 {
				t.Fatal("agent opens changed desktop")
			}
			id := "term-0000000000ab"
			_, err = manager.Execute(
				ctx,
				windowmanager.CommandRequest{
					WorkspaceID:      bridgeTestWorkspace,
					ExpectedRevision: after.Revision,
					Actor:            windowmanager.Actor{Kind: "system", ID: "test"},
					Payload: windowmanager.OpenWindowCommand{
						Window: windowmanager.WindowSpec{
							App:         "terminal",
							InstanceKey: &id,
							Route: windowmanager.RouteIntent{
								Pathname: "/terminal/" + id,
								Search:   windowmanager.RouteSearch{},
							},
						},
					},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			notifier.Notify(
				ctx,
				terminalpkg.Event{
					Kind:        kind,
					WorkspaceID: bridgeTestWorkspace,
					ProfileID:   "profile-a",
					TerminalID:  terminalpkg.ID(id),
					Actor:       terminalpkg.Actor{Kind: terminalpkg.ActorKindAgent},
				},
			)
			after, err = manager.Snapshot(ctx, bridgeTestWorkspace)
			if err != nil {
				t.Fatal(err)
			}
			if len(after.Windows) != 0 {
				t.Fatalf("stale windows: %+v", after.Windows)
			}
			for _, entries := range [][]windowmanager.HistoryEntry{after.History.Undo, after.History.Redo} {
				for _, entry := range entries {
					if len(entry.Before.Windows) != 0 || len(entry.After.Windows) != 0 {
						t.Fatal("history can resurrect terminal")
					}
				}
			}
		})
	}
}
